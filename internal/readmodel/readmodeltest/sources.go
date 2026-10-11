package readmodeltest

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FluxGroup is the API group of the Flux source kinds a ModulePackage's
// sourceRef may name.
const FluxGroup = "source.toolkit.fluxcd.io"

// FluxKinds are the three source kinds the controller reads. Kinds does not
// hold them, because the F1 cluster does not run Flux: a test that wants a
// cluster serving them passes KindsWithFlux to Discovery, and one that wants
// a Flux installed later calls SetDiscovery with it after the Model starts.
var FluxKinds = []Kind{
	{schema.GroupVersionKind{Group: FluxGroup, Version: "v1", Kind: "OCIRepository"}, "ocirepositories", true},
	{schema.GroupVersionKind{Group: FluxGroup, Version: "v1", Kind: "GitRepository"}, "gitrepositories", true},
	{schema.GroupVersionKind{Group: FluxGroup, Version: "v1", Kind: "Bucket"}, "buckets", true},
}

// KindsWithFlux returns Kinds and FluxKinds.
func KindsWithFlux() []Kind {
	return append(slices.Clone(Kinds), FluxKinds...)
}

// ConstructedAnnotation marks an object a fixture built by hand, on a
// cluster that never held it.
const ConstructedAnnotation = "testing.opmodel.dev/constructed"

// The constructed Flux sources and package of WithSources and
// WithUnversionedSourcePackage. SourceReady is the OCIRepository F1's
// package pkg/podinfo names.
const (
	SourceNamespace  = "pkg"
	SourceReady      = "podinfo-release"
	SourceNotReady   = "podinfo-notready"
	SourceNoArtifact = "podinfo-noartifact"

	// SourceRevision is the artifact revision of SourceReady, in the form
	// source-controller writes for an OCI artifact: tag, at sign, digest.
	SourceRevision = "v6.7.0@sha256:4f1c9a2e8b0d5a37c61e94f2d08b7a53e1c96d4f0a8b2c7e5d3f19a6b4c8e027"
	// SourceNotReadyReason and SourceNotReadyMessage are what
	// source-controller writes when the registry does not hold the tag.
	SourceNotReadyReason  = "OCIOperationFailed"
	SourceNotReadyMessage = "failed to pull artifact from 'oci://ghcr.io/stefanprodan/manifests/podinfo': GET https://ghcr.io/v2/stefanprodan/manifests/podinfo/manifests/6.7.0: MANIFEST_UNKNOWN: manifest unknown"
	// SourceArtifactURL is the in-cluster fetch URL of SourceReady's artifact,
	// which the portal must never serve.
	SourceArtifactURL = "http://source-controller.flux-system.svc.cluster.local./ocirepository/pkg/podinfo-release/4f1c9a2e8b0d5a37c61e94f2d08b7a53e1c96d4f0a8b2c7e5d3f19a6b4c8e027.tar.gz"

	// UnversionedPackage is the name of the package WithUnversionedSourcePackage adds.
	UnversionedPackage = "podinfo-unversioned"
)

// WithSources returns objs plus three constructed OCIRepositories in
// SourceNamespace, which F1's package pkg/podinfo and the source
// kinds' discovery lack: SourceReady (Ready=True with an artifact revision),
// SourceNotReady (Ready=False with a reason and message, and no artifact) and
// SourceNoArtifact (Ready=True with no status.artifact, which the controller
// counts as not ready).
//
// CONSTRUCTED, not captured: the fixture cluster does not run Flux
// (F1's package reads "no matches for kind OCIRepository"). Each object is
// the shape fluxcd source-controller v1 writes, with the names F1's package
// pkg/podinfo gives its sourceRef (name podinfo-release, namespace pkg), and
// carries ConstructedAnnotation.
func WithSources(t testing.TB, objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	t.Helper()
	return append(slices.Clone(objs),
		ociRepository(SourceReady, "6f3c1b9a-0b57-4d6e-8b1e-5d1a2f3c4e01", readyCondition("True", "Succeeded",
			"stored artifact for digest '"+SourceRevision+"'"), true),
		ociRepository(SourceNotReady, "6f3c1b9a-0b57-4d6e-8b1e-5d1a2f3c4e02", readyCondition("False", SourceNotReadyReason,
			SourceNotReadyMessage), false),
		ociRepository(SourceNoArtifact, "6f3c1b9a-0b57-4d6e-8b1e-5d1a2f3c4e03", readyCondition("True", "Succeeded",
			"stored artifact for digest '"+SourceRevision+"'"), false),
	)
}

func readyCondition(status, reason, message string) []any {
	return []any{map[string]any{
		"type": "Ready", "status": status, "reason": reason, "message": message,
		"observedGeneration": int64(1), "lastTransitionTime": "2026-10-04T18:04:00Z",
	}}
}

func ociRepository(name, uid string, conds []any, artifact bool) *unstructured.Unstructured {
	status := map[string]any{"conditions": conds, "observedGeneration": int64(1)}
	if artifact {
		status["artifact"] = map[string]any{
			"path":           "ocirepository/pkg/" + name + "/4f1c9a2e.tar.gz",
			"url":            SourceArtifactURL,
			"revision":       SourceRevision,
			"digest":         "sha256:7d2a96c0e51f4b38a9d60c7e1f5b8a24c3e90d17b6f4a85e2c1d7b93f06a4e58",
			"lastUpdateTime": "2026-10-04T18:04:00Z",
			"size":           int64(4096),
		}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": FluxGroup + "/v1",
		"kind":       "OCIRepository",
		"metadata": map[string]any{
			"name": name, "namespace": SourceNamespace, "uid": uid, "generation": int64(1),
			"resourceVersion":   "1200",
			"creationTimestamp": "2026-10-04T18:03:50Z",
			"annotations":       map[string]any{ConstructedAnnotation: "true"},
		},
		"spec": map[string]any{
			"interval": "1m",
			"url":      "oci://ghcr.io/stefanprodan/manifests/podinfo",
			"ref":      map[string]any{"tag": "6.7.0"},
		},
		"status": status,
	}}
}

// WithUnversionedSourcePackage returns objs plus a ModulePackage
// pkg/podinfo-unversioned whose spec.sourceRef names kind OCIRepository and
// the name SourceReady and no apiVersion, which the controller accepts
// because it reads the group from the kind alone (opm-operator
// internal/source/resolve.go).
//
// CONSTRUCTED, not captured: the package is pkg/podinfo renamed, with the
// apiVersion removed from its sourceRef. Like F1's package it holds no
// inventory.
func WithUnversionedSourcePackage(t testing.TB, objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	t.Helper()
	pkg := Find(t, objs, "ModulePackage", "podinfo").DeepCopy()
	pkg.SetName(UnversionedPackage)
	pkg.SetUID("00000000-0000-4000-8000-00000000c0df")
	pkg.SetAnnotations(map[string]string{ConstructedAnnotation: "true"})
	unstructured.RemoveNestedField(pkg.Object, "spec", "sourceRef", "apiVersion")
	return append(slices.Clone(objs), pkg)
}
