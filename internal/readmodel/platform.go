package readmodel

import (
	"context"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// Platform returns the Platform with its subscriptions, resolved catalogs
// and registrations. g must cover get platforms cluster. The registrations
// are authorized for the caller on their own: when the caller may not list
// them the view says so instead of showing none (portal:D7:R3, portal:D11:R5).
func (m *Model) Platform(ctx context.Context, who authz.Identity, g authz.Grant) (PlatformView, error) {
	if err := covers(who, g, "get", platforms, "", platformName); err != nil {
		return PlatformView{}, err
	}
	u, err := m.getHeld(ctx, platforms, "", platformName)
	if err != nil {
		return PlatformView{}, err
	}
	view := PlatformView{
		Name:            u.GetName(),
		UID:             string(u.GetUID()),
		Type:            str(u.Object, "spec", "type"),
		OperatorVersion: str(u.Object, "status", "operatorVersion"),
		Applied:         health.ReadApplied(u),
		Conditions:      conditions(u),
		Subscriptions:   subscriptions(u),
	}
	view.Registrations, view.RegistrationsAccess = m.readableRegistrations(ctx, who)
	if len(view.Registrations) > 0 {
		held := m.holders(ctx, who)
		for i := range view.Registrations {
			r := &view.Registrations[i]
			r.HeldBy, r.HeldByPartial = held.byName[r.Name], held.partial
		}
	}
	view.Catalogs = catalogs(u, view.Registrations)
	return view, nil
}

// Registration returns one TransformerRegistration. g must cover get
// transformerregistrations name.
func (m *Model) Registration(ctx context.Context, who authz.Identity, g authz.Grant, name string) (RegistrationView, error) {
	if err := covers(who, g, "get", registrations, "", name); err != nil {
		return RegistrationView{}, err
	}
	u, err := m.getHeld(ctx, registrations, "", name)
	if err != nil {
		return RegistrationView{}, err
	}
	return registrationView(u), nil
}

// readableRegistrations returns every registration when the caller may
// list them, and an access state saying why not otherwise.
func (m *Model) readableRegistrations(ctx context.Context, who authz.Identity) ([]RegistrationView, health.Access) {
	if access := m.callerAccess(ctx, who, "list", registrations, "", ""); access != health.AccessOK {
		return nil, access
	}
	objs, err := m.listHeld(ctx, registrations, "")
	if err != nil {
		return nil, health.AccessNotReadable
	}
	out := make([]RegistrationView, 0, len(objs))
	for _, r := range objs {
		out = append(out, registrationView(r))
	}
	return out, health.AccessOK
}

func registrationView(u *unstructured.Unstructured) RegistrationView {
	return RegistrationView{
		Name:     u.GetName(),
		Catalog:  str(u.Object, "spec", "catalog"),
		Version:  str(u.Object, "spec", "version"),
		Provides: strs(u.Object, "spec", "provides"),
		Provider: ObjectRef{
			Group: opmGroup, Version: opmVersion, Kind: "ModuleInstance",
			Namespace: str(u.Object, "spec", "providerRef", "namespace"),
			Name:      str(u.Object, "spec", "providerRef", "name"),
		},
		Standing:   health.ReadRegistration(u),
		Applied:    health.ReadApplied(u),
		Conditions: conditions(u),
	}
}

// subscriptions reads spec.registry, a map keyed by catalog, sorted by
// catalog.
func subscriptions(u *unstructured.Unstructured) []Subscription {
	registry := object(u.Object, "spec", "registry")
	out := make([]Subscription, 0, len(registry))
	for catalog, raw := range registry {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		s := Subscription{Catalog: catalog, Version: str(entry, "version")}
		if enable, found := boolean(entry, "enable"); found {
			s.Enable = &enable
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Catalog < out[j].Catalog })
	return out
}

// catalogs reads status.registry in the operator's order and names the
// readable registrations that claim each catalog.
func catalogs(u *unstructured.Unstructured, regs []RegistrationView) []Catalog {
	raw := maps(u.Object, "status", "registry")
	out := make([]Catalog, 0, len(raw))
	for _, e := range raw {
		enabled, _ := boolean(e, "enabled")
		c := Catalog{
			Catalog: str(e, "catalog"),
			Version: str(e, "version"),
			Enabled: enabled,
			Source:  str(e, "source"),
		}
		for i := range regs {
			if regs[i].Catalog == c.Catalog {
				c.Registrations = append(c.Registrations, regs[i].Name)
			}
		}
		out = append(out, c)
	}
	return out
}
