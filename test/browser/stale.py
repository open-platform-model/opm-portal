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
            return 0 if steps(r) and r.ok else 1
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
    r.ctl("fetch", mode="ok")
    r.ctl("refuse", on=0)
    r.ctl("release")
    page.clock.run_for(500)
    s = page.evaluate(STATE)
    if not r.check(s["mark"] == "Reconnecting" and not s["hidden"],
                   f"the pending fetch answers while the stream is down: the mark reads {s['mark']!r}, the banner stays"):
        return False
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


if __name__ == "__main__":
    sys.exit(main())
