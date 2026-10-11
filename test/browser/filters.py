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

9. The provider badge of `default/backup-provider` on Installed (installed-11, WCAG 2.2 1.4.13):
   Tab reaches the badge and shows its tooltip; the tooltip's box lies inside the viewport and no
   element clips it (every probe point of the box hits the box), on its own row and on the last row
   of the table (filtered to that row), at 900 px (where the table scrolls inside its own wrap and the
   page does not scroll sideways) and on a 360 px viewport; a click on the badge does not
   navigate; a click on the row's Owner cell opens /instances/default/podinfo; Escape closes the
   tooltip with focus kept on the badge; the pointer can move from the badge onto the open box
   without closing it; and on a touch viewport a tap opens it without leaving the page and a second
   tap closes it (Firefox runs the click path where its touch emulation is not available).

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

        # 9: the provider badge on Installed.
        ROW = 'tr:has(a[href="/instances/default/backup-provider"])'
        if not provider_badge(browser, fresh, base, check, engine, ROW):
            return False
    finally:
        browser.close()
    return ok


# Where a box is on screen and whether every probe point of it hits the box itself, so that no element
# of the table or the page clips or covers it.
BOX = """(sel) => {
    const box = document.querySelector(sel);
    box.scrollIntoView({block: "nearest", inline: "nearest"}); // the page may scroll to a box below the fold
    const r = box.getBoundingClientRect();
    const probes = [[0.1, 0.1], [0.9, 0.1], [0.1, 0.9], [0.9, 0.9], [0.5, 0.5]];
    const hit = probes.every(([fx, fy]) => {
        const el = document.elementFromPoint(r.left + r.width * fx, r.top + r.height * fy);
        return el && el.closest(".tipbox") === box;
    });
    return {left: r.left, right: r.right, top: r.top, bottom: r.bottom, w: r.width, h: r.height, hit,
            vw: window.innerWidth, vh: window.innerHeight,
            scroll: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth),
            wrap: getComputedStyle(document.querySelector(".installed-wrap")).overflowX};
}"""


def tab_to(page, selector, limit=60):
    """Presses Tab until the element matching selector has focus; returns whether it got there."""
    for _ in range(limit):
        page.keyboard.press("Tab")
        if page.evaluate("(s) => document.activeElement && document.activeElement.matches(s)", selector):
            return True
    return False


def provider_badge(browser, fresh, base, check, engine, ROW):
    badge_sel = f"{ROW} .tip > button.tip-t"
    box_sel = f"{ROW} .tipbox"

    def inside(b):
        # A pixel of slack: the page's scroll height is a whole number, so a box that ends at the page's
        # bottom edge can sit a fraction of a pixel past the viewport after the page has scrolled to it.
        return b["left"] >= 0 and b["right"] <= b["vw"] and b["top"] >= 0 and b["bottom"] <= b["vh"] + 1

    # Keyboard on a wide viewport: the tooltip shows on Tab, opens past the row, is not clipped.
    page = fresh()
    page.goto(base + PATH, wait_until="load")
    page.evaluate("document.fonts.ready.then(() => true)")
    page.click("#f-q")
    if not check(page, tab_to(page, ".tip-t"), "Tab reaches the provider badge"):
        return False
    badge, box = page.locator(badge_sel), page.locator(box_sel)
    named = page.evaluate("() => document.activeElement.closest('tr').querySelector('a.row-link').getAttribute('href')")
    if not check(page, named == "/instances/default/backup-provider" and box.is_visible(),
                 f"focus on the badge of {named} shows its tooltip"):
        return False
    b = page.evaluate(BOX, box_sel)
    if not check(page, inside(b) and b["hit"] and b["wrap"] == "visible",
                 f"the tooltip lies inside the viewport ({b['left']:.0f} to {b['right']:.0f} of {b['vw']}) and nothing clips it (wrap overflow {b['wrap']!r}, hit {b['hit']})"):
        return False
    text = box.inner_text()
    if not check(page, "default.backup-provider" in text and "accepted and active" in text, f"the tooltip names the registration and its standing: {text!r}"):
        return False
    page.keyboard.press("Escape")
    on_badge = page.evaluate("document.activeElement.classList.contains('tip-t')")
    if not check(page, not box.is_visible() and on_badge, f"Escape closes the tooltip with focus kept on the badge ({on_badge})"):
        return False
    page.context.close()

    # A click on the badge does not navigate; the pointer reaches the open box; a click on the Owner cell opens the item.
    page = fresh()
    page.goto(base + PATH, wait_until="load")
    badge, box = page.locator(badge_sel), page.locator(box_sel)
    badge.click()
    page.wait_for_timeout(400)
    stayed = page.evaluate("location.pathname + location.search") == PATH
    if not check(page, stayed, "a click on the badge does not navigate"):
        return False
    page.mouse.move(2, 790)
    page.wait_for_timeout(100)
    x, y = (lambda r: (r["x"] + r["width"] / 2, r["y"] + r["height"] / 2))(badge.bounding_box())
    page.mouse.move(x, y)
    shown = box.is_visible()
    r = box.bounding_box()
    page.mouse.move(r["x"] + r["width"] / 2, r["y"] + r["height"] / 2, steps=12)
    if not check(page, shown and box.is_visible(), f"the pointer moves from the badge onto the open box without closing it ({shown})"):
        return False
    # The row's own link overlays the cell, so a real pointer click at the cell's centre is the test, not a
    # click that waits for the cell itself to receive it.
    owner = page.locator('tr:has(a[href="/instances/default/podinfo"]) td[data-label="Owner"]')
    owner.scroll_into_view_if_needed()
    cell = owner.bounding_box()
    page.mouse.click(cell["x"] + cell["width"] / 2, cell["y"] + cell["height"] / 2)
    page.wait_for_function("() => location.pathname === '/instances/default/podinfo'", timeout=8000)
    check(page, True, "a click on the Owner cell of default/podinfo opens /instances/default/podinfo")
    page.context.close()

    # The last row of the table: the box opens past the row and is not clipped.
    page = fresh()
    page.goto(base + PATH + "?q=backup-provider", wait_until="load")
    page.evaluate("document.fonts.ready.then(() => true)")
    rows = page.evaluate("document.querySelectorAll('#list tbody tr').length")
    page.locator(badge_sel).focus()
    b = page.evaluate(BOX, box_sel)
    if not check(page, rows == 1 and inside(b) and b["hit"],
                 f"on the last row ({rows} row) the tooltip lies inside the viewport ({b['bottom']:.0f} of {b['vh']}) and nothing clips it (hit {b['hit']})"):
        return False
    page.context.close()

    # Between 761 and 1039 px the 940 px table scrolls inside its own wrap and the page does not (the
    # web-ui requirement on page width): the page's scroll width is the viewport's, the wrap scrolls, and
    # the tooltip opens in the cell's flow, inside the viewport and not clipped.
    ctx = browser.new_context(viewport={"width": 900, "height": 800}, color_scheme="light")
    page = ctx.new_page()
    page.goto(base + PATH + "?q=backup-provider", wait_until="load")
    page.evaluate("document.fonts.ready.then(() => true)")
    page.locator(badge_sel).focus()
    b = page.evaluate(BOX, box_sel)
    wrap = page.evaluate("(() => { const w = document.querySelector('.installed-wrap'); return {scroll: w.scrollWidth, client: w.clientWidth, ox: getComputedStyle(w).overflowX}; })()")
    if not check(page, b["scroll"] <= 900 and wrap["ox"] == "auto" and wrap["scroll"] > wrap["client"],
                 f"at 900 px the page scrolls {b['scroll']} (at most 900) and the table scrolls inside its own wrap (overflow {wrap['ox']!r}, {wrap['scroll']} in {wrap['client']})"):
        return False
    if not check(page, inside(b) and b["hit"] and b["h"] > 0,
                 f"at 900 px the tooltip lies inside the viewport ({b['left']:.0f} to {b['right']:.0f}) and nothing clips it (hit {b['hit']})"):
        return False
    page.context.close()

    # A 360 px viewport (touch where the engine has it): a tap opens, a second tap closes, no navigation.
    how = "tap"
    try:
        ctx = browser.new_context(viewport={"width": 360, "height": 700}, has_touch=True, color_scheme="light")
        page = ctx.new_page()
        page.goto(base + PATH + "?q=backup-provider", wait_until="load")
        page.locator(badge_sel).tap()
    except Exception as err:
        how = f"click (touch emulation is not available: {str(err).splitlines()[0]})"
        ctx = browser.new_context(viewport={"width": 360, "height": 700}, color_scheme="light")
        page = ctx.new_page()
        page.goto(base + PATH + "?q=backup-provider", wait_until="load")
        page.locator(badge_sel).click()
    print(f"{engine}: the touch checks run the {how} path")
    badge, box = page.locator(badge_sel), page.locator(box_sel)
    first = box.is_visible()

    def activate():
        if how == "tap":
            badge.tap()
        else:
            badge.click()

    if how == "tap":
        b = page.evaluate(BOX, box_sel)
        if not check(page, first and b["left"] >= 0 and b["right"] <= 360 and b["scroll"] <= 360 and b["hit"],
                     f"a tap opens the tooltip ({first}); it spans {b['left']:.0f} to {b['right']:.0f} of 360, the page scrolls {b['scroll']}, hit {b['hit']}"):
            return False
        activate()
        closed = not box.is_visible()
        left = page.evaluate("location.pathname + location.search") == PATH + "?q=backup-provider"
        if not check(page, closed and left, f"a second tap closes it ({closed}) and the page did not navigate ({left})"):
            return False
        activate()
        b = page.evaluate(BOX, box_sel)
        check(page, box.is_visible() and b["right"] <= 360, "the tooltip opens again and stays inside 360 px")
    else:
        activate()
        second = box.is_visible()
        b = page.evaluate(BOX, box_sel)
        if not check(page, (not first) and second and b["left"] >= 0 and b["right"] <= 360 and b["hit"],
                     f"the first click closes the hovered tooltip ({not first}), the next opens it ({second}) inside 360 px ({b['left']:.0f} to {b['right']:.0f}, hit {b['hit']})"):
            return False
    page.keyboard.press("Escape")
    page.context.close()
    return True


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    engine, base, context = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, context) else 1


if __name__ == "__main__":
    sys.exit(main())
