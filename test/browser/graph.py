"""graph.py: graph interactions on an instance page, in a real browser.

Usage: python3 graph.py <browser> <site URL>

The site is the portal's UI over the F1 capture, under the portal's page policy, which forbids
eval: the script waits with URL predicates and selectors, never page.wait_for_function. Checks, each printing one line, exiting 1 on the
first failure:

1. Clear selection: after a node is selected and the zoom set, "Clear selection" drops the focus
   from the address without a new history entry, keeps the zoom, and undims the graph.
2. A Resources row's Graph link opens the Graph tab spotlighting that node.
3. A group node expands in place and fits the view to its members; "Whole graph" collapses it.
4. A live refresh keeps the tab, the focus, the selection and the zoom. The refresh is driven by
   dispatching the stream's upsert event for the page's topic on the stream listener, as the SSE
   extension does.
5. Full screen enters, survives a live refresh, and one Escape leaves it.
6. The details panel after a node selection: activating the podinfo-podinfo Deployment node from the
   keyboard (and with a pointer click where the engine hit-tests the node) fills #detail with that
   node's name and its Object fact, and marks the node selected; the instance's own node fills it
   with an Applied and a Health badge. The panel request must not inherit the page's hx-select.
"""

import json
import sys

from playwright.sync_api import sync_playwright

PODINFO = "/instances/default/podinfo"
DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"
ROOT = "mi:default/podinfo"


def check(engine, ok, line):
    print(f"{engine}: {line}")
    return ok


def set_zoom(page, pct):
    page.evaluate(
        "pct => { const r = document.querySelector('[data-zoom-range]'); r.value = String(pct);"
        " r.dispatchEvent(new Event('input', {bubbles: true})); }", pct)


def zoom(page):
    return page.evaluate("document.querySelector('[data-zoom-range]').value")


def activate(page, selector):
    """Activates a graph node from the keyboard, as a keyboard user does. A pointer click on an SVG
    link inside the scrolled graph frame is not reliably hit-tested by Firefox's automation."""
    page.locator(selector).first.focus()
    page.keyboard.press("Enter")


def live_refresh(page, topic):
    page.evaluate(
        "topic => document.getElementById('stream-events').dispatchEvent(new CustomEvent('sse:upsert',"
        " {detail: {data: JSON.stringify({topic: topic, item: {}})}}))", topic)
    page.wait_for_selector("#graph.refreshed", timeout=10000)


def run(p, engine, base):
    browser = getattr(p, engine).launch()
    try:
        page = browser.new_context(viewport={"width": 1440, "height": 900}).new_page()

        page.goto(base + PODINFO + "?tab=graph")
        set_zoom(page, 120)
        activate(page, f'.node[data-node="{DEPLOY}"]')
        page.wait_for_url(lambda u: "focus=" in u)
        before = page.evaluate("history.length")
        page.locator("[data-clear-selection]").click()
        page.wait_for_url(lambda u: "focus=" not in u)
        after, dims, cur = page.evaluate(
            "[history.length, document.querySelectorAll('.graph .dim').length,"
            " document.querySelectorAll('.node[aria-current]').length]")
        if not check(engine, after == before and zoom(page) == "120" and dims == 0 and cur == 0,
                     f"clear selection: history {before}->{after}, zoom {zoom(page)}, dimmed {dims}, selected {cur}"):
            return False

        page.goto(base + PODINFO + "?tab=resources")
        page.locator("#components a", has_text="Graph").first.click()
        page.wait_for_url("**tab=graph**")
        page.wait_for_selector(".node[aria-current]")
        dims = page.evaluate("document.querySelectorAll('.graph .node.dim').length")
        if not check(engine, "focus=" in page.url and dims > 0, f"Resources row opened {page.url}, {dims} nodes dimmed"):
            return False

        page.goto(base + "/instances/cert-manager/cert-manager?tab=graph")
        fitted = zoom(page)
        activate(page, ".node[data-group]")
        page.wait_for_url("**expand=**")
        page.wait_for_selector(".graph[data-fitted]")
        members = page.evaluate("document.querySelectorAll('.node[data-member-of]').length")
        grouped = zoom(page)
        if not check(engine, members > 0 and grouped != fitted,
                     f"group expanded to {page.url}: {members} members, zoom {fitted}% -> {grouped}%"):
            return False
        page.locator("[data-whole-graph]").click()
        page.wait_for_url(lambda u: "expand=" not in u)
        if not check(engine, page.locator(".node[data-member-of]").count() == 0, f"Whole graph returned to {page.url}"):
            return False

        page.goto(base + PODINFO + f"?tab=graph&focus={DEPLOY}")
        set_zoom(page, 150)
        url = page.url
        live_refresh(page, "instance:default/podinfo")
        state = page.evaluate(
            "[location.href, document.querySelector('[data-zoom-range]').value,"
            f" !!document.querySelector('.node[data-node=\"{DEPLOY}\"][aria-current]')]")
        if not check(engine, state == [url, "150", True], f"after a live refresh: {json.dumps(state)}"):
            return False

        page.mouse.move(1, 1)
        page.locator("[data-graph-full]").click()
        page.wait_for_selector('[data-graph-full][aria-pressed="true"]')
        live_refresh(page, "instance:default/podinfo")
        full = page.evaluate("!!document.fullscreenElement || !!document.querySelector('.graph-full')")
        pressed = page.get_attribute("[data-graph-full]", "aria-pressed")
        if not check(engine, full and pressed == "true", f"full screen through a refresh: full {full}, pressed {pressed}"):
            return False
        page.keyboard.press("Escape")
        page.wait_for_selector('[data-graph-full][aria-pressed="false"]')
        left = page.evaluate("!document.fullscreenElement && !document.querySelector('.graph-full')")
        still = page.evaluate(f"!!document.querySelector('.node[data-node=\"{DEPLOY}\"][aria-current]')")
        if not check(engine, left and still, f"one Escape left full screen ({left}) and kept the selection ({still})"):
            return False

        # 6. The details panel after a node selection (instance-43).
        page.goto(base + PODINFO + "?tab=graph")
        activate(page, f'.node[data-node="{DEPLOY}"]')
        page.wait_for_selector("#detail .frag-h")
        name, facts, cur = page.evaluate(
            "[document.querySelector('#detail .frag-h').textContent.trim(),"
            " document.querySelector('#detail .facts').textContent,"
            f" !!document.querySelector('.node[data-node=\"{DEPLOY}\"][aria-current=\"true\"]')]")
        if not check(engine, name == "podinfo-podinfo" and "Deployment" in facts and cur,
                     f"keyboard selection filled the panel: {name!r}, Object fact has Deployment {'Deployment' in facts}, "
                     f"node marked {cur}"):
            return False
        if engine != "firefox":
            page.goto(base + PODINFO + "?tab=graph")
            page.locator(f'.node[data-node="{DEPLOY}"]').first.click()
            page.wait_for_selector("#detail .frag-h")
            clicked = page.evaluate("document.querySelector('#detail .frag-h').textContent.trim()")
            if not check(engine, clicked == "podinfo-podinfo", f"pointer selection filled the panel: {clicked!r}"):
                return False
        page.goto(base + PODINFO + "?tab=graph")
        activate(page, f'.node[data-node="{ROOT}"]')
        page.wait_for_selector("#detail .axes .applied")
        applied, health = page.evaluate(
            "[document.querySelectorAll('#detail .axes .applied').length,"
            " document.querySelectorAll('#detail .axes .health').length]")
        if not check(engine, applied == 1 and health == 1, f"the instance node's panel: {applied} Applied badge, {health} Health badge"):
            return False
    finally:
        browser.close()
    return True


def main() -> int:
    engine, base = sys.argv[1], sys.argv[2].rstrip("/")
    with sync_playwright() as p:
        return 0 if run(p, engine, base) else 1


if __name__ == "__main__":
    sys.exit(main())
