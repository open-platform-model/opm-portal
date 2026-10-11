"""phone.py: every page holds its width at a 360 px wide viewport, in a real browser.

Usage: python3 phone.py <browser> <site URL> <path>...

The site is the portal's UI over the F1 capture. The viewport is 360 by 640 with the light colour
scheme (the themes change colours, not layout). For each path the script loads the page, waits
300 ms for the layout to settle, and measures the larger of documentElement.scrollWidth and
body.scrollWidth. The body has overflow-x hidden, so a scrollbar would not show content that the
page clips; scrollWidth still counts it. Boxes that scroll inside themselves (the graph, the YAML
view, the log panes, the masthead nav) do not count: an element past the edge inside such a box is
the intended way to show wide content.

Checks, each printing one line per page, then exiting 1 if any failed:

1. The check can fail: on the first page the script adds an element 500 px wide and requires the
   measure to rise above 360, so a layout change that blinds the measure fails the test.
2. Each page's measure is at most 360. A failing page lists up to six elements whose right edge is
   past 360 and that have no scrolling ancestor.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import sync_playwright

WIDTH = 360
HEIGHT = 640

# The wider of the two scroll widths. The page policy forbids eval inside the page, but
# page.evaluate runs through the browser's debugging protocol, as the other scripts use it.
MEASURE = "Math.max(document.documentElement.scrollWidth, document.body.scrollWidth)"

# Up to six elements whose right edge is past the viewport and that no scroll box contains. An
# ancestor that scrolls (overflow-x auto or scroll) holds its content; body and html are not such a
# box, because the body clips with overflow-x hidden and that is the loss this test looks for.
PAST_EDGE = """
width => {
  const found = [];
  for (const el of document.body.querySelectorAll('*')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.right <= width + 0.5) continue;
    let held = false;
    for (let a = el.parentElement; a && a !== document.body; a = a.parentElement) {
      const ox = getComputedStyle(a).overflowX;
      if (ox === 'auto' || ox === 'scroll') { held = true; break; }
    }
    if (held) continue;
    const cls = typeof el.className === 'string' && el.className ? '.' + el.className.trim().split(/\\s+/).join('.') : '';
    found.push(el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + cls + ' right=' + Math.round(r.right));
    if (found.length === 6) break;
  }
  return found;
}
"""

# A 500 px element, sized through the CSSOM: the page policy forbids a style attribute, not this.
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


def shoot(page, index: int = 0) -> None:
    """Save a screenshot of page into OPM_PORTAL_BROWSER_SHOTS, when it is set.

    The page's index in the path list joins the file name, so each failing page keeps its own."""
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + f"-page{index}.png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def open_page(page, url):
    page.goto(url, wait_until="load")
    # A late font swap can widen a page: wait for the fonts, then keep 300 ms as a margin.
    page.evaluate("document.fonts.ready.then(() => true)")
    page.wait_for_timeout(300)


def run(p, engine, base, paths):
    browser = getattr(p, engine).launch()
    failed = []
    try:
        page = browser.new_context(
            viewport={"width": WIDTH, "height": HEIGHT}, color_scheme="light"
        ).new_page()

        open_page(page, base + paths[0])
        page.evaluate(INJECT)
        probed = page.evaluate(MEASURE)
        page.evaluate(REMOVE)
        print(f"{engine}: the check can fail: a 500 px element measures scrollWidth {probed}")
        if probed <= WIDTH:
            print(f"{engine}: FAIL the measure did not rise above {WIDTH} for a 500 px element")
            shoot(page)
            return False

        for index, path in enumerate(paths):
            open_page(page, base + path)
            width = page.evaluate(MEASURE)
            if width <= WIDTH:
                print(f"{engine}: {path} scrollWidth {width}")
                continue
            failed.append(path)
            past = page.evaluate(PAST_EDGE, WIDTH)
            print(f"{engine}: FAIL {path} scrollWidth {width} over {WIDTH}; past the edge: {'; '.join(past) or 'none found'}")
            shoot(page, index)
    finally:
        browser.close()
    print(f"{engine}: {len(paths) - len(failed)} of {len(paths)} pages hold {WIDTH} px")
    return not failed


def main() -> int:
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    engine, base, paths = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3:]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, paths) else 1


if __name__ == "__main__":
    sys.exit(main())
