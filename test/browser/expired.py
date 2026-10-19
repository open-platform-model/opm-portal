"""expired.py: an expired session stops the page's stream and says so, in a real browser.

Usage: python3 expired.py <browser> <page URL>

The page's stream sends its open event and then its expired event. Checks, each printing one line,
exit 1 on the first failure:

1. The live mark reads "Session expired, reload" and stays so: the server counts the stream requests
   (GET <page URL>streams), and past the stream's 300 ms retry several times over it still reads one.
2. The banner is shown in the header. It reads "Session expired. Showing data read" and a time, then a
   period. The time is a time element of the time partial's form: the instant in datetime, the same
   instant in UTC in title ("YYYY-MM-DD HH:MM:SS UTC"), and relative words. The instant is the last
   message the stream delivered (its open event), a few seconds ago at most, not the page's load later.
3. The banner has no role and no aria-live, and the live mark is the only element whose text changed
   among the page's live regions.
4. The Reload link loads the current address as a new page with a new stream: the page opens a second
   stream (the caller counts two), and shows the expired banner again.
"""

import re
import sys
from datetime import datetime, timezone
from urllib.parse import urljoin

from playwright.sync_api import sync_playwright

WANT = "Session expired, reload"

STATE = """() => {
    const b = document.getElementById("region-banner");
    const t = b.querySelector("time");
    return {
        hidden: b.hidden,
        text: b.querySelector(".rb-text").textContent.replace(/\\s+/g, " ").trim(),
        datetime: t.getAttribute("datetime"), title: t.getAttribute("title"), rel: t.textContent,
        role: b.getAttribute("role"), live: b.getAttribute("aria-live"),
        reload: b.querySelector(".rb-reload").href,
        inHeader: b.parentElement.id === "masthead",
    };
}"""

WATCH = """() => {
    window.__changed = new Set();
    const live = document.querySelectorAll('[role="status"], [role="alert"], [aria-live]:not([aria-live="off"])');
    for (const el of live) {
        new MutationObserver(() => window.__changed.add(el.id || el.tagName)).observe(el, {subtree: true, childList: true, characterData: true});
    }
}"""


def main() -> int:
    engine, url = sys.argv[1], sys.argv[2]
    with sync_playwright() as p:
        browser = getattr(p, engine).launch()
        try:
            page = browser.new_page()
            page.goto(url)
            page.evaluate(WATCH)
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
            n = page.request.get(urljoin(url, "/streams")).text()
            print(f"{engine}: streams opened before the reload: {n}")
            if n != "1":
                return 1

            s = page.evaluate(STATE)
            print(f"{engine}: banner {s}")
            if s["hidden"] or not s["inHeader"]:
                print(f"{engine}: FAIL the banner is not shown in the header")
                return 1
            m = re.fullmatch(r"Session expired\. Showing data read (just now|\d+[smhd] ago)\.", s["text"])
            if not m:
                print(f"{engine}: FAIL the banner reads {s['text']!r}")
                return 1
            at = datetime.strptime(s["datetime"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
            want_title = at.strftime("%Y-%m-%d %H:%M:%S UTC")
            age = (datetime.now(timezone.utc) - at).total_seconds()
            if s["title"] != want_title or not (0 <= age < 30):
                print(f"{engine}: FAIL title {s['title']!r} want {want_title!r}, age {age:.0f}s (want under 30)")
                return 1
            if s["role"] is not None or s["live"] is not None:
                print(f"{engine}: FAIL the banner has role={s['role']!r} aria-live={s['live']!r}")
                return 1
            changed = page.evaluate("() => Array.from(window.__changed)")
            print(f"{engine}: live regions that changed: {changed}")
            if changed != ["live"]:
                print(f"{engine}: FAIL the live mark is not the only live region that changed")
                return 1

            # The Reload link is a full page load: a new page, a new stream.
            page.locator("#region-banner .rb-reload").click()
            page.wait_for_load_state("load")
            page.locator("#live .live-text").filter(has_text=WANT).wait_for(timeout=15000)
            page.wait_for_timeout(1500)
            s = page.evaluate(STATE)
            print(f"{engine}: after Reload the banner reads {s['text']!r}")
            if s["hidden"] or not s["text"].startswith("Session expired. Showing data read"):
                print(f"{engine}: FAIL the reloaded page does not show the expired banner")
                return 1
        finally:
            browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
