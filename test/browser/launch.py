"""launch.py: launch opm-portal's local session in a real browser.

Usage: python3 launch.py <browser> <start> <landing URL>

<start> is either the file path of the launch page `opm-portal serve --open` writes, opened as
file:// so the launch is a navigation a cross-site document started, or the printed launch URL,
opened directly. The browser must end on the landing URL with the session: the page it shows
must say "signed in", and so must a reload. Prints one line per check and exits 1 on the first
failure.
"""

import sys

from playwright.sync_api import sync_playwright


def main() -> int:
    engine, start, landing = sys.argv[1], sys.argv[2], sys.argv[3]
    if start.startswith("/"):
        start = "file://" + start
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            page = browser.new_page()
            page.goto(start)
            page.wait_for_url(landing, timeout=15000)
            page.wait_for_load_state("load")
            for step in ("after launch", "after reload"):
                if step == "after reload":
                    page.reload()
                body = page.inner_text("body").strip()
                print(f"{engine} {step}: {page.url} -> {body!r}")
                if body != "signed in":
                    return 1
        finally:
            browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
