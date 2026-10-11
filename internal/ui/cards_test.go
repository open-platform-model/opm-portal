package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The summary cards of an instance or package page: the word, the hue and the
// summary of every state, composed from recorded fields only (design.md
// decision 2 of align-owner-pages).

var cardsNow = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func cardsHandler() *Handler { return &Handler{cfg: Config{Now: func() time.Time { return cardsNow }}} }

func failedEntries(n int) []v1.HistoryEntry {
	out := make([]v1.HistoryEntry, 0, n)
	for i := n; i >= 1; i-- {
		out = append(out, v1.HistoryEntry{Action: "reconcile", Outcome: "Failed", Sequence: int64(i)})
	}
	return out
}

func podinfoPackage(interval string) *v1.Package {
	p := &v1.Package{}
	p.Ref = v1.ObjectRef{Group: "opmodel.dev", Kind: "ModulePackage", Namespace: "pkg", Name: "podinfo"}
	p.Interval = interval
	return p
}

func podinfoInstance(objects int64) *v1.Instance {
	i := &v1.Instance{}
	i.Ref = v1.ObjectRef{Group: "opmodel.dev", Kind: "ModuleInstance", Namespace: "default", Name: "podinfo"}
	i.InventoryCount = objects
	return i
}

// TestAppliedBlockEveryState: the tone table and the summary table of
// decision 2, a row each: the hue, the word, the summary, the cadence named
// only for SourceNotReady, no failed-run clause for a zero run, and the
// Reconciling word only on an Applied owner.
func TestAppliedBlockEveryState(t *testing.T) {
	reconciling := v1.Condition{Type: "Reconciling", Status: "True", Reason: "Progressing"}
	for _, tc := range []struct {
		name     string
		view     ownerView
		hue      string
		word     string
		summary  string
		hasMark  bool // the Reconciling mark note
		noNoteOf string
	}{
		{"applied instance", ownerView{Instance: podinfoInstance(2), Reconcile: v1.Reconcile{State: "Applied"}, History: []v1.HistoryEntry{{Outcome: "Succeeded"}}},
			"applied", "Applied", "2 objects applied", false, ""},
		{"applied with Reconciling true", ownerView{Instance: podinfoInstance(2), Reconcile: v1.Reconcile{State: "Applied"}, Conditions: []v1.Condition{reconciling}, History: []v1.HistoryEntry{{Outcome: "Succeeded"}}},
			"progressing", "Reconciling", "2 objects applied", false, ""},
		{"applied package with a fetched revision", ownerView{Package: func() *v1.Package {
			p := podinfoPackage("1m")
			p.InventoryCount = 3
			p.SourceArtifact = &v1.SourceArtifact{Revision: "6.5.1@sha256:abcdef0123456789abcdef", Digest: "sha256:ffff"}
			return p
		}(), Reconcile: v1.Reconcile{State: "Applied"}, History: []v1.HistoryEntry{{Outcome: "Succeeded"}}},
			"applied", "Applied", "Fetched 6.5.1@abcdef012345, 3 objects applied", false, ""},
		{"applied package with only a digest", ownerView{Package: func() *v1.Package {
			p := podinfoPackage("1m")
			p.InventoryCount = 1
			p.SourceArtifact = &v1.SourceArtifact{Digest: "sha256:0123456789abcdef"}
			return p
		}(), Reconcile: v1.Reconcile{State: "Applied"}, History: []v1.HistoryEntry{{Outcome: "Succeeded"}}},
			"applied", "Applied", "Fetched 0123456789ab, 1 object applied", false, ""},
		{"nothing applied yet", ownerView{Instance: podinfoInstance(0), Reconcile: v1.Reconcile{State: "Applied"}},
			"applied", "Applied", "Nothing applied yet", false, ""},
		{"failed source, three failed attempts, cadence named", ownerView{Package: podinfoPackage("1m"), History: failedEntries(3), Reconcile: v1.Reconcile{State: "Failed", Reason: "SourceNotReady", Retrying: true}},
			"degraded", "Failed, retrying", "Failed on the last 3 attempts, retrying every 1 min", false, ""},
		{"failed source, a zero run, cadence named", ownerView{Package: podinfoPackage("30s"), History: []v1.HistoryEntry{{Outcome: "Succeeded"}}, Reconcile: v1.Reconcile{State: "Failed", Reason: "SourceNotReady", Retrying: true}},
			"degraded", "Failed, retrying", "SourceNotReady, retrying every 30 s", false, ""},
		{"failed apply, no cadence for another reason", ownerView{Package: podinfoPackage("1m"), History: failedEntries(2), Reconcile: v1.Reconcile{State: "Failed", Reason: "ApplyFailed", Retrying: true}},
			"degraded", "Failed, retrying", "Failed on the last 2 attempts, retrying", false, ""},
		{"failed instance, one attempt, not retrying", ownerView{Instance: podinfoInstance(0), History: failedEntries(1), Reconcile: v1.Reconcile{State: "Failed", Reason: "ApplyFailed"}},
			"degraded", "Failed", "Failed on the last attempt", false, ""},
		{"failed with Reconciling true keeps Failed and the mark", ownerView{Package: podinfoPackage("1m"), Conditions: []v1.Condition{reconciling}, History: failedEntries(3), Reconcile: v1.Reconcile{State: "Failed", Reason: "SourceNotReady", Retrying: true}},
			"degraded", "Failed, retrying", "Failed on the last 3 attempts, retrying every 1 min", true, ""},
		{"stalled", ownerView{Instance: podinfoInstance(1), Reconcile: v1.Reconcile{State: "Stalled", Reason: "InvalidSpec"}},
			"degraded", "Stalled", "InvalidSpec", false, ""},
		{"reconciling state", ownerView{Instance: podinfoInstance(1), Conditions: []v1.Condition{reconciling}, Reconcile: v1.Reconcile{State: "Reconciling", Reason: "Progressing"}},
			"progressing", "Reconciling", "Progressing", false, ""},
		{"managed externally", ownerView{Instance: podinfoInstance(1), Reconcile: v1.Reconcile{State: "ManagedExternally"}},
			"neutral", "Managed externally", "Applied by the opm CLI; the controller does not manage it", false, ""},
		{"suspended", ownerView{Instance: podinfoInstance(1), Reconcile: v1.Reconcile{State: "Suspended"}},
			"neutral", "Suspended", "Reconciling is suspended", false, ""},
		{"unknown", ownerView{Instance: podinfoInstance(1), Reconcile: v1.Reconcile{State: "Unknown"}},
			"unknown", "Unknown", "The portal does not know this applied state", false, ""},
		{"a state the UI does not know", ownerView{Instance: podinfoInstance(1), Reconcile: v1.Reconcile{State: "Exploding"}},
			"unknown", "Unknown", "The portal does not know this applied state", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.view
			v.Kind, v.Namespace, v.Name, v.Topic, v.Base = instanceKind, "default", "podinfo", "instance:default/podinfo", "/instances/default/podinfo"
			if v.Package != nil {
				v.Kind, v.Namespace, v.Topic, v.Base = packageKind, "pkg", "package:pkg/podinfo", "/packages/pkg/podinfo"
			}
			b := cardsHandler().appliedBlock(&v)
			if b.Hue != tc.hue || b.State != tc.word || b.Summary != tc.summary {
				t.Errorf("hue %q word %q summary %q; want %q %q %q", b.Hue, b.State, b.Summary, tc.hue, tc.word, tc.summary)
			}
			mark := slices.ContainsFunc(b.Notes, func(n sbNote) bool { return n.Class == "reconciling" })
			if mark != tc.hasMark {
				t.Errorf("the Reconciling mark is %v; want %v (notes %+v)", mark, tc.hasMark, b.Notes)
			}
			if b.Caption != "Warning events, last hour" || b.None != "No reconcile errors" {
				t.Errorf("caption %q none %q", b.Caption, b.None)
			}
		})
	}
}

// TestAppliedBlockNotes: the reason's meaning and the controller's message
// stay beneath the summary, and the since stamp is the Reconcile's.
func TestAppliedBlockNotes(t *testing.T) {
	since := cardsNow.Add(-5 * time.Minute)
	v := ownerView{Instance: podinfoInstance(2), Topic: "instance:default/podinfo", Base: "/instances/default/podinfo",
		Reconcile:  v1.Reconcile{State: "Failed", Reason: "ApplyFailed", Message: "boom <b>", Since: &since},
		Conditions: []v1.Condition{{Type: "Ready", Reason: "ApplyFailed", Meaning: "An object failed to apply."}}}
	b := cardsHandler().appliedBlock(&v)
	if len(b.Notes) != 2 || b.Notes[0].Text != "An object failed to apply." || b.Notes[1].Text != "boom <b>" || b.Notes[1].Class != "says" {
		t.Errorf("notes %+v", b.Notes)
	}
	if b.When == nil || !b.When.Equal(since) || b.WhenText != "since" {
		t.Errorf("when %v %q", b.When, b.WhenText)
	}
	// In-cluster the message is omitted by the read API; the card then holds the meaning only.
	v.Reconcile.Message = ""
	if b := cardsHandler().appliedBlock(&v); len(b.Notes) != 1 {
		t.Errorf("without the controller's message: notes %+v", b.Notes)
	}
}

// TestWarningCountsAreTheOwnersOwn: of a merged feed, only the Warning events
// regarding the owner are counted, count first, folded by reason.
func TestWarningCountsAreTheOwnersOwn(t *testing.T) {
	v := ownerView{Kind: instanceKind, Namespace: "default", Name: "podinfo", Topic: "instance:default/podinfo", Base: "/instances/default/podinfo",
		Instance: podinfoInstance(2), Reconcile: v1.Reconcile{State: "Applied"}}
	owner := v.ownerRef()
	pod := v1.ObjectRef{Kind: "Pod", Namespace: "default", Name: "podinfo-1"}
	v.Events.Items = []v1.Event{
		{Type: "Warning", Reason: "ApplyFailed", Count: 2, Regarding: owner},
		{Type: "Warning", Reason: "BackOff", Count: 9, Regarding: pod},
		{Type: "Normal", Reason: "Applied", Count: 1, Regarding: owner},
		{Type: "Warning", Reason: "ApplyFailed", Count: 1, Regarding: owner},
	}
	b := cardsHandler().appliedBlock(&v)
	if len(b.Reasons) != 1 || b.Reasons[0].Text != "ApplyFailed" || b.Reasons[0].N != 3 || !strings.Contains(b.Reasons[0].Href, "type=Warning") {
		t.Errorf("reasons %+v; want ApplyFailed counted 3 and no BackOff from the Pod", b.Reasons)
	}
	// A feed that could not be read is said so; "No reconcile errors" would claim a clean bill.
	v.Events = eventsView{Problem: &v1.Problem{Code: v1.CodeUpstreamUnavailable}}
	if b := cardsHandler().appliedBlock(&v); len(b.Reasons) != 0 || b.None != "Events could not be read" {
		t.Errorf("unreadable feed: reasons %+v none %q", b.Reasons, b.None)
	}
}

// TestHealthBlock: the word and hue of each state, the summary table, the
// when (none when the counts total zero), and the partial note.
func TestHealthBlock(t *testing.T) {
	eval := cardsNow.Add(-2 * time.Minute)
	for _, tc := range []struct {
		name         string
		health       v1.Health
		hue, word    string
		summary      string
		whenText     string
		hasWhen      bool
		partialNoted bool
	}{
		{"healthy and live", v1.Health{State: "Healthy", Live: true, Counts: v1.HealthCounts{Healthy: 4}}, "healthy", "Healthy", "All 4 resources healthy", "checked live", false, false},
		{"one resource", v1.Health{State: "Healthy", Live: true, Counts: v1.HealthCounts{Healthy: 1}}, "healthy", "Healthy", "All 1 resource healthy", "checked live", false, false},
		{"degraded and missing", v1.Health{State: "Degraded", Live: true, Counts: v1.HealthCounts{Healthy: 2, Degraded: 1, Missing: 1}}, "degraded", "Degraded", "2 of 4 resources unhealthy", "checked live", false, false},
		{"missing", v1.Health{State: "Missing", Live: true, Counts: v1.HealthCounts{Missing: 1}}, "missing", "Missing", "1 of 1 resource unhealthy", "checked live", false, false},
		{"progressing", v1.Health{State: "Progressing", Live: true, Counts: v1.HealthCounts{Healthy: 1, Progressing: 2}}, "progressing", "Progressing", "2 of 3 resources rolling out", "checked live", false, false},
		{"unknown counts", v1.Health{State: "Unknown", Live: true, Counts: v1.HealthCounts{Unknown: 2}}, "unknown", "Unknown", "2 of 2 resources in an unknown state", "checked live", false, false},
		{"not live, evaluated", v1.Health{State: "Healthy", Live: false, EvaluatedAt: &eval, Counts: v1.HealthCounts{Healthy: 2}}, "healthy", "Healthy", "All 2 resources healthy", "not live, evaluated", true, false},
		{"not live, no time", v1.Health{State: "Healthy", Live: false, Counts: v1.HealthCounts{Healthy: 2}}, "healthy", "Healthy", "All 2 resources healthy", "not live", false, false},
		{"counted nothing", v1.Health{State: "Unknown", Live: false, EvaluatedAt: &eval}, "unknown", "Unknown", "Nothing applied yet, so nothing to check", "", false, false},
		{"partial", v1.Health{State: "Healthy", Live: true, Partial: true, Counts: v1.HealthCounts{Healthy: 1}}, "healthy", "Healthy", "All 1 resource healthy", "checked live", false, true},
		{"a state the UI does not know", v1.Health{State: "Melting", Live: true, Counts: v1.HealthCounts{Healthy: 1}}, "unknown", "Unknown", "All 1 resource healthy", "checked live", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := ownerView{Topic: "instance:default/podinfo", Health: tc.health}
			b := cardsHandler().healthBlock(&v, nil)
			if b.Hue != tc.hue || b.State != tc.word || b.Summary != tc.summary || b.WhenText != tc.whenText || (b.When != nil) != tc.hasWhen {
				t.Errorf("hue %q word %q summary %q when %q/%v; want %q %q %q %q/%v", b.Hue, b.State, b.Summary, b.WhenText, b.When != nil, tc.hue, tc.word, tc.summary, tc.whenText, tc.hasWhen)
			}
			noted := slices.ContainsFunc(b.Notes, func(n sbNote) bool {
				return n.Text == "Partial: some objects could not be read and are left out." && n.Class == "partial"
			})
			if noted != tc.partialNoted {
				t.Errorf("partial note %v; want %v", noted, tc.partialNoted)
			}
			if b.State == "Healthy" && tc.partialNoted && b.State != "Healthy" {
				t.Error("a partial health changed the state word")
			}
			if b.None != "No unhealthy resources" {
				t.Errorf("none %q", b.None)
			}
		})
	}
}

// TestProviderBlockEveryVerdict (OP-A4): every verdict of a claim has a word,
// a hue and a non-empty summary through three inputs, the controller's text
// (local mode), the reason alone (in-cluster: the message is omitted) and
// neither (no Platform given), and the fallback sentence is the one of
// decision 2.
func TestProviderBlockEveryVerdict(t *testing.T) {
	type input struct {
		name string
		reg  func(v1.ProviderClaim) *v1.Registration
		want func(word, fallback, summary string) bool
	}
	verdicts := []struct {
		name     string
		claim    v1.ProviderClaim
		word     string
		hue      string
		fallback string
	}{
		{"Accepted and active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true, Active: true, Reason: "Accepted"}, "Active", "healthy", "Accepted and active."},
		{"Accepted and not active", v1.ProviderClaim{Verdict: "Accepted", Accepted: true, Reason: "ProviderNotReady"}, "Accepted", "progressing", "Accepted; not active yet, so it provides nothing yet."},
		{"Pending", v1.ProviderClaim{Verdict: "Pending", Reason: "Waiting"}, "Pending", "progressing", "The controller has not decided on this claim yet."},
		{"Refused", v1.ProviderClaim{Verdict: "Refused", Reason: "CatalogUnresolved"}, "Refused", "degraded", "The controller refused this claim."},
		{"Removal blocked", v1.ProviderClaim{Verdict: "RemovalBlocked", Accepted: true, Active: true, Reason: "InUse"}, "Removal blocked", "unknown", "Accepted and active; its removal waits until no module uses what it provides."},
		{"Unknown", v1.ProviderClaim{Verdict: "Unknown"}, "Unknown", "unknown", "The portal does not know this claim's standing."},
		{"a verdict the UI does not know", v1.ProviderClaim{Verdict: "Levitating", Reason: "Odd"}, "Unknown", "unknown", "The portal does not know this claim's standing."},
		{"no verdict", v1.ProviderClaim{}, "Unknown", "unknown", "The portal does not know this claim's standing."},
	}
	inputs := []input{
		{"the controller's text (local mode)", func(c v1.ProviderClaim) *v1.Registration {
			return &v1.Registration{Name: "default.x", Message: "text from the controller", ActiveMessage: "active text from the controller", Verdict: c.Verdict, Accepted: c.Accepted, Active: c.Active, Reason: c.Reason}
		}, func(word, _, summary string) bool {
			return summary == "text from the controller" || summary == "active text from the controller" ||
				(word == "Active" && strings.HasPrefix(summary, "Accepted and active")) || strings.HasPrefix(summary, "Provides ")
		}},
		{"the reason alone (in-cluster)", func(c v1.ProviderClaim) *v1.Registration {
			return &v1.Registration{Name: "default.x", Verdict: c.Verdict, Accepted: c.Accepted, Active: c.Active, Reason: c.Reason}
		}, func(word, fallback, summary string) bool {
			// A registration that gave no provides reads as providing none.
			return summary == fallback || (word == "Active" && summary == "Accepted and active; it provides no contract.")
		}},
		{"neither (no registration given)", func(v1.ProviderClaim) *v1.Registration { return nil },
			func(_, fallback, summary string) bool { return summary == fallback }},
	}
	for _, vd := range verdicts {
		for _, in := range inputs {
			t.Run(vd.name+"/"+in.name, func(t *testing.T) {
				claim := vd.claim
				claim.Registration, claim.Access = "default.x", v1.AccessOK
				v := ownerView{Topic: "instance:default/x", Base: "/instances/default/x", ProviderOf: []v1.ProviderClaim{claim}}
				if reg := in.reg(claim); reg != nil {
					v.plat = platformRead{Read: true, Doc: v1.Platform{Registrations: []v1.Registration{*reg}}}
				}
				b := cardsHandler().providerBlock(&v)
				if b.State != vd.word || b.Hue != vd.hue {
					t.Errorf("word %q hue %q; want %q %q", b.State, b.Hue, vd.word, vd.hue)
				}
				if strings.TrimSpace(b.Summary) == "" {
					t.Fatal("the state word stands over an empty summary")
				}
				if !in.want(vd.word, vd.fallback, b.Summary) {
					t.Errorf("summary %q does not follow the input rule (fallback %q)", b.Summary, vd.fallback)
				}
			})
		}
	}
}

// TestProviderBlockMeaningAndReasons: the reason's meaning beats the fixed
// sentence, a non-success reason is a count linking to the Provider tab, a
// success shows "No registration problems", and an unreadable claim is a
// problem, not a state.
func TestProviderBlockMeaningAndReasons(t *testing.T) {
	ready := cardsNow.Add(-time.Hour)
	reg := v1.Registration{Name: "default.x", Reason: "CatalogUnresolved", Conditions: []v1.Condition{
		{Type: "Ready", Reason: "CatalogUnresolved", Meaning: "The claimed catalog could not be fetched.", LastTransitionTime: &ready},
	}}
	claim := v1.ProviderClaim{Registration: "default.x", Access: v1.AccessOK, Verdict: "Refused", Reason: "CatalogUnresolved"}
	v := ownerView{Topic: "package:pkg/x", Base: "/packages/pkg/x", ProviderOf: []v1.ProviderClaim{claim,
		{Registration: "default.y", Access: v1.AccessOK, Verdict: "Pending"}},
		plat: platformRead{Read: true, Doc: v1.Platform{Registrations: []v1.Registration{reg}}}}
	b := cardsHandler().providerBlock(&v)
	if b.Summary != "The claimed catalog could not be fetched." || b.WhenText != "refused" || b.When == nil {
		t.Errorf("summary %q when %q/%v", b.Summary, b.WhenText, b.When)
	}
	if len(b.Reasons) != 1 || b.Reasons[0].Text != "CatalogUnresolved" || b.Reasons[0].N != 1 || b.Reasons[0].Href != "/packages/pkg/x?tab=provider" {
		t.Errorf("reasons %+v", b.Reasons)
	}
	if len(b.Notes) != 1 || !strings.Contains(b.Notes[0].Text, "default.y") || !strings.HasPrefix(b.Notes[0].Text, "Pending") {
		t.Errorf("the second claim is a note: %+v", b.Notes)
	}
	if len(b.Links) != 1 || b.Links[0].Text != "Open the Provider tab" {
		t.Errorf("links %+v", b.Links)
	}
	if !strings.Contains(b.Follow, platformTopic) {
		t.Errorf("follow %q lacks the platform topic", b.Follow)
	}
}

// TestProviderBlockSuccessAndProblems: success shows "No registration
// problems"; a claim the caller may not read, or the portal cannot, is a
// problem region and not a state.
func TestProviderBlockSuccessAndProblems(t *testing.T) {
	ok := ownerView{Topic: "instance:default/x", Base: "/instances/default/x", ProviderOf: []v1.ProviderClaim{{Registration: "default.x", Access: v1.AccessOK, Verdict: "Accepted", Active: true}}}
	if b := cardsHandler().providerBlock(&ok); len(b.Reasons) != 0 || b.None != "No registration problems" {
		t.Errorf("accepted and active: reasons %+v none %q", b.Reasons, b.None)
	}
	locked := ownerView{Topic: "instance:default/x", Base: "/instances/default/x", ProviderOf: []v1.ProviderClaim{{Registration: "default.x", Access: v1.AccessForbidden}}}
	if b := cardsHandler().providerBlock(&locked); b.Hue != "locked" || b.Problem == nil || b.Problem.Code != v1.CodeForbidden {
		t.Errorf("forbidden claim: hue %q problem %+v", b.Hue, b.Problem)
	}
	unread := ownerView{Topic: "instance:default/x", Base: "/instances/default/x", ProviderOf: []v1.ProviderClaim{{Registration: "default.x", Access: v1.AccessNotReadable}}}
	if b := cardsHandler().providerBlock(&unread); b.Hue != "neutral" || b.Problem == nil || b.Problem.Code != v1.CodeNotReadableByPortal {
		t.Errorf("unreadable claim: hue %q problem %+v", b.Hue, b.Problem)
	}
}

// TestHistoryStrip: a dot per attempt, oldest first, marked by outcome, each
// a tip with its own id and a name; the fold holds the attempts and the
// ten-attempt note; a block without a strip draws none (the fragment goldens
// of the state block stay unchanged).
func TestHistoryStrip(t *testing.T) {
	fin := cardsNow.Add(-time.Minute)
	history := []v1.HistoryEntry{
		{Sequence: 3, Action: "reconcile", Outcome: "Failed", Message: "boom <script>alert(1)</script>", FinishedAt: &fin},
		{Sequence: 2, Action: "reconcile", Outcome: "Succeeded", FinishedAt: &fin},
		{Sequence: 1, Action: "reconcile", Outcome: "weird", FinishedAt: &fin},
	}
	s := historyOf(history)
	if s.Count != "last 3 attempts" || len(s.Dots) != 3 {
		t.Fatalf("count %q dots %d", s.Count, len(s.Dots))
	}
	for i, want := range []struct{ class, mark, id string }{{"dot-unknown", "…", "attempt-1"}, {"dot-succeeded", "✓", "attempt-2"}, {"dot-failed", "✕", "attempt-3"}} {
		d := s.Dots[i]
		if d.Class != want.class || d.Mark != want.mark || d.Tip.ID != want.id || d.Tip.Label == "" {
			t.Errorf("dot %d = %+v; want %+v", i, d, want)
		}
	}
	if one := historyOf(history[:1]); one.Count != "1 attempt" {
		t.Errorf("one attempt reads %q", one.Count)
	}
	out := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "applied-card", Label: "Applied status", Hue: "degraded", State: "Failed", History: s}))
	for _, want := range []string{`<div class="hist">`, `class="adot dot-unknown"`, `<details class="hist-fold" id="history-fold">`, "Show history ▾", "Hide history ▴",
		"At most ten attempts are kept", `outcome outcome-failed">Failed<`, `outcome outcome-succeeded">Applied<`, `outcome outcome-unknown">Unknown<`,
		"boom &lt;script&gt;alert(1)&lt;/script&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("strip lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<script>alert") {
		t.Error("an attempt's message is drawn as markup")
	}
	// The note is inside the opened body: after <details, before </details>.
	fold := between(out, "<details", "</details>")
	if !strings.Contains(fold, "At most ten attempts are kept") {
		t.Error("the ten-attempt note is outside the fold")
	}
}

// TestHistoryStripEmpty: no attempts draw the label and "none recorded", no
// fold; a block without a strip draws none.
func TestHistoryStripEmpty(t *testing.T) {
	empty := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "applied", State: "Applied", History: historyOf(nil)}))
	if !strings.Contains(empty, "none recorded") || strings.Contains(empty, "<details") {
		t.Errorf("no history: %s", empty)
	}
	bare := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "applied", State: "Applied"}))
	if strings.Contains(bare, "hist") {
		t.Errorf("a block without a strip draws one:\n%s", bare)
	}
}

func TestHumanInterval(t *testing.T) {
	for in, want := range map[string]string{"1m": "1 min", "5m0s": "5 min", "30s": "30 s", "2h": "2 h", "90s": "90 s", "1m30s": "90 s", "nonsense": "nonsense", "": "", "0s": "0s"} {
		if got := humanInterval(in); got != want {
			t.Errorf("humanInterval(%q) = %q; want %q", in, got, want)
		}
	}
}
