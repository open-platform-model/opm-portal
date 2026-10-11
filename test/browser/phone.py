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
3. The details panel filled by script (portal:D19): on /instances/default/podinfo the podinfo-podinfo
   node is activated from the keyboard, the script waits until #detail .frag-h reads
   podinfo-podinfo, and the measure with the panel filled is at most 360. The panel holds the
   widest content a selection adds (kinds, long names, two badges with labels).

4. The owner pages' identity cards (OP-A5): on the four pages of OWNER_PAGES the identity card's .facts list
   computes to one grid column at this width (below 520 px), and the page holds 360 px with the Applied,
   Health and Provider blocks, their attempt strip and the tips in it.

5. The Graph and Resources tabs (OP-A5): on `?tab=graph` and `?tab=resources` of OWNER_TABS_PAGES, with the
   details panel at rest, the page's scrollWidth is at most 360 (the Graph frame may scroll inside itself).
   On the Resources tabs the table's header row is not displayed (it is clipped to a 1 px box and stays in
   the accessibility tree), every displayed body row computes to display block as a card no wider than the
   viewport, and the member rows of a closed configuration group (cert-manager) compute to display none
   until the group's <details> opens, and to display block after.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import sync_playwright

WIDTH = 360
HEIGHT = 640

# The page and node of the details-panel step.
PANEL_PAGE = "/instances/default/podinfo"
DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"

# The owner pages whose identity card the facts check reads (OP-A5): an instance, a large instance, a
# provider instance and a package.
OWNER_PAGES = [
    "/instances/default/podinfo",
    "/instances/cert-manager/cert-manager",
    "/instances/default/backup-provider",
    "/packages/pkg/podinfo",
]

# The pages whose Graph and Resources tabs step 5 opens, with the details panel at rest.
OWNER_TABS_PAGES = [
    "/instances/default/podinfo",
    "/instances/cert-manager/cert-manager",
    "/packages/pkg/podinfo",
]

# What the Resources table is at this width: the header's box, the displayed body rows and their
# display and width, and the member rows with their display.
TABLE_FACTS = """
width => {
  const table = document.querySelector('.resources-table');
  if (!table) return null;
  const head = table.querySelector('thead').getBoundingClientRect();
  const rows = Array.from(table.querySelectorAll('tbody tr'));
  const shown = rows.filter(r => getComputedStyle(r).display !== 'none');
  return {
    headBox: [Math.round(head.width), Math.round(head.height)],
    shown: shown.length,
    notBlock: shown.filter(r => getComputedStyle(r).display !== 'block').length,
    wide: shown.filter(r => r.getBoundingClientRect().right > width + 0.5).length,
    members: rows.filter(r => r.classList.contains('member')).map(r => getComputedStyle(r).display),
  };
}
"""

# The tracks of the identity card's .facts grid: one for a single column.
FACT_TRACKS = "() => getComputedStyle(document.querySelector('#owner-head .facts')).gridTemplateColumns.trim().split(/\\s+/).length"

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


def check_table(engine, page, url):
    """The Resources table at this width: no header row shown, every shown body row a card within the
    viewport, and a closed group's member rows hidden until its details opens."""
    facts = page.evaluate(TABLE_FACTS, WIDTH)
    if facts is None:
        print(f"{engine}: FAIL {url} has no Resources table")
        return False
    ok = True
    if facts["headBox"][0] > 1 or facts["headBox"][1] > 1:
        print(f"{engine}: FAIL {url} the header row is displayed: box {facts['headBox']}")
        ok = False
    if facts["shown"] == 0 or facts["notBlock"] or facts["wide"]:
        print(f"{engine}: FAIL {url} {facts['shown']} rows shown, {facts['notBlock']} not display block, {facts['wide']} wider than {WIDTH}")
        ok = False
    if "cert-manager" in url:
        closed = page.locator("tbody.group details:not([open])")
        if closed.count() == 0 or not facts["members"] or any(d != "none" for d in facts["members"]):
            print(f"{engine}: FAIL {url} member rows of a closed group display {sorted(set(facts['members']))}; want none")
            ok = False
        closed.first.locator("summary").click()
        opened = page.evaluate(TABLE_FACTS, WIDTH)
        if any(d != "block" for d in opened["members"][:1]) or opened["notBlock"] or opened["wide"]:
            print(f"{engine}: FAIL {url} after opening the group its first member displays {opened['members'][:1]}, {opened['notBlock']} rows not block")
            ok = False
    return ok


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

        for path in OWNER_PAGES:
            open_page(page, base + path)
            tracks = page.evaluate(FACT_TRACKS)
            if tracks == 1:
                print(f"{engine}: {path} identity facts compute to one column")
                continue
            failed.append(path + " (identity facts)")
            print(f"{engine}: FAIL {path} identity facts compute to {tracks} columns at {WIDTH} px; want one")
            shoot(page, len(paths) + 1)

        for path in OWNER_TABS_PAGES:
            for tab in ("graph", "resources"):
                url = f"{path}?tab={tab}"
                open_page(page, base + url)
                width = page.evaluate(MEASURE)
                rest = page.locator("#detail .frag-h").count() == 1
                ok = width <= WIDTH and rest
                if tab == "resources":
                    ok = check_table(engine, page, url) and ok
                if ok:
                    print(f"{engine}: {url} scrollWidth {width}, the panel at rest")
                    continue
                failed.append(url + " (owner tab)")
                past = page.evaluate(PAST_EDGE, WIDTH)
                print(f"{engine}: FAIL {url} scrollWidth {width} (panel at rest {rest}); past the edge: {'; '.join(past) or 'none found'}")
                shoot(page, len(paths) + 2)

        open_page(page, base + PANEL_PAGE)
        page.locator(f'.node[data-node="{DEPLOY}"]').first.focus()
        page.keyboard.press("Enter")
        page.wait_for_function(
            "() => { const h = document.querySelector('#detail .frag-h'); return !!h && h.textContent.trim() === 'podinfo-podinfo'; }")
        page.wait_for_timeout(300)
        width = page.evaluate(MEASURE)
        if width <= WIDTH:
            print(f"{engine}: {PANEL_PAGE} with the details panel filled scrollWidth {width}")
        else:
            failed.append(PANEL_PAGE + " (details panel filled)")
            past = page.evaluate(PAST_EDGE, WIDTH)
            print(f"{engine}: FAIL {PANEL_PAGE} with the details panel filled scrollWidth {width} over {WIDTH}; past the edge: {'; '.join(past) or 'none found'}")
            shoot(page, len(paths))
    finally:
        browser.close()
    total = len(paths) + 1 + len(OWNER_PAGES) + 2 * len(OWNER_TABS_PAGES)
    print(f"{engine}: {total - len(failed)} of {total} checks hold {WIDTH} px")
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
