package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// renderMain executes one page template's content over a synthetic view,
// for the states F1 does not hold.
func renderMain(t *testing.T, name string, main any) string {
	t.Helper()
	h := &Handler{cfg: Config{Now: time.Now}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages[name].ExecuteTemplate(&b, "content", page{Path: "/x", Main: main}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestAcceptedAndActiveApart (0030:D4:R4/R7): an accepted, inactive claim
// shows two pills on their own, and a blocked removal reads as removal
// blocked while accepted and active, never as refused.
func TestAcceptedAndActiveApart(t *testing.T) {
	v := platformView{Platform: v1.Platform{
		Name:                "cluster",
		RegistrationsAccess: v1.AccessOK,
		Registrations: []v1.Registration{
			{Name: "pending", Catalog: "c@v0", Accepted: true, Active: false, Verdict: "Accepted"},
			{Name: "leaving", Catalog: "d@v0", Accepted: true, Active: true, Verdict: "RemovalBlocked", Reason: "DependentsRemain", Message: "2 instances still demand it"},
		},
	}}
	out := renderMain(t, "platform", v)
	pending := between(out, ">pending<", "</li>")
	if !strings.Contains(pending, ">accepted<") || !strings.Contains(pending, ">inactive<") {
		t.Errorf("accepted, inactive claim:\n%s", pending)
	}
	leaving := between(out, ">leaving<", "</li>")
	if !strings.Contains(leaving, "Removal blocked") || !strings.Contains(leaving, ">accepted<") || !strings.Contains(leaving, ">active<") ||
		strings.Contains(leaving, "Refused") || !strings.Contains(leaving, "DependentsRemain") {
		t.Errorf("removal-blocked claim:\n%s", leaving)
	}
}

// TestSecretsOfferNoYAML (0030:D8:R1): a Secret in an inventory says its
// data is never read and links to nothing.
func TestSecretsOfferNoYAML(t *testing.T) {
	h := &Handler{cfg: Config{Now: time.Now}}
	ref := v1.ObjectRef{Version: "v1", Kind: "Secret", Namespace: "default", Name: "db"}
	v := ownerView{Kind: instanceKind, Namespace: "default", Name: "app", Components: h.components("/instances/default/app", []v1.Component{{
		Name:    "db",
		Objects: []v1.InventoryObject{{Ref: ref, Access: v1.AccessOK, Health: &v1.ObjectHealth{State: "Healthy"}, Live: true}},
	}})}
	out := between(renderMain(t, "owner", v), `id="components"`, "</section>")
	if !strings.Contains(out, "Secret data is never read") || strings.Contains(out, "kind=Secret") {
		t.Errorf("Secret row:\n%s", out)
	}
}

// TestPolledObjectsSayWhenTheyWereRead (0030:D3:R5): a polled object and
// a health that is not live show when they were evaluated.
func TestPolledObjectsSayWhenTheyWereRead(t *testing.T) {
	h := &Handler{cfg: Config{Now: func() time.Time { return time.Date(2026, 10, 5, 12, 1, 0, 0, time.UTC) }}}
	read := time.Date(2026, 10, 5, 12, 0, 30, 0, time.UTC)
	ref := v1.ObjectRef{Version: "v1", Kind: "Service", Namespace: "default", Name: "web"}
	v := ownerView{
		Kind: instanceKind, Namespace: "default", Name: "app",
		Health: v1.Health{State: "Healthy", Live: false, EvaluatedAt: &read},
		Components: h.components("/instances/default/app", []v1.Component{{
			Name:    "web",
			Objects: []v1.InventoryObject{{Ref: ref, Access: v1.AccessOK, Health: &v1.ObjectHealth{State: "Healthy"}, EvaluatedAt: &read}},
		}}),
	}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages["owner"].ExecuteTemplate(&b, "content", page{Path: "/x", Main: v}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "not live · read <time datetime=\"2026-10-05T12:00:30Z\"") || !strings.Contains(out, "health evaluated <time") || !strings.Contains(out, "Healthy (not live)") {
		t.Errorf("polled object without its read time:\n%s", between(out, `id="owner-head"`, `id="conditions"`))
	}
}
