package ui

import (
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// link is one plain link: the words and where they go.
type link struct {
	Text string
	Href string
}

// sbNote is one secondary line of a state block.
type sbNote struct {
	Text  string
	Class string // "" (muted ink), "locked" (the lock style), "partial", "reconciling"
	Mono  bool   // a reason or other machine word
}

// stateBlock is one summary block: the state of one axis or one standing, as
// a recorded source reports it (portal:D19:R3). Pages build it with
// newStateBlock; the "state-block" partial draws it. The block never invents
// a time: When is a recorded time or nil, and WhenText with a nil When is
// words only.
type stateBlock struct {
	ID       string     // the section's id, for links and live refresh
	Label    string     // the accessible name, e.g. "Applied status"
	Eyebrow  string     // e.g. "Applied · the controller"
	When     *time.Time // a recorded time, or nil: never invented; drawn by the "time" partial
	WhenText string     // the words before the time, e.g. "since"; with When nil, shown alone as words (e.g. "checked live"), never as a time
	Hue      string     // applied, healthy, progressing, degraded, unknown, neutral, missing, locked
	State    string     // the word, e.g. "Applied", "Degraded"
	Summary  string
	Notes    []sbNote    // secondary lines under the summary, in order
	Caption  string      // what the reasons count, shown above them; "" for none
	Reasons  []countLink // Class gives a reason its own hue; N zero shows the reason without a count
	None     string      // shown when Reasons is empty, e.g. "No unhealthy resources"
	Links    []link      // footer links, e.g. "Open the Provider tab"
	Follow   string      // data-follow topics
	Problem  *v1.Problem // set: the block renders the problem region in place of the state
}

// stateBlockHues are the hues the state block draws: the seven tones and the
// locked standing.
var stateBlockHues = map[string]bool{
	"applied": true, "healthy": true, "progressing": true, "degraded": true,
	"unknown": true, "neutral": true, "missing": true, "locked": true,
}

// newStateBlock returns b with its hue settled. A hue the block does not know
// reads unknown, never an error hue (portal:D2:R3). A block with a problem
// takes locked for a forbidden read and neutral for any other, and shows no
// state word, so a source the caller may not read never reads as a state.
func newStateBlock(b stateBlock) stateBlock {
	switch {
	case b.Problem != nil && b.Problem.Code == v1.CodeForbidden:
		b.Hue = "locked"
	case b.Problem != nil:
		b.Hue = "neutral"
	case !stateBlockHues[b.Hue]:
		b.Hue = "unknown"
	}
	return b
}

// tipData is what the "tip" partial draws: a trigger with its text box
// (portal:D19, WCAG 2.2 1.4.13). The box holds text only.
type tipData struct {
	ID      string // the box's id, which the trigger's aria-describedby names
	Trigger string // the words the trigger draws
	Text    string // the tip
	End     bool   // align the box to the trigger's end, near the right edge
}
