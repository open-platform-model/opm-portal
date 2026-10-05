package readmodel

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// The provider joins: which owners hold a TransformerRegistration, and which
// registrations an owner holds. An owner is any ModuleInstance or
// ModulePackage, whatever its kind, and holding means its status.inventory
// names the registration (portal:D15:R1). Both joins scan held state and
// read nothing of their own; the registration's standing is its own status,
// never one the portal computes (portal:D15:R2).

// kindRegistration is the TransformerRegistration kind as inventories name
// it.
const kindRegistration = "TransformerRegistration"

// heldRegistrations returns the names of the TransformerRegistrations
// owner's status.inventory holds, in inventory order.
func heldRegistrations(owner *unstructured.Unstructured) []string {
	var out []string
	for _, e := range maps(owner.Object, "status", "inventory", "entries") {
		if str(e, "group") == opmGroup && str(e, "kind") == kindRegistration {
			out = append(out, str(e, "name"))
		}
	}
	return out
}

// claims resolves the provider claims of the owners one view reads. It asks
// for the caller's list of registrations once, and only when an owner holds
// one (portal:D15:R4).
type claims struct {
	m      *Model
	ctx    context.Context
	who    authz.Identity
	asked  bool
	access health.Access
}

func (m *Model) newClaims(ctx context.Context, who authz.Identity) *claims {
	return &claims{m: m, ctx: ctx, who: who}
}

// of returns owner's provider claims, nil when its inventory holds none. A
// claim the caller may not read carries its name and access only: the name
// comes from the owner's inventory, which the caller is reading.
func (c *claims) of(owner *unstructured.Unstructured) []ProviderClaim {
	names := heldRegistrations(owner)
	if len(names) == 0 {
		return nil
	}
	if !c.asked {
		c.access = c.m.callerAccess(c.ctx, c.who, "list", registrations, "", "")
		c.asked = true
	}
	out := make([]ProviderClaim, 0, len(names))
	for _, name := range names {
		pc := ProviderClaim{Registration: name, Access: c.access}
		if c.access == health.AccessOK {
			u, err := c.m.getHeld(c.ctx, registrations, "", name)
			switch {
			case err == nil:
				pc.Standing = health.ReadRegistration(u)
				pc.ProviderRefMatches = owner.GetKind() == "ModuleInstance" &&
					str(u.Object, "spec", "providerRef", "namespace") == owner.GetNamespace() &&
					str(u.Object, "spec", "providerRef", "name") == owner.GetName()
			case errors.Is(err, ErrNotFound):
				// The inventory names a registration the cluster does not
				// hold (yet, or any more): there is no verdict to read.
				pc.Standing = health.Registration{Verdict: health.VerdictUnknown}
			default:
				pc.Access = health.AccessNotReadable
			}
		}
		out = append(out, pc)
	}
	return out
}

// holderIndex maps registration names to the owners whose inventory holds
// them.
type holderIndex struct {
	byName map[string][]ObjectRef
	// partial: the caller may not list ModuleInstances or ModulePackages
	// cluster-wide, or the read model does not hold them everywhere, so a
	// holder may be missing whether or not one was found (portal:D15).
	partial bool
}

// holders returns, for the caller, every registration's holders among the
// ModuleInstances and ModulePackages the caller may list in the owner's
// namespace, instances first, each kind sorted by namespace and name.
func (m *Model) holders(ctx context.Context, who authz.Identity) holderIndex {
	idx := holderIndex{byName: map[string][]ObjectRef{}}
	if len(m.cfg.Namespaces) > 0 {
		idx.partial = true
	}
	for _, res := range []schema.GroupVersionResource{moduleInstances, modulePackages} {
		clusterWide := m.callerAccess(ctx, who, "list", res, "", "") == health.AccessOK
		if !clusterWide {
			idx.partial = true
		}
		allowed := map[string]bool{}
		for _, scope := range m.scopesFor(true) {
			objs, err := m.listHeld(ctx, res, scope)
			if err != nil {
				idx.partial = true
				continue
			}
			for _, o := range objs {
				names := heldRegistrations(o)
				if len(names) == 0 {
					continue
				}
				ns := o.GetNamespace()
				may, seen := allowed[ns]
				if !seen {
					may = clusterWide || m.callerAccess(ctx, who, "list", res, ns, "") == health.AccessOK
					allowed[ns] = may
				}
				if !may {
					continue
				}
				for _, name := range names {
					idx.byName[name] = append(idx.byName[name], refOf(o))
				}
			}
		}
	}
	return idx
}

// holdsRegistration reports whether owner's inventory holds any
// TransformerRegistration.
func holdsRegistration(owner *unstructured.Unstructured) bool {
	return len(heldRegistrations(owner)) > 0
}
