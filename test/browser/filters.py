"""filters.py: filter forms apply as the user types and announce their count, in a real browser.

Usage: python3 filters.py <browser> <site URL> <context>

The site is the portal's UI over the F1 capture. Checks, each printing one line, exiting 1 on the
first failure:

1. Typing `backup` in Installed's search applies after the pause and keeps focus and caret in the
   field: the address becomes /installed?q=backup, the history entry is replaced (its length does
   not grow), the list holds the two backup instances, and the focused element is the search field
   with its caret at the end.
2. Back after typing returns to the page before the filters were touched (the Platform page).
3. Enter submits once: typing and pressing Enter at once makes one request.
4. Characters typed while a request is in flight are kept and applied: with the first request held
   (`ba`) and `ck` typed meanwhile, the page ends on /installed?q=back with `back` in the field,
   the caret after it, and focus kept.
5. A select applies at once, and the remembered Installed filter is still stored after a typed
   search (the replaced entry is remembered as a pushed one is).
6. The Apply button is not visible until it takes keyboard focus (Tab reaches it, and it shows),
   and a form with no script would show it (the opm-js class is what hides it).
7. A text field that is not a search applies when the user leaves it, on a form of any page: the
   owner page's Events form, whose Reason field is such a field.
8. The announced count: the header's status region holds the new count after typing ("Showing 2 of
   6"), after a select change, after a chip is removed and after "Clear all"; a followed refresh
   leaves it as it was; a change of the stream's state rewrites the stream's text only; a boosted
   move to the Platform page empties it; the region is not atomic and is the only element with
   role status.

The browser script also counts what the test needs: htmx requests, and the requests it holds. With
OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import sync_playwright

PATH = "/installed"
OWNER_EVENTS = "/instances/default/podinfo?tab=events"

# Counts htmx requests and, on demand, holds each request's send until released, so the test can type
# while a request is in flight. An aborted held request is dropped when released.
INIT = """
(() => {
  window.__reqs = 0; window.__hold = false; window.__held = [];
  document.addEventListener("htmx:beforeRequest", () => { window.__reqs++; });
  const send = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function (body) {
    if (window.__hold) { window.__held.push([this, body]); return; }
    return send.call(this, body);
  };
  window.__release = () => {
    window.__hold = false;
    for (const [x, b] of window.__held.splice(0)) { try { send.call(x, b); } catch (e) { /* aborted */ } }
  };
})();
"""

STATE = """() => {
    const a = document.activeElement;
    return {
        active: a && a.id, value: a && a.value, caret: a && a.selectionStart,
        path: location.pathname + location.search, hist: history.length, reqs: window.__reqs,
        note: document.querySelector("#live .live-note").textContent,
        text: document.querySelector("#live .live-text").textContent,
        shown: (document.getElementById("shown") || {textContent: ""}).textContent.trim(),
        rows: document.querySelectorAll("#list tbody tr").length,
    };
}"""
HIDDEN = """() => {
    const b = document.querySelector(".field-submit").getBoundingClientRect();
    return {w: b.width, h: b.height, js: document.documentElement.classList.contains("opm-js")};
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


def run(p, engine, base, context):
    browser = getattr(p, engine).launch()
    ok = True

    def check(page, cond, line):
        nonlocal ok
        print(f"{engine}: {'' if cond else 'FAIL '}{line}")
        if not cond:
            ok = False
            shoot(page)
        return cond

    def fresh():
        ctx = browser.new_context(viewport={"width": 1280, "height": 800})
        ctx.add_init_script(INIT)
        return ctx.new_page()

    def wait_path(page, want, timeout=8000):
        page.wait_for_function("([w]) => location.pathname + location.search === w", arg=[want], timeout=timeout)

    def wait_note(page, want, timeout=8000):
        page.wait_for_function("([w]) => document.querySelector('#live .live-note').textContent === w", arg=[want], timeout=timeout)

    try:
        # 1, 2, 5 (remembered), 6 (Apply), 8 (announced count): from the Platform page to Installed.
        page = fresh()
        page.goto(base + "/", wait_until="load")
        page.click('nav a[href="/installed"]')
        wait_path(page, PATH)
        before = page.evaluate(STATE)
        if not check(page, page.evaluate(HIDDEN)["js"], "script adds opm-js to the root"):
            return False
        hidden = page.evaluate(HIDDEN)
        if not check(page, hidden["w"] <= 1 and hidden["h"] <= 1,
                     f"the Apply button is hidden until focused ({hidden['w']:.0f} x {hidden['h']:.0f})"):
            return False
        page.click("#f-q")
        page.keyboard.type("backup", delay=30)
        wait_path(page, "/installed?q=backup")
        page.wait_for_function("() => document.querySelectorAll('#list tbody tr').length === 2", timeout=8000)
        got = page.evaluate(STATE)
        if not check(page, got["active"] == "f-q" and got["value"] == "backup" and got["caret"] == 6,
                     f"typing applies after the pause and keeps focus and caret (focus {got['active']!r}, value {got['value']!r}, caret {got['caret']})"):
            return False
        if not check(page, got["hist"] == before["hist"] and got["rows"] == 2,
                     f"the history entry is replaced (length {before['hist']} -> {got['hist']}) and 2 rows are listed"):
            return False
        stored = page.evaluate("(k) => window.localStorage.getItem(k)", f"opm-portal.filters.installed:{context}")
        if not check(page, stored == "q=backup", f"the remembered Installed filter is stored after the typed search ({stored!r})"):
            return False
        wait_note(page, "Showing 2 of 6")
        check(page, page.evaluate(STATE)["note"] == "Showing 2 of 6", "the count is announced once through the header's status region: 'Showing 2 of 6'")

        # 8: the region is not atomic and the only status element.
        live = page.evaluate("""() => ({
            atomic: document.getElementById("live").getAttribute("aria-atomic"),
            status: document.querySelectorAll('[role="status"]').length })""")
        if not check(page, live["atomic"] == "false" and live["status"] == 1,
                     f"#live is not atomic (aria-atomic {live['atomic']!r}) and is the only role=status ({live['status']})"):
            return False

        # 6: Tab reaches the Apply button and shows it.
        page.click("#f-q")
        shown = None
        for _ in range(12):
            page.keyboard.press("Tab")
            if page.evaluate("() => document.activeElement && document.activeElement.closest('.field-submit') !== null"):
                shown = page.evaluate(HIDDEN)
                break
        if not check(page, shown is not None and shown["w"] > 1 and shown["h"] > 1,
                     f"Tab reaches the Apply button and it shows ({shown and round(shown['w'])} x {shown and round(shown['h'])})"):
            return False

        # 5, 8: a select applies at once and updates the count.
        page.select_option("#f-kind", "package")
        wait_path(page, "/installed?q=backup&kind=package")
        wait_note(page, "Showing 0 of 6")
        check(page, True, "a select applies at once and the count follows: 'Showing 0 of 6'")

        # 8: removing a chip, and "Clear all", each announce their count.
        page.click('a.fchip[aria-label="Remove Kind filter"]')
        wait_path(page, "/installed?q=backup")
        wait_note(page, "Showing 2 of 6")
        check(page, True, "removing a chip announces its count: 'Showing 2 of 6'")
        page.click("a.fclear")
        wait_path(page, PATH)
        wait_note(page, "Showing 6 of 6")
        check(page, True, "Clear all announces its count: 'Showing 6 of 6'")

        # 8: a followed refresh leaves the note; a change of the stream's state rewrites the stream's text only.
        page.click("#f-q")
        page.keyboard.type("backup", delay=30)
        wait_note(page, "Showing 2 of 6")
        page.evaluate("() => { window.__shown = document.getElementById('shown'); }")
        page.evaluate("""() => document.getElementById("stream-events").dispatchEvent(
            new CustomEvent("sse:upsert", { detail: { data: JSON.stringify({ topic: "instances" }) } }))""")
        page.wait_for_function("() => document.getElementById('shown') !== window.__shown", timeout=10000)
        after = page.evaluate(STATE)
        if not check(page, after["note"] == "Showing 2 of 6", f"a followed refresh leaves the note as it was ({after['note']!r})"):
            return False
        page.evaluate("() => document.body.dispatchEvent(new CustomEvent('htmx:sseError'))")
        after = page.evaluate(STATE)
        if not check(page, after["text"] == "Reconnecting" and after["note"] == "Showing 2 of 6",
                     f"a change of the stream's state rewrites its text only (text {after['text']!r}, note {after['note']!r})"):
            return False

        # 8: a boosted move to another page empties the note.
        page.click('nav a[href="/"]')
        wait_path(page, "/")
        wait_note(page, "")
        check(page, True, "a boosted move to the Platform page leaves the note empty")
        page.context.close()

        # 2: Back after typing.
        page = fresh()
        page.goto(base + "/", wait_until="load")
        page.click('nav a[href="/installed"]')
        wait_path(page, PATH)
        page.click("#f-q")
        page.keyboard.type("backup", delay=30)
        wait_path(page, "/installed?q=backup")
        page.go_back()
        wait_path(page, "/")
        page.wait_for_function("() => document.title.startsWith('Platform')", timeout=8000)
        check(page, page.title().startswith("Platform"), "Back after typing returns to the page before the filters were touched")
        page.context.close()

        # 3: Enter submits once.
        page = fresh()
        page.goto(base + PATH, wait_until="load")
        page.click("#f-q")
        page.keyboard.type("backup", delay=20)
        page.keyboard.press("Enter")
        wait_path(page, "/installed?q=backup")
        page.wait_for_timeout(800)
        got = page.evaluate(STATE)
        check(page, got["reqs"] == 1, f"Enter submits once ({got['reqs']} request)")
        page.context.close()

        # 4: characters typed while a request is in flight are kept and applied.
        page = fresh()
        page.goto(base + PATH, wait_until="load")
        page.evaluate("() => { window.__hold = true; }")
        page.click("#f-q")
        page.keyboard.type("ba", delay=30)
        page.wait_for_function("() => window.__held.length >= 1", timeout=8000)
        page.keyboard.type("ck", delay=0)
        page.evaluate("() => window.__release()")
        wait_path(page, "/installed?q=back")
        page.wait_for_timeout(500)
        got = page.evaluate(STATE)
        check(page, got["value"] == "back" and got["caret"] == 4 and got["active"] == "f-q" and got["path"] == "/installed?q=back",
              f"characters typed in flight are kept and applied (value {got['value']!r}, caret {got['caret']}, focus {got['active']!r}, {got['path']})")
        page.context.close()

        # 7: a text field that is not a search applies when the user leaves it.
        page = fresh()
        page.goto(base + OWNER_EVENTS, wait_until="load")
        field = page.query_selector("form[data-filters] input:not([type=search]):not([type=hidden])")
        if not check(page, field is not None, "the owner page's Events form has a text field"):
            return False
        field.click()
        page.keyboard.type("BackOff", delay=20)
        page.keyboard.press("Tab")
        page.wait_for_function("() => location.search.includes('reason=BackOff')", timeout=8000)
        check(page, True, "a text field applies when the user leaves it, on a form of another page")
        page.context.close()
    finally:
        browser.close()
    return ok


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    engine, base, context = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, context) else 1


if __name__ == "__main__":
    sys.exit(main())
