package ui

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

var regexpCounts = regexp.MustCompile(`\d+ provided`)

const backupContract = "opmodel.dev/catalogs/opm/traits/backup@v1alpha1"

// providerOwner is an instance that holds one claim, for the claim view.
func providerOwner(c v1.ProviderClaim) *ownerView {
	return &ownerView{Kind: instanceKind, Namespace: "default", Name: "x", ProviderOf: []v1.ProviderClaim{c}}
}

// platformWith serves one registration that lists contracts.
func platformWith(contracts ...string) *v1.Platform {
	return &v1.Platform{Registrations: []v1.Registration{{Name: "default.x", Catalog: "c.example/cat@v0", Provides: contracts}}}
}

func users(n int) []v1.InstanceSummary {
	out := make([]v1.InstanceSummary, 0, n)
	for i := range n {
		out = append(out, v1.InstanceSummary{
			Ref:             v1.ObjectRef{Group: "opmodel.dev", Kind: "ModuleInstance", Namespace: "default", Name: fmt.Sprintf("user-%d", i)},
			RenderContracts: []string{backupContract},
		})
	}
	return out
}

// TestProviderContractsTextEveryVerdict (OP-A4): every verdict, and one the
// UI does not know, has a Contracts text; a claim that provides nothing
// never reads "N provided, M in use", and Removal blocked says so beside
// its counts.
func TestProviderContractsTextEveryVerdict(t *testing.T) {
	counts := regexpCounts
	for _, tc := range []struct {
		name  string
		claim v1.ProviderClaim
		want  string
		// serves: the claim provides, so "N provided" may appear.
		serves bool
	}{
		{"Accepted and active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true, Active: true}, "2 provided, 1 in use", true},
		{"Accepted, not active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true}, "2 listed; none provided until the claim is active", false},
		{"Pending", v1.ProviderClaim{Verdict: "Pending"}, "2 listed; none provided until the controller decides", false},
		{"Refused", v1.ProviderClaim{Verdict: "Refused"}, "none: the claim is refused", false},
		{"Removal blocked", v1.ProviderClaim{Verdict: "RemovalBlocked", Accepted: true, Active: true}, "2 provided, 1 in use; removal is blocked", true},
		{"Unknown", v1.ProviderClaim{Verdict: "Unknown"}, "not known: the portal does not know this claim's standing", false},
		{"a verdict the UI does not know", v1.ProviderClaim{Verdict: "Levitating", Accepted: true, Active: true}, "not known: the portal does not know this claim's standing", false},
		{"no verdict", v1.ProviderClaim{}, "not known: the portal does not know this claim's standing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.claim.Registration, tc.claim.Access = "default.x", v1.AccessOK
			v := providerOwner(tc.claim)
			p := platformWith(backupContract, "c.example/other@v1")
			got := claimsOf(providerTabView{}, v, p, users(1)).Claims[0]
			if got.ContractsText != tc.want {
				t.Errorf("Contracts = %q, want %q", got.ContractsText, tc.want)
			}
			if !tc.serves && counts.MatchString(got.ContractsText) {
				t.Errorf("a claim that provides nothing reads as counts: %q", got.ContractsText)
			}
			if got.Intro == "" {
				t.Error("no intro")
			}
			if !tc.serves {
				for _, c := range got.Contracts {
					if c.Button != "" || len(c.First) > 0 || c.Total > 0 {
						t.Errorf("a claim that provides nothing names users of %s: %+v", c.Contract, c)
					}
				}
			}
		})
	}
}

// TestProviderContractsTextWhenUseIsNotFullyReadable: no in-use count when
// a list of users is incomplete or locked, since M would be too low.
func TestProviderContractsTextWhenUseIsNotFullyReadable(t *testing.T) {
	claim := v1.ProviderClaim{Registration: "default.x", Access: v1.AccessOK, Verdict: "Accepted", Accepted: true, Active: true}
	for name, tab := range map[string]providerTabView{
		"incomplete": {UsedByIncomplete: true},
		"locked":     {UsedByLocked: true},
		"a failure":  {UsedByProblem: &v1.Problem{Code: v1.CodeNotReadableByPortal}},
	} {
		got := claimsOf(tab, providerOwner(claim), platformWith(backupContract), users(1)).Claims[0]
		if got.ContractsText != "1 provided; use not fully readable" {
			t.Errorf("%s: Contracts = %q", name, got.ContractsText)
		}
	}
	blocked := claim
	blocked.Verdict = "RemovalBlocked"
	got := claimsOf(providerTabView{UsedByIncomplete: true}, providerOwner(blocked), platformWith(backupContract), nil).Claims[0]
	if got.ContractsText != "1 provided; use not fully readable; removal is blocked" {
		t.Errorf("blocked and not fully readable: %q", got.ContractsText)
	}
}

// TestProviderUsedByButtons: up to three users, then a count; "Show in
// Installed" up to three, "All N in Installed" over three; a complete list
// with no user says so and offers no link; an incomplete one keeps the link.
func TestProviderUsedByButtons(t *testing.T) {
	claim := v1.ProviderClaim{Registration: "default.x", Access: v1.AccessOK, Verdict: "Accepted", Accepted: true, Active: true}
	for _, tc := range []struct {
		name       string
		n          int
		tab        providerTabView
		button     string
		empty      bool
		first, rem int
	}{
		{"none, complete", 0, providerTabView{}, "", true, 0, 0},
		{"none, incomplete", 0, providerTabView{UsedByIncomplete: true}, "Show in Installed \u2192", false, 0, 0},
		{"one", 1, providerTabView{}, "Show in Installed \u2192", false, 1, 0},
		{"three", 3, providerTabView{}, "Show in Installed \u2192", false, 3, 0},
		{"four", 4, providerTabView{}, "All 4 in Installed \u2192", false, 3, 1},
		{"locked", 0, providerTabView{UsedByLocked: true}, "All in Installed \u2192", false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := claimsOf(tc.tab, providerOwner(claim), platformWith(backupContract), users(tc.n)).Claims[0].Contracts[0]
			if got.Button != tc.button || got.Empty != tc.empty || len(got.First) != tc.first || got.More != tc.rem {
				t.Errorf("got button %q empty %v first %d more %d", got.Button, got.Empty, len(got.First), got.More)
			}
			if got.Button != "" && !strings.Contains(got.ButtonTitle, backupContract) {
				t.Errorf("title %q", got.ButtonTitle)
			}
		})
	}
}

// TestProviderTabRendersEveryVerdict: the tab for each verdict says what the
// claim does, draws its contracts as served only when it serves them, and
// carries the card fallback hooks (the table class and a data-label on each
// cell).
func TestProviderTabRendersEveryVerdict(t *testing.T) {
	h := cardsHandler()
	for _, tc := range []struct {
		name  string
		claim v1.ProviderClaim
		want  []string
		not   []string
	}{
		{"active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true, Active: true, Reason: "Accepted"},
			[]string{"modules that use the contracts below render through it", "1 provided, 1 in use", "Show in Installed", `class="table rows provides-table"`, `data-label="Used by"`},
			[]string{`<span class="mono">Accepted</span>`}},
		{"accepted, not active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true, Reason: "ProviderNotReady"},
			[]string{"accepted the claim, but it is not active", "1 listed; none provided until the claim is active", "Not provided until the claim is active.", `<span class="mono">ProviderNotReady</span>`},
			[]string{"Show in Installed", "1 provided"}},
		{"pending", v1.ProviderClaim{Verdict: "Pending", Reason: "Waiting"},
			[]string{"has not decided on the claim yet", "1 listed; none provided until the controller decides", "Not provided until the controller decides."},
			[]string{"Show in Installed", "1 provided"}},
		{"refused", v1.ProviderClaim{Verdict: "Refused", Reason: "CatalogUnresolved"},
			[]string{"refused the claim, so it provides nothing", "none: the claim is refused", "Nothing. The controller refused the claim"},
			[]string{"Show in Installed", "provided,", "contract-pill"}},
		{"removal blocked", v1.ProviderClaim{Verdict: "RemovalBlocked", Accepted: true, Active: true, Reason: "InUse"},
			[]string{"1 provided, 1 in use; removal is blocked", "Show in Installed", `<span class="mono">InUse</span>`}, nil},
		{"unknown", v1.ProviderClaim{Verdict: "Levitating"},
			[]string{"does not know this claim&#39;s standing", "not known: the portal does not know this claim&#39;s standing"},
			[]string{"Show in Installed", "contract-pill"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.claim.Registration, tc.claim.Access = "default.x", v1.AccessOK
			v := providerOwner(tc.claim)
			v.Tab = tabProvider
			p := platformWith(backupContract)
			v.Provider = claimsOf(providerTabView{}, v, p, users(1))
			v.Base = "/instances/default/x"
			tabs := renderOwnerTab(t, h, v)
			for _, w := range tc.want {
				if !strings.Contains(tabs, w) {
					t.Errorf("tab lacks %q:\n%s", w, tabs)
				}
			}
			for _, w := range tc.not {
				if strings.Contains(tabs, w) {
					t.Errorf("tab holds %q:\n%s", w, tabs)
				}
			}
		})
	}
}

// TestPackageRefusedClaimProviderTab: a package holding a refused claim
// says the controller refused it and that it provides nothing, in local mode
// with the controller's message and in-cluster without it.
func TestPackageRefusedClaimProviderTab(t *testing.T) {
	local := mainOf(newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll).get(t, "/packages/pkg/provider?tab=provider").body)
	tab := between(local, `id="provider"`, "</section>")
	for _, want := range []string{"This package is also a provider", "The controller refused the claim, so it provides nothing.",
		"<dt>Contracts</dt><dd>none: the claim is refused</dd>", "Nothing. The controller refused the claim", "Claim names provider ModuleInstance pkg/provider, which does not exist",
		`<span class="mono">ProviderMismatch</span>`, `<span class="cond-badge tone-abnormal">True</span>`} {
		if !strings.Contains(tab, want) {
			t.Errorf("local package Provider tab lacks %q:\n%s", want, tab)
		}
	}
	if strings.Contains(tab, "provided, ") || strings.Contains(tab, "contract-pill") {
		t.Errorf("a refused claim lists what it provides:\n%s", tab)
	}
	in := mainOf(newInClusterSite(t, apitest.F1PackageHolder(t), apitest.AllowAll).get(t, "/packages/pkg/provider?tab=provider").body)
	cluster := between(in, `id="provider"`, "</section>")
	if !strings.Contains(cluster, "none: the claim is refused") || strings.Contains(cluster, "which does not exist") {
		t.Errorf("in-cluster package Provider tab:\n%s", cluster)
	}
}

// TestProviderTabUsersNotFullyReadable: a caller who may list instances in
// one namespace only gets no in-use count and a note that the list may be
// incomplete.
func TestProviderTabUsersNotFullyReadable(t *testing.T) {
	rule := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Verb != "list" || ra.Resource != "moduleinstances" || ra.Namespace != ""
	}
	res := newSite(t, apitest.F1(t), rule).get(t, "/instances/default/backup-provider?tab=provider")
	tab := between(mainOf(res.body), `id="provider"`, "</section>")
	if res.status != http.StatusOK || !strings.Contains(tab, "<dt>Contracts</dt><dd>1 provided; use not fully readable</dd>") ||
		!strings.Contains(tab, "The list may be incomplete") || strings.Contains(tab, "1 in use") {
		t.Errorf("%d\n%s", res.status, tab)
	}
}

// TestProviderTabHostileText: cluster text on the tab is escaped.
func TestProviderTabHostileText(t *testing.T) {
	s := newSite(t, hostileF1(t), apitest.AllowAll)
	for _, path := range []string{"/instances/default/backup-provider?tab=provider"} {
		body := s.get(t, path).body
		if strings.Contains(body, "<script>alert") || strings.Contains(body, "<img src=x") || !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("%s does not escape the cluster text", path)
		}
	}
}

// renderOwnerTab renders the owner page content over v and returns the
// Provider tab's section.
func renderOwnerTab(t *testing.T, h *Handler, v *ownerView) string {
	t.Helper()
	v.Cards = h.cardsOf(v)
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages["owner"].ExecuteTemplate(&b, "content", page{Path: "/x", Main: *v}); err != nil {
		t.Fatal(err)
	}
	return between(b.String(), `id="provider"`, "</section>")
}
