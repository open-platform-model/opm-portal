"""graph.py: graph interactions on an instance page, in a real browser.

Usage: python3 graph.py <browser> <site URL>

The site is the portal's UI over the F1 capture, under the portal's page policy, which forbids
eval: the script waits with URL predicates and selectors, never page.wait_for_function. Checks, each printing one line, exiting 1 on the
first failure:

1. Clear selection: after a node is selected and the zoom set, "Clear selection" drops the focus
   from the address without a new history entry, keeps the zoom, and undims the graph.
2. A Resources row's name link (stretched over the row) opens the Graph tab spotlighting that node.
3. A group node expands in place and fits the view to its members; Overview collapses it.
4. A live refresh keeps the tab, the focus, the selection and the zoom. The refresh is driven by
   dispatching the stream's upsert event for the page's topic on the stream listener, as the SSE
   extension does.
5. Full screen enters and survives a live refresh; Escape clears the spotlight first, and the next one leaves it.
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
   clear it, the resting node wears the plain ring, and keyboard focus on it shows the focus ring.
10. Fit: the fitted cert-manager graph fills more than half the pane at 1920 px, Fit is pressed after
    load and not after a zoom, and the full screen button swaps its label and is not also a toggle (no `aria-pressed`).
11. Groups: RBAC opens at no less than 60 % with its frame's top visible; opening CRDs closes RBAC;
    Collapse from the frame leaves the group selected with its panel and loads no new page.
12. Full screen: opening and closing a group, and Overview, keep full screen; Escape steps back one
    thing per press (the selection, then the group, then full screen).
13. Ctrl + wheel zoom keeps the point under the cursor within 2 px and never animates; Fit and the
    slider animate (a transition runs, except under reduced motion).
14. A live refresh keeps the open group, the zoom and the selection.
15. Motion: expanding a group moves its members (animations run) and, under reduced motion, runs none.
"""

import json
import sys
import time

from playwright.sync_api import expect, sync_playwright

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
RBAC = "grp:configuration/mi/cert-manager/cert-manager/rbac"
CRDS = "grp:configuration/mi/cert-manager/cert-manager/crds"


def check(engine, ok, line):
    print(f"{engine}: {line}")
    return ok


def panel_is(page, title):
    """Waits until the panel's heading reads title. The panel is never empty (it rests on the
    owner), so a wait for any heading returns before the selected node's panel arrives."""
    expect(page.locator("#detail .frag-h").first).to_have_text(title)


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


def settle(page, expr, tries=60):
    """Polls a page expression until it is true: the group fit scrolls after its zoom animation."""
    for _ in range(tries):
        if page.evaluate(expr):
            return True
        time.sleep(0.1)
    return False


def pane(page):
    return page.evaluate(
        "(() => { const f = document.querySelector('.graph-frame').getBoundingClientRect();"
        " return [f.top, f.bottom, f.left, f.right]; })()")


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
        page.locator("#components a.row-link", has_text="podinfo-podinfo").first.click()
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
        page.locator("[data-overview]").click()
        page.wait_for_url(lambda u: "expand=" not in u)
        if not check(engine, page.locator(".node[data-member-of]").count() == 0, f"Overview returned to {page.url}"):
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
        page.wait_for_selector('[data-graph-full].is-full')
        live_refresh(page, "instance:default/podinfo")
        full = page.evaluate("!!document.fullscreenElement || !!document.querySelector('.graph-full')")
        pressed = page.evaluate("document.querySelector('[data-graph-full]').hasAttribute('aria-pressed')")
        if not check(engine, full and not pressed, f"full screen through a refresh: full {full}, aria-pressed present {pressed}"):
            return False
        page.keyboard.press("Escape")
        page.wait_for_url(lambda u: "focus=" not in u)
        mid = page.evaluate(
            f"[!!document.querySelector('.graph-stage.graph-full'), !!document.querySelector('.node[data-node=\"{DEPLOY}\"][aria-current]')]")
        page.keyboard.press("Escape")
        page.wait_for_selector('[data-graph-full]:not(.is-full)')
        left = page.evaluate("!document.querySelector('.graph-stage.graph-full')")
        if not check(engine, mid == [True, False] and left,
                     f"the first Escape cleared the spotlight and kept full screen ({mid}); the second left full screen ({left})"):
            return False

        # 6. The details panel after a node selection (instance-43).
        page.goto(base + PODINFO + "?tab=graph")
        activate(page, f'.node[data-node="{DEPLOY}"]')
        panel_is(page, "podinfo-podinfo")
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
            panel_is(page, "podinfo-podinfo")
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
        # WebKit scrolls the focused node into view after focusin, so the card is placed again on that
        # scroll: poll until it stands clear of the node (or give up and report where it is).
        for _ in range(40):
            card_left = page.evaluate(
                f"(() => {{ const c = document.querySelector('.node-card:not([hidden])').getBoundingClientRect();"
                f" const n = document.querySelector('.node[data-node=\"{POD}\"]').getBoundingClientRect();"
                " return [c.left, c.right, n.left, n.right]; })()")
            if card_left[1] <= card_left[2] + 1 or card_left[0] >= card_left[3] - 1:
                break
            page.wait_for_timeout(50)
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
        page.locator("#config-components-2 > summary").click()
        page.locator('a.row-link[href*="expand=grp%3Aconfiguration%2Fmi%2Fcert-manager%2Fcert-manager%2Frbac"]').first.click()
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
        # The keyboard's focus ring outranks the resting ring: it is 4 px in the focus colour.
        # Reached by Shift+Tab then Tab, so the engine counts it as keyboard focus (Firefox does
        # not for a script's focus()).
        page.locator(f'.node[data-node="{ROOT}"]').first.focus()
        page.keyboard.press("Shift+Tab")
        page.keyboard.press("Tab")
        visible, focus_ring = page.evaluate(
            "id => { const n = document.querySelector(`.node[data-node=\"${id}\"]`);"
            " return [n.matches(':focus-visible'), getComputedStyle(n.querySelector('.node-ring')).strokeWidth]; }", ROOT)
        if not check(engine, visible and focus_ring == "4px",
                     f"keyboard focus on the resting node shows the focus ring: focus-visible {visible}, ring {focus_ring}"):
            return False
        page.keyboard.press("Escape")

        # 10. Fit fills the pane; Fit is pressed until the user zooms; the full screen label swaps.
        big = browser.new_context(viewport={"width": 1920, "height": 1080}).new_page()
        big.goto(base + CERT + "?tab=graph")
        big.wait_for_selector(".graph-frame")
        filled, scale, pressed = big.evaluate(
            "(() => { const f = document.querySelector('.graph-frame');"
            " const nodes = [...document.querySelectorAll('.graph .node-box')].map(n => n.getBoundingClientRect());"
            " const top = Math.min(...nodes.map(r => r.top)), bottom = Math.max(...nodes.map(r => r.bottom));"
            " return [(bottom - top) / f.clientHeight, document.querySelector('[data-zoom-range]').value,"
            " document.querySelector('[data-zoom=fit]').getAttribute('aria-pressed')]; })()")
        if not check(engine, filled > 0.5 and pressed == "true",
                     f"the fitted cert-manager graph fills {filled:.2f} of the pane at 1920 px (zoom {scale}%), Fit pressed {pressed}"):
            return False
        set_zoom(big, 80)
        after = big.get_attribute("[data-zoom=fit]", "aria-pressed")
        big.locator("[data-zoom=fit]").click()
        back = big.get_attribute("[data-zoom=fit]", "aria-pressed")
        if not check(engine, after == "false" and back == "true" and zoom(big) == scale,
                     f"Fit is not pressed after a zoom ({after}) and is again after Fit ({back}, zoom {zoom(big)}%)"):
            return False
        label = big.get_attribute("[data-graph-full]", "aria-label")
        big.locator("[data-graph-full]").click()
        big.wait_for_selector('[data-graph-full].is-full')
        label2 = big.get_attribute("[data-graph-full]", "aria-label")
        toggle = big.evaluate("document.querySelector('[data-graph-full]').hasAttribute('aria-pressed')")
        big.keyboard.press("Escape")
        big.wait_for_selector('[data-graph-full]:not(.is-full)')
        if not check(engine, label != label2 and label2 == "Exit full screen (Esc)" and not toggle,
                     f"the full screen button swaps its label and is not also a toggle: {label!r} -> {label2!r}, aria-pressed {toggle}"):
            return False
        big.close()

        # 11. Groups: RBAC at no less than 60 % with its frame's top visible; CRDs closes RBAC.
        page = browser.new_context(viewport={"width": 1440, "height": 900}).new_page()
        page.goto(base + CERT + "?tab=graph")
        activate(page, f'.node[data-node="{RBAC}"]')
        page.wait_for_url("**expand=**")
        page.wait_for_selector(".group-frame-box")
        ok = settle(page, "document.querySelector('.graph[data-fit-settled]') !== null")
        shown = settle(page,
                       "(() => { const f = document.querySelector('.graph-frame').getBoundingClientRect();"
                       " const r = document.querySelector('.group-frame').getBoundingClientRect();"
                       " return r.top >= f.top - 1 && r.top < f.bottom; })()")
        pct = int(zoom(page))
        if not check(engine, ok and shown and pct >= 60,
                     f"RBAC opened at {pct}% with its frame's top visible ({shown})"):
            return False
        activate(page, f'.node[data-node="{CRDS}"]')
        page.wait_for_url(lambda u: "crds" in u)
        page.wait_for_selector(f'[data-frame-for="{CRDS}"]')
        frames = page.evaluate("[...document.querySelectorAll('.group-frame-box')].map(f => f.getAttribute('data-frame-for'))")
        if not check(engine, frames == [CRDS] and "rbac" not in page.url,
                     f"opening CRDs closed RBAC: frames {frames}, {page.url}"):
            return False
        for how in ("the keyboard", "a click"):
            page.goto(base + CERT + "?tab=graph&expand=" + RBAC)
            page.evaluate("window.__still = 1")
            if how == "a click":
                page.evaluate("document.querySelector('.group-collapse').dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}))")
            else:
                activate(page, ".group-collapse")
            page.wait_for_url(lambda u: "node=" in u and "expand=" not in u)
            page.wait_for_selector(f'.node[data-node="{RBAC}"][aria-current="true"]')
            name = page.evaluate("document.querySelector('#detail .frag-h').textContent.trim()")
            still = page.evaluate("window.__still")
            if not check(engine, still == 1 and name != "",
                         f"Collapse from the frame with {how} selected the group {name!r} without a new page ({still}), {page.url}"):
                return False

        # 12. Full screen survives opening and closing a group; Escape steps back one thing per press.
        page.goto(base + CERT + "?tab=graph")
        page.evaluate("window.__still = 1")
        page.locator("[data-graph-full]").click()
        page.wait_for_selector('[data-graph-full].is-full')
        activate(page, f'.node[data-node="{RBAC}"]')
        page.wait_for_selector(".group-frame-box")
        page.wait_for_selector('[data-graph-full].is-full')
        in_full = page.evaluate("!!document.querySelector('.graph-stage.graph-full') && window.__still === 1")
        pressed = page.evaluate("document.querySelector('[data-graph-full]').classList.contains('is-full')")
        page.locator("[data-overview]").click()
        page.wait_for_url(lambda u: "expand=" not in u)
        page.wait_for_selector(f'.node[data-node="{RBAC}"]')
        page.wait_for_selector('[data-graph-full].is-full')
        in_full2 = page.evaluate("!!document.querySelector('.graph-stage.graph-full') && window.__still === 1")
        if not check(engine, in_full and pressed and in_full2,
                     f"full screen kept across opening a group ({in_full}, {pressed}) and Overview ({in_full2})"):
            return False
        activate(page, f'.node[data-node="{RBAC}"]')
        page.wait_for_selector(".graph[data-fit-settled]")
        member = page.evaluate("document.querySelector('.node[data-member-of]').getAttribute('data-node')")
        activate(page, f'.node[data-node="{member}"]')
        page.wait_for_url(lambda u: "node=" in u)
        page.wait_for_selector(".node-card:not([hidden])")
        page.keyboard.press("Escape")
        page.wait_for_selector(".node-card:not([hidden])", state="detached")
        card_first = "node=" in page.url
        page.keyboard.press("Escape")
        page.wait_for_url(lambda u: "node=" not in u)
        step1 = page.evaluate("[!!document.querySelector('.group-frame-box'), !!document.querySelector('.graph-stage.graph-full')]")
        page.keyboard.press("Escape")
        page.wait_for_url(lambda u: "expand=" not in u)
        page.wait_for_selector(f'.node[data-node="{RBAC}"]')
        page.wait_for_selector('[data-graph-full].is-full')
        step2 = page.evaluate("[!!document.querySelector('.group-frame-box'), !!document.querySelector('.graph-stage.graph-full')]")
        page.keyboard.press("Escape")
        page.wait_for_selector('[data-graph-full]:not(.is-full)')
        step3 = page.evaluate("!!document.querySelector('.graph-stage.graph-full')")
        if not check(engine, card_first and step1 == [True, True] and step2 == [False, True] and step3 is False,
                     f"Escape hid the card ({card_first}), cleared the selection ({step1}), closed the group ({step2}), then left full screen ({not step3})"):
            return False

        # 13. Wheel zoom keeps the point under the cursor and never animates; Fit and the slider do.
        page.goto(base + CERT + "?tab=graph")
        set_zoom(page, 100)
        page.wait_for_timeout(500)
        anchor = page.evaluate(
            """() => {
              const f = document.querySelector('.graph-frame');
              const fr = f.getBoundingClientRect();
              const x = fr.left + fr.width / 2, y = fr.top + fr.height / 2;
              const svg = document.querySelector('.graph-frame > svg');
              const vb = svg.viewBox.baseVal;
              const r0 = svg.getBoundingClientRect();
              const s0 = parseFloat(svg.getAttribute('width')) / vb.width;
              const px = (x - r0.left) / s0, py = (y - r0.top) / s0;
              svg.dispatchEvent(new WheelEvent('wheel', {ctrlKey: true, deltaY: -300, clientX: x, clientY: y, bubbles: true, cancelable: true}));
              const r1 = svg.getBoundingClientRect();
              const s1 = parseFloat(svg.getAttribute('width')) / vb.width;
              return [r1.left + px * s1 - x, r1.top + py * s1 - y, s0, s1, document.querySelector('.graph').classList.contains('zoom-anim')];
            }""")
        if not check(engine, abs(anchor[0]) <= 2 and abs(anchor[1]) <= 2 and anchor[3] > anchor[2] and not anchor[4],
                     f"wheel zoom {anchor[2]:.2f} -> {anchor[3]:.2f} kept the point under the cursor within 2 px (dx {anchor[0]:.1f}, dy {anchor[1]:.1f}) without the animation class ({anchor[4]})"):
            return False
        page.locator("[data-zoom=fit]").click()
        running = page.evaluate("document.getAnimations().length")
        ctx_red = browser.new_context(viewport={"width": 1440, "height": 900}, reduced_motion="reduce").new_page()
        ctx_red.goto(base + CERT + "?tab=graph")
        set_zoom(ctx_red, 140)
        ctx_red.locator("[data-zoom=fit]").click()
        red = ctx_red.evaluate("document.getAnimations().length")
        if not check(engine, running > 0 and red == 0,
                     f"Fit animates the zoom ({running} animations) and runs none under reduced motion ({red})"):
            return False

        # 14. A live refresh keeps the open group, the zoom and the selection.
        page.goto(base + CERT + f"?tab=graph&expand={RBAC}")
        page.wait_for_selector(".group-frame-box")
        member = page.evaluate("document.querySelector('.node[data-member-of]').getAttribute('data-node')")
        activate(page, f'.node[data-node="{member}"]')
        page.wait_for_url(lambda u: "node=" in u)
        page.wait_for_selector(f'.node[data-node="{member}"][aria-current]')
        set_zoom(page, 90)
        url = page.url
        live_refresh(page, "instance:cert-manager/cert-manager")
        state = page.evaluate(
            "[location.href, document.querySelector('[data-zoom-range]').value, document.querySelectorAll('.group-frame-box').length,"
            f" !!document.querySelector('.node[data-node=\"{member}\"][aria-current]')]")
        if not check(engine, state == [url, "90", 1, True], f"after a live refresh with a group open: {json.dumps(state)}"):
            return False

        # 15. Motion: members leave the group's node (animations run); none under reduced motion.
        page.goto(base + CERT + "?tab=graph")
        activate(page, f'.node[data-node="{RBAC}"]')
        page.wait_for_selector(".group-frame-box")
        moving = settle(page, "[...document.querySelectorAll('.node[data-member-of]')].some(n => n.getAnimations().length > 0)", 10)
        page.wait_for_timeout(900)
        left = page.evaluate("document.querySelectorAll('.node[data-member-of]').length")
        quiet = browser.new_context(viewport={"width": 1440, "height": 900}, reduced_motion="reduce").new_page()
        quiet.goto(base + CERT + "?tab=graph")
        activate(quiet, f'.node[data-node="{RBAC}"]')
        quiet.wait_for_selector(".group-frame-box")
        seen = 0
        for _ in range(8):
            seen = max(seen, quiet.evaluate("document.getAnimations().length"))
            seen = max(seen, quiet.evaluate("[...document.querySelectorAll('.node[data-member-of]')].filter(n => n.getAnimations().length > 0).length"))
            time.sleep(0.05)
        if not check(engine, moving and left > 0 and seen == 0,
                     f"expanding a group ran animations ({moving}) and left {left} members; under reduced motion {seen} ran"):
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
