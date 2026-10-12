package readmodeltest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ServiceNameLabel is the label that ties a slice to its Service.
const ServiceNameLabel = "kubernetes.io/service-name"

// lastAppliedAnnotation is the annotation a client-side apply fills with
// the whole applied object.
const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// PodinfoServiceName and PodinfoServiceNamespace name the F1 Service whose
// slice WithEndpointSlices adds.
const (
	PodinfoServiceName      = "podinfo-podinfo"
	PodinfoServiceNamespace = "default"
)

// endpointSlicesFile is the live capture of the podinfo Service's slice,
// relative to this package's directory.
var endpointSlicesFile = filepath.Join("..", "testdata", "endpointslices-podinfo.yaml")

// Sentinels are marker strings WithSentinels writes into a copy of the
// recorded slice, so a leak test can search a serving for them. Address,
// PodName and PortName get the index of their endpoint or port appended, so
// that the endpoints stay distinct. LabelValue replaces every label value
// except the one of ServiceNameLabel, which the portal selects by.
type Sentinels struct {
	Address    string
	Hostname   string
	NodeName   string
	Zone       string
	PodName    string
	PortName   string
	LabelValue string
}

// All returns the sentinel strings, for a leak test to search for.
func (s Sentinels) All() []string {
	return []string{s.Address, s.Hostname, s.NodeName, s.Zone, s.PodName, s.PortName, s.LabelValue}
}

// SliceOption changes one copy WithEndpointSlices adds.
type SliceOption func(t testing.TB, sl *unstructured.Unstructured)

// WithSentinels writes s into every address, hostname, node name, zone,
// target Pod name, port name and label value of each copy.
func WithSentinels(s Sentinels) SliceOption {
	return func(t testing.TB, sl *unstructured.Unstructured) {
		t.Helper()
		n := 0
		eachMap(t, sl, "endpoints", func(e map[string]any) {
			e["addresses"] = []any{s.Address + "-" + strconv.Itoa(n)}
			e["hostname"] = s.Hostname
			e["nodeName"] = s.NodeName
			e["zone"] = s.Zone
			if ref, ok := e["targetRef"].(map[string]any); ok {
				ref["name"] = s.PodName + "-" + strconv.Itoa(n)
			}
			n++
		})
		n = 0
		eachMap(t, sl, "ports", func(p map[string]any) {
			p["name"] = s.PortName + "-" + strconv.Itoa(n)
			n++
		})
		labels := sl.GetLabels()
		for k := range labels {
			if k != ServiceNameLabel {
				labels[k] = s.LabelValue
			}
		}
		sl.SetLabels(labels)
	}
}

// WithNoReadyEndpoints sets conditions.ready false on every endpoint of each
// copy, the form of a Service whose Pods all fail their readiness probe.
func WithNoReadyEndpoints() SliceOption {
	return func(t testing.TB, sl *unstructured.Unstructured) {
		t.Helper()
		eachMap(t, sl, "endpoints", func(e map[string]any) {
			e["conditions"] = map[string]any{"ready": false, "serving": false, "terminating": false}
		})
	}
}

// eachMap calls fn on every map of the list field of sl, then writes the list
// back. A field that is absent is skipped.
func eachMap(t testing.TB, sl *unstructured.Unstructured, field string, fn func(map[string]any)) {
	t.Helper()
	list, found, err := unstructured.NestedSlice(sl.Object, field)
	if err != nil {
		t.Fatalf("%s of %s: %v", field, sl.GetName(), err)
	}
	if !found {
		return
	}
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			fn(m)
		}
	}
	if err := unstructured.SetNestedSlice(sl.Object, list, field); err != nil {
		t.Fatalf("%s of %s: %v", field, sl.GetName(), err)
	}
}

// WithEndpointSlices returns objs plus the EndpointSlices of the podinfo
// Service, each owned by F1's Service default/podinfo-podinfo (the owner
// reference's uid and name are set from objs) and carrying the label
// ServiceNameLabel with that name.
//
// CAPTURED, then re-owned: internal/readmodel/testdata/
// endpointslices-podinfo.yaml was read from the e2e fixture cluster, whose
// podinfo Service has another uid than the one in F1's capture. F1 itself
// holds no slice. The slice's target Pods are the ones of that other
// cluster; nothing reads them but the sentinel tests.
func WithEndpointSlices(t testing.TB, objs []*unstructured.Unstructured, opts ...SliceOption) []*unstructured.Unstructured {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the readmodeltest sources")
	}
	recorded, err := loadEndpointSlices(filepath.Join(filepath.Dir(here), endpointSlicesFile))
	if err != nil {
		t.Fatalf("%s: %v", endpointSlicesFile, err)
	}
	svc := findService(t, objs, PodinfoServiceNamespace, PodinfoServiceName)
	out := slices.Clone(objs)
	for _, sl := range recorded {
		sl = sl.DeepCopy()
		sl.SetOwnerReferences(reown(sl, string(svc.GetUID()), svc.GetName()))
		for _, o := range opts {
			o(t, sl)
		}
		out = append(out, sl)
	}
	return out
}

func findService(t testing.TB, objs []*unstructured.Unstructured, namespace, name string) *unstructured.Unstructured {
	t.Helper()
	for _, o := range objs {
		if o.GetKind() == "Service" && o.GetNamespace() == namespace && o.GetName() == name {
			return o
		}
	}
	t.Fatalf("no Service %s/%s", namespace, name)
	return nil
}

// reown returns sl's owner references with the controller reference of kind
// Service set to uid and name. It fails the load earlier when there is none.
func reown(sl *unstructured.Unstructured, uid, name string) []metav1.OwnerReference {
	refs := sl.GetOwnerReferences()
	for i := range refs {
		if refs[i].Kind == "Service" && refs[i].Controller != nil && *refs[i].Controller {
			refs[i].UID = types.UID(uid)
			refs[i].Name = name
		}
	}
	return refs
}

// loadEndpointSlices reads the recorded file and refuses what the capture
// check refuses under testdata/clusters: a Secret, managedFields and the
// last-applied annotation. It also refuses an object that is not an
// EndpointSlice or has no controller owner reference of kind Service, so a
// stale or hand-edited file fails here and not in a later test.
func loadEndpointSlices(path string) ([]*unstructured.Unstructured, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	objs, err := decodeList(raw)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, errors.New("the file holds no object")
	}
	for _, o := range objs {
		id := o.GetKind() + " " + o.GetNamespace() + "/" + o.GetName()
		switch {
		case o.GetKind() == "Secret":
			return nil, fmt.Errorf("%s: a Secret may not be recorded", id)
		case o.GetKind() != "EndpointSlice":
			return nil, fmt.Errorf("%s: only EndpointSlice objects may be recorded", id)
		case len(o.GetManagedFields()) > 0:
			return nil, fmt.Errorf("%s: metadata.managedFields must be removed", id)
		}
		if _, ok := o.GetAnnotations()[lastAppliedAnnotation]; ok {
			return nil, fmt.Errorf("%s: the %s annotation must be removed", id, lastAppliedAnnotation)
		}
		if o.GetLabels()[ServiceNameLabel] == "" {
			return nil, fmt.Errorf("%s: the %s label is missing", id, ServiceNameLabel)
		}
		if !hasControllerService(o) {
			return nil, fmt.Errorf("%s: no controller owner reference of kind Service", id)
		}
	}
	return objs, nil
}

func hasControllerService(o *unstructured.Unstructured) bool {
	for _, r := range o.GetOwnerReferences() {
		if r.Kind == "Service" && r.Controller != nil && *r.Controller {
			return true
		}
	}
	return false
}
