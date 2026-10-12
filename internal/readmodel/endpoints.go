package readmodel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// endpointSlices is the resource the ready count of a Service is read from.
// The portal keeps one number from it and never a slice.
var endpointSlices = schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}

// serviceNameLabel ties an EndpointSlice to its Service.
const serviceNameLabel = "kubernetes.io/service-name"

// The address types the EndpointSlice controller makes. A slice of any other
// type (FQDN) is hand-made and has no defined meaning, so it adds nothing.
const (
	addressTypeIPv4 = "IPv4"
	addressTypeIPv6 = "IPv6"
)

// endpointsState is what the read gives for one Service.
type endpointsState struct {
	Ready  *int64 // set when a count was made, 0 included
	Unread bool   // set when the read was not made or failed; never with Ready
}

// endpointKey names the Service whose count is cached. The uid is part of
// it, so a Service deleted and made again does not inherit its predecessor's
// count.
type endpointKey struct {
	namespace, name string
	uid             types.UID
}

// endpointListing is the cached outcome of one successful list: the count
// and whether any slice was kept. It holds no slice.
type endpointListing struct {
	at    time.Time
	ready int64
	found bool
}

// withEndpoints fills EndpointsReady or EndpointsUnread on the Service objects
// of comps. Only InstanceWithEndpoints and PackageWithEndpoints reach it,
// after componentsOf; Instance, Package, inventoryHealth, componentsOf, the
// list views and ReachPod do not, so a view that does not show the count asks
// for no review and makes no list of EndpointSlices.
func (e *evaluator) withEndpoints(ctx context.Context, ev evaluation, comps []Component) {
	for i := range ev.entries {
		entry := ev.entries[i]
		if !isReadableService(entry) {
			continue
		}
		st := e.endpointsOf(ctx, entry)
		if st == (endpointsState{}) {
			continue
		}
		ref := objectRefOf(entry.Ref)
		for c := range comps {
			if comps[c].Name != entry.Ref.Component {
				continue
			}
			for o := range comps[c].Objects {
				if obj := &comps[c].Objects[o]; obj.Ref == ref {
					obj.EndpointsReady = st.Ready
					obj.EndpointsUnread = st.Unread
				}
			}
		}
	}
}

// evaluated reads owner's inventory for who and groups it by component, with
// the ready count of each Service when withEndpoints is set. Instance and
// Package share it.
func (m *Model) evaluated(ctx context.Context, who authz.Identity, owner *unstructured.Unstructured, withEndpoints bool) (evaluation, []Component) {
	ev := m.newEvaluator(who)
	res := ev.inventoryHealth(ctx, owner)
	components := componentsOf(res)
	if withEndpoints {
		ev.withEndpoints(ctx, res, components)
	}
	return res, components
}

// isReadableService reports whether entry is a core Service the caller may
// read and that exists. Nothing else can carry a count.
func isReadableService(entry health.Entry) bool {
	return entry.Access == health.AccessOK && entry.Object != nil && entry.Ref.Group == "" && entry.Ref.Kind == "Service"
}

// objectRefOf copies an entry's reference into the view's form.
func objectRefOf(r health.Ref) ObjectRef {
	return ObjectRef{Group: r.Group, Version: r.Version, Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}
}

// endpointsOf decides one Service entry: the state, or the zero value for
// "show nothing". The caller's review of the slices comes first, then the
// reader's, then the read; a refusal, an undecidable review or a failed read
// is Unread and never a count of 0.
func (e *evaluator) endpointsOf(ctx context.Context, entry health.Entry) endpointsState {
	svc := entry.Object
	ns := svc.GetNamespace()
	if access := e.m.callerAccess(ctx, e.who, "list", endpointSlices, ns, ""); access != health.AccessOK {
		// The caller's own refusal is expected, not a portal fault; an
		// undecidable review is one.
		if access == health.AccessForbidden {
			unreadEndpoints(ctx, svc, slog.LevelDebug, "the caller may not list endpointslices")
		} else {
			unreadEndpoints(ctx, svc, slog.LevelWarn, "the review of the caller could not be decided")
		}
		return endpointsState{Unread: true}
	}
	if !e.m.readerMay(ctx, "list", endpointSlices, ns, "") {
		unreadEndpoints(ctx, svc, slog.LevelWarn, "the portal's reader may not list endpointslices")
		return endpointsState{Unread: true}
	}
	ready, found, err := e.m.readyEndpoints(ctx, svc)
	if err != nil {
		unreadEndpoints(ctx, svc, slog.LevelWarn, listFailure(err))
		return endpointsState{Unread: true}
	}
	if !found {
		return endpointsState{}
	}
	return endpointsState{Ready: &ready}
}

// errPartialList is the failure of a list that has more pages.
var errPartialList = errors.New("the list is not whole")

// listFailure names why a list failed, in words of the portal's own. The
// client's error text is not logged: it can carry a URL or a server message.
func listFailure(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "the list of endpointslices timed out"
	case errors.Is(err, errPartialList):
		return "the list of endpointslices has more than one page"
	default:
		return "the list of endpointslices failed"
	}
}

// unreadEndpoints logs why no count was served: the Service's namespace and
// name and a reason in words, nothing a slice holds. The level is Debug for
// the caller's own refusal and Warn for a fault of the portal or its role.
func unreadEndpoints(ctx context.Context, svc *unstructured.Unstructured, level slog.Level, reason string) {
	slog.Log(ctx, level, "endpoint count not served",
		"namespace", svc.GetNamespace(), "service", svc.GetName(), "reason", reason)
}

// readyEndpoints lists svc's slices as the reader, at most once per
// ChildrenTTL, and counts them. A failed, timed-out or partial list is an
// error and is not cached, so the next read asks again. No slice is kept.
// The caller and the reader are reviewed before this runs, on every serve.
func (m *Model) readyEndpoints(ctx context.Context, svc *unstructured.Unstructured) (ready int64, found bool, err error) {
	key := endpointKey{namespace: svc.GetNamespace(), name: svc.GetName(), uid: svc.GetUID()}
	now := m.cfg.Now()
	m.mu.Lock()
	cached, ok := m.endpointList[key]
	m.mu.Unlock()
	if ok && now.Before(cached.at.Add(m.cfg.ChildrenTTL)) {
		return cached.ready, cached.found, nil
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	list, err := m.cfg.Dynamic.Resource(endpointSlices).Namespace(key.namespace).
		List(ctx, metav1.ListOptions{LabelSelector: serviceNameLabel + "=" + key.name})
	if err != nil {
		return 0, false, fmt.Errorf("listing endpointslices: %w", err)
	}
	if list.GetContinue() != "" {
		return 0, false, fmt.Errorf("listing endpointslices: %w", errPartialList)
	}
	ready, found = countReady(list.Items, svc.GetUID())
	m.mu.Lock()
	m.endpointList[key] = endpointListing{at: now, ready: ready, found: found}
	m.mu.Unlock()
	return ready, found, nil
}

// countReady counts the ready endpoints of the Service with uid svc. A slice
// counts only if its address type is IPv4 or IPv6 and its controller owner
// reference reaches svc. Within one address type each endpoint counts once,
// keyed by its target uid, else its first address, else on its own, and as
// ready only if no copy says ready is false (an unset ready is true). The
// result is the larger sum of the two address types, so a dual-stack Service
// does not count a Pod twice. found is false when no slice was kept; a kept
// slice with no ready endpoint gives 0 and true.
func countReady(slices []unstructured.Unstructured, svc types.UID) (ready int64, found bool) {
	reach := map[types.UID]int{svc: 0}
	seen := map[string]map[string]bool{addressTypeIPv4: {}, addressTypeIPv6: {}}
	alone := 0
	for i := range slices {
		s := &slices[i]
		byKey, ok := seen[str(s.Object, "addressType")]
		if !ok {
			continue
		}
		owner, ok := controllerOf(s)
		if !ok {
			continue
		}
		if _, ok := reachInventory(owner, nil, reach); !ok {
			continue
		}
		found = true
		for _, ep := range maps(s.Object, "endpoints") {
			key := endpointIdentity(ep)
			if key == "" {
				alone++
				key = fmt.Sprintf("alone:%d", alone)
			}
			ok := endpointReady(ep)
			if prev, dup := byKey[key]; dup {
				ok = ok && prev
			}
			byKey[key] = ok
		}
	}
	for _, byKey := range seen {
		var n int64
		for _, ok := range byKey {
			if ok {
				n++
			}
		}
		ready = max(ready, n)
	}
	return ready, found
}

// endpointIdentity returns the key that tells two copies of one endpoint
// apart from two endpoints, or "" when the endpoint names neither a target
// nor an address.
func endpointIdentity(ep map[string]any) string {
	if uid := str(ep, "targetRef", "uid"); uid != "" {
		return "uid:" + uid
	}
	if addrs := strs(ep, "addresses"); len(addrs) > 0 && addrs[0] != "" {
		return "addr:" + addrs[0]
	}
	return ""
}

// endpointReady reads conditions.ready: Kubernetes documents an unset value
// as true, so only an explicit false (or a value that is not a boolean, which
// the API server does not store) is not ready. A Service with
// publishNotReadyAddresses can list a Pod that is not ready as true; the
// count follows the slice as recorded.
func endpointReady(ep map[string]any) bool {
	raw, found, err := unstructured.NestedFieldNoCopy(ep, "conditions", "ready")
	if err != nil {
		return false
	}
	if !found || raw == nil {
		return true
	}
	ready, ok := raw.(bool)
	return ok && ready
}
