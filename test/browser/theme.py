"""theme.py: the stored theme and the remembered filters, in a real browser.

Usage: python3 theme.py <browser> <site URL> <context>

The site is the portal's UI over the F1 capture. Checks, each printing one line, exiting 1 on the
first failure:

1. A stored Dark theme is applied before first paint on a light system: by DOMContentLoaded the
   root carries data-theme="dark" and the body is painted with the dark background.
2. A stored Installed filter opens filtered on a full load: /installed becomes
   /installed?kind=package and lists only packages.
3. A boosted navigation restores it too: from the Platform page, the header's Installed link
   requests /installed?kind=package (an htmx request) and pushes that URL.
4. A stale stored value is dropped: health=Bogus&namespace=default opens
   /installed?namespace=default, and a stored query that validates to nothing is removed.
"""

import sys

from playwright.sync_api import sync_playwright

DARK_BG = "rgb(12, 15, 20)"


def main() -> int:
    engine, base, context = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3]
    key = f"opm-portal.filters.installed:{context}"
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            page = browser.new_context(color_scheme="light").new_page()
            page.goto(base + "/")
            page.evaluate("localStorage.setItem('opm-portal.theme', 'dark')")
            page.add_init_script(
                "document.addEventListener('DOMContentLoaded', () => {"
                " window.__theme = document.documentElement.getAttribute('data-theme');"
                " window.__bg = getComputedStyle(document.body).backgroundColor; });"
            )
            page.goto(base + "/")
            theme, bg = page.evaluate("[window.__theme, window.__bg]")
            print(f"{engine}: at DOMContentLoaded data-theme={theme!r} background={bg!r}")
            if theme != "dark" or bg != DARK_BG:
                return 1

            page.evaluate(f"localStorage.setItem({key!r}, 'kind=package')")
            page.goto(base + "/installed")
            print(f"{engine}: full load of /installed opened {page.url}")
            if not page.url.endswith("/installed?kind=package"):
                return 1
            kinds = page.locator("#list .kind").all_text_contents()
            if not kinds or any(k.strip() != "Package" for k in kinds):
                print(f"{engine}: rows {kinds!r}, want packages only")
                return 1

            page.goto(base + "/")
            with page.expect_request(lambda r: "/installed" in r.url and r.headers.get("hx-request") == "true") as req:
                page.locator('#nav a[href="/installed"]').click()
            page.wait_for_url("**/installed?kind=package")
            print(f"{engine}: boosted Installed link requested {req.value.url}, pushed {page.url}")
            if not req.value.url.endswith("/installed?kind=package"):
                return 1

            page.evaluate(f"localStorage.setItem({key!r}, 'health=Bogus&namespace=default')")
            page.goto(base + "/installed")
            print(f"{engine}: stale stored query opened {page.url}")
            if not page.url.endswith("/installed?namespace=default"):
                return 1
            page.evaluate(f"localStorage.setItem({key!r}, 'health=Bogus')")
            page.goto(base + "/installed")
            left = page.evaluate(f"localStorage.getItem({key!r})")
            print(f"{engine}: a stored query of nothing valid opened {page.url}, left {left!r}")
            if not page.url.endswith("/installed") or left is not None:
                return 1
        finally:
            browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
