"""logs.py: keep a tailed log pane through region refreshes in a real browser.

Usage: python3 logs.py <browser> <page URL>

The page is a logs region the Go test serves under the page policy with the page script. Its first
render lists Pod a's pane, its second adds Pod b's, and every later one lists no Pod. Stream events
are dispatched on the page's stream listener the way the SSE extension does. The pane, scrolled to
its bottom and focused, must keep its scroll offset, its focus and its tail through a refresh that
adds a pane and one that leaves no Pod, and a new line must scroll into view. Prints one line per
check and exits 1 on the first failure. With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a
screenshot there.
"""

import json
import os
import sys

from playwright.sync_api import sync_playwright

PANE = "#log_a_c"
TOPIC = "log:ns/a/c"

SEND = """([name, data]) => document.getElementById("stream-events")
    .dispatchEvent(new CustomEvent(name, { detail: { data: JSON.stringify(data) } }))"""

STATE = """() => {
    const d = document.querySelector("%s");
    const pre = d && d.querySelector(".log-pane");
    if (!pre) { return null; }
    const last = pre.lastElementChild.getBoundingClientRect();
    const box = pre.getBoundingClientRect();
    return {
        top: pre.scrollTop,
        bottom: pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 1,
        lastShown: last.bottom <= box.bottom + 1 && last.top >= box.top - 1,
        focused: document.activeElement === pre,
        same: pre === window.tailed,
        open: d.open,
        gone: d.classList.contains("log-gone"),
        panes: Array.from(document.querySelectorAll("details.log")).map((p) => p.id),
    };
}""" % PANE


def main() -> int:
    engine, url = sys.argv[1], sys.argv[2]
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            page = browser.new_page(viewport={"width": 1000, "height": 700})
            try:
                rc = check(page, engine, url)
            except Exception:
                shoot(page)
                raise
            if rc:
                shoot(page)
            return rc
        finally:
            browser.close()


def check(page, engine: str, url: str) -> int:
    page.goto(url)
    page.wait_for_load_state("load")
    page.evaluate(f'() => {{ document.querySelector("{PANE}").open = true; }}')
    page.wait_for_function(f'() => document.querySelector("{PANE}").getAttribute("data-following") === "{TOPIC}"')
    items = [{"type": "line", "text": f"line {i}"} for i in range(500)]
    page.evaluate(SEND, ["sse:snapshot", {"topic": TOPIC, "items": items}])
    page.evaluate(f"""() => {{
        const pre = document.querySelector("{PANE} .log-pane");
        window.tailed = pre;
        pre.focus();
        pre.scrollTop = pre.scrollHeight;
    }}""")
    start = page.evaluate(STATE)
    print(f"{engine} tailing: {json.dumps(start)}")
    if not start or start["top"] <= 0 or not start["bottom"] or not start["focused"]:
        return fail(engine, "the pane did not scroll to its bottom with focus")

    for step, ready, want in (
        ("refresh adding a Pod", '() => document.getElementById("log_b_c") !== null', ["log_a_c", "log_b_c"]),
        ("refresh with no Pod", '() => document.getElementById("log_b_c") === null', ["log_a_c"]),
    ):
        before = page.evaluate(STATE)
        page.evaluate(SEND, ["sse:upsert", {"topic": "owner:t"}])
        page.wait_for_function(ready, timeout=10000)
        kept = page.evaluate(STATE)
        print(f"{engine} {step}: {json.dumps(kept)}")
        if not kept or not kept["same"] or not kept["open"]:
            return fail(engine, f"{step}: the pane was replaced or closed")
        if kept["top"] != before["top"] or not kept["bottom"]:
            return fail(engine, f"{step}: scrollTop {before['top']} became {kept['top']}")
        if not kept["focused"]:
            return fail(engine, f"{step}: the pane lost focus")
        if kept["panes"] != want:
            return fail(engine, f"{step}: panes {kept['panes']}, want {want}")
        if kept["gone"] != (want == ["log_a_c"]):
            return fail(engine, f"{step}: gone mark is {kept['gone']}")

        page.evaluate(SEND, ["sse:log", {"topic": TOPIC, "item": {"type": "line", "text": f"after {step}"}}])
        tail = page.evaluate(STATE)
        print(f"{engine} new line after {step}: {json.dumps(tail)}")
        if tail["top"] <= kept["top"] or not tail["bottom"] or not tail["lastShown"]:
            return fail(engine, f"{step}: the new line did not scroll into view")
    return 0


def shoot(page) -> None:
    """Save a screenshot of page into OPM_PORTAL_BROWSER_SHOTS, when it is set."""
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def fail(engine: str, why: str) -> int:
    print(f"{engine} FAIL: {why}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
