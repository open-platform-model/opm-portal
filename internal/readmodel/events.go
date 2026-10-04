package readmodel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// events is the events API the read model reads (tier 4): the one that
// carries regarding, series and the reporting controller.
var events = schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}

// Event is one line of an object's recent-activity feed: every event about
// the object with the same type, reason and note, folded into one. Events
// expire after about an hour and are never the source of a status
// (0030:D9:R1/R2).
type Event struct {
	Type                string
	Reason              string
	Note                string
	ReportingController string
	Regarding           ObjectRef
	FieldPath           string
	// Count sums the repeats, however each was recorded: a separate event,
	// an event series, or the deprecated count kubelet events use.
	Count int64
	// LastSeen is the latest occurrence among the folded events.
	LastSeen time.Time
}

// EventNamespace returns the namespace the events about an object live in:
// its own, or default for a cluster-scoped object such as the Platform or a
// TransformerRegistration (capture, observation 5).
func EventNamespace(about ObjectRef) string {
	if about.Namespace == "" {
		return eventNamespaceForClusterScoped
	}
	return about.Namespace
}

// eventNamespaceForClusterScoped is where Kubernetes records events about
// cluster-scoped objects (0030:D9:R4).
const eventNamespaceForClusterScoped = "default"

// Events returns the recent events about one object, read when asked,
// folded per 0030:D9:R3 and newest first. g must cover list events in
// EventNamespace(about). The reader must be allowed the same list, or the
// feed is unavailable.
func (m *Model) Events(ctx context.Context, who authz.Identity, g authz.Grant, about ObjectRef) ([]Event, error) {
	namespace := EventNamespace(about)
	if err := covers(who, g, "list", events, namespace, ""); err != nil {
		return nil, err
	}
	if !m.readerMay(ctx, "list", events, namespace, "") {
		return nil, ErrUnavailable
	}
	selector := fields.Set{
		"regarding.kind":      about.Kind,
		"regarding.name":      about.Name,
		"regarding.namespace": about.Namespace,
	}.AsSelector().String()
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	list, err := m.cfg.Dynamic.Resource(events).Namespace(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("%w: listing events: %w", ErrUnavailable, err)
	}
	var matching []*unstructured.Unstructured
	for i := range list.Items {
		// The API server has applied the selector; this repeats it so the
		// feed never shows an event about another object.
		if regards(&list.Items[i], about) {
			matching = append(matching, &list.Items[i])
		}
	}
	return foldEvents(matching), nil
}

// regards reports whether ev is about the object about.
func regards(ev *unstructured.Unstructured, about ObjectRef) bool {
	r := object(ev.Object, "regarding")
	if str(r, "kind") != about.Kind || str(r, "name") != about.Name || str(r, "namespace") != about.Namespace {
		return false
	}
	if about.Group == "" {
		return true
	}
	gv, err := schema.ParseGroupVersion(str(r, "apiVersion"))
	return err == nil && gv.Group == about.Group
}

// foldEvents folds events with the same regarded object (by uid), type,
// reason and note into one line. The event recorder folds only repeats about
// an unchanged object version into a series, and kubelet events count
// through the deprecated fields, so the portal folds itself (0030:D9:R3).
func foldEvents(evs []*unstructured.Unstructured) []Event {
	byKey := map[string]int{}
	var out []Event
	for _, ev := range evs {
		r := object(ev.Object, "regarding")
		line := Event{
			Type:                str(ev.Object, "type"),
			Reason:              str(ev.Object, "reason"),
			Note:                str(ev.Object, "note"),
			ReportingController: str(ev.Object, "reportingController"),
			Regarding:           regardingRef(r),
			FieldPath:           str(r, "fieldPath"),
			Count:               occurrences(ev),
			LastSeen:            lastSeen(ev),
		}
		key := strings.Join([]string{str(r, "uid"), line.Type, line.Reason, line.Note}, "\x00")
		i, seen := byKey[key]
		if !seen {
			byKey[key] = len(out)
			out = append(out, line)
			continue
		}
		out[i].Count += line.Count
		if line.LastSeen.After(out[i].LastSeen) {
			out[i].LastSeen = line.LastSeen
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

// occurrences is how many times one event object records its event
// happening.
func occurrences(ev *unstructured.Unstructured) int64 {
	if n, ok := i64(ev.Object, "series", "count"); ok && n > 0 {
		return n
	}
	if n, ok := i64(ev.Object, "deprecatedCount"); ok && n > 0 {
		return n
	}
	return 1
}

// lastSeen is the latest time one event object records. Kubelet events have
// no eventTime (capture, observation 4).
func lastSeen(ev *unstructured.Unstructured) time.Time {
	var latest time.Time
	for _, path := range [][]string{
		{"series", "lastObservedTime"},
		{"eventTime"},
		{"deprecatedLastTimestamp"},
		{"metadata", "creationTimestamp"},
	} {
		if t := timestamp(ev.Object, path...); t.After(latest) {
			latest = t
		}
	}
	return latest
}

func regardingRef(r map[string]any) ObjectRef {
	gv, err := schema.ParseGroupVersion(str(r, "apiVersion"))
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return ObjectRef{Group: gv.Group, Version: gv.Version, Kind: str(r, "kind"), Namespace: str(r, "namespace"), Name: str(r, "name")}
}
