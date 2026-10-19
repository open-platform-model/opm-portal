package readmodel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
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
// (portal:D9:R1/R2).
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
// cluster-scoped objects (portal:D9:R4).
const eventNamespaceForClusterScoped = "default"

// Events returns the recent events about one object, read when asked,
// folded per portal:D9:R3 and newest first. g must cover list events in
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

// eventListConcurrency bounds the lists EventsAbout runs at once.
const eventListConcurrency = 4

// eventGroup is one list of EventsAbout: the events of one regarded kind in
// one event namespace.
type eventGroup struct {
	namespace, kind string
	refs            int
}

// EventsAbout returns the folded events about each of refs, merged newest
// first (portal:D9:R6). It reads one list per event namespace and regarded
// kind, never one per object, at most four at a time under readTimeout; each
// list carries a regarding.kind field selector, and the result is filtered to
// refs by group, kind, namespace and name, so the feed never shows an event
// about another object.
//
// refs[0] is the owner. owner is the caller's grant to list events in the
// owner's event namespace, which is not optional: the reader denied there,
// or a list failing there, is ErrUnavailable, as for Events. others maps
// every other event namespace to the caller's grant to list events there. A
// ref whose namespace has no grant, whose namespace the reader may not list,
// or whose list failed is left out and counted in skipped; no list runs in a
// namespace without a grant or without the reader's.
//
// The caller decides which refs the caller may read: EventsAbout reads
// events about the refs it is given and nothing else (portal:D7).
func (m *Model) EventsAbout(ctx context.Context, who authz.Identity, owner authz.Grant, others map[string]authz.Grant, refs []ObjectRef) (evs []Event, skipped int, err error) {
	if len(refs) == 0 {
		return nil, 0, nil
	}
	ownerNamespace := EventNamespace(refs[0])
	grants := make(map[string]authz.Grant, len(others)+1)
	for ns, g := range others {
		grants[ns] = g
	}
	grants[ownerNamespace] = owner
	wanted, groups := groupRefs(refs)
	runnable, skipped, err := m.runnableGroups(ctx, who, grants, groups, ownerNamespace)
	if err != nil {
		return nil, 0, err
	}
	found, failed, err := m.listGroups(ctx, runnable, wanted, ownerNamespace)
	if err != nil {
		return nil, 0, err
	}
	var matching []*unstructured.Unstructured
	for _, part := range found {
		matching = append(matching, part...)
	}
	return foldEvents(matching), skipped + failed, nil
}

// groupRefs names the wanted refs and groups them into one list each: by
// event namespace and regarded kind, in a fixed order.
func groupRefs(refs []ObjectRef) (wanted map[string]bool, groups []*eventGroup) {
	wanted = make(map[string]bool, len(refs))
	byGroup := map[[2]string]*eventGroup{}
	for _, r := range refs {
		key := reachKey(r.Group, r.Kind, r.Namespace, r.Name)
		if wanted[key] {
			continue
		}
		wanted[key] = true
		gk := [2]string{EventNamespace(r), r.Kind}
		if byGroup[gk] == nil {
			byGroup[gk] = &eventGroup{namespace: gk[0], kind: gk[1]}
		}
		byGroup[gk].refs++
	}
	groups = make([]*eventGroup, 0, len(byGroup))
	for _, g := range byGroup {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].namespace != groups[j].namespace {
			return groups[i].namespace < groups[j].namespace
		}
		return groups[i].kind < groups[j].kind
	})
	return wanted, groups
}

// runnableGroups decides each event namespace once, before any list runs: a
// namespace without a grant, or that the portal's reader may not list, is
// skipped and its refs counted. The owner's own namespace is not optional.
func (m *Model) runnableGroups(ctx context.Context, who authz.Identity, grants map[string]authz.Grant, groups []*eventGroup, ownerNamespace string) (runnable []*eventGroup, skipped int, err error) {
	readable := map[string]bool{}
	for _, g := range groups {
		ok, decided := readable[g.namespace]
		if !decided {
			grant, granted := grants[g.namespace]
			if granted {
				if err := covers(who, grant, "list", events, g.namespace, ""); err != nil {
					return nil, 0, err
				}
				ok = m.readerMay(ctx, "list", events, g.namespace, "")
			}
			readable[g.namespace] = ok
			if !ok && g.namespace == ownerNamespace && granted {
				return nil, 0, ErrUnavailable
			}
		}
		if !ok {
			skipped += g.refs
			continue
		}
		runnable = append(runnable, g)
	}
	return runnable, skipped, nil
}

// listGroups runs the lists, at most eventListConcurrency at a time. Each list
// writes its own slot, so the lines reach the fold in a fixed order whatever
// order the lists finish in: lines with one time keep it. failed counts the
// refs of a list that failed outside the owner's namespace; one that fails
// there fails the feed.
func (m *Model) listGroups(ctx context.Context, runnable []*eventGroup, wanted map[string]bool, ownerNamespace string) (found [][]*unstructured.Unstructured, failed int, err error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		ownerErr error
		slots    = make(chan struct{}, eventListConcurrency)
	)
	found = make([][]*unstructured.Unstructured, len(runnable))
	for i, g := range runnable {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			items, err := m.listEventsRegarding(ctx, g)
			if err != nil {
				mu.Lock()
				defer mu.Unlock()
				if g.namespace == ownerNamespace && ownerErr == nil {
					ownerErr = fmt.Errorf("%w: listing events: %w", ErrUnavailable, err)
				}
				failed += g.refs
				return
			}
			for j := range items {
				// The server applied the kind selector; this repeats it, so an
				// event is taken from the one list of its own kind.
				r := regardingRef(object(items[j].Object, "regarding"))
				if r.Kind == g.kind && wanted[reachKey(r.Group, r.Kind, r.Namespace, r.Name)] {
					found[i] = append(found[i], &items[j])
				}
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if ownerErr != nil {
		return nil, 0, ownerErr
	}
	return found, failed, nil
}

// listEventsRegarding lists the events of one kind in one namespace under
// readTimeout. The field selector narrows the list on the server; the caller
// filters to the objects it wants.
func (m *Model) listEventsRegarding(ctx context.Context, g *eventGroup) ([]unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	selector := fields.Set{"regarding.kind": g.kind}.AsSelector().String()
	list, err := m.cfg.Dynamic.Resource(events).Namespace(g.namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// reachKey names an object for the filter of EventsAbout.
func reachKey(group, kind, namespace, name string) string {
	return strings.Join([]string{group, kind, namespace, name}, "\x00")
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
// through the deprecated fields, so the portal folds itself (portal:D9:R3).
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
