package readmodel

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// The Flux source a ModulePackage names (portal:D20). The portal reads the
// three kinds the controller reads, for the caller, with get
// only: the kind is chosen by spec.sourceRef.kind alone and the group is
// always FluxGroup, because the controller ignores sourceRef.apiVersion.
// Reading any other kind a spec field names would let a package author steer
// what the portal reads.

// FluxGroup is the API group of the Flux source kinds a ModulePackage's
// sourceRef may name.
const FluxGroup = "source.toolkit.fluxcd.io"

// fluxVersion is the version the portal reads the three kinds at.
const fluxVersion = "v1"

// fluxSources maps the three kinds to the resource each is served as.
var fluxSources = map[string]string{
	"OCIRepository": "ocirepositories",
	"GitRepository": "gitrepositories",
	"Bucket":        "buckets",
}

// SourceSupported reports whether kind is one of the three Flux source kinds
// the controller reads (OCIRepository, GitRepository, Bucket).
func SourceSupported(kind string) bool {
	_, ok := fluxSources[kind]
	return ok
}

// SourceReadiness is what the portal could say about a package's source.
type SourceReadiness string

// The readiness values. Ready is the Ready condition True with a
// status.artifact; NoArtifact is the same condition with no artifact, which
// the controller counts as not ready (opm-controller internal/source/resolve.go).
const (
	SourceReady         SourceReadiness = "ready"
	SourceNotReady      SourceReadiness = "notReady"
	SourceNoArtifact    SourceReadiness = "noArtifact"
	SourceUnknown       SourceReadiness = "unknown"
	SourceNotFound      SourceReadiness = "notFound"
	SourceKindNotServed SourceReadiness = "kindNotServed"
	SourceUnsupported   SourceReadiness = "unsupportedKind"
)

// SourceState is a package's Flux source as the caller read it.
type SourceState struct {
	// Access says how reading the source went: ok, or forbidden and
	// notReadable when it was not read. Readiness is empty then.
	Access    health.Access
	Readiness SourceReadiness
	// Reason, Message and Revision are the Ready condition's reason and
	// message and status.artifact.revision, as the cluster holds them. The
	// artifact's URL is never read. The code that serves them cuts them to
	// their caps.
	Reason   string
	Message  string
	Revision string
}

// Source reads the Flux source ref names, for the caller. kind is the
// resolved kind and g must cover get on it in ref's namespace and name; the
// reader must be allowed the same get. A core Secret is refused first
// (ErrWithheld), then a kind outside the three is answered as unsupported
// with no review and no read. A refused or failed read is the state's
// Access, never an error: only a grant that does not cover the read is
// (ErrNotCovered).
func (m *Model) Source(ctx context.Context, who authz.Identity, g authz.Grant, kind ResolvedKind, ref ObjectRef) (SourceState, error) {
	if isSecret(kind.Resource) || ref.Kind == "Secret" {
		return SourceState{}, ErrWithheld
	}
	if resource, ok := fluxSources[ref.Kind]; !ok || kind.Resource.Group != FluxGroup || kind.Resource.Resource != resource {
		return SourceState{Access: health.AccessOK, Readiness: SourceUnsupported}, nil
	}
	if err := covers(who, g, "get", kind.Resource, ref.Namespace, ref.Name); err != nil {
		return SourceState{}, err
	}
	if !m.readerMay(ctx, "get", kind.Resource, ref.Namespace, ref.Name) {
		return SourceState{Access: health.AccessNotReadable}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	obj, err := m.cfg.Dynamic.Resource(kind.Resource).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return SourceState{Access: health.AccessOK, Readiness: SourceNotFound}, nil
	case err != nil:
		return SourceState{Access: health.AccessNotReadable}, nil
	}
	return readSourceState(obj), nil
}

// readSourceState reads the Ready condition and the artifact of a source.
// The artifact's URL is not read.
func readSourceState(obj *unstructured.Unstructured) SourceState {
	st := SourceState{Access: health.AccessOK, Readiness: SourceUnknown}
	ready := ""
	for _, c := range maps(obj.Object, "status", "conditions") {
		if str(c, "type") == "Ready" {
			ready, st.Reason, st.Message = str(c, "status"), str(c, "reason"), str(c, "message")
			break
		}
	}
	artifact := object(obj.Object, "status", "artifact")
	st.Revision = str(obj.Object, "status", "artifact", "revision")
	switch {
	case ready == "True" && artifact != nil:
		st.Readiness = SourceReady
	case ready == "True":
		st.Readiness = SourceNoArtifact
	case ready == "False":
		st.Readiness = SourceNotReady
	}
	return st
}

// recheckSourceKinds asks discovery, with one call, whether the cluster now
// serves the Flux source kinds the Model's cached discovery does not know.
// When it does, the cache is reset so the next lookup finds them. No
// inventory reaches a Flux source, so nothing else would refresh them: a
// Flux installed after the portal started would read "kind not installed"
// until a restart. Only the timer calls it, never a request.
func (m *Model) recheckSourceKinds() {
	list, err := m.cfg.Discovery.ServerResourcesForGroupVersion(schema.GroupVersion{Group: FluxGroup, Version: fluxVersion}.String())
	if err != nil || list == nil {
		return
	}
	for i := range list.APIResources {
		r := &list.APIResources[i]
		if _, ok := fluxSources[r.Kind]; !ok {
			continue
		}
		if _, err := m.kinds.lookup(schema.GroupVersionKind{Group: FluxGroup, Kind: r.Kind}); err != nil {
			m.kinds.mapper.Reset()
			return
		}
	}
}

// sourceWatcher runs recheckSourceKinds every SourceKindsInterval until the
// Model stops.
func (m *Model) sourceWatcher() {
	t := time.NewTicker(m.cfg.SourceKindsInterval)
	defer t.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-t.C:
			m.recheckSourceKinds()
		}
	}
}
