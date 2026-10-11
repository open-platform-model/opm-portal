"""stale.py: the banner of a lost stream, the resync and the sticky header, in a real browser.

Usage: python3 stale.py <browser> <site URL> <server now, RFC 3339>

The site (serveStaleSite in cmd/opm-portal/browser_test.go) is the UI over the F1 capture with a stream
the script drives (frames, a drop, refused connections) and a page fetch it controls (ok, hold, abort,
500, 401), over POST /ctl/*. The page clock is Playwright's, set 601 s ahead of the server's and paused
before the load, so the page's skew is exact and run_for moves time by a known amount.
Checks, each printing one line, exit 1 on the first failure:

1. A drop 20 s after a heartbeat: the mark reads "Reconnecting"; the banner reads "Showing data read
   20s ago. Reconnecting." with the heartbeat's instant in datetime and in UTC in title; no role, no
   aria-live; of the page's live regions only the mark changed; 75 s more and it reads "1m ago".
2. The stream is back and the fetch is held: the mark still reads "Reconnecting", the banner stays,
   and a heartbeat that arrives meanwhile does not move its datetime, not even when a second drop writes
   the banner afresh. One fetch is made. The fetch that was pending when the stream dropped again does
   not end the outage; the next one, made after the stream is back, does: the followed regions are
   replaced, the mark reads "Live" and the banner is hidden.
3. A fetch that fails (reset, then 500) keeps the banner with "Offline" and the mark; each next
   heartbeat tries again; one that returns 200 hides the banner. Three fetches in all.
4. A fetch answered 401: the mark reads "Signed out", the banner reads "Signed out. Showing data read"
   and the time, and a heartbeat afterwards makes no new fetch.
5. A heartbeat while healthy is the time of the data: after a drop 5 s later the banner shows it.
6. html's scroll-padding-top is the header's height plus 8 px, with the banner and without; the sticky
   details panel's offset is the same height plus 8 px (wide viewport).
7. A reader scrolled 3000 px down: the drop leaves the banner and its Reload link in the viewport, at
   the top. Shift+Tab to a link above the viewport and a fragment link leave the target below the
   header, with the banner and without it.

8. The details panel follows its page (the Graph tab, the Deployment node selected): the object
   changes (the pages render over the image-break capture, a topic upsert arrives), and the panel and
   the graph node agree on the new health, the node stays selected and the address keeps `node`.
9. Focus on the panel's YAML link, and on its Clear selection pill, is on the same control of the new
   panel after a replacement.
10. A YAML view does not follow: after a change its text and element stay, a note appears with a
    `time` (datetime, UTC title, data-relative) and the words "The object may have changed since";
    "Reload this view" shows the object's current YAML and no note.
11. A request into the panel that is open during a change keeps its response; the refresh runs once
    the request has ended and leaves the selected node's panel.
12. The selected node's object is gone: the panel returns to the resting node and `node` leaves the
    address.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import re
import sys
from datetime import datetime, timedelta, timezone
from urllib.parse import quote

from playwright.sync_api import expect, sync_playwright

PATH = "/instances/default/podinfo"
OFF_MS = 601 * 1000

STATE = """() => {
    const b = document.getElementById("region-banner");
    const t = b.querySelector("time");
    const h = document.getElementById("masthead").getBoundingClientRect();
    return {
        mark: document.querySelector("#live .live-text").textContent,
        hidden: b.hidden,
        text: b.querySelector(".rb-text").textContent.replace(/\\s+/g, " ").trim(),
        datetime: t.getAttribute("datetime"), title: t.getAttribute("title"),
        role: b.getAttribute("role"), live: b.getAttribute("aria-live"),
        pad: parseFloat(getComputedStyle(document.documentElement).scrollPaddingTop), headerBottom: h.bottom,
        headerHeight: h.height,
    };
}"""

WATCH = """() => {
    window.__changed = new Set();
    const live = document.querySelectorAll('[role="status"], [role="alert"], [aria-live]:not([aria-live="off"])');
    for (const el of live) {
        new MutationObserver(() => window.__changed.add(el.id || el.tagName)).observe(el, {subtree: true, childList: true, characterData: true});
    }
}"""

# Puts a spacer between two links in main, so that the link before it can be above the viewport.
SPACER = """() => {
    const main = document.querySelector("main");
    const wrap = document.createElement("div");
    wrap.id = "probe-wrap";
    const a = document.createElement("a");
    a.id = "probe-a"; a.href = "#probe-h"; a.textContent = "probe a";
    const h = document.createElement("h2");
    h.id = "probe-h"; h.tabIndex = -1; h.textContent = "probe heading";
    const gap = document.createElement("div");
    gap.style.height = "5000px";          // the CSSOM, which the page policy allows
    const b = document.createElement("a");
    b.id = "probe-b"; b.href = "#probe-h"; b.textContent = "probe b";
    const tail = document.createElement("div");
    tail.style.height = "2000px";
    wrap.append(a, h, gap, b, tail);
    main.append(wrap);
}"""


def shoot(page):
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out or page is None:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


class Run:
    def __init__(self, p, engine, base, served_iso):
        self.p, self.engine, self.base = p, engine, base
        self.served = datetime.fromisoformat(served_iso.replace("Z", "+00:00"))
        self.served_ms = int(self.served.timestamp() * 1000)
        self.start = self.served + timedelta(milliseconds=OFF_MS)
        self.load_ms = int(self.start.timestamp() * 1000) + 1000
        self.browser = getattr(p, engine).launch()
        self.page = None
        self.ok = True

    def check(self, good, line):
        print(f"{self.engine}: {'' if good else 'FAIL '}{line}")
        if not good:
            self.ok = False
            shoot(self.page)
        return good

    def ctl(self, cmd, **q):
        qs = "&".join(f"{k}={quote(str(v))}" for k, v in q.items())
        r = self.page.request.post(f"{self.base}/ctl/{cmd}" + (f"?{qs}" if qs else ""))
        assert r.ok, (cmd, r.status)

    def state(self):
        streams, fetches = self.page.request.get(self.base + "/ctl/state").text().split()
        return int(streams), int(fetches)

    def beat(self):
        """A heartbeat from the server; returns the page's clock at its arrival (the clock is paused)."""
        now = self.page.evaluate("Date.now()")
        self.ctl("send", frame="event: heartbeat\ndata: {}")
        self.page.wait_for_timeout(300)
        return now

    def until(self, fn, what, timeout=10.0, step_ms=None):
        """Waits for fn(), advancing the page clock by step_ms each round when given."""
        import time
        end = time.time() + timeout
        while time.time() < end:
            if fn():
                return True
            if step_ms:
                self.page.clock.run_for(step_ms)
            self.page.wait_for_timeout(60)
        return self.check(False, f"timed out waiting for {what}: {self.page.evaluate(STATE)}")

    def iso(self, browser_ms):
        """The datetime the page writes for an instant of its own clock."""
        sec = (browser_ms + (self.served_ms - self.load_ms)) // 1000
        return datetime.fromtimestamp(sec, tz=timezone.utc)

    def new_page(self, path=PATH, width=1280, height=800):
        if self.page is not None:
            self.page.context.close()
        ctx = self.browser.new_context(viewport={"width": width, "height": height})
        self.page = ctx.new_page()
        # Every drop below is a clean end of the connection; the browser retries after 300 ms.
        self.ctl("fetch", mode="ok")
        self.ctl("refuse", on=0)
        self.ctl("render", mode="ok")
        self.page.clock.install(time=self.start)
        self.page.clock.pause_at(self.start + timedelta(seconds=1))
        self.page.goto(self.base + path)
        self.page.wait_for_selector("#detail")
        self.until(lambda: self.page.evaluate(STATE)["mark"] == "Live", "the mark to read Live")
        self.page.clock.run_for(100)
        self.page.evaluate(WATCH)
        return self.page

    def lose(self, refuse):
        self.ctl("refuse", on=1 if refuse else 0)
        self.ctl("drop")
        return self.until(lambda: not self.page.evaluate(STATE)["hidden"], "the banner to show")

    def close(self):
        self.browser.close()


def banner_words(s):
    return s["text"]


def main() -> int:
    engine, base, served_iso = sys.argv[1], sys.argv[2], sys.argv[3]
    with sync_playwright() as p:
        r = Run(p, engine, base, served_iso)
        try:
            return 0 if steps(r) and panel_steps(r) and r.ok else 1
        finally:
            r.close()


def steps(r: Run) -> bool:
    # 1 and 2. The drop, the banner, the held resync.
    page = r.new_page()
    page.clock.run_for(1000)
    beat_ms = r.beat()
    page.clock.run_for(20000)
    if not r.lose(refuse=True):
        return False
    s = page.evaluate(STATE)
    want_dt = r.iso(beat_ms)
    if not r.check(s["mark"] == "Reconnecting" and s["text"] == "Showing data read 20s ago. Reconnecting.",
                   f"after a drop 20 s after a heartbeat the mark reads {s['mark']!r} and the banner {s['text']!r}"):
        return False
    if not r.check(s["datetime"] == want_dt.strftime("%Y-%m-%dT%H:%M:%SZ") and s["title"] == want_dt.strftime("%Y-%m-%d %H:%M:%S UTC"),
                   f"the time carries {s['datetime']!r} and {s['title']!r} (want the heartbeat's {want_dt.strftime('%Y-%m-%dT%H:%M:%SZ')})"):
        return False
    if not r.check(s["role"] is None and s["live"] is None, "the banner has no role and no aria-live"):
        return False
    changed = page.evaluate("() => Array.from(window.__changed)")
    if not r.check(changed == ["live"], f"of the live regions only the mark changed ({changed})"):
        return False
    page.clock.run_for(75000)
    s = page.evaluate(STATE)
    if not r.check(s["text"] == "Showing data read 1m ago. Reconnecting." and s["datetime"] == want_dt.strftime("%Y-%m-%dT%H:%M:%SZ"),
                   f"75 s later the banner reads {s['text']!r} with the same datetime"):
        return False

    # The stream returns; the fetch is held.
    r.ctl("fetch", mode="hold")
    r.ctl("refuse", on=0)
    # The page's own reconnect timer backs off (up to 64 s of page time): move the clock until the
    # stream is open again and the resync fetch is made and held.
    if not r.until(lambda: r.state()[1] == 1, "the stream to open again and the fetch to start", timeout=90, step_ms=2000):
        return False
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Reconnecting" and not s["hidden"],
                   f"the stream is open and the fetch pending: the mark reads {s['mark']!r}, banner hidden {s['hidden']}"):
        return False
    held_dt = s["datetime"]
    r.beat()
    page.clock.run_for(200)
    s = page.evaluate(STATE)
    if not r.check(s["datetime"] == held_dt and s["mark"] == "Reconnecting" and r.state()[1] == 1,
                   f"a heartbeat meanwhile does not move the time ({s['datetime']} = {held_dt}), nor start a second fetch"):
        return False
    # The connection drops again while the fetch is pending: the banner is written afresh, and its time
    # is still the one from before the first outage, not the heartbeat that arrived in between.
    r.ctl("refuse", on=1)
    r.ctl("drop")
    page.wait_for_timeout(600)
    s = page.evaluate(STATE)
    if not r.check(s["datetime"] == held_dt and s["text"].endswith("Reconnecting.") and not s["hidden"],
                   f"a second drop with the fetch pending writes the banner afresh with the same time ({s['datetime']} = {held_dt})"):
        return False
    # The answer of the fetch that was pending before the stream dropped again does not end the outage.
    # The refusal stays on until the read below, so no reconnect, the page's own or the extension's
    # first retry (500 ms), can open the stream and start a resync before the state is read.
    r.ctl("fetch", mode="ok")
    r.ctl("release")
    page.wait_for_timeout(300)   # real time: the answer reaches the page
    page.clock.run_for(500)
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Reconnecting" and not s["hidden"],
                   f"the pending fetch answers while the stream is down: the mark reads {s['mark']!r}, the banner stays"):
        return False
    r.ctl("refuse", on=0)
    if not r.until(lambda: page.evaluate(STATE)["hidden"], "the banner to go once the stream is back and a fetch is answered", timeout=90, step_ms=2000):
        return False
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Live" and r.state()[1] == 2, f"the fetch answered: the mark reads {s['mark']!r}, the banner is hidden, two fetches in all"):
        return False
    pad_ok = abs(s["pad"] - (s["headerHeight"] + 8)) < 1
    if not r.check(pad_ok, f"without the banner scroll-padding-top is {s['pad']:.1f}, the header {s['headerHeight']:.1f} + 8"):
        return False

    # 3. A fetch that fails is tried again on the next heartbeat.
    page = r.new_page()
    page.clock.run_for(1000)
    r.ctl("fetch", mode="abort")
    streams0, fetches0 = r.state()
    if not r.lose(refuse=False):
        return False
    page.wait_for_timeout(600)
    page.clock.run_for(500)
    if not r.until(lambda: page.evaluate(STATE)["mark"] == "Offline", "the mark to read Offline", step_ms=100):
        return False
    s = page.evaluate(STATE)
    # A browser may repeat a GET whose connection was reset; the page counts one failed resync.
    c1 = r.state()[1]
    if not r.check(s["text"].endswith("Offline.") and not s["hidden"] and c1 >= fetches0 + 1,
                   f"a fetch that fails keeps the banner: {s['text']!r} ({c1 - fetches0} requests)"):
        return False
    r.ctl("fetch", mode="500")
    r.beat()
    page.clock.run_for(500)
    if not r.until(lambda: r.state()[1] == c1 + 1, "the next heartbeat to try again", step_ms=100):
        return False
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Offline" and not s["hidden"], f"a 500 keeps the banner and the mark: {s['mark']!r}"):
        return False
    r.ctl("fetch", mode="ok")
    r.beat()
    page.clock.run_for(500)
    if not r.until(lambda: page.evaluate(STATE)["hidden"], "the banner to go after the third fetch", step_ms=100):
        return False
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Live" and r.state()[1] == c1 + 2, f"the next fetch returned 200: mark {s['mark']!r}, banner hidden"):
        return False
    # The next drop reads the instant of the answer, not the one before the outage.
    page.clock.run_for(5000)
    if not r.lose(refuse=True):
        return False
    s = page.evaluate(STATE)
    if not r.check(s["text"] == "Showing data read 5s ago. Reconnecting.", f"after the resync the time is the answer's: {s['text']!r}"):
        return False

    # 4. Signed out.
    page = r.new_page()
    page.clock.run_for(1000)
    r.ctl("fetch", mode="401")
    _, fetches0 = r.state()
    if not r.lose(refuse=False):
        return False
    page.wait_for_timeout(600)
    page.clock.run_for(500)
    if not r.until(lambda: page.evaluate(STATE)["mark"] == "Signed out", "the mark to read Signed out", step_ms=100):
        return False
    s = page.evaluate(STATE)
    if not r.check(re.fullmatch(r"Signed out\. Showing data read \d+s ago\.", s["text"]) is not None and not s["hidden"],
                   f"a 401 keeps the banner: {s['text']!r}"):
        return False
    r.beat()
    page.clock.run_for(1000)
    s = page.evaluate(STATE)
    if not r.check(r.state()[1] == fetches0 + 1 and s["mark"] == "Signed out",
                   f"a heartbeat after the 401 makes no new fetch ({r.state()[1] - fetches0} in all)"):
        return False

    # 6 and 7. The header, the scrolled reader and the focus.
    for with_banner in (False, True):
        page = r.new_page()
        page.evaluate(SPACER)
        if with_banner and not r.lose(refuse=True):
            return False
        s = page.evaluate(STATE)
        label = "with the banner" if with_banner else "without the banner"
        if not r.check(abs(s["pad"] - (s["headerHeight"] + 8)) < 1 and (s["hidden"] != with_banner),
                       f"{label}: scroll-padding-top {s['pad']:.1f} = header {s['headerHeight']:.1f} + 8"):
            return False
        sticky = page.evaluate("() => parseFloat(getComputedStyle(document.getElementById('detail')).top)")
        if not r.check(abs(sticky - (s["headerHeight"] + 8)) < 1,
                       f"{label}: the sticky details panel sits {sticky:.1f} px from the top, below the header ({s['headerHeight']:.1f} + 8)"):
            return False
        # Shift+Tab to a link above the viewport.
        page.evaluate("() => { document.getElementById('probe-b').focus(); window.scrollTo(0, 3000); }")
        page.wait_for_timeout(100)
        if with_banner:
            box = page.evaluate("""() => {
                const b = document.getElementById('region-banner').getBoundingClientRect();
                const a = document.querySelector('#region-banner .rb-reload').getBoundingClientRect();
                return {top: b.top, bottom: b.bottom, linkTop: a.top, linkBottom: a.bottom, vh: innerHeight, y: scrollY};
            }""")
            if not r.check(box["y"] >= 2900 and box["top"] >= 0 and box["bottom"] <= box["vh"] and box["linkTop"] >= 0 and box["linkBottom"] <= box["vh"],
                           f"scrolled {box['y']:.0f} px down the banner ({box['top']:.0f}..{box['bottom']:.0f}) and its link are in the viewport"):
                return False
        page.keyboard.press("Shift+Tab")
        page.wait_for_timeout(200)
        pos = page.evaluate("() => ({id: document.activeElement.id, top: document.activeElement.getBoundingClientRect().top, hb: document.getElementById('masthead').getBoundingClientRect().bottom})")
        if not r.check(pos["id"] != "probe-b" and pos["top"] >= pos["hb"] - 0.5,
                       f"{label}: Shift+Tab to {pos['id']!r} puts it at {pos['top']:.1f}, below the header bottom {pos['hb']:.1f}"):
            return False
        # A fragment link.
        page.evaluate("() => window.scrollTo(0, 3000)")
        page.locator("#probe-a").evaluate("el => el.scrollIntoView({block: 'center'})")
        page.evaluate("() => { document.getElementById('probe-a').click(); }")
        page.wait_for_timeout(200)
        pos = page.evaluate("() => ({top: document.getElementById('probe-h').getBoundingClientRect().top, hb: document.getElementById('masthead').getBoundingClientRect().bottom})")
        if not r.check(pos["top"] >= pos["hb"] - 0.5, f"{label}: a fragment link puts its heading at {pos['top']:.1f}, below the header bottom {pos['hb']:.1f}"):
            return False
    return True

DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"
POD = "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"
SERVICE = "obj:_/Service/default/podinfo-podinfo"
GRAPH = PATH + "?tab=graph&node="
UPSERT = 'event: upsert\ndata: {"topic":"instance:default/podinfo"}'

PANEL = """() => {
    const d = document.getElementById("detail");
    const hp = d.querySelector(".axes .health");
    const node = document.querySelector('.node[data-node="%s"]');
    const tok = (el) => el ? ((el.getAttribute("class") || "").match(/health-\\w+/) || [""])[0] : "";
    const sel = document.querySelector('.node[aria-current="true"]');
    const v = d.querySelector("[data-view]");
    return {
        follow: d.getAttribute("data-follow"), topics: d.getAttribute("data-topics"),
        panelHealth: tok(hp), nodeHealth: tok(node), search: location.search,
        selected: sel ? sel.getAttribute("data-node") : "",
        heading: (d.querySelector("h2") || {textContent: ""}).textContent.trim(),
        kicker: (d.querySelector(".kicker") || {textContent: ""}).textContent.trim(),
        view: v ? v.getAttribute("data-view") : "",
    };
}""" % DEPLOY

NOTE = """() => {
    const n = document.querySelector("#detail [data-view] > .stale-note");
    if (!n) return null;
    const t = n.querySelector("time");
    return {hidden: n.hidden, text: n.textContent.replace(/\\s+/g, " ").trim(),
            datetime: t.getAttribute("datetime"), title: t.getAttribute("title"), relative: t.hasAttribute("data-relative"),
            reload: n.querySelector("a").textContent};
}"""


class Held:
    """Requests the script holds until it releases them."""

    def __init__(self, page, match):
        self.routes = []
        page.route(match, lambda route: self.routes.append(route))

    def release(self):
        routes, self.routes = self.routes, []
        for r in routes:
            r.continue_()


def settle(page, ms=900):
    """Moves the paused page clock so the refresh timer, htmx's settle and the swaps run."""
    for _ in range(ms // 100):
        page.clock.run_for(100)
        page.wait_for_timeout(40)


def mark(page):
    page.evaluate("() => { window.__same = document.getElementById('detail'); }")


def replaced(page):
    return page.evaluate("() => document.getElementById('detail') !== window.__same")


def panel_steps(r: Run) -> bool:
    page = r.new_page(GRAPH + quote(DEPLOY))
    page.wait_for_selector("#detail .axes .health")
    s0 = page.evaluate(PANEL)
    if not r.check(s0["follow"] == s0["topics"] and (s0["topics"] or "").startswith("instance:") and s0["selected"] == DEPLOY and "node=" in s0["search"],
                   f"8: a node panel follows its topic ({s0['follow']!r}), the node is selected"):
        return False

    # 8. The object changes: graph and panel agree on the new health.
    mark(page)
    r.ctl("render", mode="broken")
    r.ctl("send", frame=UPSERT)
    if not r.until(lambda: replaced(page), "the panel to be replaced", step_ms=100):
        return False
    settle(page)
    s1 = page.evaluate(PANEL)
    if not r.check(s1["panelHealth"] == s1["nodeHealth"] and s1["panelHealth"] != s0["panelHealth"] and s1["selected"] == DEPLOY and "node=" in s1["search"],
                   f"8: after the change the panel ({s1['panelHealth']}) and the node ({s1['nodeHealth']}) agree, were {s0['panelHealth']}; the node stays selected, the address keeps node"):
        return False

    # 9. Focus on a panel link, and on the Clear selection pill, survives a replacement.
    for key in ("panel-yaml", "panel-clear"):
        page.evaluate("(k) => document.querySelector('#detail [data-focus-key=\"' + k + '\"]').focus()", key)
        mark(page)
        r.ctl("render", mode="ok" if key == "panel-yaml" else "broken")
        r.ctl("send", frame=UPSERT)
        if not r.until(lambda: replaced(page), f"the panel to be replaced under focus on {key}", step_ms=100):
            return False
        settle(page, 600)
        got = page.evaluate("() => { const a = document.activeElement; return {key: a.getAttribute('data-focus-key'), inside: !!a.closest('#detail')}; }")
        if not r.check(got["key"] == key and got["inside"], f"9: focus on {key} is on the same control of the new panel ({got})"):
            return False

    # 10. A YAML view does not follow; it says when it was read.
    r.ctl("render", mode="ok")
    page.locator('#detail [data-focus-key="panel-yaml"]').click()
    if not r.until(lambda: page.evaluate(PANEL)["view"] == "yaml", "the YAML view to load", step_ms=100):
        return False
    settle(page, 300)
    s2 = page.evaluate(PANEL)
    n0 = page.evaluate(NOTE)
    if not r.check(s2["follow"] is None and s2["topics"] and n0 and n0["hidden"],
                   f"10: a YAML view drops data-follow ({s2['follow']!r}) and holds a hidden note"):
        return False
    page.evaluate("() => { window.__pre = document.querySelector('#detail pre'); window.__text = window.__pre.textContent; }")
    r.ctl("render", mode="broken")
    r.ctl("send", frame=UPSERT)
    if not r.until(lambda: (page.evaluate(NOTE) or {}).get("hidden") is False, "the note to show", step_ms=100):
        return False
    n1 = page.evaluate(NOTE)
    same = page.evaluate("() => document.querySelector('#detail pre') === window.__pre && window.__pre.textContent === window.__text")
    if not r.check(same and "The object may have changed since" in n1["text"] and re.fullmatch(r"\d{4}-\d\d-\d\d \d\d:\d\d:\d\d UTC", n1["title"] or "")
                   and n1["relative"] and n1["datetime"].endswith("Z") and n1["reload"] == "Reload this view",
                   f"10: after a change the YAML stays and the note reads {n1['text']!r} ({n1['datetime']}, {n1['title']})"):
        return False
    page.locator("#detail .stale-note a").click()
    if not r.until(lambda: page.evaluate("() => document.querySelector('#detail pre') !== window.__pre"), "the view to reload", step_ms=100):
        return False
    settle(page, 300)
    n2 = page.evaluate(NOTE)
    s3 = page.evaluate(PANEL)
    if not r.check(n2 is not None and n2["hidden"] and s3["view"] == "yaml" and s3["follow"] is None,
                   "10: Reload this view shows the current YAML and a note that is hidden again"):
        return False

    # 11. A request into the panel that is open during a change keeps its response.
    page = r.new_page(GRAPH + quote(DEPLOY))
    page.wait_for_selector("#detail .axes .health")
    mark(page)
    held = Held(page, "**/instances/default/podinfo/node?*")
    page.evaluate("(n) => document.querySelector('.node[data-node=\"' + n + '\"]').dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}))", SERVICE)
    page.wait_for_timeout(300)
    if not r.check(len(held.routes) == 1, f"11: the panel request of the Service node is held ({len(held.routes)})"):
        return False
    r.ctl("render", mode="broken")
    r.ctl("send", frame=UPSERT)
    settle(page, 1200)
    if not r.check(not replaced(page), "11: the refresh left the panel alone while its request was open"):
        return False
    held.release()
    page.unroute_all()
    settle(page, 2000)
    s4 = page.evaluate(PANEL)
    if not r.check(s4["selected"] == SERVICE and "Service" in page.inner_text("#detail") and "Deployment" not in page.inner_text("#detail").split("Edges")[0][:120],
                   f"11: the panel the request returned is the one shown ({s4['kicker']!r}, {s4['search']})"):
        return False

    # 12. The selected node is gone.
    page = r.new_page(GRAPH + quote(POD))
    page.wait_for_selector("#detail .axes .health")
    r.ctl("render", mode="broken")
    r.ctl("send", frame=UPSERT)
    if not r.until(lambda: "node=" not in page.evaluate("location.search"), "the address to stop naming the node", step_ms=100):
        return False
    settle(page, 1500)
    s5 = page.evaluate(PANEL)
    if not r.check(s5["selected"] == "" and "no such node" not in page.inner_text("#detail") and s5["kicker"] != "",
                   f"12: the selection cleared and the panel is the resting node's ({s5['kicker']!r}, {s5['heading']!r})"):
        return False
    return True


if __name__ == "__main__":
    sys.exit(main())
