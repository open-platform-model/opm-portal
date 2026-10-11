// OPM Portal page script. Served from the portal under a policy that
// allows no inline script and no eval. It keeps the tab's one stream on
// the topics the page needs, re-renders the regions a change touches, fills
// log panes as text, opens graph node panels, and pans and zooms graphs.
// Untrusted text (log lines) is only ever assigned as textContent.
(function () {
  "use strict";

  var body = document.body;
  var listener = document.getElementById("stream-events");
  var streamBase = body.getAttribute("data-stream-base") || "";

  // ---- Address after a launch ----
  // The launch answers with the landing page itself, so it reads without
  // script. Firefox and WebKit treat a reload of that page as part of the
  // cross-site navigation an --open launch started and withhold the
  // SameSite=Strict session cookie, so the page moves once, from its own
  // origin, to its own address; the spent token leaves the history.
  var main = document.getElementById("main");
  if (location.pathname === "/launch" && main) {
    location.replace(main.getAttribute("data-canonical") || "/");
    return;
  }
  window.setTimeout(function () { body.classList.add("loaded"); }, 900);

  // ---- Theme ----
  // prefs.js applied the stored theme before first paint; the menu changes
  // it for this page and keeps it in the browser when storage allows.
  var prefs = window.opmPortalPrefs || null;

  var themeNames = { light: "Light", dark: "Dark", system: "System" };

  function markTheme() {
    var cur = document.documentElement.getAttribute("data-theme") || "system";
    document.querySelectorAll("[data-theme-choice]").forEach(function (b) {
      b.setAttribute("aria-pressed", b.getAttribute("data-theme-choice") === cur ? "true" : "false");
    });
    document.querySelectorAll("[data-theme-summary]").forEach(function (s) {
      s.setAttribute("aria-label", "Theme: " + (themeNames[cur] || "System"));
    });
  }

  // A click outside closes the theme menu; Escape closes it too (the one
  // Escape handler, under Graph view).
  document.addEventListener("click", function (evt) {
    var open = document.querySelector("details.theme[open]");
    if (open && !open.contains(evt.target)) {
      open.open = false;
    }
  });
  markTheme();

  document.addEventListener("click", function (evt) {
    var b = evt.target.closest && evt.target.closest("[data-theme-choice]");
    if (!b) {
      return;
    }
    var choice = b.getAttribute("data-theme-choice");
    if (prefs) {
      prefs.setTheme(choice);
    } else if (choice === "light" || choice === "dark") {
      document.documentElement.setAttribute("data-theme", choice);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
    markTheme();
    var menu = b.closest("details");
    if (menu) {
      menu.open = false;
    }
  });

  // ---- Header ----
  // The header slims once a sentinel 48 px down the page leaves the
  // viewport; the transition is off under reduced motion (portal.css).
  var sentinel = document.getElementById("top-sentinel");
  var masthead = document.getElementById("masthead");
  if (sentinel && masthead && "IntersectionObserver" in window) {
    new IntersectionObserver(function (entries) {
      masthead.classList.toggle("compact", !entries[entries.length - 1].isIntersecting);
    }).observe(sentinel);
  }

  // The header's height is a custom property on the root, so the page keeps
  // clear of it (WCAG 2.2 2.4.11): portal.css reads it for scroll-padding-top
  // and for the sticky details panel's offset. The border box is read, not
  // the content box, which leaves out the padding. It is set through the
  // CSSOM (a style attribute would break the page policy), at load, when
  // the banner comes or goes, and whenever the header's size changes (the
  // slimmed header, a wrapped header on a phone, a larger text size).
  function syncMastheadHeight() {
    if (masthead) {
      document.documentElement.style.setProperty("--masthead-h", masthead.getBoundingClientRect().height + "px");
    }
  }
  syncMastheadHeight();
  if (masthead && "ResizeObserver" in window) {
    new ResizeObserver(syncMastheadHeight).observe(masthead);
  }

  // ---- Relative times ----
  // The server writes each time as "4m ago" and nothing updates it, so the
  // script counts: every 30 seconds, and once when the tab shows again, it
  // rewrites the text of each time[data-relative] from its datetime. The
  // table is relative() in internal/ui/view.go; internal/ui/testdata/
  // relative-time.json holds both copies to the same rows. Ages run against
  // the server's clock, read once from the page's meta (the browser clock
  // may be minutes off, and a clock offset does not change in a session).
  // Only the text node is written, and only when it differs, so a tick
  // that changes nothing touches nothing, and tabular numerals keep the
  // width still. A page without the meta, or an element without a
  // parseable datetime, is left as the server wrote it.
  var clockSkew = null;
  (function () {
    var meta = document.querySelector('meta[name="opm-portal-now"]');
    var served = meta ? Date.parse(meta.getAttribute("content") || "") : NaN;
    if (!isNaN(served)) {
      clockSkew = served - Date.now();
    }
  })();

  function relativeText(ageMs) {
    if (ageMs < 0) {
      return "just now";
    }
    var s = Math.floor(ageMs / 1000);
    if (s < 60) {
      return s + "s ago";
    }
    var m = Math.floor(s / 60);
    if (m < 60) {
      return m + "m ago";
    }
    var h = Math.floor(s / 3600);
    if (h < 48) {
      return h + "h ago";
    }
    return Math.floor(s / 86400) + "d ago";
  }

  function tickTimes() {
    if (clockSkew === null) {
      return;
    }
    var now = Date.now() + clockSkew;
    var els = document.querySelectorAll("time[data-relative]");
    for (var i = 0; i < els.length; i++) {
      var at = Date.parse(els[i].getAttribute("datetime") || "");
      var node = els[i].firstChild;
      if (isNaN(at) || !node || node.nodeType !== 3) {
        continue;
      }
      var text = relativeText(now - at);
      if (node.nodeValue !== text) {
        node.nodeValue = text;
      }
    }
  }
  window.setInterval(tickTimes, 30000);
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") {
      tickTimes();
    }
  });

  // ---- Loading ----
  // htmx requests are the only ones counted (the stream's refresh, a log
  // or topic POST use fetch). The registry is a Map keyed by the request,
  // not a counter: one request may fire more than one end event (an
  // error answer fires responseError, then afterRequest), and a second
  // delete of the same key does nothing. A request that runs for a second
  // sets body.is-loading (the bar on the header); while one of them
  // targets #detail, that panel is aria-busy and says "Loading..." in
  // its first line. The bar ends when the last request has ended. Only
  // classes, attributes and text are set, never style.
  var openRequests = new Map();
  // Topics whose refresh found a request open into #detail: the refresh left
  // the panel alone, and re-marks them when that request has ended.
  var heldTopics = new Set();
  var loadingTimer = 0;
  var loadingShown = false;

  function paintLoading() {
    body.classList.toggle("is-loading", loadingShown);
    var detail = document.getElementById("detail");
    if (!detail) {
      return;
    }
    var busy = false;
    if (loadingShown) {
      openRequests.forEach(function (target) {
        if (target === detail || (target && target.id === "detail")) {
          busy = true;
        }
      });
    }
    var line = detail.querySelector(":scope > .detail-loading");
    if (busy) {
      detail.setAttribute("aria-busy", "true");
      if (!line) {
        line = document.createElement("p");
        line.className = "detail-loading";
        line.textContent = "Loading...";
        detail.insertBefore(line, detail.firstChild);
      }
    } else {
      detail.removeAttribute("aria-busy");
      if (line) {
        line.remove();
      }
    }
  }

  // detailBusy says a request into the details panel is open.
  function detailBusy() {
    var busy = false;
    openRequests.forEach(function (target) {
      if (target && target.id === "detail") {
        busy = true;
      }
    });
    return busy;
  }

  function requestStarted(evt) {
    var xhr = evt.detail && evt.detail.xhr;
    if (!xhr) {
      return;
    }
    openRequests.set(xhr, evt.detail.target || null);
    if (!loadingTimer && !loadingShown) {
      loadingTimer = window.setTimeout(function () {
        loadingTimer = 0;
        loadingShown = openRequests.size > 0;
        paintLoading();
      }, 1000);
    } else if (loadingShown) {
      paintLoading();
    }
  }

  function requestEnded(evt) {
    var xhr = evt.detail && evt.detail.xhr;
    if (!xhr) {
      return;
    }
    openRequests.delete(xhr);
    if (openRequests.size === 0) {
      window.clearTimeout(loadingTimer);
      loadingTimer = 0;
      loadingShown = false;
    }
    paintLoading();
    if (heldTopics.size > 0 && !detailBusy()) {
      var held = heldTopics;
      heldTopics = new Set();
      held.forEach(changed);
    }
  }

  document.addEventListener("htmx:beforeRequest", requestStarted);
  ["htmx:afterRequest", "htmx:sendError", "htmx:responseError", "htmx:timeout"].forEach(function (name) {
    document.addEventListener(name, requestEnded);
  });

  // ---- Remembered filters ----
  // A boosted link to a remembered list view whose URL carries none of the
  // view's filters is requested, and pushed, with the filters the browser
  // remembers for it; htmx requests and pushes the path left in
  // htmx:configRequest (design.md, the boosted filter restore spike). Only
  // a boosted link is rewritten: a form submit, a chip or "Clear filters"
  // (data-filters-explicit), a region refresh, a panel fetch and the
  // topic change never are.
  document.addEventListener("htmx:configRequest", function (evt) {
    var d = evt.detail;
    if (d && d.verb === "get" && d.elt && d.elt.matches && d.elt.matches("form[data-filters]")) {
      dropEmpty(d);
      return;
    }
    if (!prefs || !d || !d.boosted || d.verb !== "get" || !d.elt || d.elt.tagName !== "A" ||
        d.elt.hasAttribute("data-filters-explicit")) {
      return;
    }
    var u;
    try {
      u = new URL(d.path, location.href);
    } catch (e) {
      return;
    }
    if (u.origin !== location.origin) {
      return;
    }
    var next = prefs.restored(u.pathname, u.search);
    if (next !== null) {
      d.path = next + u.hash;
    }
  });

  // dropEmpty leaves the fields a filter form submits empty out of its
  // request, so the address shows only the filters that apply.
  function dropEmpty(d) {
    var empty = [];
    if (d.formData && d.formData.forEach) {
      d.formData.forEach(function (v, k) {
        if (v === "") {
          empty.push(k);
        }
      });
    }
    empty.forEach(function (k) {
      try {
        d.formData.delete(k);
        delete d.parameters[k];
      } catch (e) {
        // The request keeps the empty field; it filters nothing.
      }
    });
  }

  // After each render, a remembered view stores the filters it shows;
  // a document prefs.js is replacing stores nothing.
  function rememberFilters() {
    if (prefs && !prefs.restoring) {
      prefs.remember(location.pathname, location.search);
    }
  }
  rememberFilters();
  document.addEventListener("htmx:pushedIntoHistory", rememberFilters);
  // A filter form replaces its history entry, so Back leaves the filtered
  // view instead of walking back through every keystroke.
  document.addEventListener("htmx:replacedInHistory", rememberFilters);
  document.addEventListener("htmx:historyRestore", function () {
    rememberFilters();
    markTheme();
    setLiveNote("");
  });

  // "Clear filters" forgets the view first, so nothing restores them.
  document.addEventListener("click", function (evt) {
    var a = evt.target.closest && evt.target.closest("a[data-clear-filters]");
    if (a && prefs) {
      prefs.forget(a.getAttribute("data-clear-filters"));
    }
  }, true);

  // ---- Filter forms ----
  // A filter form's selects apply at once; without script its button does.
  document.addEventListener("change", function (evt) {
    var sel = evt.target;
    if (sel && sel.matches && sel.matches("form[data-filters] select") && sel.form && sel.form.requestSubmit) {
      sel.form.requestSubmit();
    }
  });

  // A search or text field of a form[data-filters] applies after a pause in
  // typing. The pending timer names its form by the form's data-filters
  // value and looks it up when it fires, so a swap that replaced the form
  // never leaves it submitting a detached one. A submit clears it, so Enter
  // submits once, and so does a request the form did not make (a link
  // followed while a key was pending).
  var FILTER_PAUSE = 300;
  var typingTimer = 0;
  var typingForm = "";

  function armFilter(form) {
    window.clearTimeout(typingTimer);
    typingForm = form.getAttribute("data-filters") || "";
    typingTimer = window.setTimeout(function () {
      typingTimer = 0;
      var live = typingForm ? document.querySelector('form[data-filters="' + CSS.escape(typingForm) + '"]') : null;
      if (live && live.requestSubmit) {
        live.requestSubmit();
      }
    }, FILTER_PAUSE);
  }

  function disarmFilter() {
    window.clearTimeout(typingTimer);
    typingTimer = 0;
  }

  function filterInput(el, selector) {
    return !!(el && el.matches && el.form && el.matches(selector) && el.form.matches("form[data-filters]"));
  }

  document.addEventListener("input", function (evt) {
    if (filterInput(evt.target, "input[type=search]")) {
      armFilter(evt.target.form);
    }
  });
  document.addEventListener("change", function (evt) {
    if (filterInput(evt.target, "input:not([type=search]):not([type=hidden]):not([type=checkbox]):not([type=radio])")) {
      armFilter(evt.target.form);
    }
  });
  document.addEventListener("submit", function (evt) {
    if (evt.target && evt.target.matches && evt.target.matches("form[data-filters]")) {
      disarmFilter();
    }
  }, true);

  // The field the user is typing in is recorded just before a page swap
  // replaces it. htmx puts focus and caret back on a field with an id, but
  // the swap renders the value the server received, so a key typed while
  // the request was in flight would be lost: after the page settles the live
  // value and caret are restored, and the pause starts again so the newer
  // text is applied next.
  var typed = null;
  document.addEventListener("htmx:beforeSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    var a = document.activeElement;
    typed = null;
    if (t && t.id === "main" && a && a.id && t.contains(a) && filterInput(a, "input:not([type=hidden])")) {
      try {
        typed = { id: a.id, value: a.value, start: a.selectionStart, end: a.selectionEnd };
      } catch (e) {
        typed = null;
      }
    }
  });

  // isMainSettle says an htmx:afterSettle is the one of the swapped page. A
  // boosted request settles twice, once for #nav (hx-select-oob) and once for
  // #main, with the same detail; what must happen once per request acts on
  // the #main event.
  function isMainSettle(evt) {
    return !!(evt.target && evt.target.id === "main");
  }

  document.addEventListener("htmx:afterSettle", function (evt) {
    if (!isMainSettle(evt)) {
      return;
    }
    var rec = typed;
    typed = null;
    var el = rec && document.getElementById(rec.id);
    if (!el || !el.form || el.value === rec.value) {
      return;
    }
    el.value = rec.value;
    if (document.activeElement !== el) {
      el.focus({ preventScroll: true });
    }
    try {
      el.setSelectionRange(rec.start, rec.end);
    } catch (e) {
      // The field takes no selection; the value is back.
    }
    armFilter(el.form);
  });

  document.addEventListener("htmx:beforeRequest", function (evt) {
    var elt = evt.detail && evt.detail.elt;
    var own = !!(elt && elt.matches && elt.matches("form[data-filters]"));
    var chip = !!(elt && elt.matches && elt.matches("a.fchip, a.fclear") && elt.closest("form[data-filters]"));
    if (typingTimer && !own) {
      disarmFilter();
    }
    // A filter change started this request: a field, a chip's remove link
    // or "Clear all" of a filter form. Its settled count is announced.
    var form = own ? elt : (chip ? elt.closest("form[data-filters]") : null);
    filterChange = form ? form.getAttribute("data-filters") || "" : null;
  });

  // The count of a filter change that has applied is announced once through
  // the header's polite status region, in its hidden .live-note, so no live
  // region is added to the page. The region is not atomic, so a change of
  // the stream's state (.live-text) presents that text only. A page move
  // empties the note, which belongs to the page it was written for.
  // It holds the data-filters name of the form that made the request, or
  // null when no filter form did.
  var filterChange = null;

  function setLiveNote(text) {
    var note = document.querySelector("#live .live-note");
    if (note && note.textContent !== text) {
      note.textContent = text;
    }
  }

  document.addEventListener("htmx:afterSettle", function (evt) {
    if (!isMainSettle(evt)) {
      return;
    }
    if (filterChange === null) {
      setLiveNote("");
      return;
    }
    // The count is the one inside the form that made the request: a page
    // with two filter forms (the Platform page) has two counts. A form that
    // shows none (a list the caller may not read) announces nothing, never
    // another form's count.
    var form = filterChange ? document.querySelector('form[data-filters="' + CSS.escape(filterChange) + '"]') : null;
    var count = form && form.querySelector(".shown");
    filterChange = null;
    setLiveNote(count ? (count.textContent || "").replace(/\s+/g, " ").trim() : "");
  });

  // ---- Stream topics ----
  var streamID = "";
  var expired = false;
  var opened = words(body.getAttribute("data-opened-topics"));
  var subscribed = new Set(opened);
  var primed = new Set();
  var logTopics = new Set();
  var pending = null;

  function words(s) {
    return (s || "").split(/\s+/).filter(Boolean);
  }

  function pageTopics() {
    var m = document.getElementById("main");
    var want = new Set(m ? words(m.getAttribute("data-topics")) : []);
    logTopics.forEach(function (t) { want.add(t); });
    return want;
  }

  function post(add, remove) {
    if (!streamID || (add.length === 0 && remove.length === 0)) {
      return Promise.resolve();
    }
    return fetch(streamBase + "/" + encodeURIComponent(streamID) + "/topics", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ add: add, remove: remove })
    }).then(function (res) {
      if (!res.ok && !expired) {
        postFailed = true;
        setStream("lost", "Topics refused");
      } else if (res.ok && postFailed) {
        // The topics reached the server again: what the page missed while
        // they did not is read back before the banner goes.
        postFailed = false;
        if (lost && !expired && connected) {
          beginResync();
        }
      }
    }).catch(function () {
      if (!expired) {
        postFailed = true;
        setStream("lost", "Offline");
      }
    });
  }

  // sync moves the stream to the topics the page wants now.
  function sync() {
    if (pending) {
      window.clearTimeout(pending);
    }
    pending = window.setTimeout(function () {
      pending = null;
      var want = pageTopics();
      var add = [], remove = [];
      want.forEach(function (t) { if (!subscribed.has(t)) { add.push(t); } });
      subscribed.forEach(function (t) { if (!want.has(t)) { remove.push(t); primed.delete(t); } });
      subscribed = want;
      post(add, remove);
    }, 50);
  }

  function setLive(ok, text) {
    body.classList.toggle("is-live", ok);
    body.classList.toggle("is-offline", !ok);
    var el = document.querySelector("#live .live-text");
    if (el) {
      el.textContent = text;
    }
  }

  // ---- The stream's state and the banner ----
  // One function sets the live mark and the banner, so they cannot
  // disagree. The mark is the one announcer (#live is a polite status
  // region); the banner has no role and no live region of its own, and
  // holds the same words with the time of the data on screen.
  //
  // lastRead is that time, in the browser's clock: the page's load, then the
  // last message the stream delivered (a heartbeat counts) while it is
  // healthy. It freezes at the first sign of a loss and moves again only
  // when a resync has re-rendered the followed regions, so the banner never
  // says "read just now" over regions that were drawn before the outage.
  var banner = document.getElementById("region-banner");
  var lastRead = Date.now();
  var lost = false;
  var resyncing = false;
  var resyncEpoch = 0;
  var everOpened = false;
  var connected = true;
  var postFailed = false;

  // state: "live" | "lost" | "expired" | "signedout". word is the live
  // mark's text ("Reconnecting"); it also ends a lost banner.
  function setStream(state, word) {
    lost = state !== "live";
    setLive(state === "live", word);
    showBanner(state, word);
  }

  function pad2(n) {
    return (n < 10 ? "0" : "") + n;
  }

  function showBanner(state, word) {
    if (!banner) {
      return;
    }
    if (state === "live") {
      banner.hidden = true;
      banner.removeAttribute("data-state");
    } else {
      var lead = banner.querySelector(".rb-lead");
      var tail = banner.querySelector(".rb-word");
      var when = banner.querySelector("time");
      if (lead) {
        lead.textContent = state === "expired" ? "Session expired. " : (state === "signedout" ? "Signed out. " : "");
      }
      if (tail) {
        tail.textContent = state === "lost" ? " " + word + "." : "";
      }
      if (when) {
        // The time partial's form: the instant in datetime, the same instant
        // in UTC in title, and the relative words, which the tick keeps true.
        var skew = clockSkew === null ? 0 : clockSkew;
        var at = new Date(Math.floor((lastRead + skew) / 1000) * 1000);
        when.setAttribute("datetime", at.toISOString().replace(".000Z", "Z"));
        when.setAttribute("title", at.getUTCFullYear() + "-" + pad2(at.getUTCMonth() + 1) + "-" + pad2(at.getUTCDate()) +
          " " + pad2(at.getUTCHours()) + ":" + pad2(at.getUTCMinutes()) + ":" + pad2(at.getUTCSeconds()) + " UTC");
        when.textContent = relativeText(Date.now() + skew - at.getTime());
      }
      banner.setAttribute("data-state", state);
      banner.hidden = false;
    }
    syncMastheadHeight();
  }

  // streamMessage records the time of a message the healthy stream delivered.
  function streamMessage() {
    if (!lost && !expired) {
      lastRead = Date.now();
    }
  }

  // The resync: after a loss, every followed region is read again from one
  // fetch of the page, and the mark and the banner are true again only when
  // that fetch has been answered 200 and the regions are replaced.
  function markAllDirty() {
    pageTopics().forEach(function (t) { changed(t); });
  }

  function settleLive() {
    resyncing = false;
    lastRead = Date.now();
    setStream("live", "Live");
  }

  function beginResync() {
    resyncing = true;
    resyncEpoch++;
    var any = false;
    pageTopics().forEach(function (t) {
      if (following(t).some(function (el) { return el.id; })) {
        any = true;
      }
    });
    if (!any) {
      // Nothing follows a topic: there is nothing to read again.
      settleLive();
      return;
    }
    markAllDirty();
  }

  // A resync that failed is tried again on the next stream event of any
  // kind, once the failed fetch has settled and nothing else waits.
  function retryResync() {
    if (resyncing && !inflight && !refreshTimer && dirty.size === 0) {
      markAllDirty();
    }
  }

  function parse(evt) {
    var msg = evt.detail;
    if (!msg || typeof msg.data !== "string") {
      return null;
    }
    try {
      return JSON.parse(msg.data);
    } catch (e) {
      return null;
    }
  }

  function following(topic) {
    var out = [];
    document.querySelectorAll("[data-follow]").forEach(function (el) {
      if (words(el.getAttribute("data-follow")).indexOf(topic) >= 0) {
        out.push(el);
      }
    });
    return out;
  }

  // changed marks topic dirty. 400 ms after the first mark the page is
  // fetched once and every region following a dirty topic is replaced by
  // the region of the same id from that one document, so the regions of a
  // refresh come from one render. One fetch runs at a time: marks that
  // arrive meanwhile wait for it to settle, so an older render never lands
  // over a newer one. Regions carry no htmx attributes, so links inside
  // them inherit nothing from them.
  var dirty = new Set();
  var refreshTimer = null;
  var inflight = false;
  function changed(topic) {
    dirty.add(topic);
    schedule();
  }

  function schedule() {
    if (!refreshTimer && !inflight && dirty.size > 0) {
      refreshTimer = window.setTimeout(refresh, 400);
    }
  }

  function refresh() {
    refreshTimer = null;
    var topics = dirty;
    dirty = new Set();
    var regions = [];
    topics.forEach(function (t) {
      following(t).forEach(function (el) {
        if (el.id && regions.indexOf(el) < 0) {
          regions.push(el);
        }
      });
    });
    // Only the first fetch that starts after the stream's open event is the
    // resync; one already in flight at that event reads the old render.
    var isResync = resyncing;
    var epoch = resyncEpoch;
    markViewOld(topics);
    if (regions.length === 0) {
      if (isResync && connected && !expired) {
        settleLive();
      }
      return;
    }
    inflight = true;
    var url = location.pathname + location.search;
    fetch(url, { credentials: "same-origin", headers: { "Accept": "text/html" } }).then(function (res) {
      if (res.status === 401) {
        // The session is gone; a fetch answered so is not tried again.
        resyncing = false;
        if (!expired) {
          setStream("signedout", "Signed out");
        }
        return null;
      }
      if (isResync && res.status !== 200) {
        throw new Error("status " + res.status);
      }
      return res.text();
    }).then(function (text) {
      if (text === null) {
        return;
      }
      if (url !== location.pathname + location.search) {
        // The page moved on while this render was read: what changed
        // still has to reach the page now open.
        topics.forEach(function (t) { dirty.add(t); });
        return;
      }
      var doc = new DOMParser().parseFromString(text, "text/html");
      var views = [];
      var focused = focusKey(regions);
      regions.forEach(function (el) {
        var next = el.id && doc.getElementById(el.id);
        if (!next || !el.isConnected) {
          return;
        }
        if (el.id === "detail" && detailBusy()) {
          // htmx resolved its target when the request started; replacing the
          // panel under it would drop the response. Try again when it ends.
          words(el.getAttribute("data-follow")).forEach(function (t) { heldTopics.add(t); });
          return;
        }
        keepOpen(el, next);
        keepViews(el, views);
        var fresh = document.importNode(next, true);
        var shown = fresh;
        if (mergeLogs(el, fresh)) {
          shown = el;
        } else {
          el.querySelectorAll("details.log").forEach(unfollow);
          el.replaceWith(fresh);
        }
        if (typeof htmx !== "undefined") {
          htmx.process(shown);
        }
        shown.classList.add("refreshed");
        window.setTimeout(function () { shown.classList.remove("refreshed"); }, 900);
      });
      syncDetailFollow();
      restoreGraphs();
      restoreViews(views);
      refocus(focused);
      if (isResync && epoch === resyncEpoch && connected && !expired) {
        settleLive();
      }
    }).catch(function () {
      if (!expired) {
        resyncing = true;
        setStream("lost", "Offline");
      }
    }).then(function () {
      inflight = false;
      schedule();
    });
  }

  // The details panel follows the page while it holds a node panel: #detail
  // carries data-topics always, and data-follow (the same topics) exactly while
  // it holds no YAML or Events view. A view is a fragment the address does not
  // hold, so a refresh does not replace it; markViewOld shows its note instead.
  function syncDetailFollow() {
    var d = document.getElementById("detail");
    if (!d) {
      return;
    }
    var topics = d.getAttribute("data-topics");
    if (topics && !d.querySelector("[data-view]")) {
      d.setAttribute("data-follow", topics);
    } else {
      d.removeAttribute("data-follow");
    }
  }

  document.addEventListener("htmx:afterSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (t && t.id === "detail") {
      syncDetailFollow();
    }
  });

  // markViewOld shows the note of a YAML or Events view in the panel once a
  // topic the panel follows has changed. Nothing else in the view changes.
  function markViewOld(topics) {
    var d = document.getElementById("detail");
    var note = d && d.querySelector("[data-view] > .stale-note[hidden]");
    if (!note) {
      return;
    }
    var mine = words(d.getAttribute("data-topics"));
    var hit = false;
    topics.forEach(function (t) {
      if (mine.indexOf(t) >= 0) {
        hit = true;
      }
    });
    if (hit) {
      note.hidden = false;
    }
  }

  // keepOpen carries the open state of details groups with an id from the
  // region on the page to its replacement.
  function keepOpen(from, to) {
    if (from.matches("details[id]") && to.matches("details[id]") && from.id === to.id) {
      to.open = from.open;
    }
    from.querySelectorAll("details[id]").forEach(function (d) {
      var twin = to.querySelector("#" + CSS.escape(d.id));
      if (twin) {
        twin.open = d.open;
      }
    });
  }

  // focusKey names the focused element when a refreshed region holds it:
  // by id, or a graph node by its graph and node ids; refocus focuses its
  // twin in the fresh render, so keyboard focus survives a refresh.
  function focusKey(regions) {
    var a = document.activeElement;
    if (!a || a === document.body || !regions.some(function (r) { return r.contains(a); })) {
      return null;
    }
    var g = a.closest && a.closest(".graph[data-graph]");
    return {
      id: a.id || "",
      graph: g ? g.getAttribute("data-graph") : "",
      node: a.getAttribute("data-node") || "",
      key: (a.getAttribute && a.getAttribute("data-focus-key")) || ""
    };
  }

  function refocus(key) {
    if (!key) {
      return;
    }
    var el = null;
    if (key.node && key.graph) {
      el = document.querySelector('.graph[data-graph="' + CSS.escape(key.graph) + '"] .node[data-node="' + CSS.escape(key.node) + '"]');
    } else if (key.id) {
      el = document.getElementById(key.id);
    } else if (key.key) {
      el = document.querySelector('[data-focus-key="' + CSS.escape(key.key) + '"]');
    }
    if (el && el.focus) {
      el.focus({ preventScroll: true });
    }
  }

  // keepViews records where each graph frame in a region is scrolled to,
  // and restoreViews scrolls the replacement frames back there, after
  // restoreGraphs has sized them.
  function keepViews(from, out) {
    from.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      var f = frameOf(g);
      if (f) {
        out.push({ id: g.getAttribute("data-graph"), left: f.scrollLeft, top: f.scrollTop });
      }
    });
  }

  function restoreViews(views) {
    views.forEach(function (v) {
      var g = document.querySelector('.graph[data-graph="' + CSS.escape(v.id) + '"]');
      var f = g && frameOf(g);
      if (f) {
        f.scrollLeft = v.left;
        f.scrollTop = v.top;
      }
    });
  }

  // mergeLogs refreshes a region holding log panes in place, from its
  // fresh render: a kept pane is never detached, because a detached element
  // loses its scroll offset, its tail and keyboard focus. A pane the fresh
  // render still lists stays as it is, with its text, open state and
  // stream; a Pod new since the last render gets the fresh, closed pane,
  // appended to the list. An open pane whose Pod is gone stays, marked
  // gone, so its last lines can still be read; a closed one is removed and
  // stops following. The rest of the region, around the list, is replaced
  // from the fresh render. It answers false, changing nothing, when either
  // render has no pane list as a direct child.
  function mergeLogs(el, fresh) {
    var list = el.querySelector(":scope > .logs");
    var next = fresh.querySelector(":scope > .logs");
    if (!list || !next) {
      return false;
    }
    list.querySelectorAll("details.log").forEach(function (d) {
      var twin = d.id ? next.querySelector("#" + CSS.escape(d.id)) : null;
      if (twin) {
        d.classList.remove("log-gone");
        twin.remove();
      } else if (d.open) {
        d.classList.add("log-gone");
      } else {
        unfollow(d);
        d.remove();
      }
    });
    next.querySelectorAll("details.log").forEach(function (d) {
      list.appendChild(d);
    });
    Array.from(el.childNodes).forEach(function (n) {
      if (n !== list) {
        el.removeChild(n);
      }
    });
    var before = true;
    Array.from(fresh.childNodes).forEach(function (n) {
      if (n === next) {
        before = false;
      } else if (before) {
        el.insertBefore(n, list);
      } else {
        el.appendChild(n);
      }
    });
    Array.from(el.attributes).forEach(function (a) {
      if (!fresh.hasAttribute(a.name)) {
        el.removeAttribute(a.name);
      }
    });
    Array.from(fresh.attributes).forEach(function (a) {
      el.setAttribute(a.name, a.value);
    });
    return true;
  }

  // unfollow stops the stream a pane follows, for a pane leaving the page.
  function unfollow(d) {
    var f = d.getAttribute("data-following");
    if (f) {
      logTopics.delete(f);
      sync();
    }
  }

  if (listener) {
    // The stream's own open event carries the stream id; EventSource's
    // connection-level open event, which shares the name, carries no data.
    listener.addEventListener("sse:open", function (evt) {
      var data = parse(evt);
      if (!data || !data.stream) {
        return;
      }
      streamID = data.stream;
      connected = true;
      streamMessage();
      var reopened = lost && everOpened;
      everOpened = true;
      if (!reopened) {
        // The first open of a page, or a loss before it: the page was drawn
        // a moment ago and nothing was missed.
        postFailed = false;
        setStream("live", "Live");
      }
      var want = pageTopics();
      var remove = [];
      opened.forEach(function (t) { if (!want.has(t)) { remove.push(t); } });
      subscribed.forEach(function (t) { if (!want.has(t) && remove.indexOf(t) < 0) { remove.push(t); } });
      subscribed = want;
      primed.clear();
      post(Array.from(want), remove);
      if (reopened) {
        // The mark keeps the word of the loss and the banner stays, until
        // the resync is answered.
        beginResync();
      }
    });

    // Every message of a healthy stream is the time of the data on screen;
    // after a loss it also retries a resync that failed. The expired event
    // is not one: it ends the stream.
    ["sse:snapshot", "sse:upsert", "sse:delete", "sse:k8sevent", "sse:log", "sse:logend", "sse:closed", "sse:heartbeat"].forEach(function (name) {
      listener.addEventListener(name, function () {
        streamMessage();
        retryResync();
      });
    });

    ["sse:snapshot", "sse:upsert", "sse:delete", "sse:k8sevent"].forEach(function (name) {
      listener.addEventListener(name, function (evt) {
        var data = parse(evt);
        if (!data || !data.topic) {
          return;
        }
        if (data.topic.indexOf("log:") === 0) {
          if (name === "sse:snapshot") {
            logSnapshot(data.topic, data.items || []);
          }
          return;
        }
        if (name === "sse:snapshot" && !primed.has(data.topic)) {
          // The first snapshot of a topic is what the page already shows.
          primed.add(data.topic);
          return;
        }
        changed(data.topic);
      });
    });

    listener.addEventListener("sse:log", function (evt) {
      var data = parse(evt);
      if (data && data.topic) {
        logLines(data.topic, [data.item], false);
      }
    });
    listener.addEventListener("sse:logend", function (evt) {
      var data = parse(evt);
      if (data && data.topic) {
        logLines(data.topic, [data.item], false);
        logState(data.topic, "ended");
      }
    });
    listener.addEventListener("sse:closed", function (evt) {
      var data = parse(evt);
      if (!data || !data.topic) {
        return;
      }
      subscribed.delete(data.topic);
      primed.delete(data.topic);
      following(data.topic).forEach(function (el) {
        el.classList.add("topic-closed");
        el.setAttribute("title", "No longer live: " + (data.code || "closed"));
      });
      logState(data.topic, "closed: " + (data.code || "closed"));
    });

    // The session ended. The extension closes the stream on this event
    // (sse-close), so it does not reconnect into a refusal; the page keeps
    // what it shows and says how to go on.
    listener.addEventListener("sse:expired", function () {
      streamID = "";
      expired = true;
      connected = false;
      resyncing = false;
      setStream("expired", "Session expired, reload");
    });
  }

  body.addEventListener("htmx:sseError", function () {
    if (!expired) {
      connected = false;
      resyncing = false;
      setStream("lost", "Reconnecting");
    }
  });
  // The connection opened. After a loss it is not yet true: the mark and the
  // banner wait for the resync.
  body.addEventListener("htmx:sseOpen", function () {
    connected = true;
    if (!lost) {
      setLive(true, "Live");
    }
  });

  // A boosted navigation swaps #main: move the stream to the new page.
  document.addEventListener("htmx:afterSettle", function () {
    sync();
    keepFull();
    restoreGraphs();
    rememberFilters();
  });

  // ---- Logs ----
  var maxLogLines = 2000;

  function paneFor(topic) {
    var found = null;
    document.querySelectorAll("details.log").forEach(function (d) {
      if (d.getAttribute("data-following") === topic) {
        found = d;
      }
    });
    return found;
  }

  function logState(topic, text) {
    var d = paneFor(topic);
    if (d) {
      var s = d.querySelector(".log-state");
      if (s) {
        s.textContent = text;
      }
    }
  }

  function lineText(m) {
    if (!m) {
      return "";
    }
    switch (m.type) {
      case "line":
        return (m.text || "") + (m.cut ? "  [cut " + m.cut + " bytes]" : "");
      case "marker":
        return "[" + (m.marker || "marker") + (m.dropped ? ": " + m.dropped + " lines dropped" : "") + "]";
      case "end":
        return "[end of stream" + (m.reason ? ": " + m.reason : "") + "]";
      default:
        return m.text ? m.text : "[" + (m.type || "message") + "]";
    }
  }

  function logLines(topic, items, replace) {
    var d = paneFor(topic);
    if (!d) {
      return;
    }
    var pre = d.querySelector(".log-pane");
    if (!pre) {
      return;
    }
    var atBottom = pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 8;
    if (replace) {
      pre.textContent = "";
    }
    items.forEach(function (m) {
      var line = document.createElement("span");
      if (m && m.type !== "line") {
        line.className = "mark";
      }
      line.textContent = lineText(m) + "\n";
      pre.appendChild(line);
    });
    while (pre.childNodes.length > maxLogLines) {
      pre.removeChild(pre.firstChild);
    }
    if (atBottom) {
      pre.scrollTop = pre.scrollHeight;
    }
  }

  function logSnapshot(topic, items) {
    logLines(topic, items, true);
    logState(topic, "following");
  }

  function followLog(d) {
    var prev = d.querySelector("input[data-log-previous]");
    var topic = prev && prev.checked ? prev.getAttribute("data-log-previous") : d.getAttribute("data-log-topic");
    var old = d.getAttribute("data-following");
    if (old) {
      logTopics.delete(old);
    }
    if (d.open) {
      d.setAttribute("data-following", topic);
      logTopics.add(topic);
      logState(topic, "connecting");
    } else {
      d.removeAttribute("data-following");
    }
    sync();
  }

  document.addEventListener("toggle", function (evt) {
    var d = evt.target;
    if (d && d.matches && d.matches("details.log")) {
      followLog(d);
    }
  }, true);
  // followOpenLogs follows every open pane that does not follow yet: a pane the
  // server renders open may fire its toggle event before this script listens,
  // so the pane is followed from what is open now, at load and after every swap.
  function followOpenLogs() {
    document.querySelectorAll("details.log[open]").forEach(function (d) {
      if (!d.hasAttribute("data-following")) {
        followLog(d);
      }
    });
  }
  followOpenLogs();
  document.addEventListener("htmx:afterSettle", followOpenLogs);
  document.addEventListener("change", function (evt) {
    var c = evt.target;
    if (c && c.matches && c.matches("input[data-log-previous]")) {
      var d = c.closest("details.log");
      if (d && d.open) {
        followLog(d);
      }
    }
  });
  // Panes swapped away with their page stop following.
  var mainSwapped = false;
  var tabClicked = false;
  // A swap a tab link started puts focus on the new current tab; any other
  // puts it on the page.
  document.addEventListener("click", function (evt) {
    tabClicked = !!(evt.target.closest && evt.target.closest(".tabs a"));
  }, true);
  document.addEventListener("htmx:beforeSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (t && t.id === "main") {
      logTopics.clear();
      selected.clear();
      zooms.clear();
      hovered = null;
      mainSwapped = true;
    }
  });
  // After a page swap, focus moves to the current tab, or to the page, so
  // keyboard users are not left on a detached element.
  document.addEventListener("htmx:afterSettle", function (evt) {
    if (!mainSwapped || !evt.target || evt.target.id !== "main") {
      return;
    }
    mainSwapped = false;
    var m = document.getElementById("main");
    var tab = tabClicked && m && m.querySelector(".tabs a[aria-current]");
    tabClicked = false;
    // htmx put focus back on a field with an id inside the new page (a
    // filter form being used): leave it there.
    var active = document.activeElement;
    if (m && active && active !== m && active !== document.body && m.contains(active)) {
      return;
    }
    var target = tab || m;
    if (target && target.focus) {
      target.focus({ preventScroll: true });
    }
  });

  // ---- Graph nodes ----
  // Two states, as on the canvas. A selection (node=) shows the node's panel
  // and a ring and dims nothing. A spotlight (focus=) selects too, adds a
  // halo, centres the node and dims everything outside its chain: its
  // ancestors and its descendants, two directed walks, never an undirected
  // closure. Hover and keyboard focus dim the same chain while they last.
  // selected maps a graph id to its selected node, spot to its spotlit one;
  // a render that replaces the graph is the truth for both (the address
  // carries them), and centred remembers which focus value was centred.
  var selected = new Map();
  var spot = new Map();
  var centred = new Map();

  // chainOf is id, its ancestors (edges walked backward) and its
  // descendants (edges walked forward).
  function chainOf(g, id) {
    var back = Object.create(null), fwd = Object.create(null);
    g.querySelectorAll("path[data-from]").forEach(function (e) {
      var f = e.getAttribute("data-from"), t = e.getAttribute("data-to");
      (back[t] = back[t] || []).push(f);
      (fwd[f] = fwd[f] || []).push(t);
    });
    var out = new Set([id]);
    [back, fwd].forEach(function (next) {
      var seen = new Set([id]), stack = [id];
      while (stack.length) {
        var n = stack.pop();
        (next[n] || []).forEach(function (m) {
          if (!seen.has(m)) {
            seen.add(m);
            out.add(m);
            stack.push(m);
          }
        });
      }
    });
    return out;
  }

  // dimBy gives cls to every node outside lit and every edge with an end
  // outside it; no lit set clears the class.
  function dimBy(g, lit, cls) {
    g.querySelectorAll("path[data-from]").forEach(function (e) {
      e.classList.toggle(cls, !!lit && !(lit.has(e.getAttribute("data-from")) && lit.has(e.getAttribute("data-to"))));
    });
    g.querySelectorAll(".node").forEach(function (n) {
      n.classList.toggle(cls, !!lit && !lit.has(n.getAttribute("data-node")));
    });
  }

  // markSelected rings the selected node and, for a spotlight, adds the
  // halo and dims what lies outside the chain.
  function markSelected(g) {
    var gid = g.getAttribute("data-graph");
    var want = selected.get(gid) || "";
    var hot = spot.get(gid) || "";
    dimBy(g, hot ? chainOf(g, hot) : null, "dim");
    g.querySelectorAll(".node").forEach(function (n) {
      var id = n.getAttribute("data-node");
      if (want && id === want) {
        n.setAttribute("aria-current", "true");
      } else {
        n.removeAttribute("aria-current");
      }
      if (hot && id === hot) {
        n.setAttribute("data-spotlight", "");
      } else {
        n.removeAttribute("data-spotlight");
      }
    });
    g.classList.toggle("has-selection", !!want);
    var clear = g.querySelector("[data-clear-selection]");
    if (clear) {
      clear.hidden = !want;
    }
  }

  // setParams writes the selection into a URL: the node, the focus, or
  // neither. The server ranks focus above node for the panel and the
  // spotlight, so the one that is not meant stays out of the address.
  function setParams(u, node, focus) {
    [["node", node], ["focus", focus]].forEach(function (kv) {
      if (kv[1]) {
        u.searchParams.set(kv[0], kv[1]);
      } else {
        u.searchParams.delete(kv[0]);
      }
    });
  }

  // setSelectionParams keeps node and focus in the address, so a refresh, a
  // reload and a shared link show the same selection or spotlight.
  function setSelectionParams(node, focus) {
    var u = new URL(location.href);
    setParams(u, node, focus);
    history.replaceState(history.state, "", u.pathname + u.search + u.hash);
    // The tab links keep the selection (portal:D9): the server wrote them for the
    // address the page was opened at, so they follow the selection as it changes.
    document.querySelectorAll(".owner-tabs a").forEach(function (a) {
      var t = new URL(a.getAttribute("href"), location.href);
      setParams(t, node, focus);
      a.setAttribute("href", t.pathname + t.search);
      // A boosted link keeps the path it was processed with, so a new href
      // reaches a click only after htmx processes the link again.
      if (typeof htmx !== "undefined") {
        htmx.process(a);
      }
    });
  }

  // resetDetail brings the panel back to the node the page rests on: the
  // graph names that node's panel in data-resting-panel, and a page with no
  // graph names it in data-root-panel on the panel itself. A page that names
  // none says what to do.
  function resetDetail(g) {
    var target = document.getElementById("detail");
    if (!target) {
      return;
    }
    var url = (g && g.getAttribute("data-resting-panel")) || target.getAttribute("data-root-panel");
    if (url && typeof htmx !== "undefined") {
      htmx.ajax("GET", url, { source: g || target, target: "#detail", swap: "innerHTML", select: "unset" });
      return;
    }
    var p = document.createElement("p");
    p.className = "muted";
    p.textContent = "Select a graph node, an object's YAML or its events to see them here.";
    target.replaceChildren(p);
    syncDetailFollow();
  }

  function clearSelection(g) {
    var gid = g.getAttribute("data-graph");
    selected.delete(gid);
    spot.delete(gid);
    markSelected(g);
    setSelectionParams("", "");
    resetDetail(g);
  }

  function intoView(el) {
    var r = el.getBoundingClientRect();
    if (r.top < 0 || r.top > window.innerHeight - 80) {
      el.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  }

  // centreSpot scrolls the frame so the spotlit node is in its middle, once
  // per focus value: smooth unless the user asks for reduced motion.
  function centreSpot(g) {
    var gid = g.getAttribute("data-graph");
    var hot = spot.get(gid);
    var frame = frameOf(g);
    var svg = g.querySelector(".graph-frame > svg");
    if (!hot || centred.get(gid) === hot || !frame || !svg || !frame.clientWidth) {
      return;
    }
    var box = g.querySelector('.node[data-node="' + CSS.escape(hot) + '"] .node-box');
    if (!box) {
      return;
    }
    centred.set(gid, hot);
    var s = parseFloat(svg.getAttribute("width")) / svg.viewBox.baseVal.width;
    var cx = (box.x.baseVal.value + box.width.baseVal.value / 2) * s;
    var cy = (box.y.baseVal.value + box.height.baseVal.value / 2) * s;
    var reduced = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    frame.scrollTo({
      left: Math.max(0, cx - frame.clientWidth / 2),
      top: Math.max(0, cy - frame.clientHeight / 2),
      behavior: reduced ? "auto" : "smooth"
    });
  }

  function openNode(a) {
    var panel = a.getAttribute("data-panel");
    var target = document.getElementById("detail");
    if (!panel || !target || typeof htmx === "undefined") {
      return false;
    }
    var g = a.closest(".graph[data-graph]");
    if (g) {
      var gid = g.getAttribute("data-graph");
      selected.set(gid, a.getAttribute("data-node"));
      spot.delete(gid);
      markSelected(g);
      setSelectionParams(a.getAttribute("data-node"), "");
    }
    // select "unset": the node sits inside #app, whose hx-select="#main" the
    // request would inherit, and the panel response holds no #main.
    htmx.ajax("GET", panel, { source: a, target: "#detail", swap: "innerHTML", select: "unset" }).then(function () {
      intoView(target);
    });
    return true;
  }

  // The Logs tab opened at a pane (a Pod row's Logs link from Resources)
  // opens that pane.
  function openHashedPane() {
    if (!location.hash) {
      return;
    }
    var d = document.getElementById(decodeURIComponent(location.hash.slice(1)));
    if (d && d.matches("details.log") && !d.open) {
      d.open = true;
      d.scrollIntoView({ block: "start" });
    }
  }
  openHashedPane();
  document.addEventListener("htmx:afterSettle", openHashedPane);

  // A group node expands in place: the page is loaded with the group in
  // its expand parameter and the view fitted to the group's members. The
  // Collapse pill, Overview, Expand all, Collapse all and the grouping
  // switch go the same way: an ajax swap of #main that does not scroll to
  // the top, keeps full screen, and (for a group node) starts the members
  // at the node they come from.
  var fullKeep = false;
  var motion = null;

  function reducedMotion() {
    return !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  }

  function expandGroup(a) {
    if (typeof htmx === "undefined") {
      return false;
    }
    var href = a.getAttribute("href");
    fullKeep = anyFull();
    motion = null;
    var box = a.hasAttribute && a.hasAttribute("data-group") ? a.querySelector(".node-box") : null;
    if (box) {
      motion = { group: a.getAttribute("data-node"), x: box.x.baseVal.value, y: box.y.baseVal.value };
    }
    htmx.ajax("GET", href, { source: document.getElementById("main"), target: "#main", select: "#main", swap: "outerHTML", push: href });
    return true;
  }

  var VIEW_LINKS = ".graph .group-collapse, .graph [data-overview], .graph [data-expand-all], .graph [data-collapse-all], .graph [data-group-switch]";

  document.addEventListener("click", function (evt) {
    var clear = evt.target.closest && evt.target.closest("[data-clear-selection]");
    if (clear) {
      // The panel's Clear selection sits outside the graph: find the page's.
      var cg = clear.closest(".graph[data-graph]") || document.querySelector(".graph[data-graph]");
      if (cg) {
        evt.preventDefault();
        clearSelection(cg);
      }
      return;
    }
    var vl = evt.target.closest && evt.target.closest(VIEW_LINKS);
    if (vl) {
      if (expandGroup(vl)) {
        evt.preventDefault();
      }
      return;
    }
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (!a) {
      // A click on the empty graph clears the selection; a drag does not.
      var empty = evt.target.closest && evt.target.closest(".graph-frame");
      var eg = empty && empty.closest(".graph[data-graph]");
      if (eg && !moved && selected.has(eg.getAttribute("data-graph")) && !evt.target.closest("a")) {
        clearSelection(eg);
      }
      return;
    }
    if (moved || (a.hasAttribute("data-group") ? expandGroup(a) : openNode(a))) {
      // A drag that ends on a node is a pan, not a selection.
      evt.preventDefault();
    }
  });
  document.addEventListener("keydown", function (evt) {
    var pill = evt.target.closest && evt.target.closest(".graph .group-collapse");
    if (pill && evt.key === "Enter" && expandGroup(pill)) {
      evt.preventDefault();
      return;
    }
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (a && (evt.key === "Enter" || evt.key === " ") && (a.hasAttribute("data-group") ? expandGroup(a) : openNode(a))) {
      evt.preventDefault();
    }
  });

  // playMotion moves the members of the group just expanded from the node
  // they come from to their place over 420 ms while they fade in, and fades
  // the frame and the edges in. Nothing moves under reduced motion.
  function playMotion(g) {
    var m = motion;
    motion = null;
    if (!m || reducedMotion()) {
      return;
    }
    var ids = new Set();
    g.querySelectorAll('.node[data-member-of="' + CSS.escape(m.group) + '"]').forEach(function (n) { ids.add(n.getAttribute("data-node")); });
    g.querySelectorAll("path[data-from]").forEach(function (e) {
      if (ids.has(e.getAttribute("data-from")) && !e.getAttribute("data-from").startsWith("grp:")) {
        ids.add(e.getAttribute("data-to"));
      }
    });
    var nodes = [];
    g.querySelectorAll(".node").forEach(function (n) {
      var b = n.querySelector(".node-box");
      if (b && ids.has(n.getAttribute("data-node"))) {
        n.style.transition = "none";
        n.style.transform = "translate(" + (m.x - b.x.baseVal.value) + "px, " + (m.y - b.y.baseVal.value) + "px)";
        n.style.opacity = "0";
        nodes.push(n);
      }
    });
    g.classList.add("expanding");
    void g.getBoundingClientRect();
    nodes.forEach(function (n) {
      n.style.transition = "transform 420ms cubic-bezier(0.2, 0.8, 0.2, 1), opacity 260ms ease-out";
      n.style.transform = "translate(0px, 0px)";
      n.style.opacity = "1";
    });
    window.setTimeout(function () {
      nodes.forEach(function (n) {
        n.style.transition = "";
        n.style.transform = "";
        n.style.opacity = "";
      });
      g.classList.remove("expanding");
    }, 480);
  }

  // ---- Pan and zoom ----
  // A graph is drawn at its natural size in a scrolling frame. Zoom sets
  // the SVG's width and height attributes (no style attribute, so no
  // policy exception); dragging the background scrolls the frame. The frame
  // is as high as the zoomed graph, between 320 and 760 px, set through the
  // CSSOM; in full screen it fills the window.
  var zooms = new Map(); // graph id -> scale the user chose
  var moved = false;
  var zoomTimers = new Map();

  function frameOf(g) { return g.querySelector(".graph-frame"); }

  // animateZoom lets the next size change of the SVG run as a transition
  // (the CSS is under no-preference); wheel zoom never calls it.
  function animateZoom(g) {
    var gid = g.getAttribute("data-graph");
    g.classList.add("zoom-anim");
    window.clearTimeout(zoomTimers.get(gid));
    zoomTimers.set(gid, window.setTimeout(function () { g.classList.remove("zoom-anim"); }, 350));
  }

  // afterZoom runs fn once the size change has settled.
  function afterZoom(fn) {
    if (reducedMotion()) {
      fn();
    } else {
      window.setTimeout(fn, 330);
    }
  }

  function currentScale(g) {
    var svg = g.querySelector(".graph-frame > svg");
    return parseFloat(svg.getAttribute("width")) / svg.viewBox.baseVal.width;
  }

  function apply(g) {
    var svg = g.querySelector(".graph-frame > svg");
    if (!svg) {
      return;
    }
    var vb = svg.viewBox.baseVal;
    var gid = g.getAttribute("data-graph");
    var s = zooms.get(gid) || fitScale(g);
    var frame = frameOf(g);
    svg.setAttribute("width", String(Math.round(vb.width * s)));
    svg.setAttribute("height", String(Math.round(vb.height * s)));
    if (frame) {
      frame.style.height = isFull(stageOf(g)) ? "" : Math.max(320, Math.min(760, Math.round(vb.height * s))) + "px";
      frame.style.setProperty("--grid-size", (24 * s) + "px");
    }
    var range = g.querySelector("[data-zoom-range]");
    var out = g.querySelector("[data-zoom-value]");
    var fit = g.querySelector("[data-zoom=fit]");
    if (range) {
      range.value = String(Math.round(s * 100));
    }
    if (out) {
      out.textContent = Math.round(s * 100) + "%";
    }
    if (fit) {
      fit.setAttribute("aria-pressed", zooms.has(gid) ? "false" : "true");
    }
  }

  // fitGroup fits the view to the group just expanded, once per render,
  // unless the user zoomed: the scale is max(0.6, min(1.5, wFit, hFit)) of
  // the group's frame, centred, or from its top less 12 px when it is
  // taller than the pane. It zooms with the animation and then scrolls,
  // unless a spotlit node is to be centred, which does its own.
  function fitGroup(g) {
    var group = g.getAttribute("data-fit-group");
    var gid = g.getAttribute("data-graph");
    var frame = frameOf(g);
    var svg = g.querySelector(".graph-frame > svg");
    if (!group || !frame || !svg || g.hasAttribute("data-fitted") || zooms.has(gid)) {
      return;
    }
    g.setAttribute("data-fitted", "");
    var x0, y0, w, h;
    var rect = g.querySelector('[data-frame-for="' + CSS.escape(group) + '"] .group-frame');
    if (rect) {
      x0 = rect.x.baseVal.value;
      y0 = rect.y.baseVal.value;
      w = rect.width.baseVal.value;
      h = rect.height.baseVal.value;
    } else {
      var x1 = -Infinity, y1 = -Infinity;
      x0 = Infinity;
      y0 = Infinity;
      g.querySelectorAll('.node[data-member-of="' + CSS.escape(group) + '"] .node-box').forEach(function (r) {
        x0 = Math.min(x0, r.x.baseVal.value);
        y0 = Math.min(y0, r.y.baseVal.value);
        x1 = Math.max(x1, r.x.baseVal.value + r.width.baseVal.value);
        y1 = Math.max(y1, r.y.baseVal.value + r.height.baseVal.value);
      });
      if (x0 === Infinity) {
        return;
      }
      w = x1 - x0;
      h = y1 - y0;
    }
    var spotted = g.hasAttribute("data-focus");
    var s = Math.max(0.6, Math.min(1.5, (frame.clientWidth - 2) / w, (frame.clientHeight - 32) / h));
    zooms.set(gid, s);
    if (!spotted) {
      animateZoom(g);
    }
    apply(g);
    var scroll = function () {
      var cw = frame.clientWidth, ch = frame.clientHeight;
      var left = Math.max(0, (x0 + w / 2) * s - cw / 2);
      var top = h * s <= ch ? Math.max(0, (y0 + h / 2) * s - ch / 2) : Math.max(0, y0 * s - 12);
      frame.scrollTo({ left: left, top: top, behavior: reducedMotion() ? "auto" : "smooth" });
      g.setAttribute("data-fit-settled", "");
    };
    if (spotted) {
      scroll();
    } else {
      afterZoom(scroll);
    }
  }

  // fitScale is the scale of the overview: min(1.5, widthFit, heightFit),
  // 2 in full screen. A tall graph keeps a floor of 0.6 (0.75 on a narrow
  // frame) and takes the width fit, so its labels stay legible and the frame
  // scrolls.
  function fitScale(g) {
    var svg = g.querySelector(".graph-frame > svg");
    var frame = frameOf(g);
    if (!svg || !frame || !frame.clientWidth) {
      return 1;
    }
    var vb = svg.viewBox.baseVal;
    var full = isFull(stageOf(g));
    var cap = full ? 2 : 1.5;
    var wFit = (frame.clientWidth - 2) / vb.width;
    var hFit = ((full ? frame.clientHeight : 760) - 2) / vb.height;
    var floor = frame.clientWidth < 600 ? 0.75 : 0.6;
    var s = Math.min(cap, wFit, hFit);
    if (s < floor) {
      s = Math.max(floor, Math.min(cap, wFit));
    }
    return s;
  }

  // restoreGraphs sizes every graph (fitted until the user zooms) and puts
  // back its selected node; it runs on load and after every swap.
  function restoreGraphs() {
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      var gid = g.getAttribute("data-graph");
      var prev = selected.get(gid);
      var cur = g.querySelector(".node[aria-current]");
      if (prev && !g.querySelector('.node[data-node="' + CSS.escape(prev) + '"]')) {
        // The selected node is gone (the object was deleted): the panel no
        // longer describes it, and the address stops naming it.
        setSelectionParams("", "");
        resetDetail(g);
      }
      // The render is the truth: it was made from the address.
      if (cur) {
        selected.set(gid, cur.getAttribute("data-node"));
      } else {
        selected.delete(gid);
      }
      var focus = g.getAttribute("data-focus");
      if (focus && cur) {
        spot.set(gid, focus);
      } else {
        spot.delete(gid);
      }
      apply(g);
      markSelected(g);
      fitGroup(g);
      centreSpot(g);
      playMotion(g);
    });
    markFull();
    showHovered();
  }
  restoreGraphs();

  document.addEventListener("click", function (evt) {
    var b = evt.target.closest && evt.target.closest(".graph-tools button[data-zoom]");
    if (b) {
      var g = b.closest(".graph");
      zooms.delete(g.getAttribute("data-graph"));
      animateZoom(g);
      apply(g);
    }
  });

  document.addEventListener("input", function (evt) {
    var r = evt.target;
    if (r && r.matches && r.matches("[data-zoom-range]")) {
      var g = r.closest(".graph[data-graph]");
      zooms.set(g.getAttribute("data-graph"), Math.min(2, Math.max(0.3, Number(r.value) / 100)));
      animateZoom(g);
      apply(g);
    }
  });

  // ---- Full screen ----
  // A fixed overlay over the page (the stage gains .graph-full), not the
  // browser's Fullscreen API: Esc then reaches the page, which steps back one
  // thing per press, and a swap of #main can keep the overlay. A navigation
  // leaves full screen. The stage holds the graph's region but is not itself
  // refreshed, so a live refresh replaces the graph inside it and the view
  // stays full screen.
  function stageOf(g) {
    return (g.closest && g.closest("[data-graph-stage]")) || g;
  }

  function isFull(stage) {
    return stage.classList.contains("graph-full");
  }

  // markFull sets every Full screen button to what its stage is, and
  // resizes the graphs: after entering, leaving, and every refresh.
  function markFull() {
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      var full = isFull(stageOf(g));
      var b = g.querySelector("[data-graph-full]");
      if (b) {
        var word = full ? "Exit full screen (Esc)" : "Show the graph full screen";
        b.classList.toggle("is-full", full);
        b.setAttribute("aria-label", word);
        b.setAttribute("title", word);
      }
      apply(g);
    });
  }

  // refitAll drops the user's zoom: entering and leaving full screen refits
  // the overview, with the 2x cap in full screen.
  function refitAll() {
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) { zooms.delete(g.getAttribute("data-graph")); });
    markFull();
  }

  function leaveFull() {
    document.querySelectorAll(".graph-full").forEach(function (el) { el.classList.remove("graph-full"); });
    refitAll();
  }

  function anyFull() {
    return !!document.querySelector(".graph-full");
  }

  document.addEventListener("click", function (evt) {
    var b = evt.target.closest && evt.target.closest("[data-graph-full]");
    if (!b) {
      return;
    }
    var stage = stageOf(b.closest(".graph[data-graph]"));
    if (isFull(stage)) {
      leaveFull();
      return;
    }
    stage.classList.add("graph-full");
    refitAll();
  });
  document.addEventListener("htmx:beforeSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (t && t.id === "main") {
      centred.clear();
      if (!fullKeep) {
        leaveFull();
      }
    }
  });
  // A swap the graph started keeps full screen. htmx gives the new stage the
  // old stage's attributes until it settles and then applies its own, so the
  // class goes back on when the swap settles, before the graphs are sized.
  var fullRestore = false;
  document.addEventListener("htmx:afterSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (fullKeep && t && t.id === "main") {
      fullKeep = false;
      fullRestore = true;
    }
  });
  function keepFull() {
    if (!fullRestore) {
      return;
    }
    fullRestore = false;
    var stage = document.querySelector("[data-graph-stage]");
    if (stage) {
      stage.classList.add("graph-full");
    }
  }

  // ---- Info tips (WCAG 2.2 1.4.13, 4.1.2) ----
  // The CSS shows a .tip's box on hover and on focus, and the pointer can reach
  // the box. The trigger is a native button, so Enter and Space reach the click
  // handler with no key handler of ours. These delegated handlers, which keep
  // working on markup htmx swaps in, add what CSS cannot: Escape dismisses
  // (is-dismissed), a click or tap toggles (is-open and is-dismissed), a tap
  // outside closes, and an opened tip closes when focus or the pointer leaves
  // it. They set classes only, never a style attribute.
  var tipDown = null; // {tip, shown}: the tip a pointer went down on, and whether its box was shown then

  function tipOf(node) {
    return node && node.closest ? node.closest(".tip") : null;
  }

  // tipShown says whether a tip's box is on screen now, from layout, so it
  // follows the CSS whatever rule shows it.
  function tipShown(tip) {
    var box = tip.querySelector(".tipbox");
    return !!box && box.getClientRects().length > 0;
  }

  // fitTip keeps a shown box inside the viewport: the CSS puts it under the
  // start of its trigger, and a trigger near the right edge of a phone would
  // push the page sideways. The offset is a CSSOM property, never a style
  // attribute in the markup, and is read fresh each time the box shows.
  function fitTip(tip) {
    var box = tip.querySelector(".tipbox");
    if (!box) {
      return;
    }
    box.style.left = "";
    if (box.getClientRects().length === 0) {
      return;
    }
    var edge = 8;
    var wide = document.documentElement.clientWidth;
    var r = box.getBoundingClientRect();
    var shift = 0;
    if (r.right > wide - edge) {
      shift = wide - edge - r.right;
    }
    if (r.left + shift < edge) {
      shift = edge - r.left;
    }
    if (shift !== 0) {
      // An offset, not a transform: WebKit counts a box's untranslated place in
      // the page's scroll width. It also keeps that place when an absolute box
      // moves after its first layout, so the box is taken out of the absolute
      // layout for one read, which makes WebKit measure its new place.
      var offset = r.left + shift - tip.getBoundingClientRect().left;
      box.style.position = "static";
      void box.offsetWidth;
      box.style.left = offset + "px";
      box.style.position = "";
    }
  }

  function fitTipSoon(tip) {
    fitTip(tip);
    window.requestAnimationFrame(function () { fitTip(tip); });
  }

  // tipHeld says whether focus or the pointer is on a tip now. Only a held tip
  // can be dismissed: is-dismissed clears when focus and the pointer leave, so
  // one set on a tip that neither holds would stay and hide the next show.
  function tipHeld(tip) {
    return tip.matches(":focus-within") || tip.matches(":hover");
  }

  // dismissTips hides every tip that is shown and leaves focus where it is.
  function dismissTips() {
    var any = false;
    document.querySelectorAll(".tip").forEach(function (tip) {
      if (tipShown(tip)) {
        tip.classList.remove("is-open");
        if (tipHeld(tip)) {
          tip.classList.add("is-dismissed");
        }
        any = true;
      }
    });
    return any;
  }

  function toggleTip(tip, shown) {
    if (shown) {
      tip.classList.remove("is-open");
      tip.classList.add("is-dismissed");
    } else {
      tip.classList.remove("is-dismissed");
      tip.classList.add("is-open");
      fitTipSoon(tip);
    }
  }
  // Reaching a tip again (a mouse in from outside, or focus in) lifts a dismiss. A touch
  // has no hover, and its pointerover comes just before the pointerdown that reads
  // whether the box is shown, so it lifts nothing.
  document.addEventListener("pointerover", function (evt) {
    var tip = tipOf(evt.target);
    if (tip) {
      if (evt.pointerType !== "touch" && !tip.contains(evt.relatedTarget)) {
        tip.classList.remove("is-dismissed");
      }
      fitTipSoon(tip);
    }
  });
  document.addEventListener("focusin", function (evt) {
    var tip = tipOf(evt.target);
    if (tip) {
      tip.classList.remove("is-dismissed");
      fitTipSoon(tip);
    }
  });

  document.addEventListener("pointerdown", function (evt) {
    var tip = tipOf(evt.target);
    if (!tip) {
      // A tap or click outside every tip closes the opened ones. A tip
      // that still holds focus (a touch screen may not move it) is blurred, or
      // :focus-within would keep its box on screen.
      document.querySelectorAll(".tip.is-open").forEach(function (t) { t.classList.remove("is-open"); });
      var held = tipOf(document.activeElement);
      if (held) {
        document.activeElement.blur();
      }
      tipDown = null;
      return;
    }
    if (evt.target.closest(".tipbox")) {
      tipDown = null;
      return;
    }
    // Read the box now, before focus moves: a tap focuses the trigger, and
    // :focus-within alone then shows the box, so reading after the focus would
    // close the tip the tap opened. A touch has no hover, so its hover does not count.
    var visible = tipShown(tip);
    if (evt.pointerType === "touch" && !tip.classList.contains("is-open") && !tip.matches(":focus-within")) {
      visible = false;
    }
    tipDown = { tip: tip, shown: visible };
  });
  document.addEventListener("click", function (evt) {
    var tip = tipOf(evt.target);
    if (!tip || evt.target.closest(".tipbox")) {
      tipDown = null;
      return;
    }
    var shown = tipDown && tipDown.tip === tip ? tipDown.shown : tipShown(tip);
    tipDown = null;
    toggleTip(tip, shown);
  });
  // A tip closes when focus or the pointer leaves it. The pointer that leaves
  // ends is-open and, when focus is not in the tip, is-dismissed; focus that
  // leaves does the same unless the pointer still rests on the tip. A touch has
  // no hover, so a touch pointer leaving (it fires after the tap lifts) does not
  // count. relatedTarget names where the pointer or focus went.
  document.addEventListener("pointerout", function (evt) {
    var tip = tipOf(evt.target);
    if (!tip || evt.pointerType === "touch" || tip.contains(evt.relatedTarget)) {
      return;
    }
    tip.classList.remove("is-open");
    if (!tip.matches(":focus-within")) {
      tip.classList.remove("is-dismissed");
    }
  });
  document.addEventListener("focusout", function (evt) {
    var tip = tipOf(evt.target);
    if (!tip || tip.contains(evt.relatedTarget)) {
      return;
    }
    tip.classList.remove("is-open");
    if (!tip.matches(":hover")) {
      tip.classList.remove("is-dismissed");
    }
  });

  // Escape steps back one thing per press: a shown info tip, the theme
  // menu, an open hover card, a selection that is not the resting node, an
  // open configuration group (as Overview does), then full screen.
  document.addEventListener("keydown", function (evt) {
    if (evt.key !== "Escape" || evt.defaultPrevented) {
      return;
    }
    if (dismissTips()) {
      evt.preventDefault();
      return;
    }
    var menu = document.querySelector("details.theme[open]");
    if (menu) {
      menu.open = false;
      var s = menu.querySelector("summary");
      if (s) {
        s.focus();
      }
      evt.preventDefault();
      return;
    }
    if (document.querySelector(".node-card:not([hidden])")) {
      hideCards();
      hovered = null;
      evt.preventDefault();
      return;
    }
    var cleared = false;
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      if (selected.has(g.getAttribute("data-graph"))) {
        clearSelection(g);
        cleared = true;
      }
    });
    if (cleared) {
      evt.preventDefault();
      return;
    }
    var overview = document.querySelector(".graph [data-overview]");
    if (overview && document.querySelector(".graph .group-frame-box") && expandGroup(overview)) {
      evt.preventDefault();
      return;
    }
    if (anyFull()) {
      leaveFull();
      evt.preventDefault();
    }
  });

  // ---- Hover cards ----
  // The server renders each node's card hidden beside the graph; the card
  // shows on pointer hover and on keyboard focus, placed by CSSOM
  // properties, never a style attribute.
  var hovered = null; // {graph, node}

  function cardFor(n) {
    var id = n.getAttribute("aria-describedby");
    return id ? document.getElementById(id) : null;
  }

  // showCard places the card at the node's right edge plus 12 px, or left of
  // the node when it does not fit there, clamped to the graph, through CSSOM
  // properties; it also dims, while it lasts, what lies outside the node's
  // chain.
  function showCard(n) {
    var card = cardFor(n);
    var g = n.closest(".graph[data-graph]");
    if (!card || !g) {
      return;
    }
    hideCards();
    var box = g.getBoundingClientRect();
    var r = n.getBoundingClientRect();
    card.hidden = false;
    var w = card.offsetWidth, h = card.offsetHeight;
    var left = r.right - box.left + 12;
    if (left + w > g.clientWidth - 4) {
      left = r.left - box.left - 12 - w;
    }
    left = Math.max(4, Math.min(left, g.clientWidth - w - 4));
    var top = Math.max(4, Math.min(r.top - box.top, g.clientHeight - h - 4));
    card.style.left = left + "px";
    card.style.top = top + "px";
    hovered = { graph: g.getAttribute("data-graph"), node: n.getAttribute("data-node") };
    g.classList.add("chain-on");
    dimBy(g, chainOf(g, n.getAttribute("data-node")), "chain-dim");
  }

  function hideCards() {
    document.querySelectorAll(".node-card:not([hidden])").forEach(function (c) { c.hidden = true; });
    document.querySelectorAll(".graph.chain-on").forEach(function (g) {
      g.classList.remove("chain-on");
      dimBy(g, null, "chain-dim");
    });
  }

  function showHovered() {
    if (!hovered) {
      return;
    }
    var g = document.querySelector('.graph[data-graph="' + CSS.escape(hovered.graph) + '"]');
    var n = g && g.querySelector('.node[data-node="' + CSS.escape(hovered.node) + '"]');
    if (n) {
      showCard(n);
    } else {
      hovered = null;
    }
  }

  // WebKit scrolls a focused node into view after it has sent focusin, so a
  // card placed on focusin sits where the node was before the scroll. A
  // scroll of the frame places the open card again, once for each frame.
  var cardScroll = false;
  document.addEventListener("scroll", function (evt) {
    if (!hovered || cardScroll || !(evt.target.closest && evt.target.closest(".graph-frame"))) {
      return;
    }
    cardScroll = true;
    window.requestAnimationFrame(function () {
      cardScroll = false;
      showHovered();
    });
  }, true);

  document.addEventListener("pointerover", function (evt) {
    var n = evt.target.closest && evt.target.closest(".graph .node");
    if (n && !drag) {
      showCard(n);
    }
  });
  document.addEventListener("pointerout", function (evt) {
    var n = evt.target.closest && evt.target.closest(".graph .node");
    if (n && !(evt.relatedTarget && n.contains(evt.relatedTarget))) {
      hideCards();
      hovered = null;
    }
  });
  document.addEventListener("focusin", function (evt) {
    var n = evt.target.closest && evt.target.closest(".graph .node");
    if (n) {
      showCard(n);
    }
  });
  document.addEventListener("focusout", function (evt) {
    var n = evt.target.closest && evt.target.closest(".graph .node");
    if (n) {
      hideCards();
      hovered = null;
    }
  });

  // Ctrl or Cmd + wheel zooms continuously, from 0.3 to 2, keeping the point
  // under the cursor where it is. It never animates: while a size is still
  // transitioning the scroll range is the old one and the anchor would drift.
  document.addEventListener("wheel", function (evt) {
    var g = evt.target.closest && evt.target.closest(".graph[data-graph]");
    if (!g || !(evt.ctrlKey || evt.metaKey)) {
      return;
    }
    evt.preventDefault();
    var svg = g.querySelector(".graph-frame > svg");
    var frame = frameOf(g);
    if (!svg || !frame) {
      return;
    }
    var s0 = currentScale(g);
    var r = svg.getBoundingClientRect();
    var px = (evt.clientX - r.left) / s0, py = (evt.clientY - r.top) / s0;
    var s1 = Math.min(2, Math.max(0.3, s0 * Math.exp(-evt.deltaY * 0.0015)));
    zooms.set(g.getAttribute("data-graph"), s1);
    g.classList.remove("zoom-anim");
    apply(g);
    var r2 = svg.getBoundingClientRect();
    frame.scrollLeft += r2.left + px * s1 - evt.clientX;
    frame.scrollTop += r2.top + py * s1 - evt.clientY;
  }, { passive: false });

  var drag = null;
  document.addEventListener("pointerdown", function (evt) {
    var frame = evt.target.closest && evt.target.closest(".graph-frame");
    if (!frame || evt.button !== 0 || evt.pointerType === "touch") {
      return;
    }
    drag = { frame: frame, x: evt.clientX, y: evt.clientY, left: frame.scrollLeft, top: frame.scrollTop, id: evt.pointerId };
    moved = false;
  });
  document.addEventListener("pointermove", function (evt) {
    if (!drag || evt.pointerId !== drag.id) {
      return;
    }
    var dx = evt.clientX - drag.x, dy = evt.clientY - drag.y;
    if (!moved && Math.abs(dx) + Math.abs(dy) < 5) {
      return;
    }
    if (!moved) {
      moved = true;
      drag.frame.classList.add("dragging");
    }
    drag.frame.scrollLeft = drag.left - dx;
    drag.frame.scrollTop = drag.top - dy;
  });
  function endDrag() {
    if (drag) {
      drag.frame.classList.remove("dragging");
      drag = null;
      window.setTimeout(function () { moved = false; }, 0);
    }
  }
  document.addEventListener("pointerup", endDrag);
  document.addEventListener("pointercancel", endDrag);
})();
