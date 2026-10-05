// OPM Portal display preferences. Loaded in the page head without defer,
// before the stylesheet, so it runs before first paint: it applies the
// theme the browser chose, and on a full load of a remembered list view
// whose URL carries none of the view's filters, it moves to the URL with
// the filters remembered for it. The browser keeps only the theme and each
// list view's filter query, per kubeconfig context (portal:D14). Every
// storage access is guarded: with storage blocked the page renders in the
// System theme and remembers nothing.
(function () {
  "use strict";

  var root = document.documentElement;
  var THEME_KEY = "opm-portal.theme";

  function read(key) {
    try {
      return window.localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function write(key, value) {
    try {
      if (value === null) {
        window.localStorage.removeItem(key);
      } else {
        window.localStorage.setItem(key, value);
      }
    } catch (e) {
      // Storage blocked: the choice lasts for this page only.
    }
  }

  function applyTheme(choice) {
    if (choice === "light" || choice === "dark") {
      root.setAttribute("data-theme", choice);
    } else {
      root.removeAttribute("data-theme");
    }
  }

  // setTheme applies a choice and keeps it: light, dark or system.
  function setTheme(choice) {
    applyTheme(choice);
    write(THEME_KEY, choice === "light" || choice === "dark" ? choice : null);
  }

  function theme() {
    var t = read(THEME_KEY);
    return t === "light" || t === "dark" ? t : "system";
  }

  applyTheme(theme());

  // The remembered views, their parameters and the values of each
  // enumerated one, as the server renders them into the page head.
  var views = {};
  var spec = document.querySelector('meta[name="opm-portal-filters"]');
  try {
    views = JSON.parse((spec && spec.getAttribute("content")) || "{}") || {};
  } catch (e) {
    views = {};
  }

  // The context the portal reads, which keys what is remembered: filters
  // naming one cluster's namespaces are never restored against another.
  function context() {
    return root.getAttribute("data-context") || "";
  }

  function filterKey(view) {
    return "opm-portal.filters." + view + ":" + context();
  }

  function viewsAt(path) {
    return Object.keys(views).filter(function (k) { return views[k].path === path; });
  }

  // carries reports whether params holds any of view's parameters, even
  // empty: a submitted form that cleared them is a choice too.
  function carries(view, params) {
    return Object.keys(views[view].params).some(function (p) { return params.has(p); });
  }

  // valid returns the view's own parameters of a query with values the view
  // knows, dropping everything else.
  function valid(view, query) {
    var out = new URLSearchParams();
    var params = views[view].params;
    var given;
    try {
      given = new URLSearchParams(query || "");
    } catch (e) {
      return out;
    }
    Object.keys(params).forEach(function (p) {
      var v = given.get(p);
      if (v === null || v === "" || v.length > 512) {
        return;
      }
      if (params[p] && params[p].indexOf(v) < 0) {
        return;
      }
      out.set(p, v);
    });
    return out;
  }

  // restored returns path and query plus the filters remembered for each
  // view on path whose filters query does not carry, or null when there is
  // nothing to add. A remembered query that validates to nothing is
  // removed.
  function restored(path, query) {
    if (!context()) {
      return null;
    }
    var params = new URLSearchParams(query || "");
    var added = false;
    viewsAt(path).forEach(function (view) {
      if (carries(view, params)) {
        return;
      }
      var stored = read(filterKey(view));
      if (stored === null) {
        return;
      }
      var kept = valid(view, stored);
      if (kept.toString() === "") {
        write(filterKey(view), null);
        return;
      }
      kept.forEach(function (v, p) {
        params.set(p, v);
        added = true;
      });
    });
    if (!added) {
      return null;
    }
    return path + "?" + params.toString();
  }

  // remember stores, for each remembered view on path, the filters query
  // shows, or forgets the view when it shows none. tab and focus are not
  // filters and are never stored.
  function remember(path, query) {
    if (!context()) {
      return;
    }
    viewsAt(path).forEach(function (view) {
      var kept = valid(view, query).toString();
      write(filterKey(view), kept === "" ? null : kept);
    });
  }

  function forget(view) {
    if (context() && views[view]) {
      write(filterKey(view), null);
    }
  }

  window.opmPortalPrefs = {
    setTheme: setTheme,
    restored: restored,
    remember: remember,
    forget: forget
  };

  var next = restored(location.pathname, location.search);
  if (next !== null) {
    location.replace(next);
  }
})();
