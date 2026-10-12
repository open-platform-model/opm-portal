package graph

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// The two endpoint fields of a Service node: copied from the read model for
// a node whose access is ok, on a Service only, a count of 0 kept, and never
// both on one node.

const podinfoServiceNode = "obj:_/Service/default/podinfo-podinfo"

// podinfoWithEndpoints reads default/podinfo the way the graph handler does.
func (c *cluster) podinfoWithEndpoints(t testing.TB) readmodel.InstanceDetail {
	t.Helper()
	g, err := c.get(t, moduleInstances, "default", "podinfo")
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	d, err := c.m.InstanceWithEndpoints(t.Context(), alice, g, "default", "podinfo")
	if err != nil {
		t.Fatalf("InstanceWithEndpoints(default/podinfo): %v", err)
	}
	return d
}

// endpointFields fails when a node other than a Service carries either field
// or when any node carries both.
func endpointFields(t testing.TB, g Graph) {
	t.Helper()
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.EndpointsReady != nil && n.EndpointsUnread {
			t.Errorf("node %s carries both endpoint fields", n.ID)
		}
		isService := n.Ref != nil && n.Ref.Group == "" && n.Ref.Kind == "Service"
		if !isService && (n.EndpointsReady != nil || n.EndpointsUnread) {
			t.Errorf("node %s (%v) carries an endpoint field", n.ID, n.Kind)
		}
	}
}

func TestServiceNodeCarriesTheReadyCount(t *testing.T) {
	objs := readmodeltest.WithEndpointSlices(t, f1(t))
	g := Instance(newCluster(t, objs, readmodeltest.AllowAll).podinfoWithEndpoints(t), Options{})
	n := nodeByID(t, g, podinfoServiceNode)
	if n.EndpointsReady == nil || *n.EndpointsReady != 2 || n.EndpointsUnread || n.Access != health.AccessOK {
		t.Fatalf("Service node = ready %v, unread %t, access %q, want 2, false, ok", n.EndpointsReady, n.EndpointsUnread, n.Access)
	}
	endpointFields(t, g)
}

func TestServiceNodeCarriesTheUnreadMark(t *testing.T) {
	objs := readmodeltest.WithEndpointSlices(t, f1(t))
	g := Instance(newCluster(t, objs, readmodeltest.DenyResources("endpointslices")).podinfoWithEndpoints(t), Options{})
	n := nodeByID(t, g, podinfoServiceNode)
	if n.EndpointsReady != nil || !n.EndpointsUnread || n.Access != health.AccessOK {
		t.Fatalf("Service node = ready %v, unread %t, access %q, want no count, true, ok", n.EndpointsReady, n.EndpointsUnread, n.Access)
	}
	endpointFields(t, g)
}

// A count of 0 survives omitempty: it is a count, not an absent one.
func TestACountOfZeroSurvivesTheJSON(t *testing.T) {
	objs := readmodeltest.WithEndpointSlices(t, f1(t), readmodeltest.WithNoReadyEndpoints())
	g := Instance(newCluster(t, objs, readmodeltest.AllowAll).podinfoWithEndpoints(t), Options{})
	n := nodeByID(t, g, podinfoServiceNode)
	if n.EndpointsReady == nil || *n.EndpointsReady != 0 {
		t.Fatalf("EndpointsReady = %v, want a count of 0", n.EndpointsReady)
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"endpointsReady":0`) || strings.Contains(string(raw), "endpointsUnread") {
		t.Errorf("Service node JSON = %s, want endpointsReady 0 and no endpointsUnread", raw)
	}
	other, err := json.Marshal(nodeByID(t, g, "mi:default/podinfo"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(other), "endpoints") {
		t.Errorf("the root node JSON holds an endpoint field: %s", other)
	}
}

// Without slices, or from the plain view, a Service node carries neither.
func TestServiceNodeWithoutACountCarriesNeither(t *testing.T) {
	for name, g := range map[string]Graph{
		"no slice recorded": Instance(newCluster(t, f1(t), readmodeltest.AllowAll).podinfoWithEndpoints(t), Options{}),
		"the plain view": Instance(newCluster(t, readmodeltest.WithEndpointSlices(t, f1(t)), readmodeltest.AllowAll).
			instance(t, "default", "podinfo"), Options{}),
	} {
		n := nodeByID(t, g, podinfoServiceNode)
		if n.EndpointsReady != nil || n.EndpointsUnread {
			t.Errorf("%s: Service node = ready %v, unread %t, want neither", name, n.EndpointsReady, n.EndpointsUnread)
		}
		endpointFields(t, g)
	}
}

// A Service the caller may not read is a locked node: the builder copies
// nothing, whatever the view holds.
func TestAForbiddenServiceNodeCarriesNeither(t *testing.T) {
	objs := readmodeltest.WithEndpointSlices(t, f1(t))
	g := Instance(newCluster(t, objs, readmodeltest.DenyResources("services")).podinfoWithEndpoints(t), Options{})
	n := nodeByID(t, g, podinfoServiceNode)
	if n.Access != health.AccessForbidden || n.EndpointsReady != nil || n.EndpointsUnread {
		t.Fatalf("Service node = access %q, ready %v, unread %t, want forbidden and neither", n.Access, n.EndpointsReady, n.EndpointsUnread)
	}

	// The builder alone: a view object that is not readable but holds the
	// fields still gives a node without them.
	two := int64(2)
	d := newCluster(t, objs, readmodeltest.AllowAll).podinfoWithEndpoints(t)
	for c := range d.Components {
		for o := range d.Components[c].Objects {
			if obj := &d.Components[c].Objects[o]; obj.Ref.Kind == "Service" {
				obj.Access, obj.EndpointsReady, obj.EndpointsUnread = health.AccessForbidden, &two, false
			}
		}
	}
	if n := nodeByID(t, Instance(d, Options{}), podinfoServiceNode); n.EndpointsReady != nil || n.EndpointsUnread {
		t.Errorf("a forbidden object gave a node with ready %v, unread %t", n.EndpointsReady, n.EndpointsUnread)
	}
}

// Package graphs get the fields from the same builder.
func TestPackageGraphServiceNodeCarriesTheReadyCount(t *testing.T) {
	objs := readmodeltest.WithServicePackage(t, readmodeltest.WithEndpointSlices(t, f1(t)))
	c := newCluster(t, objs, readmodeltest.AllowAll)
	g, err := c.get(t, modulePackages, readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName)
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.m.PackageWithEndpoints(t.Context(), alice, g, readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName)
	if err != nil {
		t.Fatal(err)
	}
	pg := Package(d, nil, Options{})
	n := nodeByID(t, pg, podinfoServiceNode)
	if n.EndpointsReady == nil || *n.EndpointsReady != 2 || n.EndpointsUnread {
		t.Fatalf("package Service node = ready %v, unread %t, want 2 and false", n.EndpointsReady, n.EndpointsUnread)
	}
	endpointFields(t, pg)
}
