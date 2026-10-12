"""phone_endpoints.py: a Service's Endpoints fact holds 360 px, in a real browser.

Usage: python3 phone_endpoints.py <browser> <counted site URL> <unread site URL>

Both sites are the portal's UI over the F1 capture plus the recorded EndpointSlice of podinfo's
Service. On the first the caller may list EndpointSlices, so the panel reads `2 ready`; on the second
the caller may not, so it reads `Could not be read`. The viewport is 360 by 640 with the light colour
scheme. For each site the script opens /instances/default/podinfo, selects podinfo's Service node from
the keyboard, waits for the Endpoints fact and checks:

1. The check can fail: on the first site the script adds an element 500 px wide and requires the
   measure (the larger of documentElement.scrollWidth and body.scrollWidth) to rise above 360.
2. The Endpoints fact reads the wanted words, and the wanted words are the only ones: `2 ready` on the
   first site, `Could not be read` on the second, never `0 ready` there.
3. The page's measure with the panel filled is at most 360, and the fact's label and value end at or
   before the right edge of the viewport.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import expect, sync_playwright

WIDTH = 360
HEIGHT = 640
PAGE = "/instances/default/podinfo"
SERVICE = "obj:_/Service/default/podinfo-podinfo"

# The page policy forbids eval inside the page, not page.evaluate over the debugging protocol.
MEASURE = "Math.max(document.documentElement.scrollWidth, document.body.scrollWidth)"
INJECT = """
() => {
  const el = document.createElement('div');
  el.id = 'phone-probe';
  el.style.width = '500px';
  el.style.height = '10px';
  (document.querySelector('main') || document.body).appendChild(el);
}
"""
REMOVE = "() => document.getElementById('phone-probe').remove()"
FACT_RIGHT = """
() => {
  const dt = Array.from(document.querySelectorAll('#detail .facts dt')).find(e => e.textContent.trim() === 'Endpoints');
  if (!dt) return null;
  return Math.max(dt.getBoundingClientRect().right, dt.nextElementSibling.getBoundingClientRect().right);
}
"""


def shoot(page, index: int = 0) -> None:
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + f"-site{index}.png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def open_service_panel(page, base):
    page.goto(base + PAGE, wait_until="load")
    # A late font swap can widen a page: wait for the fonts, then keep 300 ms as a margin.
    page.evaluate("document.fonts.ready.then(() => true)")
    page.locator(f'.node[data-node="{SERVICE}"]').first.focus()
    page.keyboard.press("Enter")
    expect(page.locator('#detail .facts dt:text-is("Endpoints") + dd')).to_be_visible()
    page.wait_for_timeout(300)


def run(p, engine, sites):
    browser = getattr(p, engine).launch()
    ok = True
    try:
        page = browser.new_context(viewport={"width": WIDTH, "height": HEIGHT}, color_scheme="light").new_page()

        # 1. The measure can fail.
        open_service_panel(page, sites[0][0])
        page.evaluate(INJECT)
        probed = page.evaluate(MEASURE)
        page.evaluate(REMOVE)
        print(f"{engine}: the check can fail: a 500 px element measures scrollWidth {probed}")
        if probed <= WIDTH:
            print(f"{engine}: FAIL the measure did not rise above {WIDTH} for a 500 px element")
            shoot(page)
            return False

        for index, (base, want) in enumerate(sites):
            open_service_panel(page, base)
            text = page.locator('#detail .facts dt:text-is("Endpoints") + dd').first.text_content().strip()
            width = page.evaluate(MEASURE)
            right = page.evaluate(FACT_RIGHT)
            detail = page.locator("#detail").text_content()
            good = text == want and "0 ready" not in detail.replace(want, "") and width <= WIDTH and right is not None and right <= WIDTH
            print(f"{engine}: {'ok  ' if good else 'FAIL'} Endpoints {text!r} (want {want!r}), scrollWidth {width}, fact ends at {right}")
            if not good:
                ok = False
                shoot(page, index)
    finally:
        browser.close()
    return ok


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    engine = sys.argv[1]
    sites = [(sys.argv[2].rstrip("/"), "2 ready"), (sys.argv[3].rstrip("/"), "Could not be read")]
    with sync_playwright() as p:
        return 0 if run(p, engine, sites) else 1


if __name__ == "__main__":
    sys.exit(main())
