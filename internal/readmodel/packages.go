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
	out := make([]PackageItem, 0, len(objs))
	for _, u := range objs {
		item := packageItem(u)
		item.Health = ev.inventoryHealth(ctx, u).Instance
		out = append(out, item)
	}
	return out, nil
}

// Package returns one ModulePackage. g must cover get modulepackages
// namespace/name.
func (m *Model) Package(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) (PackageDetail, error) {
	if err := covers(who, g, "get", modulePackages, namespace, name); err != nil {
		return PackageDetail{}, err
	}
	u, err := m.getHeld(ctx, modulePackages, namespace, name)
	if err != nil {
		return PackageDetail{}, err
	}
	res := m.newEvaluator(who).inventoryHealth(ctx, u)
	item := packageItem(u)
	item.Health = res.Instance
	return PackageDetail{
		PackageItem: item,
		Conditions:  conditions(u),
		History:     history(u),
		LastApplied: lastApplied(u),
		Components:  componentsOf(res),
	}, nil
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
		Path:           str(u.Object, "spec", "path"),
		Applied:        health.ReadApplied(u),
		InventoryCount: inventoryCount(u, inventory(u)),
		LastAppliedAt:  timestamp(u.Object, "status", "lastAppliedAt"),
	}
}
