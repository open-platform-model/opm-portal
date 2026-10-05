package api

import (
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/graph"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// The functions below map read-model views and graphs onto the wire types.
// They copy; they decide nothing.

func meta(kind string) v1.TypeMeta { return v1.TypeMeta{APIVersion: v1.APIVersion, Kind: kind} }

// timePtr is nil for the zero time, so an unknown time is absent, never
// year one.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

func objectRef(r readmodel.ObjectRef) v1.ObjectRef {
	return v1.ObjectRef{Group: r.Group, Version: r.Version, Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}
}

func condition(c health.Condition) v1.Condition {
	out := v1.Condition{
		Type:               c.Type,
		Status:             string(c.Status),
		Reason:             c.Reason,
		Message:            c.Message,
		LastTransitionTime: timePtr(c.LastTransitionTime),
		ObservedGeneration: c.ObservedGeneration,
		Tone:               string(health.ConditionTone(c)),
	}
	if e, ok := health.Explain(c.Reason); ok {
		out.Meaning, out.NextStep = e.Meaning, e.NextStep
	}
	return out
}

func conditions(cs []health.Condition) []v1.Condition {
	out := make([]v1.Condition, 0, len(cs))
	for _, c := range cs {
		out = append(out, condition(c))
	}
	return out
}

func reconcile(a health.Applied) v1.Reconcile {
	r := v1.Reconcile{
		State:    string(a.State),
		Reason:   a.Reason,
		Message:  a.Message,
		Since:    timePtr(a.Since),
		Retrying: a.Retrying,
	}
	if len(a.Notes) > 0 {
		r.Notes = conditions(a.Notes)
	}
	return r
}

func healthCounts(c health.Counts) v1.HealthCounts {
	return v1.HealthCounts{
		Healthy: c.Healthy, Progressing: c.Progressing, Degraded: c.Degraded, Missing: c.Missing,
		Unknown: c.Unknown, Forbidden: c.Forbidden, NotReadable: c.NotReadable, Withheld: c.Withheld,
	}
}

func healthSummary(h health.Summary) v1.Health {
	return v1.Health{
		State:       string(h.State),
		Counts:      healthCounts(h.Counts),
		Partial:     h.Partial,
		EvaluatedAt: timePtr(h.EvaluatedAt),
		Live:        h.Live,
	}
}

func objectHealth(h health.ObjectHealth) v1.ObjectHealth {
	return v1.ObjectHealth{State: string(h.State), Reason: h.Reason, Message: h.Message}
}

func digests(d readmodel.Digests) v1.Digests {
	return v1.Digests{Source: d.Source, Config: d.Config, Render: d.Render}
}

func historyEntries(hs []readmodel.HistoryEntry) []v1.HistoryEntry {
	out := make([]v1.HistoryEntry, 0, len(hs))
	for i := range hs {
		h := &hs[i]
		out = append(out, v1.HistoryEntry{
			Action:          h.Action,
			Phase:           h.Phase,
			Outcome:         string(h.Outcome),
			Sequence:        h.Sequence,
			StartedAt:       timePtr(h.StartedAt),
			FinishedAt:      timePtr(h.FinishedAt),
			Message:         h.Message,
			InventoryCount:  h.InventoryCount,
			InventoryDigest: h.InventoryDigest,
			Digests:         digests(h.Digests),
		})
	}
	return out
}

func components(cs []readmodel.Component) []v1.Component {
	out := make([]v1.Component, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		objs := make([]v1.InventoryObject, 0, len(c.Objects))
		for j := range c.Objects {
			objs = append(objs, inventoryObject(&c.Objects[j]))
		}
		out = append(out, v1.Component{Name: c.Name, Health: healthSummary(c.Health), Objects: objs})
	}
	return out
}

// inventoryObject carries health only for an object the caller could read.
func inventoryObject(o *readmodel.InventoryObject) v1.InventoryObject {
	out := v1.InventoryObject{
		Ref:            objectRef(o.Ref),
		Access:         string(o.Access),
		ChildrenUnread: o.ChildrenUnread,
		EvaluatedAt:    timePtr(o.EvaluatedAt),
		Live:           o.Live,
	}
	if o.Access == health.AccessOK {
		h := objectHealth(o.Health)
		out.Health = &h
	}
	for i := range o.Children {
		c := &o.Children[i]
		out.Children = append(out.Children, v1.RuntimeChild{
			Ref:        objectRef(c.Ref),
			Owner:      objectRef(c.Owner),
			Health:     objectHealth(c.Health),
			Replicas:   c.Replicas,
			Containers: c.Containers,
		})
	}
	return out
}

func instanceSummary(it readmodel.InstanceItem) v1.InstanceSummary {
	contracts := it.RenderContracts
	if contracts == nil {
		contracts = []string{}
	}
	return v1.InstanceSummary{
		Ref:             objectRef(it.Ref),
		UID:             it.UID,
		Module:          v1.Module{Path: it.Module.Path, Version: it.Module.Version},
		Owner:           string(it.Owner),
		Reconcile:       reconcile(it.Applied),
		Health:          healthSummary(it.Health),
		InventoryCount:  it.InventoryCount,
		LastAppliedAt:   timePtr(it.LastAppliedAt),
		RenderContracts: contracts,
		ProviderOf:      providerClaims(it.ProviderOf),
	}
}

// providerClaims carries a claim's standing and whether its providerRef
// names the owner only when the caller may read the registrations.
func providerClaims(cs []readmodel.ProviderClaim) []v1.ProviderClaim {
	if len(cs) == 0 {
		return nil
	}
	out := make([]v1.ProviderClaim, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		pc := v1.ProviderClaim{Registration: c.Registration, Access: string(c.Access)}
		if c.Access == health.AccessOK {
			matches := c.ProviderRefMatches
			pc.Accepted, pc.Active = c.Standing.Accepted, c.Standing.Active
			pc.Verdict, pc.Reason = string(c.Standing.Verdict), c.Standing.Reason
			pc.ProviderRefMatches = &matches
		}
		out = append(out, pc)
	}
	return out
}

func instanceDoc(d readmodel.InstanceDetail) v1.Instance {
	return v1.Instance{
		TypeMeta:           meta(v1.KindInstance),
		InstanceSummary:    instanceSummary(d.InstanceItem),
		ServiceAccountName: d.ServiceAccountName,
		Conditions:         conditions(d.Conditions),
		History:            historyEntries(d.History),
		LastApplied:        digests(d.LastApplied),
		Components:         components(d.Components),
	}
}

func packageSummary(it readmodel.PackageItem) v1.PackageSummary {
	out := v1.PackageSummary{
		Ref: objectRef(it.Ref),
		UID: it.UID,
		Source: v1.SourceRef{
			APIVersion: it.Source.APIVersion,
			Kind:       it.Source.Kind,
			Namespace:  it.Source.Namespace,
			Name:       it.Source.Name,
		},
		Interval:       it.Interval,
		Path:           it.Path,
		Reconcile:      reconcile(it.Applied),
		Health:         healthSummary(it.Health),
		InventoryCount: it.InventoryCount,
		LastAppliedAt:  timePtr(it.LastAppliedAt),
		ProviderOf:     providerClaims(it.ProviderOf),
	}
	if a := it.SourceArtifact; a != nil {
		out.SourceArtifact = &v1.SourceArtifact{Revision: a.Revision, Digest: a.Digest}
	}
	for _, d := range it.DependsOn {
		out.DependsOn = append(out.DependsOn, objectRef(d))
	}
	return out
}

func packageDoc(d readmodel.PackageDetail) v1.Package {
	return v1.Package{
		TypeMeta:       meta(v1.KindPackage),
		PackageSummary: packageSummary(d.PackageItem),
		Conditions:     conditions(d.Conditions),
		History:        historyEntries(d.History),
		LastApplied:    digests(d.LastApplied),
		Components:     components(d.Components),
	}
}

func platformDoc(p readmodel.PlatformView) v1.Platform {
	out := v1.Platform{
		TypeMeta:            meta(v1.KindPlatform),
		Name:                p.Name,
		UID:                 p.UID,
		Type:                p.Type,
		OperatorVersion:     p.OperatorVersion,
		Reconcile:           reconcile(p.Applied),
		Conditions:          conditions(p.Conditions),
		Subscriptions:       make([]v1.Subscription, 0, len(p.Subscriptions)),
		Catalogs:            make([]v1.Catalog, 0, len(p.Catalogs)),
		Registrations:       make([]v1.Registration, 0, len(p.Registrations)),
		RegistrationsAccess: string(p.RegistrationsAccess),
	}
	for _, s := range p.Subscriptions {
		out.Subscriptions = append(out.Subscriptions, v1.Subscription{Catalog: s.Catalog, Version: s.Version, Enable: s.Enable})
	}
	for _, c := range p.Catalogs {
		claimants := c.Registrations
		if claimants == nil {
			claimants = []string{}
		}
		out.Catalogs = append(out.Catalogs, v1.Catalog{
			Catalog:       c.Catalog,
			Version:       c.Version,
			Enabled:       c.Enabled,
			Source:        c.Source,
			Claimants:     claimants,
			ContributedBy: graph.Contributor(c, p.Registrations),
		})
	}
	for i := range p.Registrations {
		out.Registrations = append(out.Registrations, registration(&p.Registrations[i]))
	}
	return out
}

func registration(r *readmodel.RegistrationView) v1.Registration {
	out := v1.Registration{
		Name:          r.Name,
		Catalog:       r.Catalog,
		Version:       r.Version,
		Provides:      r.Provides,
		Accepted:      r.Standing.Accepted,
		Active:        r.Standing.Active,
		Verdict:       string(r.Standing.Verdict),
		Reason:        r.Standing.Reason,
		Message:       r.Standing.Message,
		ActiveReason:  r.Standing.ActiveReason,
		ActiveMessage: r.Standing.ActiveMessage,
		Reconcile:     reconcile(r.Applied),
		HeldByPartial: r.HeldByPartial,
	}
	if len(r.Conditions) > 0 {
		out.Conditions = conditions(r.Conditions)
	}
	for _, h := range r.HeldBy {
		out.HeldBy = append(out.HeldBy, objectRef(h))
	}
	if r.Provider.Name != "" {
		p := objectRef(r.Provider)
		out.Provider = &p
	}
	return out
}

func eventListDoc(about readmodel.ObjectRef, evs []readmodel.Event) v1.EventList {
	out := v1.EventList{TypeMeta: meta(v1.KindEventList), Regarding: objectRef(about), Items: make([]v1.Event, 0, len(evs))}
	for i := range evs {
		e := &evs[i]
		out.Items = append(out.Items, v1.Event{
			Type:                e.Type,
			Reason:              e.Reason,
			Note:                e.Note,
			ReportingController: e.ReportingController,
			Regarding:           objectRef(e.Regarding),
			FieldPath:           e.FieldPath,
			Count:               e.Count,
			LastSeen:            timePtr(e.LastSeen),
		})
	}
	return out
}

func graphDoc(g graph.Graph) v1.Graph {
	out := v1.Graph{
		TypeMeta: meta(v1.KindGraph),
		Scope:    string(g.Scope),
		Root:     g.Root,
		Layout: v1.GraphLayout{
			Width:      g.Layout.Width,
			Height:     g.Layout.Height,
			NodeWidth:  g.Layout.NodeWidth,
			NodeHeight: g.Layout.NodeHeight,
			Columns:    make([]v1.GraphColumn, 0, len(g.Layout.Columns)),
		},
		Nodes: make([]v1.GraphNode, 0, len(g.Nodes)),
		Edges: make([]v1.GraphEdge, 0, len(g.Edges)),
	}
	for _, c := range g.Layout.Columns {
		out.Layout.Columns = append(out.Layout.Columns, v1.GraphColumn{Title: c.Title, X: c.X})
	}
	for i := range g.Nodes {
		out.Nodes = append(out.Nodes, graphNode(&g.Nodes[i]))
	}
	for i := range g.Edges {
		e := &g.Edges[i]
		edge := v1.GraphEdge{
			ID: e.ID, Kind: string(e.Kind), From: e.From, To: e.To, Source: e.Source,
			Verified: e.Verified, Reason: e.Reason, Route: make([]v1.GraphPoint, 0, len(e.Route)),
		}
		for _, p := range e.Route {
			edge.Route = append(edge.Route, v1.GraphPoint{X: p.X, Y: p.Y})
		}
		out.Edges = append(out.Edges, edge)
	}
	return out
}

func graphNode(n *graph.Node) v1.GraphNode {
	out := v1.GraphNode{
		ID:               n.ID,
		Kind:             string(n.Kind),
		Label:            n.Label,
		Access:           string(n.Access),
		Missing:          n.Missing,
		Owner:            n.Owner,
		RenderContracts:  n.RenderContracts,
		Version:          n.Version,
		Path:             n.Path,
		MemberOf:         n.MemberOf,
		Replicas:         n.Replicas,
		ChildrenUnread:   n.ChildrenUnread,
		HiddenScaledDown: n.HiddenScaledDown,
		Column:           n.Column,
		Row:              n.Row,
		X:                n.X,
		Y:                n.Y,
	}
	if r := n.Ref; r != nil {
		out.Ref = &v1.ObjectRef{Group: r.Group, Version: r.Version, Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}
	}
	if h := n.Health; h != nil {
		gh := v1.GraphHealth{State: string(h.State), Reason: h.Reason, Message: h.Message, Partial: h.Partial, NotLive: h.NotLive}
		if c := h.Counts; c != nil {
			counts := v1.HealthCounts{
				Healthy: c.Healthy, Progressing: c.Progressing, Degraded: c.Degraded, Missing: c.Missing,
				Unknown: c.Unknown, Forbidden: c.Forbidden, NotReadable: c.NotReadable, Withheld: c.Withheld,
			}
			gh.Counts = &counts
		}
		out.Health = &gh
	}
	if a := n.Applied; a != nil {
		out.Reconcile = &v1.Reconcile{State: string(a.State), Reason: a.Reason, Message: a.Message, Retrying: a.Retrying}
	}
	if r := n.Registration; r != nil {
		out.Registration = &v1.GraphRegistration{
			Catalog: r.Catalog, Version: r.Version, Provides: r.Provides, Accepted: r.Accepted, Active: r.Active,
			Verdict: string(r.Verdict), Reason: r.Reason, Message: r.Message,
			ActiveReason: r.ActiveReason, ActiveMessage: r.ActiveMessage,
		}
	}
	if c := n.Catalog; c != nil {
		out.Catalog = &v1.GraphCatalog{Version: c.Version, Enabled: c.Enabled, Source: c.Source}
	}
	if p := n.Platform; p != nil {
		out.Platform = &v1.GraphPlatform{Type: p.Type, OperatorVersion: p.OperatorVersion, RegistrationsAccess: string(p.RegistrationsAccess)}
	}
	if g := n.Group; g != nil {
		gg := &v1.GraphGroup{Kind: string(g.Kind), Members: g.Members}
		for _, h := range g.Hidden {
			gg.Hidden = append(gg.Hidden, v1.GraphKindCount{Kind: h.Kind, Count: h.Count})
		}
		out.Group = gg
	}
	return out
}
