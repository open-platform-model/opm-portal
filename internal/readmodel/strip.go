package readmodel

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// lastAppliedAnnotation holds a full copy of a client-side applied object,
// spec.values included (capture, observation 14).
const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// valuesKinds are the OPM kinds whose spec.values the portal never holds
// (portal:D8:R2).
var valuesKinds = map[schema.GroupKind]bool{
	{Group: opmGroup, Kind: "ModuleInstance"}: true,
	{Group: opmGroup, Kind: "ModulePackage"}:  true,
}

// droppedFields are bulky fields health never reads, removed to bound the
// memory inventory objects take. A test runs health on every captured object
// before and after and requires the same answer.
var droppedFields = map[schema.GroupKind][][]string{
	{Group: "apps", Kind: "Deployment"}:   {{"spec", "template"}},
	{Group: "apps", Kind: "StatefulSet"}:  {{"spec", "template"}},
	{Group: "apps", Kind: "DaemonSet"}:    {{"spec", "template"}},
	{Group: "apps", Kind: kindReplicaSet}: {{"spec", "template"}},
	{Group: "batch", Kind: "Job"}:         {{"spec", "template"}},
	{Group: "batch", Kind: "CronJob"}:     {{"spec", "jobTemplate"}},
	{Kind: "ConfigMap"}:                   {{"data"}, {"binaryData"}},
}

var crdKind = schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}

// stripTransform is set on every informer: it strips each object before the
// informer stores it. Anything that is not an object passes through.
func stripTransform(obj any) (any, error) {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		strip(u)
	}
	return obj, nil
}

// strip removes, in place, everything the read model must not hold: managed
// fields and the last-applied annotation from every object, spec.values from
// ModuleInstances and ModulePackages (portal:D8:R2/R3), and the bulky fields of
// droppedFields. Objects reach it freshly decoded, so nothing else shares
// them.
func strip(u *unstructured.Unstructured) {
	stripWithheld(u)
	gk := u.GroupVersionKind().GroupKind()
	for _, path := range droppedFields[gk] {
		unstructured.RemoveNestedField(u.Object, path...)
	}
	if gk == crdKind {
		dropCRDSchemas(u)
	}
}

// stripWithheld removes, in place, what the portal never serves: managed
// fields and the last-applied annotation from every object, and spec.values
// from ModuleInstances and ModulePackages. It keeps everything else, so an
// object read for a YAML view shows what kubectl shows.
func stripWithheld(u *unstructured.Unstructured) {
	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	if ann := u.GetAnnotations(); ann != nil {
		if _, ok := ann[lastAppliedAnnotation]; ok {
			delete(ann, lastAppliedAnnotation)
			u.SetAnnotations(ann)
		}
	}
	if valuesKinds[u.GroupVersionKind().GroupKind()] {
		unstructured.RemoveNestedField(u.Object, "spec", "values")
	}
}

// dropCRDSchemas removes spec.versions[].schema, which is most of a CRD's
// size and which health does not read.
func dropCRDSchemas(u *unstructured.Unstructured) {
	versions, found, err := unstructured.NestedSlice(u.Object, "spec", "versions")
	if err != nil || !found {
		return
	}
	for _, v := range versions {
		if m, ok := v.(map[string]any); ok {
			delete(m, "schema")
		}
	}
	if err := unstructured.SetNestedSlice(u.Object, versions, "spec", "versions"); err != nil {
		// The slice was just read from the same path, so this cannot fail;
		// if it did, the schemas stay and only memory is spent.
		return
	}
}
