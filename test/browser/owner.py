"""owner.py: the owner page's details panel, selection and Resources hand-off, in a real browser.

Usage: python3 owner.py <browser> <site URL>

The site is the portal's UI over the F1 capture, under the portal's page policy, which forbids
eval inside the page: the script waits with selectors and URL predicates. Checks, each printing
one line, exiting 1 on the first failure:

1. podinfo's Graph tab opens with the ModuleInstance in #detail (never an empty placeholder),
   and #detail names the panel Clear selection brings back.
2. A click on the Deployment node (from the keyboard where the engine cannot hit-test an SVG
   link) shows podinfo-podinfo in #detail, and Clear selection brings the ModuleInstance back.
3. After that click, the YAML tab link carries the selection and opens the Deployment's YAML.
4. A Resources row, followed by a click anywhere on the row (the link is stretched over it),
   opens the Graph tab focused, with the panel filled.

5. The Events tab: the Type select applies on change with no button press and narrows the table,
   Clear filters brings the whole feed back, and a Resource cell is a link that narrows the table
   to that one resource.

6. The Events, Logs and YAML panels inset every section by the same 20 px: no direct child of the
   tab panel (a note, the form, the table, a log or YAML pane) touches the panel's left or right edge.

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import expect, sync_playwright

PODINFO = "/instances/default/podinfo"
DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"
HEAD = "#detail .frag-h"
ROOT = "default/podinfo"
DEP = "podinfo-podinfo"


def check(engine, ok, line):
    print(f"{engine}: {'ok  ' if ok else 'FAIL'} {line}")
    return ok


def shoot(page) -> None:
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def panel_is(page, title):
    """Waits until the panel's heading reads title; the heading holds <wbr> hints, so it is read
    as text, not matched by a :text-is selector."""
    expect(page.locator(HEAD).first).to_have_text(title)


def activate(page, selector):
    """Activates a graph node from the keyboard, as a keyboard user does."""
    page.locator(selector).first.focus()
    page.keyboard.press("Enter")


def run(p, engine, base):
    browser = getattr(p, engine).launch()
    page = browser.new_context(viewport={"width": 1440, "height": 900}).new_page()
    try:
        # 1. The panel at rest.
        page.goto(base + PODINFO + "?tab=graph")
        panel_is(page, ROOT)
        kicker = page.locator("#detail .kicker").first.text_content().strip()
        root_panel = page.get_attribute("#detail", "data-root-panel")
        if not check(engine, kicker == "Instance" and (root_panel or "").endswith("/node?id=mi%3Adefault%2Fpodinfo"),
                     f"the Graph tab rests on the ModuleInstance: kicker {kicker!r}, root panel {root_panel!r}"):
            return False

        # 2. Select the Deployment, then Clear selection.
        activate(page, f'.node[data-node="{DEPLOY}"]')
        panel_is(page, DEP)
        selected = page.evaluate("!!document.querySelector('.node[aria-current=\"true\"]')")
        if not check(engine, selected, "a node click shows podinfo-podinfo in #detail and marks the node"):
            return False
        page.locator("[data-clear-selection]").click()
        panel_is(page, ROOT)
        placeholder = page.locator("#detail .muted", has_text="Select a graph node").count()
        if not check(engine, placeholder == 0, "Clear selection brings the ModuleInstance back, not a placeholder"):
            return False

        # 3. The YAML tab after a click.
        activate(page, f'.node[data-node="{DEPLOY}"]')
        panel_is(page, DEP)
        page.locator(".owner-tabs a", has_text="YAML").click()
        page.wait_for_url("**tab=yaml**")
        page.wait_for_selector("pre.yaml")
        text = page.locator("pre.yaml").text_content()
        if not check(engine, "kind: Deployment" in text and "focus=" in page.url,
                     f"the YAML tab opened the Deployment's YAML at {page.url}"):
            return False

        # 4. A Resources row opens the Graph focused, with the panel filled.
        page.goto(base + PODINFO + "?tab=resources")
        panel_is(page, ROOT)
        rest = page.locator("#detail .frag-h").first.text_content().strip()
        if not check(engine, rest == "default/podinfo", f"the Resources tab rests on the ModuleInstance: {rest!r}"):
            return False
        # The click lands on the Kind cell, not on the link: the link is stretched over the row.
        # Playwright's own click refuses a target another element covers, so the mouse clicks the
        # cell's centre, as a pointer does.
        cell = page.locator("#components tr.res", has_text="podinfo-podinfo").first.locator("td").first.bounding_box()
        page.mouse.click(cell["x"] + cell["width"] / 2, cell["y"] + cell["height"] / 2)
        page.wait_for_url("**tab=graph**")
        page.wait_for_selector(".node[aria-current]")
        panel_is(page, DEP)
        if not check(engine, "focus=" in page.url, f"a click on the row opened {page.url} with the panel filled"):
            return False
        return events_tab(page, engine, base) and panel_insets(page, engine, base)
    except Exception:
        print(f"{engine}: FAIL at {page.url}")  # the address shows which step stopped
        shoot(page)
        raise
    finally:
        browser.close()


def events_tab(page, engine, base):
    """5. The Type select applies on change; a resource link narrows the table."""
    rows = page.locator("table.events-table tbody tr")
    page.goto(base + PODINFO + "?tab=events")
    page.wait_for_selector("table.events-table")
    total = rows.count()
    page.select_option("#f-type", "Warning")
    page.wait_for_url("**type=Warning**")
    expect(page.locator("table.events-table tbody tr.ev-normal")).to_have_count(0)
    warnings = page.locator("table.events-table tbody tr.ev-warning").count()
    expect(page.locator(".filter-meta .shown")).to_contain_text(f"Showing {warnings} of {total}")
    if not check(engine, 0 < warnings < total, f"the Type select applied on change: {warnings} Warning rows of {total}"):
        return False
    page.locator(".fclear").click()
    page.wait_for_function("() => !location.search.includes('type=')")
    expect(rows).to_have_count(total)
    first = page.locator("table.events-table a.ev-resource").first
    name = first.locator(".mono").text_content().strip()
    first.click()
    page.wait_for_url("**resource=**")
    names = set(n.strip() for n in page.locator("table.events-table a.ev-resource .mono").all_text_contents())
    if not check(engine, names == {name} and rows.count() < total,
                 f"a resource link narrowed the table to {name}: {sorted(names)}, {rows.count()} of {total} rows"):
        return False
    return True


INSETS = """() => {
  const panel = document.querySelector('.panel.owner-panel');
  const p = panel.getBoundingClientRect();
  return [...panel.children].filter(c => !c.classList.contains('vh') && c.getBoundingClientRect().height > 0)
    .map(c => { const r = c.getBoundingClientRect();
      return [c.tagName.toLowerCase() + '.' + c.className, Math.round(r.left - p.left), Math.round(p.right - r.right)]; });
}"""


def panel_insets(page, engine, base):
    """6. Every section of the Events, Logs and YAML panels sits 20 px in from each edge."""
    for tab in ("events", "logs", "yaml"):
        page.goto(base + PODINFO + "?tab=" + tab)
        page.wait_for_selector(".panel.owner-panel")
        sections = page.evaluate(INSETS)
        bad = [s for s in sections if s[1] < 19 or s[2] < 19]
        if not check(engine, sections and not bad,
                     f"{tab}: {len(sections)} sections inset by at least 19 px each side; touching the edge: {bad}"):
            return False
    return True


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    engine, base = sys.argv[1], sys.argv[2].rstrip("/")
    with sync_playwright() as p:
        return 0 if run(p, engine, base) else 1


if __name__ == "__main__":
    sys.exit(main())
