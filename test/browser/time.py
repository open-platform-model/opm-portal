"""time.py: relative times count, in a real browser (add-shell-states-and-cluster-name).

Usage: python3 time.py <browser> <site URL> <server now, RFC 3339> <relative-time.json text>

The site's pages are rendered against a fixed server clock. The browser's clock is Playwright's,
set ten minutes ahead of it and paused before the load, so the script's skew is exact and run_for
moves time by a known amount. Checks, each printing one line, exit 1 on the first failure:

1. The browser clock is ten minutes off the page's meta, and a time set at each age of
   relative-time.json reads the text of its row (the script's table equals the server's).
2. A tick writes the text node of a time whose text changed, and nothing for one whose text is
   already right; it replaces no element.
3. The 30 second timer runs the tick, with no event from the page.
4. A time without a parseable datetime, a time without data-relative and an empty one are left alone.
5. A counting time keeps its width to one digit: tabular numerals, so 9s to 10s widens it by
   no more than a glyph (checked for every step of the first minute).
6. Becoming visible runs the tick (every VISIBLE below is a tab showing again; no timer has run).

With OPM_PORTAL_BROWSER_SHOTS set, a failure saves a screenshot there.
"""

import json
import os
import sys
from datetime import datetime, timedelta, timezone

from playwright.sync_api import sync_playwright

PATH = "/instances/default/podinfo"
OFF_MS = 10 * 60 * 1000

# Puts one time element per row at the ages of the table, as of the script's clock now + lead_ms.
MAKE = """([rows, nowMs, leadMs]) => {
    document.querySelectorAll("#probes").forEach(e => e.remove());
    const host = document.createElement("div");
    host.id = "probes";
    for (const [i, row] of rows.entries()) {
        const t = document.createElement("time");
        t.className = "probe";
        t.setAttribute("data-relative", "");
        t.setAttribute("datetime", new Date(nowMs + leadMs - row.age_seconds * 1000).toISOString());
        t.textContent = "stale " + i;
        host.append(t, " ");
    }
    document.querySelector("main").append(host);
}"""
TEXTS = "() => Array.from(document.querySelectorAll('time.probe'), e => e.textContent)"
VISIBLE = "() => document.dispatchEvent(new Event('visibilitychange'))"


def shoot(page):
    out = os.environ.get("OPM_PORTAL_BROWSER_SHOTS")
    if not out:
        return
    path = os.path.join(out, os.environ.get("OPM_PORTAL_BROWSER_SHOT", "browser") + ".png")
    try:
        page.screenshot(path=path, full_page=True)
        print(f"screenshot: {path}")
    except Exception as err:  # a failed screenshot must not hide the failure itself
        print(f"screenshot failed: {err}")


def run(p, engine, base, served_iso, rows):
    browser = getattr(p, engine).launch()
    page = None
    ok = True

    def check(good, line):
        nonlocal ok
        print(f"{engine}: {'' if good else 'FAIL '}{line}")
        if not good:
            ok = False
            shoot(page)
        return good

    try:
        served_ms = int(datetime.fromisoformat(served_iso.replace("Z", "+00:00")).timestamp() * 1000)
        page = browser.new_context(viewport={"width": 1280, "height": 800}).new_page()
        start = datetime.fromtimestamp(served_ms / 1000, tz=timezone.utc) + timedelta(milliseconds=OFF_MS)
        page.clock.install(time=start)
        page.clock.pause_at(start + timedelta(seconds=1))
        page.goto(base + PATH)
        page.wait_for_selector("main time[data-relative]")
        meta = page.evaluate("Date.parse(document.querySelector('meta[name=opm-portal-now]').content)")
        browser_now = page.evaluate("Date.now()")
        if not check(meta == served_ms and abs((browser_now - meta) - OFF_MS) <= 2000,
                     f"the browser clock is {(browser_now - meta) / 1000:.0f} s ahead of the page's meta"):
            return False

        # 1. The script's text equals the table's, at the exact boundary ages.
        page.evaluate(MAKE, [rows, served_ms, 0])
        page.evaluate(VISIBLE)
        got = page.evaluate(TEXTS)
        want = [r["text"] for r in rows]
        if not check(got == want, "every age of relative-time.json reads its row: "
                     + ("all " + str(len(rows)) if got == want else str([(r["age_seconds"], g, r["text"]) for g, r in zip(got, rows) if g != r["text"]]))):
            return False

        # 2. Only a changed text is written, and only its text node.
        page.evaluate("""() => {
            window.mutations = [];
            const probes = document.getElementById("probes");
            new MutationObserver(list => list.forEach(m => window.mutations.push([m.type, m.target.nodeType, m.target.parentElement && m.target.parentElement.tagName])))
                .observe(probes, { characterData: true, childList: true, subtree: true });
            window.first = document.querySelector("time.probe").firstChild;
            window.firstEl = document.querySelector("time.probe");
        }""")
        page.evaluate(VISIBLE)
        page.wait_for_timeout(50)
        quiet = page.evaluate("window.mutations.length")
        if not check(quiet == 0, f"a tick over times that already read right wrote nothing ({quiet} mutations)"):
            return False
        page.evaluate(MAKE, [rows, served_ms, 60_000])
        page.evaluate("""() => {
            window.mutations = [];
            const probes = document.getElementById("probes");
            new MutationObserver(list => list.forEach(m => window.mutations.push(m.type)))
                .observe(probes, { characterData: true, childList: true, subtree: true });
            window.nodes = Array.from(document.querySelectorAll("time.probe"), e => [e, e.firstChild]);
        }""")
        page.evaluate(VISIBLE)
        page.wait_for_timeout(50)
        info = page.evaluate("""() => ({
            kinds: Array.from(new Set(window.mutations)),
            same: window.nodes.every(([e, n]) => e.isConnected && e.firstChild === n),
        })""")
        if not check(info["kinds"] == ["characterData"] and info["same"],
                     f"a tick that changed every text wrote text nodes only ({info['kinds']}), elements and nodes kept ({info['same']})"):
            return False

        # 3. The 30 second timer runs the tick on its own: one minute of clock, and the texts move on.
        page.evaluate(MAKE, [rows, served_ms, 30_000])
        page.evaluate("document.querySelectorAll('time.probe').forEach(e => e.textContent = 'stale')")
        page.clock.run_for(30_000)
        got = page.evaluate(TEXTS)
        if not check(got == want, "30 s of the page's clock ran the tick with no event from the page"):
            return False

        # 4. An element the script cannot read is left as the server wrote it.
        page.evaluate("""() => {
            const mk = (html) => { const d = document.createElement("div"); d.id = "odd"; d.innerHTML = html; document.querySelector("main").append(d); };
            mk('<time class="odd" data-relative datetime="not a date">kept text</time><time class="odd" datetime="2026-10-21T16:00:00Z">kept too</time><time class="odd" data-relative datetime="2026-10-21T16:00:00Z"></time>');
        }""")
        page.evaluate(VISIBLE)
        odd = page.evaluate("Array.from(document.querySelectorAll('time.odd'), e => e.textContent)")
        if not check(odd == ["kept text", "kept too", ""], f"unreadable and unmarked times are left alone ({odd})"):
            return False

        # 5. A counting time keeps its width to one glyph, for every second of the first minute.
        page.evaluate("""([nowMs]) => {
            const t = document.createElement("time");
            t.id = "ruler"; t.setAttribute("data-relative", ""); t.setAttribute("datetime", new Date(nowMs).toISOString()); t.textContent = "x";
            // In a paragraph, so the time is inline and its box is as wide as its text (a child of main would stretch).
            const host = document.createElement("p");
            host.id = "rulerhost";
            host.append(t);
            document.querySelector("main").prepend(host);
        }""", [served_ms])
        # The page's clock stands 30 s past the load (step 3 ran it).
        widths = {}
        for age in range(0, 120):
            page.evaluate("([nowMs, age]) => document.getElementById('ruler').setAttribute('datetime', new Date(nowMs - age * 1000).toISOString())", [served_ms + 30_000, age])
            page.evaluate(VISIBLE)
            widths[age] = page.evaluate("document.getElementById('ruler').getBoundingClientRect().width")
        glyph = page.evaluate("""() => {
            // A clone of the ruler without its datetime: the tick leaves it alone, and it takes the same type.
            const m = document.getElementById("ruler").cloneNode(false);
            m.removeAttribute("id"); m.removeAttribute("datetime"); m.textContent = "0";
            document.getElementById("rulerhost").append(m);
            const w = m.getBoundingClientRect().width;
            m.remove();
            return w;
        }""")
        # "9s ago" to "10s ago" adds one digit; "59s ago" to "1m ago" drops one; all else is the same.
        worst = max(abs(widths[a + 1] - widths[a]) for a in range(0, 119))
        same = all(abs(widths[a + 1] - widths[a]) < 0.01 for a in range(10, 58))
        if not same:
            print(f"{engine}: widths by age: " + ", ".join(f"{a}:{widths[a]:.2f}" for a in range(0, 70)))
        if not check(worst <= glyph + 0.5 and same,
                     f"over two minutes the width moved by at most {worst:.2f}px (one digit is {glyph:.2f}px), and not at all from 10s to 58s ({same})"):
            return False

        return ok
    except Exception:
        if page is not None:
            shoot(page)
        raise
    finally:
        browser.close()


def main() -> int:
    if len(sys.argv) != 5:
        print(__doc__)
        return 2
    engine, base, served, table = sys.argv[1], sys.argv[2].rstrip("/"), sys.argv[3], sys.argv[4]
    with sync_playwright() as p:
        return 0 if run(p, engine, base, served, json.loads(table)) else 1


if __name__ == "__main__":
    sys.exit(main())
