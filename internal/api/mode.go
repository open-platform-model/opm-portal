package api

import (
	"fmt"
	"maps"
	"slices"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// Mode is where the portal runs. It decides what a document may carry, not
// what is read or for whom.
type Mode string

const (
	// ModeLocal serves one user with their own kubeconfig: operator text is
	// served as written, since the user's kubeconfig reads it with kubectl.
	ModeLocal Mode = "local"
	// ModeInCluster serves many users from one process: operator text is
	// omitted (omitOperatorText).
	ModeInCluster Mode = "in-cluster"
)

func (m Mode) valid() bool { return m == ModeLocal || m == ModeInCluster }

// omitOperatorText returns doc without the text the operator wrote: the
// message of every condition, reconcile state, history entry and
// registration, every event note (folding the events that leaves alike), the
// health message of an OPM object (kstatus copies its Ready message into
// it), and the condition and history messages in the raw status of an OPM
// object. Reasons, states, tone,
// meaning and next step stay. The operator's kernel does not redact secret
// values it copies into that text, and the portal cannot tell a secret from
// other text, so in-cluster it serves none of it.
//
// It errors on a document type it does not know, so a new document cannot
// leave an in-cluster server with text this function never saw.
func omitOperatorText(doc any) (any, error) {
	switch d := doc.(type) {
	case v1.InstanceList:
		for i := range d.Items {
			omitReconcile(&d.Items[i].Reconcile)
		}
		return d, nil
	case v1.Instance:
		omitDetail(&d.Reconcile, d.Conditions, d.History, d.Components)
		return d, nil
	case v1.PackageList:
		for i := range d.Items {
			omitReconcile(&d.Items[i].Reconcile)
		}
		return d, nil
	case v1.Package:
		omitDetail(&d.Reconcile, d.Conditions, d.History, d.Components)
		return d, nil
	case v1.Platform:
		omitPlatform(&d)
		return d, nil
	case v1.EventList:
		d.Items = foldWithoutNotes(d.Items)
		return d, nil
	case v1.Graph:
		for i := range d.Nodes {
			omitGraphNode(&d.Nodes[i])
		}
		return d, nil
	case v1.Object:
		if d.Ref.Group == opmGroup {
			d.Object = withoutStatusMessages(d.Object)
		}
		return d, nil
	case v1.Removed:
		return d, nil
	}
	return nil, fmt.Errorf("omitting operator text: unknown document type %T", doc)
}

// foldWithoutNotes drops every event's note and folds the events left alike
// into one line, summing their counts and keeping the latest time seen. The
// read model folds on the note too (0030:D9:R3), so without the second fold
// a note that differs between events (the operator's Applied note counts
// resources) would leave rows that look the same, and their number would
// tell how many distinct notes were hidden. Events stay newest first.
func foldWithoutNotes(items []v1.Event) []v1.Event {
	type key struct {
		typ, reason, controller, fieldPath string
		regarding                          v1.ObjectRef
	}
	at := map[key]int{}
	out := make([]v1.Event, 0, len(items))
	for i := range items {
		ev := items[i]
		ev.Note = ""
		k := key{ev.Type, ev.Reason, ev.ReportingController, ev.FieldPath, ev.Regarding}
		j, seen := at[k]
		if !seen {
			at[k] = len(out)
			out = append(out, ev)
			continue
		}
		out[j].Count += ev.Count
		if ev.LastSeen != nil && (out[j].LastSeen == nil || ev.LastSeen.After(*out[j].LastSeen)) {
			out[j].LastSeen = ev.LastSeen
		}
	}
	slices.SortStableFunc(out, func(x, y v1.Event) int { return newestFirst(x.LastSeen, y.LastSeen) })
	return out
}

// newestFirst orders two event times newest first, an event with no time
// last.
func newestFirst(x, y *time.Time) int {
	switch {
	case x == nil && y == nil:
		return 0
	case x == nil:
		return 1
	case y == nil:
		return -1
	}
	return y.Compare(*x)
}

// omitDetail omits the operator text of an instance's or package's record.
func omitDetail(r *v1.Reconcile, cs []v1.Condition, hs []v1.HistoryEntry, comps []v1.Component) {
	omitReconcile(r)
	omitConditions(cs)
	omitHistory(hs)
	omitComponents(comps)
}

func omitPlatform(p *v1.Platform) {
	omitReconcile(&p.Reconcile)
	omitConditions(p.Conditions)
	for i := range p.Registrations {
		r := &p.Registrations[i]
		r.Message, r.ActiveMessage = "", ""
		omitReconcile(&r.Reconcile)
	}
}

func omitConditions(cs []v1.Condition) {
	for i := range cs {
		cs[i].Message = ""
	}
}

// omitReconcile drops the reconcile message whole: it is the deciding
// condition's message, or a sentence of the portal's whose meaning the
// state already carries.
func omitReconcile(r *v1.Reconcile) {
	r.Message = ""
	omitConditions(r.Notes)
}

func omitHistory(hs []v1.HistoryEntry) {
	for i := range hs {
		hs[i].Message = ""
	}
}

// omitComponents drops the health message of every OPM inventory object. A
// workload's health message (a Pod waiting on an image) is the API server's
// and the kubelet's text, not the operator's, and stays.
func omitComponents(cs []v1.Component) {
	for i := range cs {
		for j := range cs[i].Objects {
			o := &cs[i].Objects[j]
			if o.Ref.Group == opmGroup && o.Health != nil {
				h := *o.Health
				h.Message = ""
				o.Health = &h
			}
		}
	}
}

func omitGraphNode(n *v1.GraphNode) {
	if r := n.Reconcile; r != nil {
		c := *r
		omitReconcile(&c)
		n.Reconcile = &c
	}
	if r := n.Registration; r != nil {
		c := *r
		c.Message, c.ActiveMessage = "", ""
		n.Registration = &c
	}
	if h := n.Health; h != nil && n.Ref != nil && n.Ref.Group == opmGroup {
		c := *h
		c.Message = ""
		n.Health = &c
	}
}

// withoutStatusMessages returns obj with the message of every entry of
// status.conditions and status.history removed, copying what it changes so
// the object it was given is left as it was.
func withoutStatusMessages(obj map[string]any) map[string]any {
	status, ok := obj["status"].(map[string]any)
	if !ok {
		return obj
	}
	status = maps.Clone(status)
	for _, field := range []string{"conditions", "history"} {
		list, ok := status[field].([]any)
		if !ok {
			continue
		}
		out := make([]any, len(list))
		for i, e := range list {
			if m, ok := e.(map[string]any); ok {
				m = maps.Clone(m)
				delete(m, "message")
				e = m
			}
			out[i] = e
		}
		status[field] = out
	}
	obj = maps.Clone(obj)
	obj["status"] = status
	return obj
}
