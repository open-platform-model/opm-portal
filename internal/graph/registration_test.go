package graph

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

const (
	backupClaim    = "obj:opmodel.dev/TransformerRegistration/_/default.backup-provider"
	holderClaim    = "obj:opmodel.dev/TransformerRegistration/_/pkg.provider"
	backupCatalog  = "testing.opmodel.dev/catalogs/controller/backup@v0"
	backupProvider = "default"
)

// TestInstanceRegistrationStanding: the node of a registration the owner's
// inventory holds carries the registration's own standing, catalog, version
// and provided contracts, as a node of the platform graph does.
func TestInstanceRegistrationStanding(t *testing.T) {
	c := newCluster(t, f1(t), readmodeltest.AllowAll)
	g := Instance(c.instance(t, backupProvider, "backup-provider"), Options{})
	r := nodeByID(t, g, backupClaim).Registration
	if r == nil || !r.Accepted || !r.Active || r.Verdict != health.VerdictAccepted || r.Catalog != backupCatalog || r.Version != "0.1.0" {
		t.Fatalf("registration = %+v, want accepted and active, verdict Accepted, catalog %s, version 0.1.0", r, backupCatalog)
	}
	platform := nodeByID(t, Platform(c.platform(t), Options{}), "treg:default.backup-provider").Registration
	if !slices.Equal(r.Provides, platform.Provides) || r.Message != platform.Message || r.ActiveMessage != platform.ActiveMessage {
		t.Errorf("instance registration %+v differs from the platform node's %+v", r, platform)
	}
	if n := nodeByID(t, Instance(c.instance(t, "default", "podinfo"), Options{}), "mi:default/podinfo"); n.Registration != nil {
		t.Errorf("an instance node carries a registration: %+v", n.Registration)
	}
}

// TestPackageRegistrationStanding: a package holding a refused claim shows
// the refusal on the claim's node.
func TestPackageRegistrationStanding(t *testing.T) {
	c := newCluster(t, readmodeltest.WithPackageHolder(t, f1(t)), readmodeltest.AllowAll)
	g := Package(c.pkgNamed(t, readmodeltest.PackageHolderNamespace, readmodeltest.PackageHolderName), nil, Options{})
	r := nodeByID(t, g, holderClaim).Registration
	if r == nil || r.Accepted || r.Active || r.Verdict != health.VerdictRefused || r.Reason != "ProviderMismatch" || r.Catalog != backupCatalog {
		t.Fatalf("registration = %+v, want neither accepted nor active, Refused, ProviderMismatch, catalog %s", r, backupCatalog)
	}
}

// TestUnreadClaimAttachesNothing: a caller who may not read registrations,
// and a registration the cluster does not hold, leave the node without a
// standing.
func TestUnreadClaimAttachesNothing(t *testing.T) {
	forbidden := newCluster(t, f1(t), readmodeltest.DenyResources("transformerregistrations"))
	g := Instance(forbidden.instance(t, backupProvider, "backup-provider"), Options{})
	n := nodeByID(t, g, backupClaim)
	if n.Registration != nil {
		t.Errorf("forbidden claim node = %+v, want no registration", n)
	}

	var objs []*unstructured.Unstructured
	for _, o := range f1(t) {
		if o.GetKind() != "TransformerRegistration" || o.GetName() != "default.backup-provider" {
			objs = append(objs, o)
		}
	}
	g = Instance(newCluster(t, objs, readmodeltest.AllowAll).instance(t, backupProvider, "backup-provider"), Options{})
	if n := nodeByID(t, g, backupClaim); n.Registration != nil {
		t.Errorf("claim the cluster does not hold = %+v, want no registration", n.Registration)
	}
}
