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
  document.addEventListener("htmx:historyRestore", function () {
    rememberFilters();
    markTheme();
  });

  // "Clear filters" forgets the view first, so nothing restores them.
  document.addEventListener("click", function (evt) {
    var a = evt.target.closest && evt.target.closest("a[data-clear-filters]");
    if (a && prefs) {
      prefs.forget(a.getAttribute("data-clear-filters"));
    }
  }, true);

  // A filter form's selects apply at once; without script its button does.
  document.addEventListener("change", function (evt) {
    var sel = evt.target;
    if (sel && sel.matches && sel.matches("form[data-filters] select") && sel.form && sel.form.requestSubmit) {
      sel.form.requestSubmit();
    }
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
        setLive(false, "Topics refused");
      }
    }).catch(function () {
      if (!expired) {
        setLive(false, "Offline");
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
    if (regions.length === 0) {
      return;
    }
    inflight = true;
    var url = location.pathname + location.search;
    fetch(url, { credentials: "same-origin", headers: { "Accept": "text/html" } }).then(function (res) {
      if (res.status === 401) {
        setLive(false, "Signed out");
        return null;
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
      restoreGraphs();
      restoreViews(views);
      refocus(focused);
    }).catch(function () {
      setLive(false, "Offline");
    }).then(function () {
      inflight = false;
      schedule();
    });
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
    return { id: a.id || "", graph: g ? g.getAttribute("data-graph") : "", node: a.getAttribute("data-node") || "" };
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
      setLive(true, "Live");
      var want = pageTopics();
      var remove = [];
      opened.forEach(function (t) { if (!want.has(t)) { remove.push(t); } });
      subscribed.forEach(function (t) { if (!want.has(t) && remove.indexOf(t) < 0) { remove.push(t); } });
      subscribed = want;
      primed.clear();
      post(Array.from(want), remove);
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
      setLive(false, "Session expired, reload");
    });
  }

  body.addEventListener("htmx:sseError", function () {
    if (!expired) {
      setLive(false, "Reconnecting");
    }
  });
  body.addEventListener("htmx:sseOpen", function () { setLive(true, "Live"); });

  // A boosted navigation swaps #main: move the stream to the new page.
  document.addEventListener("htmx:afterSettle", function () {
    sync();
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
  document.addEventListener("htmx:afterSettle", function () {
    if (!mainSwapped) {
      return;
    }
    mainSwapped = false;
    var m = document.getElementById("main");
    var tab = tabClicked && m && m.querySelector(".tabs a[aria-current]");
    tabClicked = false;
    var target = tab || m;
    if (target && target.focus) {
      target.focus({ preventScroll: true });
    }
  });

  // ---- Graph nodes ----
  // selected maps a graph id to the id of its selected node, so a refresh
  // that replaces the graph keeps the mark.
  var selected = new Map();

  // markSelected marks the selected node and spotlights it: every node
  // and edge not next to it is dimmed by a class.
  function markSelected(g) {
    var want = selected.get(g.getAttribute("data-graph"));
    if (want && !g.querySelector('.node[data-node="' + CSS.escape(want) + '"]')) {
      // The focused node is gone (the object was deleted): nothing dims,
      // and the panel no longer describes it.
      selected.delete(g.getAttribute("data-graph"));
      setFocusParam("");
      resetDetail();
      want = "";
    }
    var near = new Set();
    if (want) {
      near.add(want);
      g.querySelectorAll("path[data-from]").forEach(function (e) {
        var from = e.getAttribute("data-from"), to = e.getAttribute("data-to");
        if (from === want || to === want) {
          near.add(from);
          near.add(to);
        }
        e.classList.toggle("dim", from !== want && to !== want);
      });
    } else {
      g.querySelectorAll("path[data-from]").forEach(function (e) { e.classList.remove("dim"); });
    }
    g.querySelectorAll(".node").forEach(function (n) {
      var id = n.getAttribute("data-node");
      if (want && id === want) {
        n.setAttribute("aria-current", "true");
      } else {
        n.removeAttribute("aria-current");
      }
      n.classList.toggle("dim", !!want && !near.has(id));
    });
    var clear = g.querySelector("[data-clear-selection]");
    if (clear) {
      clear.hidden = !want;
    }
  }

  // setFocusParam keeps the focus parameter in the address, so a refresh,
  // a reload and a shared link show the same spotlight.
  function setFocusParam(id) {
    var u = new URL(location.href);
    if (id) {
      u.searchParams.set("focus", id);
    } else {
      u.searchParams.delete("focus");
    }
    history.replaceState(history.state, "", u.pathname + u.search + u.hash);
  }

  function resetDetail() {
    var target = document.getElementById("detail");
    if (target) {
      var p = document.createElement("p");
      p.className = "muted";
      p.textContent = "Select a graph node, an object's YAML or its events to see them here.";
      target.replaceChildren(p);
    }
  }

  function clearSelection(g) {
    selected.delete(g.getAttribute("data-graph"));
    markSelected(g);
    setFocusParam("");
    resetDetail();
  }

  function intoView(el) {
    var r = el.getBoundingClientRect();
    if (r.top < 0 || r.top > window.innerHeight - 80) {
      el.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  }

  function openNode(a) {
    var panel = a.getAttribute("data-panel");
    var target = document.getElementById("detail");
    if (!panel || !target || typeof htmx === "undefined") {
      return false;
    }
    var g = a.closest(".graph[data-graph]");
    if (g) {
      selected.set(g.getAttribute("data-graph"), a.getAttribute("data-node"));
      markSelected(g);
      setFocusParam(a.getAttribute("data-node"));
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
  // its expand parameter and the view fitted to the group's members.
  function expandGroup(a) {
    if (typeof htmx === "undefined") {
      return false;
    }
    var href = a.getAttribute("href");
    htmx.ajax("GET", href, { source: document.getElementById("main"), target: "#main", select: "#main", swap: "outerHTML", push: href });
    return true;
  }

  document.addEventListener("click", function (evt) {
    var clear = evt.target.closest && evt.target.closest("[data-clear-selection]");
    if (clear) {
      var cg = clear.closest(".graph[data-graph]");
      if (cg) {
        evt.preventDefault();
        clearSelection(cg);
      }
      return;
    }
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (!a) {
      // A click on the empty graph clears the selection; a drag does not.
      var empty = evt.target.closest && evt.target.closest(".graph-frame");
      var eg = empty && empty.closest(".graph[data-graph]");
      if (eg && !moved && selected.has(eg.getAttribute("data-graph"))) {
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
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (a && (evt.key === "Enter" || evt.key === " ") && (a.hasAttribute("data-group") ? expandGroup(a) : openNode(a))) {
      evt.preventDefault();
    }
  });

  // ---- Pan and zoom ----
  // A graph is drawn at its natural size in a scrolling frame. Zoom sets
  // the SVG's width and height attributes (no style attribute, so no
  // policy exception); dragging the background scrolls the frame.
  var zooms = new Map(); // graph id -> scale the user chose
  var moved = false;

  // minFit is the smallest scale a fitted graph is drawn at: on a narrow
  // screen larger, so the text stays legible and the frame scrolls.
  function minFit() {
    return window.matchMedia && window.matchMedia("(max-width: 640px)").matches ? 0.75 : 0.5;
  }

  function frameOf(g) { return g.querySelector(".graph-frame"); }

  function apply(g) {
    var svg = g.querySelector("svg");
    if (!svg) {
      return;
    }
    var vb = svg.viewBox.baseVal;
    var s = zooms.get(g.getAttribute("data-graph")) || fitScale(g);
    svg.setAttribute("width", String(Math.round(vb.width * s)));
    svg.setAttribute("height", String(Math.round(vb.height * s)));
    var range = g.querySelector("[data-zoom-range]");
    var out = g.querySelector("[data-zoom-value]");
    if (range) {
      range.value = String(Math.round(s * 100));
    }
    if (out) {
      out.textContent = Math.round(s * 100) + "%";
    }
  }

  // fitGroup fits the view to the members of the group just expanded,
  // once per render, unless the user zoomed.
  function fitGroup(g) {
    var group = g.getAttribute("data-fit-group");
    var frame = frameOf(g);
    var svg = g.querySelector("svg");
    if (!group || !frame || !svg || g.hasAttribute("data-fitted") || zooms.has(g.getAttribute("data-graph"))) {
      return;
    }
    g.setAttribute("data-fitted", "");
    var x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    g.querySelectorAll('.node[data-member-of="' + CSS.escape(group) + '"] .node-box').forEach(function (r) {
      var x = r.x.baseVal.value, y = r.y.baseVal.value;
      x0 = Math.min(x0, x);
      y0 = Math.min(y0, y);
      x1 = Math.max(x1, x + r.width.baseVal.value);
      y1 = Math.max(y1, y + r.height.baseVal.value);
    });
    if (x0 === Infinity) {
      return;
    }
    var pad = 24;
    var s = Math.max(0.2, Math.min(1.5, (frame.clientWidth - 2) / (x1 - x0 + 2 * pad)));
    zooms.set(g.getAttribute("data-graph"), s);
    apply(g);
    frame.scrollLeft = Math.max(0, (x0 - pad) * s);
    frame.scrollTop = Math.max(0, (y0 - pad) * s);
  }

  // fitScale fits the graph's width to its frame, never above 1:1 and
  // never below minFit: past that the frame scrolls.
  function fitScale(g) {
    var svg = g.querySelector("svg");
    var frame = frameOf(g);
    if (!svg || !frame || !frame.clientWidth) {
      return 1;
    }
    return Math.max(minFit(), Math.min(1, (frame.clientWidth - 2) / svg.viewBox.baseVal.width));
  }

  // restoreGraphs sizes every graph (fitted until the user zooms) and puts
  // back its selected node; it runs on load and after every swap.
  function restoreGraphs() {
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      if (!selected.has(g.getAttribute("data-graph"))) {
        var cur = g.querySelector(".node[aria-current]");
        if (cur) {
          selected.set(g.getAttribute("data-graph"), cur.getAttribute("data-node"));
        }
      }
      apply(g);
      markSelected(g);
      fitGroup(g);
    });
    markFull();
    showHovered();
  }
  restoreGraphs();

  function zoom(g, how) {
    var id = g.getAttribute("data-graph");
    var svg = g.querySelector("svg");
    var frame = frameOf(g);
    var s = zooms.get(id) || fitScale(g);
    if (how === "fit") {
      s = Math.max(0.2, Math.min(1, (frame.clientWidth - 2) / svg.viewBox.baseVal.width));
    } else {
      s = how === "in" ? s * 1.25 : s * 0.8;
    }
    zooms.set(id, Math.min(3, Math.max(0.2, s)));
    apply(g);
  }

  document.addEventListener("click", function (evt) {
    var b = evt.target.closest && evt.target.closest(".graph-tools button[data-zoom]");
    if (b) {
      zoom(b.closest(".graph"), b.getAttribute("data-zoom"));
    }
  });

  document.addEventListener("input", function (evt) {
    var r = evt.target;
    if (r && r.matches && r.matches("[data-zoom-range]")) {
      var g = r.closest(".graph[data-graph]");
      zooms.set(g.getAttribute("data-graph"), Math.min(3, Math.max(0.2, Number(r.value) / 100)));
      apply(g);
    }
  });

  // ---- Full screen ----
  // The Fullscreen API on the graph, or a class that fills the window where
  // the API is missing or refused. A navigation leaves full screen.
  // The stage is the element that goes full screen: it holds the graph's
  // region but is not itself refreshed, so a live refresh replaces the
  // graph inside it and the view stays full screen.
  function stageOf(g) {
    return (g.closest && g.closest("[data-graph-stage]")) || g;
  }

  function isFull(stage) {
    return document.fullscreenElement === stage || stage.classList.contains("graph-full");
  }

  // markFull sets every Full screen button to what its stage is, and
  // resizes the graphs: after entering, leaving, and every refresh.
  function markFull() {
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      var b = g.querySelector("[data-graph-full]");
      if (b) {
        b.setAttribute("aria-pressed", isFull(stageOf(g)) ? "true" : "false");
      }
      apply(g);
    });
  }

  function leaveFull() {
    if (document.fullscreenElement && document.exitFullscreen) {
      document.exitFullscreen().catch(function () {});
    }
    document.querySelectorAll(".graph-full").forEach(function (el) { el.classList.remove("graph-full"); });
    markFull();
  }

  function anyFull() {
    return !!document.fullscreenElement || !!document.querySelector(".graph-full");
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
    var fallback = function () {
      stage.classList.add("graph-full");
      markFull();
    };
    if (stage.requestFullscreen) {
      stage.requestFullscreen().then(markFull, fallback);
    } else {
      fallback();
    }
  });
  document.addEventListener("fullscreenchange", markFull);
  document.addEventListener("htmx:beforeSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (t && t.id === "main") {
      leaveFull();
    }
  });

  // ---- Info tips (WCAG 2.2 1.4.13) ----
  // The CSS shows a .tip's box on hover and on focus, and the pointer can reach
  // the box. These delegated handlers, which keep working on markup htmx swaps
  // in, add what CSS cannot: Escape dismisses (is-dismissed), Enter, Space and a
  // tap toggle (is-open and is-dismissed), and a tap outside closes. They set
  // classes only, never a style attribute.
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

  // dismissTips hides every tip that is shown and leaves focus where it is.
  function dismissTips() {
    var any = false;
    document.querySelectorAll(".tip").forEach(function (tip) {
      if (tipShown(tip)) {
        tip.classList.remove("is-open");
        tip.classList.add("is-dismissed");
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
  document.addEventListener("pointerover", function (evt) {
    var tip = tipOf(evt.target);
    if (tip) {
      fitTipSoon(tip);
    }
  });
  document.addEventListener("focusin", function (evt) {
    var tip = tipOf(evt.target);
    if (tip) {
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
  document.addEventListener("keydown", function (evt) {
    if ((evt.key !== "Enter" && evt.key !== " ") || evt.repeat || evt.defaultPrevented) {
      return;
    }
    var tip = tipOf(evt.target);
    if (!tip || evt.target !== tip) {
      return;
    }
    toggleTip(tip, tipShown(tip));
    evt.preventDefault();
  });
  // is-dismissed clears when the pointer and focus have both left the tip.
  // A pointer that leaves is known from relatedTarget; a hover that remains is
  // read from :hover when focus leaves.
  function releaseTip(tip, related, hoverCounts) {
    if (tip.contains(related) || tip.contains(document.activeElement) || (hoverCounts && tip.matches(":hover"))) {
      return;
    }
    tip.classList.remove("is-dismissed");
  }
  document.addEventListener("pointerout", function (evt) {
    var tip = tipOf(evt.target);
    if (tip && tip.classList.contains("is-dismissed")) {
      releaseTip(tip, evt.relatedTarget, false);
    }
  });
  document.addEventListener("focusout", function (evt) {
    var tip = tipOf(evt.target);
    if (tip && tip.classList.contains("is-dismissed")) {
      releaseTip(tip, evt.relatedTarget, true);
    }
  });

  // Escape steps back one thing per press: a shown info tip, then the theme
  // menu, then an open hover card, then full screen, then the selection.
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
    if (anyFull()) {
      leaveFull();
      evt.preventDefault();
      return;
    }
    document.querySelectorAll(".graph[data-graph]").forEach(function (g) {
      if (selected.has(g.getAttribute("data-graph"))) {
        clearSelection(g);
        evt.preventDefault();
      }
    });
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
    var left = r.left - box.left;
    var top = r.bottom - box.top + 6;
    var max = g.clientWidth - card.offsetWidth - 4;
    card.style.left = Math.max(4, Math.min(left, max)) + "px";
    card.style.top = top + "px";
    hovered = { graph: g.getAttribute("data-graph"), node: n.getAttribute("data-node") };
  }

  function hideCards() {
    document.querySelectorAll(".node-card:not([hidden])").forEach(function (c) { c.hidden = true; });
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

  document.addEventListener("wheel", function (evt) {
    var g = evt.target.closest && evt.target.closest(".graph");
    if (!g || !(evt.ctrlKey || evt.metaKey)) {
      return;
    }
    evt.preventDefault();
    zoom(g, evt.deltaY < 0 ? "in" : "out");
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
