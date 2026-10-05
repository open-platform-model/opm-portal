package readmodel

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// lifecycleMethods are the exported methods of Model that serve no caller:
// they start and stop the reader's own informers.
var lifecycleMethods = []string{"Start", "Stop"}

// nonReads are the exported methods of Model that hand out no object
// content: OnChange reports only which OPM object's view to render again,
// ResolveKind answers from discovery, which names kinds, never objects, and
// Denied reports the outcome of the reader's own access reviews at Start.
var nonReads = []string{"OnChange", "ResolveKind", "Denied"}

// ungrantedReads returns the exported methods of typ, other than exempt,
// that do not take both an authz.Identity and an authz.Grant.
func ungrantedReads(typ reflect.Type, exempt []string) []string {
	identity := reflect.TypeFor[authz.Identity]()
	grant := reflect.TypeFor[authz.Grant]()
	var out []string
	for method := range typ.Methods() {
		if slices.Contains(exempt, method.Name) {
			continue
		}
		var hasIdentity, hasGrant bool
		for in := range method.Type.Ins() {
			hasIdentity = hasIdentity || in == identity
			hasGrant = hasGrant || in == grant
		}
		if !hasIdentity || !hasGrant {
			out = append(out, method.Name)
		}
	}
	return out
}

// TestEveryReadTakesAGrant: no exported read of the Model can be called
// without the caller's identity and a grant (portal:D7). Each read's own tests
// cover that it refuses a grant that does not cover it.
func TestEveryReadTakesAGrant(t *testing.T) {
	if bad := ungrantedReads(reflect.TypeFor[*Model](), slices.Concat(lifecycleMethods, nonReads)); len(bad) > 0 {
		t.Fatalf("exported Model methods without an authz.Identity and an authz.Grant: %v", bad)
	}
}

// grantless has one read that takes a grant and one that does not, so the
// signature check is shown to catch the second.
type grantless struct{}

func (grantless) Covered(context.Context, authz.Identity, authz.Grant) error { return nil }
func (grantless) Uncovered(context.Context, authz.Identity) error            { return nil }

func TestSignatureCheckCatchesAReadWithoutAGrant(t *testing.T) {
	got := ungrantedReads(reflect.TypeFor[grantless](), nil)
	if !slices.Equal(got, []string{"Uncovered"}) {
		t.Fatalf("ungrantedReads = %v, want [Uncovered]", got)
	}
}
