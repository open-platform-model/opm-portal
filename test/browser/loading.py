"""loading.py: a slow request shows as designed, in a real browser (add-shell-states-and-cluster-name).

Usage: python3 loading.py <browser> <site URL>

The site is the portal's UI over the F1 capture. Requests are held and released by the script, so
how long they run is the script's choice, and the page's clock is Playwright's, paused and moved
by run_for, so a second is exactly a second. Checks, each printing one line, exit 1 on the first
failure:

1. A node panel request held for 900 ms shows nothing; at 1100 ms the header bar is drawn
   (body.is-loading, a 2 px ::after), #detail is aria-busy and its first line reads "Loading...";
   when the panel arrives all three are gone.
2. A request that ends in 300 ms never shows anything, not even for one frame (an observer), and
   its timer does not carry over to a request that begins at 800 ms.
3. Two requests in flight (a panel and a boosted page): the bar stays until both have ended.
4. A request answered with 500, one that is aborted, and two panel requests released together
   each end the bar and the busy state.
5. The stream's background refresh (fetch, not htmx) shows neither the bar nor the busy state.
6. The bar is static, not animated, under prefers-reduced-motion: reduce, and animated otherwise.
7. At 360 px, with the bar and the line shown, the page's scroll width is at most 360.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys
import time
from datetime import datetime, timedelta, timezone

from playwright.sync_api import expect, sync_playwright

PODINFO = "/instances/default/podinfo"
DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"
POD = "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"
T0 = datetime(2026, 10, 4, 18, 10, 0, tzinfo=timezone.utc)  # the page's start: any fixed time

STATE = """() => ({
    loading: document.body.classList.contains("is-loading"),
    busy: document.getElementById("detail").getAttribute("aria-busy"),
    line: (document.querySelector("#detail > .detail-loading") || {}).textContent || null,
    firstChild: (document.getElementById("detail").firstElementChild || {}).className || null,
})"""
BAR = """() => {
    const s = getComputedStyle(document.getElementById("masthead"), "::after");
    return { height: s.height, name: s.animationName, content: s.content, events: s.pointerEvents };
}"""


def shoot(page):
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


class Held:
    """Routes the script holds until it releases them: what a slow cluster does."""

    def __init__(self, page, match):
        self.routes = []
        page.route(match, lambda route: self.routes.append(route))

    def wait(self, n, page, timeout=10.0):
        end = time.time() + timeout
        while len(self.routes) < n and time.time() < end:
            page.wait_for_timeout(50)
        return len(self.routes) >= n

    def release(self):
        routes, self.routes = self.routes, []
        for r in routes:
            r.continue_()


def new_page(browser, base, path, reduced=None, width=1440):
    """A page whose clock is Playwright's, paused at T0 + 1 s before the load."""
    ctx = browser.new_context(viewport={"width": width, "height": 900 if width > 400 else 700}, reduced_motion=reduced)
    page = ctx.new_page()
    page.clock.install(time=T0)
    page.clock.pause_at(T0 + timedelta(seconds=1))
    page.goto(base + path)
    page.wait_for_selector(f'.node[data-node="{DEPLOY}"]')
    page.clock.run_for(100)
    return page


def activate(page, node):
    page.locator(f'.node[data-node="{node}"]').first.focus()
    page.keyboard.press("Enter")


def run(p, engine, base):
    browser = getattr(p, engine).launch()
    page = None
    ok = True

    def check(good, line):
        nonlocal ok
        print(f"{engine}: {'' if good else 'FAIL '}{line}")
        if not good:
            ok = False
            shoot(page)
        return good

    try:
        # 1. A panel request held: nothing at 900 ms, everything at 1100 ms, nothing on arrival.
        page = new_page(browser, base, PODINFO + "?tab=graph")
        panel = Held(page, lambda url: "/node?" in url)
        before = page.evaluate(STATE)
        activate(page, DEPLOY)
        if not check(panel.wait(1, page), "the panel request was made and held"):
            return False
        page.clock.run_for(900)
        s = page.evaluate(STATE)
        if not check(not s["loading"] and s["busy"] is None and s["line"] is None,
                     f"nothing is shown for a request 900 ms old ({s['loading']}, {s['busy']!r}, {s['line']!r})"):
            return False
        page.clock.run_for(200)
        s = page.evaluate(STATE)
        bar = page.evaluate(BAR)
        if not check(s["loading"] and s["busy"] == "true" and s["line"] == "Loading..." and s["firstChild"] == "detail-loading",
                     f"at 1100 ms the bar is on, #detail is aria-busy and its first line is {s['line']!r}"):
            return False
        if not check(bar["height"] == "2px" and bar["content"] != "none" and bar["events"] == "none" and bar["name"] != "none",
                     f"the bar is a 2 px ::after that takes no pointer and moves ({bar})"):
            return False
        panel.release()
        expect(page.locator("#detail .frag-h").first).to_have_text("podinfo-podinfo")
        page.clock.run_for(100)
        s = page.evaluate(STATE)
        if not check(not s["loading"] and s["busy"] is None and s["line"] is None and before["firstChild"] != "detail-loading",
                     f"on arrival the bar, aria-busy and the line are gone ({s})"):
            return False

        # 2. A fast request shows nothing, not even for a frame.
        page.evaluate("""() => {
            window.sawLoading = false;
            new MutationObserver(() => {
                if (document.body.classList.contains("is-loading") || document.getElementById("detail").hasAttribute("aria-busy")
                    || document.querySelector("#detail > .detail-loading")) { window.sawLoading = true; }
            }).observe(document.documentElement, { attributes: true, childList: true, subtree: true });
        }""")
        fast = Held(page, lambda url: "/node?" in url)
        activate(page, POD)
        if not check(fast.wait(1, page), "the second panel request was made and held"):
            return False
        page.clock.run_for(300)
        fast.release()
        expect(page.locator("#detail .frag-h").first).to_have_text("podinfo-podinfo-d9585d794-4lg6h")
        # A second request starts at 800 ms: the first one's second must not carry over to it.
        page.clock.run_for(500)
        activate(page, DEPLOY)
        if not check(fast.wait(1, page), "a second fast request was made and held at 800 ms"):
            return False
        page.clock.run_for(400)
        s = page.evaluate(STATE)
        if not check(not s["loading"] and s["busy"] is None,
                     f"1200 ms after the first request began, 400 ms into the second, nothing is shown ({s['loading']}, {s['busy']!r})"):
            return False
        fast.release()
        expect(page.locator("#detail .frag-h").first).to_have_text("podinfo-podinfo")
        page.clock.run_for(3000)
        saw = page.evaluate("window.sawLoading")
        if not check(not saw, "requests that ended in 300 ms and 400 ms were never shown, and no timer fired late"):
            return False
        page.unroute_all()

        # 3. Two requests in flight: the bar stays until both have ended.
        panel = Held(page, lambda url: "/node?" in url)
        nav = Held(page, lambda url: url.endswith("/installed"))
        activate(page, DEPLOY)
        panel.wait(1, page)
        page.locator("#nav a[href='/installed']").click()
        if not check(nav.wait(1, page), "the boosted page request was made and held"):
            return False
        page.clock.run_for(1100)
        s = page.evaluate(STATE)
        if not check(s["loading"] and s["busy"] == "true", f"with two requests open the bar is on ({s['loading']}) and #detail is busy ({s['busy']!r})"):
            return False
        panel.release()
        expect(page.locator("#detail .frag-h").first).to_have_text("podinfo-podinfo")
        page.clock.run_for(100)
        s = page.evaluate(STATE)
        if not check(s["loading"] and s["busy"] is None and s["line"] is None,
                     f"the panel's end keeps the bar for the page request but ends the busy state and its line ({s})"):
            return False
        nav.release()
        page.wait_for_url("**/installed")
        page.clock.run_for(100)
        gone = page.evaluate("!document.body.classList.contains('is-loading')")
        if not check(gone, "the page's arrival ends the bar"):
            return False
        page.unroute_all()

        # 4. A 500, an abort and a pair of panel requests each end the bar.
        for how in ("a 500 answer", "an aborted request", "two panel requests"):
            page.goto(base + PODINFO + "?tab=graph")
            page.wait_for_selector(f'.node[data-node="{DEPLOY}"]')
            page.clock.run_for(100)
            held = Held(page, lambda url: "/node?" in url)
            activate(page, DEPLOY)
            held.wait(1, page)
            if how == "two panel requests":
                # A second node request into #detail next to the first; both end together.
                activate(page, POD)
                held.wait(2, page)
            page.clock.run_for(1100)
            if not check(page.evaluate("document.body.classList.contains('is-loading')"), f"{how}: the bar is on"):
                return False
            if how == "a 500 answer":
                held.routes.pop(0).fulfill(status=500, body="boom")
            elif how == "an aborted request":
                held.routes.pop(0).abort()
            else:
                for r in held.routes:
                    r.continue_()
                held.routes = []
            page.clock.run_for(100)
            end = time.time() + 5
            while page.evaluate("document.body.classList.contains('is-loading')") and time.time() < end:
                page.wait_for_timeout(50)
                page.clock.run_for(50)
            s = page.evaluate(STATE)
            if not check(not s["loading"] and s["busy"] is None and s["line"] is None, f"{how}: the bar, aria-busy and the line end ({s})"):
                return False
            page.unroute_all()

        # 5. The stream's background refresh is not an htmx request: it shows neither.
        page.goto(base + PODINFO + "?tab=graph")
        page.wait_for_selector(f'.node[data-node="{DEPLOY}"]')
        page.clock.run_for(100)
        seen = []
        page.on("request", lambda r: seen.append(r.resource_type + " " + r.url))
        refresh = Held(page, lambda url: url.split("?")[0].endswith(PODINFO))
        page.evaluate(
            "() => document.getElementById('stream-events').dispatchEvent(new CustomEvent('sse:upsert',"
            " {detail: {data: JSON.stringify({topic: 'instance:default/podinfo', item: {}})}}))")
        page.clock.run_for(500)
        if not check(refresh.wait(1, page), "the background refresh was made and held"):
            print(f"{engine}: requests seen: {seen}")
            return False
        page.clock.run_for(2000)
        s = page.evaluate(STATE)
        if not check(not s["loading"] and s["busy"] is None and s["line"] is None,
                     f"a refresh that has run for 2 s shows neither the bar nor a busy panel ({s})"):
            return False
        refresh.release()
        page.unroute_all()

        # 6. Reduced motion: the bar is drawn and does not move.
        page.close()
        page = new_page(browser, base, PODINFO + "?tab=graph", reduced="reduce")
        panel = Held(page, lambda url: "/node?" in url)
        activate(page, DEPLOY)
        panel.wait(1, page)
        page.clock.run_for(1100)
        bar = page.evaluate(BAR)
        if not check(bar["height"] == "2px" and bar["name"] == "none", f"under reduced motion the bar is drawn and static ({bar})"):
            return False
        panel.release()

        # 7. At 360 px the bar and the line add no sideways scroll (TestBrowserPhone's measure).
        page.close()
        page = new_page(browser, base, PODINFO + "?tab=graph", width=360)
        panel = Held(page, lambda url: "/node?" in url)
        activate(page, DEPLOY)
        panel.wait(1, page)
        page.clock.run_for(1100)
        s = page.evaluate(STATE)
        wide = page.evaluate("document.documentElement.scrollWidth")
        if not check(s["loading"] and s["line"] == "Loading..." and wide <= 360,
                     f"at 360 px with the bar and the line shown the page is {wide} px wide ({s['loading']}, {s['line']!r})"):
            return False
        panel.release()
        return ok
    except Exception:
        if page is not None:
            shoot(page)
        raise
    finally:
        browser.close()


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    engine, base = sys.argv[1], sys.argv[2].rstrip("/")
    with sync_playwright() as p:
        return 0 if run(p, engine, base) else 1


if __name__ == "__main__":
    sys.exit(main())
