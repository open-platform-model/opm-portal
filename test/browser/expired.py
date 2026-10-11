"""expired.py: an expired session stops the page's stream, in a real browser.

Usage: python3 expired.py <browser> <page URL>

The page's stream sends its open event and then its expired event. The live indicator must read
"Session expired, reload" and stay so; the server counts the stream requests, so the caller checks
that the browser did not reconnect. Prints one line per check and exits 1 on the first failure.
"""

import sys

from playwright.sync_api import sync_playwright

WANT = "Session expired, reload"


def main() -> int:
    engine, url = sys.argv[1], sys.argv[2]
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            page = browser.new_page()
            page.goto(url)
            live = page.locator("#live .live-text")
            try:
                live.filter(has_text=WANT).wait_for(timeout=15000)
            except Exception:  # noqa: BLE001 - report what the page shows instead
                print(f"{engine}: live indicator {live.text_content()!r}, want {WANT!r}")
                return 1
            # text_content, not inner_text: the indicator is upper-cased by CSS.
            # Past the stream's 300 ms retry several times over: a stream
            # still open would have reconnected and been refused by now.
            page.wait_for_timeout(2500)
            text = live.text_content()
            print(f"{engine}: live indicator {text!r}")
            if text != WANT:
                return 1
        finally:
            browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
