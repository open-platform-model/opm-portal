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

  // ---- Stream topics ----
  var streamID = "";
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
      if (!res.ok) {
        setLive(false, "topics refused");
      }
    }).catch(function () { setLive(false, "offline"); });
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
        setLive(false, "signed out");
        return null;
      }
      return res.text();
    }).then(function (text) {
      if (text === null || url !== location.pathname + location.search) {
        return;
      }
      var doc = new DOMParser().parseFromString(text, "text/html");
      var views = [];
      regions.forEach(function (el) {
        var next = el.id && doc.getElementById(el.id);
        if (!next || !el.isConnected) {
          return;
        }
        keepOpen(el, next);
        keepViews(el, views);
        var fresh = document.importNode(next, true);
        mergeLogs(el, fresh);
        el.replaceWith(fresh);
        if (typeof htmx !== "undefined") {
          htmx.process(fresh);
        }
        fresh.classList.add("refreshed");
        window.setTimeout(function () { fresh.classList.remove("refreshed"); }, 900);
      });
      restoreGraphs();
      restoreViews(views);
    }).catch(function () {
      setLive(false, "offline");
    }).then(function () {
      inflight = false;
      schedule();
    });
  }

  // keepOpen carries the open state of details groups with an id from the
  // region on the page to its replacement.
  function keepOpen(from, to) {
    from.querySelectorAll("details[id]").forEach(function (d) {
      var twin = to.querySelector("#" + CSS.escape(d.id));
      if (twin) {
        twin.open = d.open;
      }
    });
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

  // mergeLogs keeps the log panes on the page in the fresh logs region:
  // a pane whose container the fresh region still lists is moved over with
  // its text, its open state and its stream; a Pod new since the page
  // loaded gets the fresh, closed pane. An open pane whose Pod is gone stays,
  // marked gone, so its last lines can still be read; a closed one is
  // dropped and stops following.
  function mergeLogs(from, to) {
    var list = to.querySelector(".logs");
    from.querySelectorAll("details.log").forEach(function (d) {
      var twin = d.id ? to.querySelector("#" + CSS.escape(d.id)) : null;
      if (twin) {
        twin.replaceWith(d);
        return;
      }
      if (d.open && list) {
        d.classList.add("log-gone");
        list.appendChild(d);
        return;
      }
      var f = d.getAttribute("data-following");
      if (f) {
        logTopics.delete(f);
        sync();
      }
    });
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
      setLive(true, "live");
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
  }

  body.addEventListener("htmx:sseError", function () { setLive(false, "reconnecting"); });
  body.addEventListener("htmx:sseOpen", function () { setLive(true, "live"); });

  // A boosted navigation swaps #main: move the stream to the new page.
  document.addEventListener("htmx:afterSettle", function () {
    sync();
    restoreGraphs();
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
  document.addEventListener("htmx:beforeSwap", function (evt) {
    var t = evt.detail && evt.detail.target;
    if (t && t.id === "main") {
      logTopics.clear();
      selected.clear();
      zooms.clear();
    }
  });

  // ---- Graph nodes ----
  // selected maps a graph id to the id of its selected node, so a refresh
  // that replaces the graph keeps the mark.
  var selected = new Map();

  function markSelected(g) {
    var want = selected.get(g.getAttribute("data-graph"));
    g.querySelectorAll(".node").forEach(function (n) {
      if (want && n.getAttribute("data-node") === want) {
        n.setAttribute("aria-current", "true");
      } else {
        n.removeAttribute("aria-current");
      }
    });
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
    }
    htmx.ajax("GET", panel, { source: a, target: "#detail", swap: "innerHTML" }).then(function () {
      intoView(target);
    });
    return true;
  }

  // A Pod's Logs link opens and shows its first container's pane.
  document.addEventListener("click", function (evt) {
    var l = evt.target.closest && evt.target.closest("a[data-log-link]");
    if (!l) {
      return;
    }
    var d = document.getElementById(l.getAttribute("data-log-link"));
    if (d && d.matches("details.log")) {
      evt.preventDefault();
      d.open = true;
      d.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  });

  document.addEventListener("click", function (evt) {
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (!a) {
      return;
    }
    if (moved || openNode(a)) {
      // A drag that ends on a node is a pan, not a selection.
      evt.preventDefault();
    }
  });
  document.addEventListener("keydown", function (evt) {
    var a = evt.target.closest && evt.target.closest(".graph .node");
    if (a && (evt.key === "Enter" || evt.key === " ") && openNode(a)) {
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
    });
  }
  restoreGraphs();

  function zoom(g, how) {
    var id = g.getAttribute("data-graph");
    var svg = g.querySelector("svg");
    var frame = frameOf(g);
    var s = zooms.get(id) || fitScale(g);
    if (how === "fit") {
      s = Math.max(0.2, Math.min(1, (frame.clientWidth - 2) / svg.viewBox.baseVal.width));
    } else if (how === "reset") {
      s = 1;
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
