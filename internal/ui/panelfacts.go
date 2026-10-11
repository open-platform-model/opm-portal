package ui

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// A fact is one labelled line of the details panel. Its value reaches the page
// as text through html/template (SR11).
type fact struct {
	Label string
	Value string
	Mono  bool
}

// The kinds whose object document gives the panel facts (design.md decision 5).
const (
	kindDeployment  = "Deployment"
	kindStatefulSet = "StatefulSet"
	kindDaemonSet   = "DaemonSet"
	kindReplicaSet  = "ReplicaSet"
	kindPod         = "Pod"
	kindService     = "Service"
)

// factKinds maps a kind to the group its facts are read for. Nothing else is
// read, so the facts are a fixed list.
var factKinds = map[string]string{
	kindDeployment: "apps", kindStatefulSet: "apps", kindDaemonSet: "apps", kindReplicaSet: "apps",
	kindPod: "", kindService: "",
}

// objectFacts reads the facts of the object a node stands for, from the
// object document the YAML view serves. It reads only for a node the caller
// may read, that is not a Secret and whose kind has facts: a locked, forbidden
// or unreadable node makes no request (OP-A3). unread is true when the
// document could not be read; the facts are then empty and the panel says so.
func (h *Handler) objectFacts(r *http.Request, n *v1.GraphNode, ctx panelContext) (facts []fact, unread bool) {
	if n.Ref == nil || n.Access != v1.AccessOK || n.Missing || ctx.base == "" || isSecret(*n.Ref) {
		return nil, false
	}
	group, ok := factKinds[n.Ref.Kind]
	if !ok || group != n.Ref.Group {
		return nil, false
	}
	var doc v1.Object
	if h.fetch(r, ctx.base+"/object", refQuery(*n.Ref), &doc) != nil {
		return nil, true
	}
	return factsOf(n.Ref.Kind, doc.Object), false
}

// factsOf lists the facts of kind from its object document.
func factsOf(kind string, obj map[string]any) []fact {
	var out []fact
	switch kind {
	case kindDeployment, kindStatefulSet:
		out = append(out, replicasFact(obj))
		out = append(out, imageFacts(obj)...)
		strategy := "strategy"
		if kind == kindStatefulSet {
			strategy = "updateStrategy"
		}
		out = appendFact(out, "Strategy", strAt(obj, "spec", strategy, "type"), false)
	case kindDaemonSet:
		out = append(out, imageFacts(obj)...)
		out = appendFact(out, "Strategy", strAt(obj, "spec", "updateStrategy", "type"), false)
	case kindReplicaSet:
		out = append(out, replicasFact(obj))
	case kindPod:
		out = appendFact(out, "Node", strAt(obj, "spec", "nodeName"), true)
		out = append(out, fact{Label: "Restarts", Value: strconv.FormatInt(restarts(obj), 10)})
		out = appendFact(out, "Started", strAt(obj, "status", "startTime"), false)
	case kindService:
		out = appendFact(out, "Type", strAt(obj, "spec", "type"), false)
		out = appendFact(out, "Ports", ports(obj), true)
	}
	return out
}

func appendFact(out []fact, label, value string, mono bool) []fact {
	if value == "" {
		return out
	}
	return append(out, fact{Label: label, Value: value, Mono: mono})
}

// replicasFact is "R ready of D desired": spec.replicas defaults to one, as
// the API server defaults it, and a status without readyReplicas reads zero.
func replicasFact(obj map[string]any) fact {
	desired, ok := number(obj, "spec", "replicas")
	if !ok {
		desired = 1
	}
	ready, _ := number(obj, "status", "readyReplicas")
	return fact{Label: "Replicas", Value: fmt.Sprintf("%d ready of %d desired", ready, desired)}
}

// imageFacts is the image of each container of a workload's pod template, one
// fact each, without userinfo (OP-A3).
func imageFacts(obj map[string]any) []fact {
	var out []fact
	for _, c := range list(obj, "spec", "template", "spec", "containers") {
		if img := strAt(c, "image"); img != "" {
			out = append(out, fact{Label: "Image", Value: imageText(img), Mono: true})
		}
	}
	return out
}

// restarts sums the restart counts of a Pod's containers.
func restarts(obj map[string]any) int64 {
	var n int64
	for _, st := range list(obj, "status", "containerStatuses") {
		c, _ := number(st, "restartCount")
		n += c
	}
	return n
}

// ports is a Service's ports as "port/protocol → targetPort", comma separated.
func ports(obj map[string]any) string {
	var parts []string
	for _, p := range list(obj, "spec", "ports") {
		port, ok := number(p, "port")
		if !ok {
			continue
		}
		proto := strAt(p, "protocol")
		if proto == "" {
			proto = "TCP"
		}
		s := fmt.Sprintf("%d/%s", port, proto)
		if t := scalar(p, "targetPort"); t != "" {
			s += " → " + t
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// --- Reading an unstructured document ---

// at walks obj down the keys; it returns nil on any miss or on a value that
// is not a map on the way.
func at(obj map[string]any, keys ...string) any {
	var cur any = obj
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func strAt(obj map[string]any, keys ...string) string {
	s, _ := at(obj, keys...).(string)
	return s
}

// number reads an integer a JSON document carries as a float64 (or as an
// integer type when a test builds the document by hand).
func number(obj map[string]any, keys ...string) (int64, bool) {
	switch v := at(obj, keys...).(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

// scalar reads a string or a number as text: a Service's targetPort is either.
func scalar(obj map[string]any, keys ...string) string {
	switch v := at(obj, keys...).(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	}
	return ""
}

// list reads a list of objects, leaving out an entry that is not one.
func list(obj map[string]any, keys ...string) []map[string]any {
	items, _ := at(obj, keys...).([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// --- Images ---

// schemePrefix is a URL scheme at the start of an image reference. A scheme
// holds no colon, so a user name with a colon before "://" is not one.
var schemePrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// digestSuffix is the tail of a reference that carries a digest: an at sign,
// one of three hash names, a colon and hex to the end of the string. The list
// is fixed: a tag that is a 40-digit git sha would match a general
// "<algorithm>:<hex>" form and keep a password (OP-A3).
var digestSuffix = regexp.MustCompile(`^@(?:sha256|sha384|sha512):[0-9A-Fa-f]+$`)

// imageText is an image reference without userinfo. It keeps any leading
// "scheme://" and cuts the rest after its last "@" that does not start a
// digest suffix, wherever a "/" stands: a password may hold one (OP-A3).
func imageText(s string) string {
	prefix := schemePrefix.FindString(s)
	rest := s[len(prefix):]
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '@' && !digestSuffix.MatchString(rest[i:]) {
			return prefix + rest[i+1:]
		}
	}
	return s
}
