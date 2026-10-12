package readmodel

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// ListPackages returns the ModulePackages in namespace ("" for every
// namespace the Model holds), sorted by namespace and name. g must cover
// list modulepackages in that namespace.
func (m *Model) ListPackages(ctx context.Context, who authz.Identity, g authz.Grant, namespace string) ([]PackageItem, error) {
	if err := covers(who, g, "list", modulePackages, namespace, ""); err != nil {
		return nil, err
	}
	objs, err := m.listHeld(ctx, modulePackages, namespace)
	if err != nil {
		return nil, err
	}
	ev := m.newEvaluator(who)
	cl := m.newClaims(ctx, who)
	out := make([]PackageItem, 0, len(objs))
	for _, u := range objs {
		item := packageItem(u)
		item.Health = ev.inventoryHealth(ctx, u).Instance
		item.ProviderOf = cl.of(u)
		out = append(out, item)
	}
	return out, nil
}

// Package returns one ModulePackage. g must cover get modulepackages
// namespace/name. It makes no review and no list of EndpointSlices: see
// PackageWithEndpoints.
func (m *Model) Package(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) (PackageDetail, error) {
	return m.pkg(ctx, who, g, namespace, name, false)
}

// PackageWithEndpoints is Package with the ready-endpoint count of each
// Service of the inventory, or the mark that it could not be read. Only the
// graph view, which shows the count, asks for it (portal:D11:R7).
func (m *Model) PackageWithEndpoints(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) (PackageDetail, error) {
	return m.pkg(ctx, who, g, namespace, name, true)
}

func (m *Model) pkg(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string, withEndpoints bool) (PackageDetail, error) {
	if err := covers(who, g, "get", modulePackages, namespace, name); err != nil {
		return PackageDetail{}, err
	}
	u, err := m.getHeld(ctx, modulePackages, namespace, name)
	if err != nil {
		return PackageDetail{}, err
	}
	res, components := m.evaluated(ctx, who, u, withEndpoints)
	item := packageItem(u)
	item.Health = res.Instance
	item.ProviderOf = m.newClaims(ctx, who).of(u)
	return PackageDetail{
		PackageItem:        item,
		ServiceAccountName: str(u.Object, "spec", "serviceAccountName"),
		Conditions:         conditions(u),
		History:            history(u),
		LastApplied:        lastApplied(u),
		Components:         components,
	}, nil
}

// PackageExists reports whether the Model holds ModulePackage
// namespace/name, without evaluating it. g must cover get modulepackages
// namespace/name; a missing package is the same not-found error Package
// returns.
func (m *Model) PackageExists(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) error {
	if err := covers(who, g, "get", modulePackages, namespace, name); err != nil {
		return err
	}
	_, err := m.getHeld(ctx, modulePackages, namespace, name)
	return err
}

func packageItem(u *unstructured.Unstructured) PackageItem {
	return PackageItem{
		Ref: refOf(u),
		UID: string(u.GetUID()),
		Source: SourceRef{
			APIVersion: str(u.Object, "spec", "sourceRef", "apiVersion"),
			Kind:       str(u.Object, "spec", "sourceRef", "kind"),
			Namespace:  str(u.Object, "spec", "sourceRef", "namespace"),
			Name:       str(u.Object, "spec", "sourceRef", "name"),
		},
		Interval:       str(u.Object, "spec", "interval"),
		SourceArtifact: sourceArtifact(u),
		DependsOn:      dependsOn(u),
		Path:           str(u.Object, "spec", "path"),
		Prune:          pruneOf(u),
		Applied:        health.ReadApplied(u),
		InventoryCount: inventoryCount(u, inventory(u)),
		LastAppliedAt:  timestamp(u.Object, "status", "lastAppliedAt"),
	}
}

// pruneOf reads spec.prune as written: nil when the package does not set it.
func pruneOf(u *unstructured.Unstructured) *bool {
	v, found := boolean(u.Object, "spec", "prune")
	if !found {
		return nil
	}
	return &v
}

// dependsOn reads spec.dependsOn as written.
func dependsOn(u *unstructured.Unstructured) []ObjectRef {
	raw := maps(u.Object, "spec", "dependsOn")
	if len(raw) == 0 {
		return nil
	}
	out := make([]ObjectRef, 0, len(raw))
	for _, d := range raw {
		out = append(out, ObjectRef{
			Group: opmGroup, Version: opmVersion, Kind: "ModulePackage",
			Namespace: str(d, "namespace"), Name: str(d, "name"),
		})
	}
	return out
}

// sourceArtifact reads the revision and digest of status.source, or nil
// when the operator recorded neither. status.source.artifactURL is a fetch
// URL inside the cluster and is not read.
func sourceArtifact(u *unstructured.Unstructured) *SourceArtifact {
	a := SourceArtifact{
		Revision: str(u.Object, "status", "source", "artifactRevision"),
		Digest:   str(u.Object, "status", "source", "artifactDigest"),
	}
	if a == (SourceArtifact{}) {
		return nil
	}
	return &a
}
