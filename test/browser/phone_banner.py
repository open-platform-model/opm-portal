"""phone_banner.py: the header with a banner and two long names holds 360 px and its height, in a real browser.

Usage: python3 phone_banner.py <browser> <site URL> <path>...

The site's kubeconfig names a 90 character context and a different 90 character cluster entry, with no
space in either (an EKS ARN is of this kind). The viewport is 360 by 640. For each path, with the banner
hidden and then shown (the script dispatches htmx:sseError on the body, which runs the page's own
loss code; the real stream stays open, so the banner stays):

1. The pill shows both names, the second with the word "cluster", stacked at this width, each on one line
   cut with an ellipsis inside the pill (nowrap, text-overflow ellipsis, the text wider than its box), the
   full values in the pill's title and in its text, and the page's scrollWidth is at most 360.
2. The banner is inside the viewport (its right edge at most 360) and its Reload link is at least 44 px
   high.
3. The header with these names is as tall as with short names (the script swaps the two names for short
   ones and measures again): the difference is at most 2 px, with and without the banner.
4. The header's bottom is the html element's scroll-padding-top less 8 px, and a link brought into view
   by Shift+Tab, and a heading brought into view by a fragment link, sit at or below the header's
   bottom.
5. Again with the root font size at 200 percent (a stand-in for a 200 percent text size, which the banner
   and the "cluster" label follow because they are sized in rem): the header must be at least 8 px taller
   than at 100 percent, so the leg can fail when the header stops responding to the text size, and
   checks 1, 2 and 4 hold with it: nothing focused is hidden behind the header (WCAG 2.2 2.4.11).

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import os
import sys

from playwright.sync_api import sync_playwright

WIDTH = 360
SPACER = """() => {
    const main = document.querySelector("main");
    const wrap = document.createElement("div");
    const a = document.createElement("a");
    a.id = "probe-a"; a.href = "#probe-h"; a.textContent = "probe a";
    const h = document.createElement("h2");
    h.id = "probe-h"; h.tabIndex = -1; h.textContent = "probe heading";
    const gap = document.createElement("div");
    gap.style.height = "5000px";
    const b = document.createElement("a");
    b.id = "probe-b"; b.href = "#probe-h"; b.textContent = "probe b";
    const tail = document.createElement("div");
    tail.style.height = "2000px";
    wrap.append(a, h, gap, b, tail);
    main.append(wrap);
}"""
MEASURE = """() => {
    const de = document.documentElement;
    const pill = document.querySelector(".conn-cluster").getBoundingClientRect();
    const entry = document.querySelector(".conn-entry");
    const names = [...document.querySelectorAll(".conn-cluster .mono")];
    const cs = names.map(n => getComputedStyle(n));
    const b = document.getElementById("region-banner");
    const br = b.getBoundingClientRect();
    return {
        scroll: Math.max(de.scrollWidth, document.body.scrollWidth),
        pillRight: pill.right, hasEntry: !!entry,
        entryStacked: entry ? entry.getBoundingClientRect().top >= names[0].getBoundingClientRect().bottom - 1 : null,
        oneLine: names.length === 2 && names.every((n, i) => cs[i].whiteSpace === "nowrap" && cs[i].textOverflow === "ellipsis" &&
            cs[i].overflow === "hidden" && n.scrollWidth > n.clientWidth &&
            n.getBoundingClientRect().height < 2 * parseFloat(cs[i].fontSize) * 1.5 - 1 &&
            n.getBoundingClientRect().right <= pill.right + 0.5),
        title: document.querySelector(".conn-cluster").title,
        pillText: document.querySelector(".conn-cluster").textContent,
        entryText: entry ? entry.textContent.replace(/\\s+/g, " ").trim().length : 0,
        bannerShown: !b.hidden, bannerRight: br.right,
        reload: b.hidden ? 0 : document.querySelector(".rb-reload").getBoundingClientRect().height,
        header: document.getElementById("masthead").getBoundingClientRect().bottom,
        pad: parseFloat(getComputedStyle(de).scrollPaddingTop),
    };
}"""


SWAP = """names => {
    const els = [...document.querySelectorAll(".conn-cluster .mono")];
    const old = els.map(e => e.textContent);
    els.forEach((e, i) => { e.textContent = names[i]; });
    return old;
}"""


def shoot(page):
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def main() -> int:
    engine, base, paths = sys.argv[1], sys.argv[2], sys.argv[3:]
    ok = True
    at100 = {}  # (path, banner) -> the header's bottom at 100 percent
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            for path in paths:
                for banner in (False, True):
                    for big in (False, True):
                        page = browser.new_context(viewport={"width": WIDTH, "height": 640}).new_page()
                        page.goto(base + path)
                        page.wait_for_selector("#live")
                        page.wait_for_function("document.querySelector('#live .live-text').textContent === 'Live'", timeout=15000)
                        label = f"{path} {'with' if banner else 'without'} the banner{' at 200% root size' if big else ''}"
                        if banner:
                            page.evaluate("() => document.body.dispatchEvent(new Event('htmx:sseError'))")
                            page.wait_for_function("!document.getElementById('region-banner').hidden")
                        if big:
                            page.evaluate("() => { document.documentElement.style.fontSize = '200%'; }")
                        page.evaluate(SPACER)
                        page.wait_for_timeout(300)
                        m = page.evaluate(MEASURE)
                        good = m["hasEntry"] and m["entryStacked"] and m["entryText"] > 90 and m["pillRight"] <= WIDTH
                        if not big:
                            good = good and m["scroll"] <= WIDTH
                        print(f"{engine}: {'' if good else 'FAIL '}{label}: both names in the pill (right edge {m['pillRight']:.0f}, entry stacked {m['entryStacked']}), scroll width {m['scroll']}")
                        ok = ok and good
                        long_ctx, long_entry = "c" * 90, "e" * 90
                        good = (m["oneLine"] and long_ctx in m["title"] and long_entry in m["title"]
                                and long_ctx in m["pillText"] and long_entry in m["pillText"])
                        print(f"{engine}: {'' if good else 'FAIL '}{label}: each name one line cut with an ellipsis {m['oneLine']}, full values in the title and the text")
                        ok = ok and good
                        if not big:
                            at100[(path, banner)] = m["header"]
                            old = page.evaluate(SWAP, ["ctx", "ent"])
                            page.wait_for_timeout(200)
                            short = page.evaluate("() => document.getElementById('masthead').getBoundingClientRect().bottom")
                            page.evaluate(SWAP, old)
                            page.wait_for_timeout(200)
                            good = abs(short - m["header"]) <= 2
                            print(f"{engine}: {'' if good else 'FAIL '}{label}: header {m['header']:.1f} px with 90 character names, {short:.1f} px with short names")
                            ok = ok and good
                        else:
                            grew = m["header"] - at100[(path, banner)]
                            good = grew >= 8
                            print(f"{engine}: {'' if good else 'FAIL '}{label}: header grew by {grew:.1f} px ({at100[(path, banner)]:.1f} to {m['header']:.1f})")
                            ok = ok and good
                        if banner:
                            good = m["bannerShown"] and m["bannerRight"] <= WIDTH and m["reload"] >= 44
                            print(f"{engine}: {'' if good else 'FAIL '}{label}: banner right edge {m['bannerRight']:.0f}, Reload {m['reload']:.0f} px high")
                            ok = ok and good
                        good = abs(m["pad"] - (m["header"] + 8)) < 1
                        print(f"{engine}: {'' if good else 'FAIL '}{label}: scroll-padding-top {m['pad']:.1f} = header bottom {m['header']:.1f} + 8")
                        ok = ok and good
                        page.evaluate("() => { document.getElementById('probe-b').focus(); window.scrollTo(0, 3000); }")
                        page.wait_for_timeout(100)
                        page.keyboard.press("Shift+Tab")
                        page.wait_for_timeout(200)
                        pos = page.evaluate("() => ({id: document.activeElement.id, top: document.activeElement.getBoundingClientRect().top, hb: document.getElementById('masthead').getBoundingClientRect().bottom})")
                        good = pos["id"] == "probe-a" and pos["top"] >= pos["hb"] - 0.5
                        print(f"{engine}: {'' if good else 'FAIL '}{label}: Shift+Tab to {pos['id']!r} at {pos['top']:.1f}, header bottom {pos['hb']:.1f}")
                        ok = ok and good
                        page.evaluate("() => window.scrollTo(0, 3000)")
                        page.evaluate("() => document.getElementById('probe-a').click()")
                        page.wait_for_timeout(200)
                        pos = page.evaluate("() => ({top: document.getElementById('probe-h').getBoundingClientRect().top, hb: document.getElementById('masthead').getBoundingClientRect().bottom})")
                        good = pos["top"] >= pos["hb"] - 0.5
                        print(f"{engine}: {'' if good else 'FAIL '}{label}: a fragment link puts the heading at {pos['top']:.1f}, header bottom {pos['hb']:.1f}")
                        ok = ok and good
                        if not ok:
                            shoot(page)
                        page.context.close()
        finally:
            browser.close()
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
