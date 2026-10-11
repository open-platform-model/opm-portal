"""tip.py: the info tip can be dismissed, hovered and opened by a tap, in a real browser.

Usage: python3 tip.py <browser> <site URL> <tip markup>

The site is the portal's UI over the F1 capture. No page shows a tip yet, so the script opens / and
inserts the markup of the golden fragment-tip.html in a paragraph at the top of #main, with the real portal.css and
portal.js loaded. The follow-on change that puts a tip on a page adds a case on that page.
Checks (WCAG 2.2 1.4.13, portal:D19), each printing one line, exiting 1 on the first failure:

Keyboard, in a window with a pointer. The trigger is a native button (WCAG 2.2 4.1.2):
1. The box is hidden at rest, and the trigger names it: it is a button, aria-describedby is the box's
   id, and the box has role="tooltip".
2. Focus shows the box.
3. Escape hides it with focus still on the trigger (dismissible without moving focus).
4. Enter shows it again, and Enter again hides it.
5. Space shows it without scrolling the page.
13. Tab away closes an opened tip (Escape, Enter, Tab: hidden), and Shift+Tab back shows it.
14. A tip dismissed by Escape is not left dismissed: Escape, Tab away, Shift+Tab back shows it.
15. Escape steps back one layer: with the theme menu open and a tip shown, the first Escape hides only
   the tip, the second closes the menu.
Pointer:
6. With the pointer on the trigger the box shows, and it stays shown with the pointer on the box,
   after the pointer crossed from the trigger to the box in steps (hoverable).
7. Escape hides it while the pointer stays on the box (dismissible without moving the pointer).
8. Once the pointer leaves, the tip is not left dismissed: hovering the trigger shows the box again.
Touch (a has_touch context at 360 px wide; Firefox runs the click path when its touch emulation is
not available, and the output says which path ran):
9. A tap on the trigger opens the box and it fits the viewport with no sideways scroll.
12. With the trigger moved to the right edge, the box is shifted to stay inside the viewport, and
   the page still does not scroll sideways.
10. A second tap closes it; a third opens it again.
11. A tap outside closes it.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import sync_playwright

WIDE = {"width": 1280, "height": 800}
PHONE = {"width": 360, "height": 700}

FRESH = """
markup => {
  document.querySelectorAll('#tip-host').forEach(t => t.remove());
  // A paragraph holds the tip, as a sentence would: a direct child of main is a grid item and would stretch it.
  document.querySelector('#main').insertAdjacentHTML('afterbegin', '<p id="tip-host">' + markup + '</p>');
}
"""


def shoot(page, name):
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + f"-{name}.png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def open_fresh(page, base, markup):
    page.goto(base + "/", wait_until="load")
    page.evaluate("document.fonts.ready.then(() => true)")
    page.evaluate(FRESH, markup)
    page.mouse.move(2, 790)
    page.wait_for_timeout(150)


def centre(locator):
    b = locator.bounding_box()
    return b["x"] + b["width"] / 2, b["y"] + b["height"] / 2


def run(p, engine, base, markup):
    browser = getattr(p, engine).launch()

    def check(page, ok, line, shot):
        print(f"{engine}: {'' if ok else 'FAIL '}{line}")
        if not ok:
            shoot(page, shot)
        return ok

    try:
        page = browser.new_context(viewport=WIDE, color_scheme="light").new_page()
        open_fresh(page, base, markup)
        tip, box = page.locator(".tip"), page.locator(".tipbox")
        trigger = page.locator(".tip > button.tip-t")

        described = trigger.get_attribute("aria-describedby")
        if not check(page, not box.is_visible() and described == box.get_attribute("id") and box.get_attribute("role") == "tooltip",
                     f"hidden at rest; the button's aria-describedby={described!r} names the role=tooltip box", "rest"):
            return False

        trigger.focus()
        if not check(page, box.is_visible(), "focus shows the box", "focus"):
            return False

        page.keyboard.press("Escape")
        on_trigger = page.evaluate("document.activeElement.classList.contains('tip-t')")
        if not check(page, not box.is_visible() and on_trigger, f"Escape hides the box with focus still on the trigger ({on_trigger})", "escape"):
            return False

        page.keyboard.press("Enter")
        shown = box.is_visible()
        page.keyboard.press("Enter")
        if not check(page, shown and not box.is_visible(), f"Enter shows it again ({shown}), and Enter again hides it", "enter"):
            return False

        page.evaluate("document.body.style.minHeight = '3000px'")  # a long page, so a scroll could happen
        before = page.evaluate("window.scrollY")
        page.keyboard.press("Space")
        shown = box.is_visible()
        after = page.evaluate("window.scrollY")
        if not check(page, shown and before == after, f"Space shows it without scrolling (scrollY {before} to {after})", "space"):
            return False

        # Tab away closes an opened tip, and Shift+Tab back shows it (it is not left hidden or dismissed).
        page.keyboard.press("Escape")
        page.keyboard.press("Enter")
        opened = box.is_visible()
        page.keyboard.press("Tab")
        away = page.evaluate("document.activeElement.classList.contains('tip-t')")
        closed = not box.is_visible()
        page.keyboard.press("Shift+Tab")
        back = box.is_visible()
        if not check(page, opened and not away and closed and back,
                     f"Escape, Enter opens it ({opened}); Tab away closes it ({closed}); Shift+Tab back shows it ({back})", "tab-away"):
            return False

        # A dismissed tip is not left dismissed once focus has left it.
        page.keyboard.press("Escape")
        dismissed = not box.is_visible()
        page.keyboard.press("Tab")
        page.keyboard.press("Shift+Tab")
        again = box.is_visible()
        if not check(page, dismissed and again, f"Escape dismisses it ({dismissed}); after Tab away and Shift+Tab back it shows again ({again})", "tab-away-dismissed"):
            return False

        # Escape closes one layer per press: the tip first, then the theme menu.
        open_fresh(page, base, markup)
        tip, box = page.locator(".tip"), page.locator(".tipbox")
        page.locator("details.theme > summary").click()
        menu = page.locator("details.theme")
        menu_open = menu.evaluate("d => d.open")
        page.locator(".tip > button.tip-t").focus()
        shown = box.is_visible()
        page.keyboard.press("Escape")
        first = (not box.is_visible()) and menu.evaluate("d => d.open")
        page.keyboard.press("Escape")
        second = not menu.evaluate("d => d.open")
        if not check(page, menu_open and shown and first and second,
                     f"menu open ({menu_open}) and tip shown ({shown}): the first Escape hides only the tip ({first}), the second closes the menu ({second})", "escape-order"):
            return False

        open_fresh(page, base, markup)
        tip, box = page.locator(".tip"), page.locator(".tipbox")
        x, y = centre(tip)
        page.mouse.move(x, y)
        on_trigger = box.is_visible()
        bx, by = centre(box)
        page.mouse.move(bx, by, steps=12)
        if not check(page, on_trigger and box.is_visible(), f"the pointer on the trigger shows the box ({on_trigger}) and it stays with the pointer on the box", "hover"):
            return False

        page.keyboard.press("Escape")
        if not check(page, not box.is_visible(), "Escape hides it while the pointer stays on the box", "hover-escape"):
            return False

        page.mouse.move(2, 790)
        page.wait_for_timeout(100)
        dismissed = page.evaluate("document.querySelector('.tip').classList.contains('is-dismissed')")
        page.mouse.move(x, y)
        if not check(page, not dismissed and box.is_visible(), f"after the pointer left, the tip is not left dismissed ({dismissed}) and shows again on hover", "hover-again"):
            return False

        touch = None
        try:
            touch = browser.new_context(viewport=PHONE, has_touch=True, color_scheme="light").new_page()
            open_fresh(touch, base, markup)
            touch.locator(".tip").tap()
            how = "tap"
        except Exception as err:
            how = f"click (touch emulation is not available: {str(err).splitlines()[0]})"
            touch = browser.new_context(viewport=PHONE, color_scheme="light").new_page()
            open_fresh(touch, base, markup)
            touch.locator(".tip").click()
        print(f"{engine}: the touch checks run the {how} path")
        tip, box = touch.locator(".tip"), touch.locator(".tipbox")
        tapped = how == "tap"

        def activate():
            if tapped:
                tip.tap()
            else:
                tip.click()

        # On the tap path the first tap above has opened the box; on the click path the click hovered
        # the tip first, so the box was shown when the pointer went down and the click closed it.
        first = box.is_visible()
        if tapped:
            r = box.bounding_box()
            fits = r["x"] >= 0 and r["x"] + r["width"] <= PHONE["width"]
            scroll = touch.evaluate("Math.max(document.documentElement.scrollWidth, document.body.scrollWidth)")
            if not check(touch, first and fits and scroll <= PHONE["width"],
                         f"a tap opens the box ({first}); it spans {r['x']:.0f} to {r['x'] + r['width']:.0f} of {PHONE['width']} and the page scrolls {scroll}", "tap"):
                return False
            activate()
            closed = not box.is_visible()
            activate()
            reopened = box.is_visible()
            if not check(touch, closed and reopened, f"a second tap closes it ({closed}), a third opens it again ({reopened})", "tap-toggle"):
                return False
        else:
            activate()
            second = box.is_visible()
            if not check(touch, not first and second, f"the first click closes the hovered box ({not first}), the next opens it ({second})", "click-toggle"):
                return False

        if tapped:
            touch.touchscreen.tap(PHONE["width"] - 10, PHONE["height"] - 10)
        else:
            touch.mouse.click(PHONE["width"] - 10, PHONE["height"] - 10)
        touch.wait_for_timeout(100)
        if not check(touch, not box.is_visible(), "a tap outside closes it", "outside"):
            return False

        # The trigger at the right edge: the box would run past the viewport without the shift.
        touch.evaluate("document.querySelector('#tip-host').style.textAlign = 'right'")
        activate()
        if not box.is_visible():
            activate()
        r = box.bounding_box()
        scroll = touch.evaluate("Math.max(document.documentElement.scrollWidth, document.body.scrollWidth)")
        inside = r is not None and r["x"] >= 0 and r["x"] + r["width"] <= PHONE["width"]
        if not check(touch, box.is_visible() and inside and scroll <= PHONE["width"],
                     f"a trigger at the right edge keeps its box inside the viewport ({r['x']:.0f} to {r['x'] + r['width']:.0f} of {PHONE['width']}), the page scrolls {scroll}", "edge"):
            return False
    finally:
        browser.close()
    return True


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    engine, base, markup = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, markup) else 1


if __name__ == "__main__":
    sys.exit(main())
