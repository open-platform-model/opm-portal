// Package ui serves the portal's web pages: the Platform, instances,
// packages and the objects they reach, rendered on the server with
// html/template and updated live with htmx over the read API's stream.
//
// The pages read nothing of their own. Every document a page shows is
// fetched through the read API's handler chain, in-process, with the
// caller's own session, so authentication, authorization and what is never
// served are the API's alone, and a client of the API can read every fact a
// page shows (0030:D2). A test fails when a file of this package imports
// the read model.
//
// Pages carry a Content-Security-Policy that allows scripts, styles, fonts,
// images and connections from the portal itself and nothing else: no inline
// script or style and no eval. Untrusted text (condition messages, event
// notes, labels, log lines) is only ever rendered by html/template or set
// as text by the page script. htmx, its SSE extension and the fonts are
// vendored under static/vendor at pinned versions with recorded checksums.
package ui
