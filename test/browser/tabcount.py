"""tabcount.py: a tab count follows the page live, in a real browser (portal:D19:R3).

Usage: python3 tabcount.py <browser> <site URL>

The site is the portal's UI over the F1 capture, which is static, so no real change arrives. The
script opens the instance page, then answers the page's own refresh request (the fetch of the same
URL that follows a stream event) with the page's real render edited as a later render would be, and
dispatches the stream event on the page's listener the way the SSE extension does. The real
portal.js then replaces the followed regions. Checks, each printing one line, exit 1 on the first
failure:

1. The strip draws the Events count (3) and the Resources count (5) as followed spans.
2. A render with a changed Events count (4) updates that span with no reload, and the strip's links
   are the same elements as before (the strip is not a followed region).
3. A render whose Events source is a problem (the count span is empty) empties the span, so the old
   number does not stay beside a feed the page can no longer read, and the empty span draws nothing.
4. A later render with the count back (3) fills the empty span again.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import re
import sys

from playwright.sync_api import sync_playwright

PATH = "/instances/default/podinfo"
SEND = """([name, data]) => document.getElementById("stream-events")
    .dispatchEvent(new CustomEvent(name, { detail: { data: JSON.stringify(data) } }))"""
EVENTS = re.compile(r'(<span class="tab-n" id="tab-n-events"[^>]*>)[^<]*(</span>)')

STATE = """() => {
    const n = document.getElementById("tab-n-events");
    const r = document.getElementById("tab-n-resources");
    return {
        events: n && n.textContent, resources: r && r.textContent,
        shown: !!n && n.getClientRects().length > 0,
        strip: document.querySelector("nav.tabs") === window.strip,
        links: Array.from(document.querySelectorAll("nav.tabs > a")).every((a, i) => a === window.links[i]),
    };
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


def run(p, engine, base):
    browser = getattr(p, engine).launch()
    try:
        page = browser.new_page(viewport={"width": 1280, "height": 800})
        page.goto(base + PATH, wait_until="load")
        page.wait_for_selector("#tab-n-events")
        page.evaluate("""() => {
            window.strip = document.querySelector("nav.tabs");
            window.links = Array.from(document.querySelectorAll("nav.tabs > a"));
        }""")

        # What the next refresh fetch is answered with: None passes the real render through, a
        # string replaces the Events count text.
        later = {"count": None}
        served = {"n": 0}

        def answer(route):
            if route.request.resource_type != "fetch" or later["count"] is None:
                route.continue_()
                return
            response = route.fetch()
            body = EVENTS.sub(lambda m: m.group(1) + later["count"] + m.group(2), response.text())
            served["n"] += 1
            route.fulfill(response=response, body=body)

        page.route("**" + PATH, answer)

        def refresh(count, want):
            later["count"] = count
            before = served["n"]
            page.evaluate(SEND, ["sse:upsert", {"topic": "instance:default/podinfo"}])
            page.wait_for_function("([want]) => document.getElementById('tab-n-events').textContent === want", arg=[want], timeout=10000)
            return page.evaluate(STATE), served["n"] - before

        def check(ok, line):
            print(f"{engine}: {'' if ok else 'FAIL '}{line}")
            if not ok:
                shoot(page)
            return ok

        start = page.evaluate(STATE)
        if not check(start["events"] == "3" and start["resources"] == "5" and start["shown"],
                     f"the strip draws Events {start['events']!r} and Resources {start['resources']!r} as followed spans"):
            return False

        got, fetched = refresh("4", "4")
        if not check(got["events"] == "4" and got["resources"] == "5" and got["strip"] and got["links"] and fetched == 1,
                     f"a render with 4 events updates the span with no reload (Events {got['events']!r}, strip kept {got['strip']}, links kept {got['links']}, {fetched} fetch)"):
            return False

        got, _ = refresh("", "")
        if not check(got["events"] == "" and not got["shown"] and got["resources"] == "5" and got["links"],
                     f"a render whose Events source is a problem empties the span ({got['events']!r}) and it draws nothing (shown {got['shown']})"):
            return False

        got, _ = refresh("3", "3")
        if not check(got["events"] == "3" and got["shown"], f"a later render fills the empty span again ({got['events']!r}, shown {got['shown']})"):
            return False
    finally:
        browser.close()
    return True


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    engine, base = sys.argv[1], sys.argv[2].rstrip("/")
    with sync_playwright() as p:
        return 0 if run(p, engine, base) else 1


if __name__ == "__main__":
    sys.exit(main())
