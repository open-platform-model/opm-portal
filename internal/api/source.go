package api

import (
	"context"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// The caps on the text of a Flux source's Ready condition and artifact,
// served in local mode and in-cluster alike (portal:D20:R6). They count
// Unicode code points. source-controller writes the text from errors it met
// (registry replies, URLs, file paths), and the cluster allows a message of
// 32768 bytes and a revision of any length.
const (
	maxSourceMessage  = 1024
	maxSourceReason   = 256
	maxSourceRevision = 256
)

// packageWithSource reads a package for who and looks up its source: the
// view the package document, the package graph and the stream all build
// from. The source is nil when the package names none.
func (s *Server) packageWithSource(ctx context.Context, who authz.Identity, o owner) (readmodel.PackageDetail, *readmodel.SourceState, error) {
	d, err := s.packageDetail(ctx, who, o)
	if err != nil {
		return readmodel.PackageDetail{}, nil, err
	}
	return d, s.lookupSource(ctx, who, d), nil
}

// lookupSource reads the Flux source a package names, for the caller, after
// the package itself was authorized and read. A failed lookup is the
// state, never a failed document (portal:D20):
//
//  1. a kind outside OCIRepository, GitRepository and Bucket is
//     unsupported, with no review and no read;
//  2. a kind discovery does not serve is kindNotServed, with no review;
//  3. the caller's get on the source, in the sourceRef's namespace (the
//     package's when it names none), by name: refused is forbidden and
//     nothing is read;
//  4. the read model's Source, which asks the reader's own get review;
//  5. the cut of the text for serving.
//
// It returns nil when the package names no source.
func (s *Server) lookupSource(ctx context.Context, who authz.Identity, d readmodel.PackageDetail) *readmodel.SourceState {
	ref := d.Source
	if ref.Kind == "" || ref.Name == "" {
		return nil
	}
	if !readmodel.SourceSupported(ref.Kind) {
		return &readmodel.SourceState{Access: health.AccessOK, Readiness: readmodel.SourceUnsupported}
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = d.Ref.Namespace
	}
	if checkNamespace(namespace) != nil || checkName(ref.Name) != nil {
		// A sourceRef no source could have is not looked up.
		return &readmodel.SourceState{Access: health.AccessForbidden}
	}
	kind, err := s.cfg.Model.ResolveKind(readmodel.FluxGroup, ref.Kind)
	if err != nil {
		return &readmodel.SourceState{Access: health.AccessOK, Readiness: readmodel.SourceKindNotServed}
	}
	g, err := s.authorize(ctx, who, authz.Attributes{Verb: verbGet, Resource: kind.Resource, Namespace: namespace, Name: ref.Name})
	if err != nil {
		if classify(err).code == v1.CodeForbidden {
			return &readmodel.SourceState{Access: health.AccessForbidden}
		}
		return &readmodel.SourceState{Access: health.AccessNotReadable}
	}
	st, err := s.cfg.Model.Source(ctx, who, g[0], kind, readmodel.ObjectRef{
		Group: kind.Resource.Group, Version: kind.Resource.Version, Kind: ref.Kind, Namespace: namespace, Name: ref.Name,
	})
	if err != nil {
		return &readmodel.SourceState{Access: health.AccessNotReadable}
	}
	st.Message = readmodel.CapText(st.Message, maxSourceMessage)
	st.Reason = readmodel.CapText(st.Reason, maxSourceReason)
	st.Revision = readmodel.CapText(st.Revision, maxSourceRevision)
	return &st
}
