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
   Each badge stands beside a visible label that names its axis (portal:D19:R3): "Applied" and
   "Health" for the instance's node, "Health" alone for the Deployment's.
7. Selection and spotlight: a click selects with `node=` and dims nothing; hovering or focusing the Pod
   dims the Service (the chain is the Pod's ancestors and descendants, not its siblings); the hover
   card sits right of the node, or left of it where there is no room on the right.
8. The Resources hand-off to a folded RBAC object expands and fits the group and shows the object
   spotlit inside the frame.
9. The Clear selection pill returns the panel to the resting node; Escape at the resting node does not
   clear it, and the resting node wears the plain ring.
"""

import json
import sys
import time

from playwright.sync_api import sync_playwright

PODINFO = "/instances/default/podinfo"
# The visible axis labels of the details panel, each with whether it has a box (a hidden one has none).
LABELS = (
    "Array.from(document.querySelectorAll('#detail .axes .axis-pair > .label'),"
    " el => [el.textContent.trim(), el.offsetWidth > 1 && el.offsetHeight > 1])"
)
DEPLOY = "obj:apps/Deployment/default/podinfo-podinfo"
SERVICE = "obj:_/Service/default/podinfo-podinfo"
POD = "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"
CERT = "/instances/cert-manager/cert-manager"
STROKE = "id => getComputedStyle(document.querySelector(`.node[data-node=\"${id}\"] .node-ring`)).strokeWidth"
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
        page.wait_for_url(lambda u: "node=" in u and "focus=" not in u)
        page.wait_for_selector("#detail .clear-pill")
        before = page.evaluate("history.length")
        page.locator("#detail .clear-pill").click()
        page.wait_for_url(lambda u: "node=" not in u)
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
        page.wait_for_selector("#detail .clear-pill")
        name, facts, cur = page.evaluate(
            "[document.querySelector('#detail .frag-h').textContent.trim(),"
            " document.querySelector('#detail .facts').textContent,"
            f" !!document.querySelector('.node[data-node=\"{DEPLOY}\"][aria-current=\"true\"]')]")
        if not check(engine, name == "podinfo-podinfo" and "Deployment" in facts and cur,
                     f"keyboard selection filled the panel: {name!r}, Object fact has Deployment {'Deployment' in facts}, "
                     f"node marked {cur}"):
            return False
        labels = page.evaluate(LABELS)
        if not check(engine, labels == [["Health", True]], f"the Deployment's panel shows a visible Health label: {labels}"):
            return False
        if engine != "firefox":
            page.goto(base + PODINFO + "?tab=graph")
            page.locator(f'.node[data-node="{DEPLOY}"]').first.click()
            page.wait_for_selector("#detail .clear-pill")
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
        labels = page.evaluate(LABELS)
        if not check(engine, labels == [["Applied", True], ["Health", True]],
                     f"the instance node's panel shows visible Applied and Health labels: {labels}"):
            return False

        # 7. Selection does not dim; the chain does (hover and focus); the card keeps to the node.
        page.goto(base + PODINFO + "?tab=graph")
        activate(page, f'.node[data-node="{SERVICE}"]')
        page.wait_for_url(lambda u: "node=" in u)
        dims, cur, halo = page.evaluate(
            "[document.querySelectorAll('.graph .dim').length,"
            " document.querySelectorAll('.node[aria-current]').length,"
            " document.querySelectorAll('.node[data-spotlight]').length]")
        if not check(engine, dims == 0 and cur == 1 and halo == 0 and "focus=" not in page.url,
                     f"a click selects with node= and dims nothing: dimmed {dims}, selected {cur}, halo {halo}, {page.url}"):
            return False
        page.goto(base + PODINFO + "?tab=graph")
        page.locator(f'.node[data-node="{POD}"]').first.focus()
        page.wait_for_selector(".node-card:not([hidden])")
        chain = page.evaluate(
            f"[...document.querySelectorAll('.graph .node.chain-dim')].map(n => n.getAttribute('data-node'))")
        if not check(engine, SERVICE in chain and POD not in chain and DEPLOY not in chain and ROOT not in chain,
                     f"focusing the Pod dims the Service and keeps its chain lit: dimmed {chain}"):
            return False
        card_left = page.evaluate(
            f"(() => {{ const c = document.querySelector('.node-card:not([hidden])').getBoundingClientRect();"
            f" const n = document.querySelector('.node[data-node=\"{POD}\"]').getBoundingClientRect();"
            " return [c.left, c.right, n.left, n.right]; })()")
        if not check(engine, card_left[1] <= card_left[2] + 1 or card_left[0] >= card_left[3] - 1,
                     f"the Pod's card keeps off its node: {json.dumps(card_left)}"):
            return False
        page.keyboard.press("Escape")
        page.wait_for_selector(".node-card:not([hidden])", state="detached")
        after = page.evaluate("document.querySelectorAll('.graph .chain-dim, .graph.chain-on').length")
        if not check(engine, after == 0, f"Escape hides the card and lifts the transient dimming: {after} left"):
            return False
        page.locator(f'.node[data-node="{ROOT}"]').first.focus()
        page.wait_for_selector(".node-card:not([hidden])")
        card = page.evaluate(
            f"(() => {{ const c = document.querySelector('.node-card:not([hidden])').getBoundingClientRect();"
            f" const n = document.querySelector('.node[data-node=\"{ROOT}\"]').getBoundingClientRect();"
            " return [c.left, n.right]; })()")
        if not check(engine, card[0] >= card[1] - 1, f"the instance's card sits right of its node: {json.dumps(card)}"):
            return False
        page.keyboard.press("Escape")

        # 8. The hand-off to a folded RBAC object opens and fits the group, with the object spotlit.
        page.goto(base + CERT + "?tab=resources")
        page.locator("#config-components > summary").click()
        page.locator('#components a[href*="expand="]', has_text="Graph").first.click()
        page.wait_for_url("**fit=**")
        page.wait_for_selector(".graph[data-fitted]")
        page.wait_for_selector(".node[data-spotlight]")
        seen = False
        for _ in range(40):
            seen = page.evaluate(
                "(() => { const n = document.querySelector('.node[data-spotlight]').getBoundingClientRect();"
                " const f = document.querySelector('.graph-frame').getBoundingClientRect();"
                " return n.left >= f.left - 1 && n.right <= f.right + 1 && n.top >= f.top - 1 && n.bottom <= f.bottom + 1; })()")
            if seen:
                break
            time.sleep(0.1)
        frames = page.locator(".group-frame").count()
        if not check(engine, seen and frames == 1,
                     f"the hand-off opened the group ({frames} frame) and shows the spotlit object inside the frame: {seen}"):
            return False

        # 9. Clear selection returns to the resting node; Escape there clears nothing.
        page.goto(base + PODINFO + "?tab=graph")
        rest_name = page.evaluate("document.querySelector('#detail .frag-h').textContent.trim()")
        ring = page.evaluate(STROKE, ROOT)
        activate(page, f'.node[data-node="{DEPLOY}"]')
        page.wait_for_selector("#detail .clear-pill")
        page.locator("#detail .clear-pill").click()
        page.wait_for_url(lambda u: "node=" not in u)
        for _ in range(40):
            name = page.evaluate("document.querySelector('#detail .frag-h').textContent.trim()")
            if name == rest_name:
                break
            time.sleep(0.1)
        pill = page.locator("#detail .clear-pill").count()
        if not check(engine, name == rest_name and pill == 0 and ring == "3px",
                     f"Clear selection returned to the resting node {rest_name!r}: panel {name!r}, pill {pill}, plain ring {ring}"):
            return False
        page.keyboard.press("Escape")
        time.sleep(0.3)
        still = page.evaluate("document.querySelector('#detail .frag-h').textContent.trim()")
        ring2 = page.evaluate(STROKE, ROOT)
        if not check(engine, still == rest_name and ring2 == "3px" and "node=" not in page.url,
                     f"Escape at the resting node clears nothing: panel {still!r}, ring {ring2}"):
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
