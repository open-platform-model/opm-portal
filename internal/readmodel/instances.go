package readmodel

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// ListInstances returns the ModuleInstances in namespace ("" for every
// namespace the Model holds), sorted by namespace and name. g must cover
// list moduleinstances in that namespace; the list holds nothing outside it
// (0030:D7:R2).
func (m *Model) ListInstances(ctx context.Context, who authz.Identity, g authz.Grant, namespace string) ([]InstanceItem, error) {
	if err := covers(who, g, "list", moduleInstances, namespace, ""); err != nil {
		return nil, err
	}
	objs, err := m.listHeld(ctx, moduleInstances, namespace)
	if err != nil {
		return nil, err
	}
	ev := m.newEvaluator(who)
	out := make([]InstanceItem, 0, len(objs))
	for _, u := range objs {
		item := instanceItem(u)
		item.Health = ev.inventoryHealth(ctx, u).Instance
		out = append(out, item)
	}
	return out, nil
}

// Instance returns one ModuleInstance with its inventory grouped by
// component. g must cover get moduleinstances namespace/name.
func (m *Model) Instance(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) (InstanceDetail, error) {
	if err := covers(who, g, "get", moduleInstances, namespace, name); err != nil {
		return InstanceDetail{}, err
	}
	u, err := m.getHeld(ctx, moduleInstances, namespace, name)
	if err != nil {
		return InstanceDetail{}, err
	}
	res := m.newEvaluator(who).inventoryHealth(ctx, u)
	item := instanceItem(u)
	item.Health = res.Instance
	return InstanceDetail{
		InstanceItem:       item,
		ServiceAccountName: str(u.Object, "spec", "serviceAccountName"),
		Conditions:         conditions(u),
		History:            history(u),
		LastApplied:        lastApplied(u),
		RenderContracts:    strs(u.Object, "status", "requiredContracts"),
		Components:         componentsOf(res),
	}, nil
}

// InstanceExists reports whether the Model holds ModuleInstance
// namespace/name, without evaluating it. g must cover get moduleinstances
// namespace/name; a missing instance is the same not-found error Instance
// returns.
func (m *Model) InstanceExists(ctx context.Context, who authz.Identity, g authz.Grant, namespace, name string) error {
	if err := covers(who, g, "get", moduleInstances, namespace, name); err != nil {
		return err
	}
	_, err := m.getHeld(ctx, moduleInstances, namespace, name)
	return err
}

func instanceItem(u *unstructured.Unstructured) InstanceItem {
	return InstanceItem{
		Ref:            refOf(u),
		UID:            string(u.GetUID()),
		Module:         ModuleRef{Path: str(u.Object, "spec", "module", "path"), Version: str(u.Object, "spec", "module", "version")},
		Owner:          ownerOf(u),
		Applied:        health.ReadApplied(u),
		InventoryCount: inventoryCount(u, inventory(u)),
		LastAppliedAt:  timestamp(u.Object, "status", "lastAppliedAt"),
	}
}
