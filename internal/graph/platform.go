package graph

import (
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// Columns of the platform scope.
const (
	colPlatform = iota
	colCatalog
	colRegistration
	colProvider
)

var platformTitles = []string{"Platform", "Catalogs", "Registrations", "Providers"}

const (
	opmGroup        = "opmodel.dev"
	sourceRegistry  = "Registration"
	kindTransformer = "TransformerRegistration"
)

// PlatformInput is what the platform graph is built from: the platform view
// and the provider instance each registration names, as the caller could
// read it. The caller authorizes and reads each provider; the graph reads
// nothing.
type PlatformInput struct {
	Platform  readmodel.PlatformView
	Providers []ProviderLookup
}

// ProviderLookup is one provider instance as the caller read it.
type ProviderLookup struct {
	Ref readmodel.ObjectRef
	// Access says how the read went. AccessOK with Instance nil means the
	// instance was read and does not exist.
	Access   health.Access
	Instance *readmodel.InstanceDetail
}

// Platform returns the platform graph: the Platform, the catalogs its
// registry resolved, the registrations and the provider instances they
// name. There is no column of consumer instances: no recorded field says
// which provider contract an instance demands (0030:D4:R3).
func Platform(in PlatformInput, opts Options) Graph {
	b := newBuilder(ScopePlatform, platformTitles, opts)
	p := in.Platform
	plat := Node{
		ID:      platformID(p.Name),
		Kind:    KindPlatform,
		Label:   p.Name,
		Ref:     &Ref{Group: opmGroup, Version: "v1alpha1", Kind: "Platform", Name: p.Name},
		Access:  health.AccessOK,
		Applied: appliedOf(p.Applied),
		Platform: &PlatformFacts{
			Type:                p.Type,
			OperatorVersion:     p.OperatorVersion,
			RegistrationsAccess: p.RegistrationsAccess,
		},
	}
	b.root = plat.ID
	b.add(plat, colPlatform)

	contributed := map[string]bool{}
	for _, c := range p.Catalogs {
		n := Node{
			ID:      catalogID(c.Catalog),
			Kind:    KindCatalog,
			Label:   c.Catalog,
			Catalog: &Catalog{Version: c.Version, Enabled: c.Enabled, Source: c.Source},
		}
		b.add(n, colCatalog)
		b.edge(EdgeResolves, plat.ID, n.ID)
		if c.Source == sourceRegistry {
			contributed[c.Catalog] = true
		}
	}

	lookups := make(map[string]*ProviderLookup, len(in.Providers))
	for i := range in.Providers {
		l := &in.Providers[i]
		lookups[instanceID(l.Ref.Namespace, l.Ref.Name)] = l
	}
	for i := range p.Registrations {
		r := &p.Registrations[i]
		n := Node{
			ID:      registrationID(r.Name),
			Kind:    KindRegistration,
			Label:   r.Name,
			Ref:     &Ref{Group: opmGroup, Version: "v1alpha1", Kind: kindTransformer, Name: r.Name},
			Access:  health.AccessOK,
			Applied: appliedOf(r.Applied),
			Registration: &Registration{
				Catalog:       r.Catalog,
				Version:       r.Version,
				Provides:      r.Provides,
				Accepted:      r.Standing.Accepted,
				Active:        r.Standing.Active,
				Verdict:       r.Standing.Verdict,
				Reason:        r.Standing.Reason,
				Message:       r.Standing.Message,
				ActiveReason:  r.Standing.ActiveReason,
				ActiveMessage: r.Standing.ActiveMessage,
			},
		}
		b.add(n, colRegistration)
		// Only the registry says a registration's catalog was contributed;
		// a refused claim names a catalog no registry entry holds.
		if contributed[r.Catalog] {
			b.edge(EdgeContributes, n.ID, catalogID(r.Catalog))
		}
		if r.Provider.Name != "" {
			b.provider(n.ID, r, lookups[instanceID(r.Provider.Namespace, r.Provider.Name)])
		}
	}
	return b.finish()
}

// provider adds the instance a registration names and the edge to it,
// verified only when the instance's inventory holds the registration
// (0030:D4:R2).
func (b *builder) provider(reg string, r *readmodel.RegistrationView, l *ProviderLookup) {
	var n Node
	switch {
	case l != nil && l.Access == health.AccessOK && l.Instance != nil:
		n = instanceNode(l.Instance.InstanceItem)
	default:
		n = Node{
			ID:    instanceID(r.Provider.Namespace, r.Provider.Name),
			Kind:  KindInstance,
			Label: r.Provider.Namespace + "/" + r.Provider.Name,
			Ref:   refOf(r.Provider),
		}
		switch {
		case l == nil:
			n.Access = health.AccessNotReadable
		case l.Access == health.AccessOK:
			n.Access = health.AccessOK
			n.Missing = true
		default:
			n.Access = l.Access
		}
	}
	b.add(n, colProvider)
	e := b.edge(EdgeProvidedBy, reg, n.ID)
	verified, reason := verifyProvider(r.Name, l)
	e.Verified = &verified
	e.Reason = reason
}

func verifyProvider(registration string, l *ProviderLookup) (verified bool, reason string) {
	switch {
	case l == nil || l.Access != health.AccessOK:
		return false, ReasonProviderUnreadable
	case l.Instance == nil:
		return false, ReasonProviderNotFound
	}
	for i := range l.Instance.Components {
		for j := range l.Instance.Components[i].Objects {
			if o := &l.Instance.Components[i].Objects[j]; o.Ref.Group == opmGroup && o.Ref.Kind == kindTransformer && o.Ref.Name == registration {
				return true, ""
			}
		}
	}
	return false, ReasonNotInProviderInventory
}
