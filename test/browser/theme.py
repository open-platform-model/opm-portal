"""theme.py: the stored theme and the remembered filters, in a real browser.

Usage: python3 theme.py <browser> <site URL> <context>

The site is the portal's UI over the F1 capture. Checks, each printing one line, exiting 1 on the
first failure:

1. A stored Dark theme is applied before first paint on a light system: by DOMContentLoaded, the
   earliest point a script can look, the root carries data-theme="dark" and the body has the dark
   background.
2. A stored Installed filter opens filtered on a full load: /installed becomes
   /installed?kind=package and lists only packages; the document that was replaced was hidden
   (opm-restoring) when its body was parsed, so the unfiltered view was never painted.
3. A boosted navigation restores it too: from the Platform page, the header's Installed link
   requests /installed?kind=package (an htmx request) and pushes that URL.
4. A stale stored value is dropped: health=Bogus&namespace=default opens
   /installed?namespace=default, and a stored query that validates to nothing is removed.
5. A link wins over the remembered filters: with health=Healthy stored, /installed?health=Degraded
   stays as it is and the stored query becomes health=Degraded.
6. Filters stored under another context are not restored.
7. With storage throwing on every call, pages render and the theme menu still switches the theme.
"""

import sys

from playwright.sync_api import sync_playwright

DARK_BG = "rgb(12, 15, 20)"

# Records, for every document, whether it was being replaced and whether its body was hidden.
RECORD = (
    "document.addEventListener('DOMContentLoaded', () => {"
    " const log = JSON.parse(sessionStorage.getItem('rec') || '[]');"
    " log.push({path: location.pathname + location.search,"
    " restoring: document.documentElement.classList.contains('opm-restoring'),"
    " hidden: getComputedStyle(document.body).visibility === 'hidden'});"
    " sessionStorage.setItem('rec', JSON.stringify(log));"
    " window.__theme = document.documentElement.getAttribute('data-theme');"
    " window.__bg = getComputedStyle(document.body).backgroundColor; });"
)

THROWING = (
    "for (const m of ['getItem', 'setItem', 'removeItem']) {"
    " Storage.prototype[m] = function () { throw new Error('blocked'); }; }"
)


def check(engine, ok, line):
    print(f"{engine}: {line}")
    return ok


def run(p, engine, base, context):
    key = f"opm-portal.filters.installed:{context}"
    browser = getattr(p, engine).launch()
    try:
        page = browser.new_context(color_scheme="light").new_page()
        page.add_init_script(RECORD)
        page.goto(base + "/")
        page.evaluate("localStorage.setItem('opm-portal.theme', 'dark')")
        page.goto(base + "/")
        theme, bg = page.evaluate("[window.__theme, window.__bg]")
        if not check(engine, theme == "dark" and bg == DARK_BG, f"at DOMContentLoaded data-theme={theme!r} background={bg!r}"):
            return False

        page.evaluate(f"localStorage.setItem({key!r}, 'kind=package'); sessionStorage.removeItem('rec')")
        page.goto(base + "/installed", wait_until="commit")
        page.wait_for_url("**/installed?kind=package")
        page.wait_for_load_state()
        rec = page.evaluate("JSON.parse(sessionStorage.getItem('rec') || '[]')")
        replaced = [r for r in rec if r["path"] == "/installed"]
        if not check(engine, all(r["restoring"] and r["hidden"] for r in replaced),
                     f"full load opened {page.url}; replaced documents {replaced!r}"):
            return False
        kinds = page.locator("#list .kind").all_text_contents()
        if not check(engine, bool(kinds) and all(k.strip() == "Package" for k in kinds), f"rows {kinds!r}"):
            return False

        page.goto(base + "/")
        with page.expect_request(lambda r: "/installed" in r.url and r.headers.get("hx-request") == "true") as req:
            page.locator('#nav a[href="/installed"]').click()
        page.wait_for_url("**/installed?kind=package")
        if not check(engine, req.value.url.endswith("/installed?kind=package"),
                     f"boosted Installed link requested {req.value.url}, pushed {page.url}"):
            return False

        page.evaluate(f"localStorage.setItem({key!r}, 'health=Bogus&namespace=default')")
        page.goto(base + "/installed", wait_until="commit")
        page.wait_for_url("**/installed?namespace=default")
        check(engine, True, f"stale stored query opened {page.url}")
        page.evaluate(f"localStorage.setItem({key!r}, 'health=Bogus')")
        page.goto(base + "/installed")
        left = page.evaluate(f"localStorage.getItem({key!r})")
        if not check(engine, page.url.endswith("/installed") and left is None,
                     f"a stored query of nothing valid opened {page.url}, left {left!r}"):
            return False

        page.evaluate(f"localStorage.setItem({key!r}, 'health=Healthy')")
        page.goto(base + "/installed?health=Degraded")
        page.wait_for_load_state()
        left = page.evaluate(f"localStorage.getItem({key!r})")
        if not check(engine, page.url.endswith("/installed?health=Degraded") and left == "health=Degraded",
                     f"a link over remembered filters opened {page.url}, stored {left!r}"):
            return False

        page.evaluate(f"localStorage.clear(); localStorage.setItem('opm-portal.filters.installed:another', 'kind=package')")
        page.goto(base + "/installed")
        if not check(engine, page.url.endswith("/installed"), f"filters of another context: opened {page.url}"):
            return False

        blocked = browser.new_context(color_scheme="light").new_page()
        blocked.add_init_script(THROWING)
        blocked.goto(base + "/installed")
        blocked.locator("details.theme > summary").click()
        blocked.locator('[data-theme-choice="dark"]').click()
        theme = blocked.evaluate("document.documentElement.getAttribute('data-theme')")
        rows = blocked.locator("#list tbody tr").count()
        if not check(engine, theme == "dark" and rows > 0, f"storage throwing: {rows} rows, theme {theme!r} after picking Dark"):
            return False
    finally:
        browser.close()
    return True


def main() -> int:
    engine, base, context = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, context) else 1


if __name__ == "__main__":
    sys.exit(main())
