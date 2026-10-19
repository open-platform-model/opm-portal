package ui

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The instance and package page's summary cards: the Applied, Health and
// Provider axes as state blocks (portal:D19:R3), and the identity card's
// facts. Every word and number on a card comes from a recorded field; a
// sentence the controller did not write is never phrased as its verdict
// (design.md decision 2).

// Ids of the cards, which the live refresh and the tests name.
const (
	appliedCardID  = "applied-card"
	healthCardID   = "health-card"
	providerCardID = "provider-card"
)

// Applied states the UI knows that carry a hue of their own.
const (
	stateApplied     = "Applied"
	stateReconciling = "Reconciling"
	stateFailed      = "Failed"
	stateStalled     = "Stalled"
	stateSuspended   = "Suspended"
	stateManaged     = "ManagedExternally"
)

// Health states.
const (
	healthHealthy     = "Healthy"
	healthProgressing = "Progressing"
	healthDegraded    = "Degraded"
	healthMissing     = "Missing"
)

// Hues of the state block that more than one card draws, and the hue and the
// word of a state the UI does not know.
const (
	hueProgressing = "progressing"
	hueDegraded    = "degraded"
	hueUnknown     = "unknown"
	wordUnknown    = "Unknown"
	wordActive     = "Active"
	ownerCLI       = "cli"
	outcomeFailed  = "Failed"
	outcomeOK      = "Succeeded"
)

// ownerCards are the cards under the identity: the Applied and Health
// blocks, the Provider block when the owner holds a claim, and the Health
// reasons the Resources tab filters by.
type ownerCards struct {
	Applied  stateBlock
	Health   stateBlock
	Provider *stateBlock
	Reasons  []countLink
}

// platformRead is the Platform document as an owner page reads it: the
// Provider card needs it on every tab, the Provider tab reuses it.
type platformRead struct {
	Doc v1.Platform
	// Read: the document was read; Locked: the caller may not read the
	// Platform; Problem: another failure.
	Read    bool
	Locked  bool
	Problem *v1.Problem
}

func (p *platformRead) registration(name string) *v1.Registration {
	if p == nil || !p.Read {
		return nil
	}
	for i := range p.Doc.Registrations {
		if p.Doc.Registrations[i].Name == name {
			return &p.Doc.Registrations[i]
		}
	}
	return nil
}

// noReasonRecorded is the words of a block that has no reason to show.
const noReasonRecorded = "No reason recorded"

// ProviderPills is the kicker's round Provider tag of each registration the
// owner holds, drawn through the tip partial (portal:D15:R2/R4).
func (v ownerView) ProviderPills() []tipData {
	return providerPills(v.Kind.Topic, v.ownerRef(), v.ProviderOf)
}

// ownerRef is the owner as an event names it.
func (v *ownerView) ownerRef() v1.ObjectRef {
	return v1.ObjectRef{Group: "opmodel.dev", Kind: v.Kind.objectKind(), Namespace: v.Namespace, Name: v.Name}
}

// cardsOf builds the cards of the owner v.
func (h *Handler) cardsOf(v *ownerView) ownerCards {
	reasons := healthReasons(v)
	c := ownerCards{Applied: h.appliedBlock(v), Health: h.healthBlock(v, reasons), Reasons: reasons}
	if len(v.ProviderOf) > 0 {
		b := h.providerBlock(v)
		c.Provider = &b
	}
	return c
}

// --- Applied ---

// appliedHue is the hue of an applied state; a state the UI does not know
// reads unknown (portal:D2:R3).
func appliedBlockHue(state string, reconciling bool) string {
	switch state {
	case stateApplied:
		if reconciling {
			return hueProgressing
		}
		return "applied"
	case stateReconciling:
		return hueProgressing
	case stateFailed, stateStalled:
		return hueDegraded
	case stateSuspended, stateManaged:
		return hueNeutral
	}
	return hueUnknown
}

// appliedWord is the word of an applied state. Reconciling is the word only
// on an Applied owner whose Reconciling condition is true: a package that is
// Failed keeps Failed and shows the mark beside it.
func appliedWord(r v1.Reconcile, reconciling bool) string {
	if r.State == stateApplied && reconciling {
		return stateReconciling
	}
	if _, ok := appliedText[r.State]; !ok {
		return wordUnknown
	}
	return appliedBadge(r).Text
}

// reconcilingCondition is the owner's Reconciling condition when it is true.
func reconcilingCondition(conds []v1.Condition) *v1.Condition {
	for i := range conds {
		if conds[i].Type == "Reconciling" && conds[i].Status == "True" {
			return &conds[i]
		}
	}
	return nil
}

// reasonMeaning is the portal's explanation of the Reconcile reason, from the
// condition that carries it.
func reasonMeaning(conds []v1.Condition, reason string) string {
	if reason == "" {
		return ""
	}
	for i := range conds {
		if conds[i].Reason == reason && conds[i].Meaning != "" {
			return conds[i].Meaning
		}
	}
	return ""
}

// failedRun is the trailing run of failed attempts: History is newest first,
// so it is the count of failed entries before the first that is not.
func failedRun(history []v1.HistoryEntry) int {
	n := 0
	for i := range history {
		if history[i].Outcome != outcomeFailed {
			break
		}
		n++
	}
	return n
}

// humanInterval says a Go duration the way a person reads it: "1 min",
// "90 s", "2 h". A value that is not a duration is shown as written.
func humanInterval(s string) string {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return s
	}
	switch {
	case d%time.Hour == 0:
		return strconv.FormatInt(int64(d/time.Hour), 10) + " h"
	case d%time.Minute == 0:
		return strconv.FormatInt(int64(d/time.Minute), 10) + " min"
	case d%time.Second == 0:
		return strconv.FormatInt(int64(d/time.Second), 10) + " s"
	}
	return s
}

// shortRevision cuts a source revision for the summary: the tag or branch,
// and a digest cut to twelve characters.
func shortRevision(rev string) string {
	if rev == "" {
		return ""
	}
	if left, right, ok := strings.Cut(rev, "@"); ok {
		return left + "@" + short(right)
	}
	return short(rev)
}

// objectsApplied says how many objects were applied.
func objectsApplied(n int64) string {
	return itoa(n) + " " + plural(int(n), "object", "objects") + " applied"
}

// appliedSummary composes the Applied card's summary from recorded facts
// only (design.md decision 2).
func (v *ownerView) appliedSummary(reconciling *v1.Condition) string {
	r := v.Reconcile
	switch r.State {
	case stateFailed, stateStalled:
		return v.failedSummary()
	case stateReconciling:
		if reconciling != nil && reconciling.Reason != "" {
			return reconciling.Reason
		}
		if r.Reason != "" {
			return r.Reason
		}
		return stateReconciling
	case stateManaged:
		return "Applied by the opm CLI; the controller does not manage it"
	case stateApplied:
		return v.appliedFacts()
	case stateSuspended:
		if r.Reason != "" {
			return r.Reason
		}
		return "Reconciling is suspended"
	}
	if r.Reason != "" {
		return r.Reason
	}
	return "The portal does not know this applied state"
}

// appliedFacts is the summary of an Applied owner: what was fetched and how
// many objects were applied, or that nothing was.
func (v *ownerView) appliedFacts() string {
	n := v.inventoryCount()
	if n == 0 && len(v.History) == 0 {
		return "Nothing applied yet"
	}
	if v.Package != nil && v.Package.SourceArtifact != nil {
		a := v.Package.SourceArtifact
		rev := shortRevision(a.Revision)
		if rev == "" {
			rev = short(a.Digest)
		}
		if rev != "" {
			return "Fetched " + rev + ", " + objectsApplied(n)
		}
	}
	return objectsApplied(n)
}

func (v *ownerView) inventoryCount() int64 {
	switch {
	case v.Instance != nil:
		return v.Instance.InventoryCount
	case v.Package != nil:
		return v.Package.InventoryCount
	}
	return 0
}

// failedSummary is the summary of a Failed or Stalled owner. The failed-run
// clause is left out when the run is zero, and the retry cadence is named
// only for SourceNotReady: the controller requeues at the package interval
// only for source-resolution failures; apply-phase failures retry on the
// package backoff and stalled ones on the stalled recheck interval
// (opm-controller internal/reconcile/modulepackage.go:515,528,542 against
// :632,665,741,762), so an interval named for those would state a false
// cadence.
func (v *ownerView) failedSummary() string {
	r := v.Reconcile
	var parts []string
	if n := failedRun(v.History); n > 0 {
		parts = append(parts, "Failed on the last "+attempts(n))
	} else if r.Reason != "" {
		parts = append(parts, r.Reason)
	}
	text := strings.Join(parts, "")
	if r.Retrying {
		retry := "retrying"
		if v.Package != nil && v.Package.Interval != "" && r.Reason == "SourceNotReady" {
			retry += " every " + humanInterval(v.Package.Interval)
		}
		if text == "" {
			return strings.ToUpper(retry[:1]) + retry[1:]
		}
		return text + ", " + retry
	}
	if text == "" {
		return noReasonRecorded
	}
	return text
}

// attempts says "attempt" or "N attempts" with the count.
func attempts(n int) string {
	if n == 1 {
		return "attempt"
	}
	return itoa(int64(n)) + " attempts"
}

func (h *Handler) appliedBlock(v *ownerView) stateBlock {
	r := v.Reconcile
	reconciling := reconcilingCondition(v.Conditions)
	word := appliedWord(r, reconciling != nil)
	b := stateBlock{
		ID: appliedCardID, Label: "Applied status", Eyebrow: "Applied · the controller",
		Hue:     appliedBlockHue(r.State, reconciling != nil),
		State:   word,
		Summary: v.appliedSummary(reconciling),
		Caption: "Warning events, last hour",
		None:    "No reconcile errors",
		Follow:  v.Topic + " events:" + v.Topic,
		History: historyOf(v.History),
	}
	if r.Since != nil {
		b.When, b.WhenText = r.Since, "since"
	}
	if m := reasonMeaning(v.Conditions, r.Reason); m != "" {
		b.Notes = append(b.Notes, sbNote{Text: m})
	}
	// The controller's own message: present in local mode, omitted in-cluster.
	// Its line breaks stay; blank lines around it do not.
	if msg := strings.TrimSpace(r.Message); msg != "" {
		b.Notes = append(b.Notes, sbNote{Text: msg, Class: "says"})
	}
	if reconciling != nil && word != stateReconciling {
		b.Notes = append(b.Notes, sbNote{Text: h.reconcilingText(reconciling), Class: "reconciling"})
	}
	if v.Events.Problem != nil {
		b.None = "Events could not be read"
		return b
	}
	b.Reasons = warningReasons(v)
	return b
}

// reconcilingText is the Reconciling mark: its reason and when it began.
func (h *Handler) reconcilingText(c *v1.Condition) string {
	text := stateReconciling
	if c.Reason != "" {
		text += ": " + c.Reason
	}
	if s := h.stamp(c.LastTransitionTime); s != nil {
		text += ", since " + s.Relative
	}
	return text
}

// warningReasons counts the Warning events about the owner itself by reason,
// count first. The merged feed holds events about its children too, and
// those are not the owner's: the counts keep the meaning they had before
// the merge (portal:D9:R1/R2).
func warningReasons(v *ownerView) []countLink {
	owner := v.ownerRef()
	counts := map[string]int{}
	var order []string
	for i := range v.Events.Items {
		e := &v.Events.Items[i]
		if e.Type != eventWarning || e.Regarding != owner {
			continue
		}
		if counts[e.Reason] == 0 {
			order = append(order, e.Reason)
		}
		counts[e.Reason] += int(e.Count)
	}
	out := make([]countLink, 0, len(order))
	for _, r := range order {
		out = append(out, countLink{Text: r, N: counts[r], Class: "warn",
			Href: v.Base + "?" + url.Values{"tab": {tabEvents}, "type": {eventWarning}, "reason": {r}}.Encode()})
	}
	return out
}

// historyOf is the attempt strip: a dot per recorded attempt, oldest first.
// There is no dot for an attempt the history does not record (portal:D9:R1).
func historyOf(history []v1.HistoryEntry) *historyStrip {
	s := &historyStrip{Rows: history}
	for i := len(history) - 1; i >= 0; i-- {
		hs := &history[i]
		outcome, mark := wordUnknown, "…"
		switch hs.Outcome {
		case outcomeOK:
			outcome, mark = outcomeOK, "✓"
		case outcomeFailed:
			outcome, mark = outcomeFailed, "✕"
		}
		title := "#" + itoa(hs.Sequence) + " " + hs.Action + ": " + strings.ToLower(outcome)
		if hs.FinishedAt != nil {
			title += ", " + hs.FinishedAt.UTC().Format("2006-01-02 15:04:05 UTC")
		}
		if hs.Message != "" {
			title += ": " + hs.Message
		}
		s.Dots = append(s.Dots, attemptDot{
			Class: "dot-" + strings.ToLower(outcome), Mark: mark,
			Tip: tipData{ID: "attempt-" + itoa(hs.Sequence), Trigger: mark, Label: "#" + itoa(hs.Sequence) + " " + strings.ToLower(outcome), Text: title},
		})
	}
	if n := len(history); n > 0 {
		s.Count = "last " + attempts(n)
		if n == 1 {
			s.Count = "1 attempt"
		}
	}
	return s
}

// --- Health ---

func healthBlockHue(state string) string {
	switch state {
	case healthHealthy:
		return "healthy"
	case healthDegraded:
		return hueDegraded
	case healthMissing:
		return "missing"
	case healthProgressing:
		return hueProgressing
	}
	return hueUnknown
}

func healthTotal(c v1.HealthCounts) int {
	return c.Healthy + c.Progressing + c.Degraded + c.Missing + c.Unknown
}

// healthSummary composes the Health card's summary from the counts (design.md
// decision 2). A total of zero counted nothing, so nothing is unhealthy.
func healthSummary(c v1.HealthCounts) string {
	total := healthTotal(c)
	of := func(n int) string {
		return itoa(int64(n)) + " of " + itoa(int64(total)) + " " + plural(total, "resource", "resources")
	}
	switch {
	case total == 0:
		return "Nothing applied yet, so nothing to check"
	case c.Degraded+c.Missing > 0:
		return of(c.Degraded+c.Missing) + " unhealthy"
	case c.Progressing > 0:
		return of(c.Progressing) + " rolling out"
	case c.Unknown > 0:
		return of(c.Unknown) + " in an unknown state"
	}
	return "All " + itoa(int64(total)) + " " + plural(total, "resource", "resources") + " healthy"
}

func (h *Handler) healthBlock(v *ownerView, reasons []countLink) stateBlock {
	hl := v.Health
	word := wordUnknown
	if _, ok := healthText[hl.State]; ok {
		word = healthText[hl.State]
	}
	b := stateBlock{
		ID: healthCardID, Label: "Health status", Eyebrow: "Health · everything it runs",
		Hue: healthBlockHue(hl.State), State: word, Summary: healthSummary(hl.Counts),
		Reasons: reasons, None: "No unhealthy resources",
		Follow: v.Topic,
	}
	switch {
	case healthTotal(hl.Counts) == 0:
		// Nothing was counted, so nothing is "checked live" or "not live".
	case hl.Live:
		b.WhenText = "checked live"
	case hl.EvaluatedAt != nil:
		b.When, b.WhenText = hl.EvaluatedAt, "not live, evaluated"
	default:
		b.WhenText = "not live"
	}
	if hl.Partial {
		b.Notes = append(b.Notes, sbNote{Text: "Partial: some objects could not be read and are left out.", Class: "partial"})
	}
	return b
}

// healthReasons counts objects and runtime children by health reason, each
// linking to the Resources tab filtered by it. A reason an object only
// mirrors from a child below it is counted once, at the child.
func healthReasons(v *ownerView) []countLink {
	counts := map[string]int{}
	add := func(h *v1.ObjectHealth) {
		if h != nil && h.Reason != "" && h.State != healthHealthy {
			counts[h.Reason]++
		}
	}
	walk := func(cs []componentView) {
		for i := range cs {
			for j := range cs[i].Objects {
				o := &cs[i].Objects[j]
				mirrored := false
				for k := range o.Children {
					add(&o.Children[k].Health)
					mirrored = mirrored || (o.Health != nil && o.Children[k].Health.Reason == o.Health.Reason)
				}
				// A workload degraded only because a child waits carries the
				// child's reason: the reason is counted once, at its source.
				if !mirrored {
					add(o.Health)
				}
			}
		}
	}
	walk(v.allComponents())
	names := make([]string, 0, len(counts))
	for r := range counts {
		names = append(names, r)
	}
	slices.Sort(names)
	out := make([]countLink, 0, len(names))
	for _, r := range names {
		out = append(out, countLink{Text: r, N: counts[r],
			Href: v.Base + "?" + url.Values{"tab": {tabResources}, "reason": {r}}.Encode()})
	}
	return out
}

// --- Provider ---

// providerFallback is the sentence for a standing the controller wrote no
// text for and whose reason has no known meaning (OP-A4), so a state word
// never stands over an empty summary. The key is the word.
var providerFallback = map[string]string{
	"Accepted":        "Accepted; not active yet, so it provides nothing yet.",
	"Pending":         "The controller has not decided on this claim yet.",
	"Refused":         "The controller refused this claim.",
	"Removal blocked": "Accepted and active; its removal waits until no module uses what it provides.",
	wordActive:        "Accepted and active.",
	wordUnknown:       "The portal does not know this claim's standing.",
}

// providerWord is the word and hue of a claim's standing, as the controller
// wrote it: a verdict the UI does not know reads Unknown.
func providerWord(c v1.ProviderClaim) (word, hue string) {
	switch {
	case c.Verdict == verdictAccepted && c.Active:
		return wordActive, "healthy"
	case c.Verdict == verdictAccepted:
		return verdictAccepted, hueProgressing
	case c.Verdict == verdictPending:
		return "Pending", hueProgressing
	case c.Verdict == verdictRefused:
		return "Refused", hueDegraded
	case c.Verdict == verdictRemovalBlocked:
		return textRemovalBlocked, hueUnknown
	}
	return wordUnknown, hueUnknown
}

// registrationMeaning is the meaning of the reason the claim or the
// registration carries, from the registration's own conditions.
func registrationMeaning(reg *v1.Registration, claim v1.ProviderClaim) string {
	if reg == nil {
		return ""
	}
	for _, reason := range []string{claim.Reason, reg.Reason, reg.ActiveReason} {
		if reason == "" {
			continue
		}
		for i := range reg.Conditions {
			if reg.Conditions[i].Reason == reason && reg.Conditions[i].Meaning != "" {
				return reg.Conditions[i].Meaning
			}
		}
	}
	return ""
}

// providerSummary is the summary of a claim: the controller's text when it
// wrote some, else the reason's meaning, else the fixed sentence for the
// standing. reg is nil when the Platform did not give the registration.
func providerSummary(c v1.ProviderClaim, reg *v1.Registration, word string) string {
	if word == wordActive && reg != nil && len(reg.Provides) > 0 {
		names := make([]string, 0, len(reg.Provides))
		for _, p := range reg.Provides {
			names = append(names, contractShort(p))
		}
		return "Provides " + strings.Join(names, ", ") + " for the platform"
	}
	if reg != nil {
		text := reg.Message
		if c.Verdict == verdictAccepted && !c.Active && reg.ActiveMessage != "" {
			text = reg.ActiveMessage
		}
		if text != "" {
			return text
		}
	}
	if m := registrationMeaning(reg, c); m != "" {
		return m
	}
	if word == wordActive && reg != nil {
		return "Accepted and active; it provides no contract."
	}
	return providerFallback[word]
}

// providerWhen is the time the standing began, from the registration's own
// conditions: active since the Active condition, else the Ready condition's
// time with the standing as words. nil when the Platform gave no
// registration.
func providerWhen(reg *v1.Registration, word string) (at *time.Time, text string) {
	if reg == nil {
		return nil, ""
	}
	want := "Ready"
	if word == wordActive {
		want = wordActive
	}
	for i := range reg.Conditions {
		if reg.Conditions[i].Type == want && reg.Conditions[i].LastTransitionTime != nil {
			return reg.Conditions[i].LastTransitionTime, providerWhenWords(word)
		}
	}
	return nil, ""
}

func providerWhenWords(word string) string {
	if word == wordActive {
		return "active since"
	}
	return strings.ToLower(word)
}

func (h *Handler) providerBlock(v *ownerView) stateBlock {
	claim := v.ProviderOf[0]
	b := stateBlock{ID: providerCardID, Label: "Provider status", Eyebrow: "Provider · its registration", Follow: v.Topic + " " + platformTopic}
	if claim.Access != v1.AccessOK {
		code := v1.CodeNotReadableByPortal
		if claim.Access == v1.AccessForbidden {
			code = v1.CodeForbidden
		}
		b.Problem = &v1.Problem{Code: code}
		return newStateBlock(b)
	}
	reg := v.plat.registration(claim.Registration)
	word, hue := providerWord(claim)
	b.Hue, b.State = hue, word
	b.Summary = providerSummary(claim, reg, word)
	b.When, b.WhenText = providerWhen(reg, word)
	if m := registrationMeaning(reg, claim); m != "" && m != b.Summary {
		b.Notes = append(b.Notes, sbNote{Text: m})
	}
	for _, other := range v.ProviderOf[1:] {
		w := claimBadge(other).Text
		b.Notes = append(b.Notes, sbNote{Text: w + " · " + other.Registration})
	}
	b.None = "No registration problems"
	if claim.Verdict != verdictAccepted || !claim.Active {
		if claim.Reason != "" {
			b.Reasons = []countLink{{Text: claim.Reason, N: 1, Href: v.Base + "?tab=" + tabProvider}}
		} else {
			b.None = noReasonRecorded
		}
	}
	b.Links = []link{{Text: "Open the Provider tab", Href: v.Base + "?tab=" + tabProvider}}
	return newStateBlock(b)
}

// --- Identity ---

// ownerIdentity is the identity card's derived facts.
type ownerIdentity struct {
	// Subtitle is the mono line under the name: the module path and the
	// version, or the package's source. Version is drawn apart, unbroken.
	Subtitle string
	Version  string
	// SourceNamespace is the namespace of a package's source, resolved to the
	// package's own when the source names none (internal/graph/instance.go).
	SourceNamespace string
	Owner           string
	Interval        string // "every 1 min", or "" when not set
	Prune           string // "on", "off" or "" when not set
	OwnerTip        tipData
	ApplierTip      tipData
}

const (
	ownerTipText   = "Who manages this object. controller: the OPM controller reconciles it. cli: the opm CLI applied it and the controller leaves it alone."
	applierTipText = "The ServiceAccount the controller acts as when it creates and updates this module's objects, so its RBAC limits what the module can change. If none is set, the controller uses its own ServiceAccount."
)

func identityOf(v *ownerView) ownerIdentity {
	id := ownerIdentity{
		Owner:      ownerController,
		OwnerTip:   tipData{ID: "tip-owner", Trigger: "Owner", Text: ownerTipText},
		ApplierTip: tipData{ID: "tip-applier", Trigger: "Applier", Text: applierTipText},
	}
	switch {
	case v.Instance != nil:
		m := v.Instance.Module
		id.Subtitle, id.Version = m.Path, m.Version
		switch v.Instance.Owner {
		case ownerCLI:
			id.Owner = "the opm CLI"
		case ownerController:
			id.Owner = "the " + ownerController
		default:
			// A spec.owner value the portal does not know is shown as
			// written and never named as the controller.
			id.Owner = v.Instance.Owner + " (unknown)"
		}
	case v.Package != nil:
		p := v.Package
		id.Owner = "the " + ownerController
		id.SourceNamespace = p.Source.Namespace
		if id.SourceNamespace == "" {
			id.SourceNamespace = p.Ref.Namespace
		}
		id.Subtitle = "from " + p.Source.Kind + " " + id.SourceNamespace + "/" + p.Source.Name
		if p.Interval != "" {
			id.Interval = "every " + humanInterval(p.Interval)
		}
		if p.Prune != nil {
			id.Prune = "off"
			if *p.Prune {
				id.Prune = "on"
			}
		}
	}
	return id
}
