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
    except Exception:
        print(f"{engine}: FAIL at {page.url}")  # the address shows which step stopped
        shoot(page)
        raise
    finally:
        browser.close()
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
