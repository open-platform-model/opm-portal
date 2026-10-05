package health

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Tone is how a condition reads: what its status means for its type. A
// condition's status alone does not say it: Stalled=True and
// Reconciling=True are true and not fine, and ContractsFulfilled=False is
// information, not a failure (portal:D3:R8).
type Tone string

// Tones.
const (
	// ToneNormal: the condition reports what is expected.
	ToneNormal Tone = "normal"
	// ToneAbnormal: the condition reports a fault a person should look at.
	ToneAbnormal Tone = "abnormal"
	// ToneProgressing: work is under way.
	ToneProgressing Tone = "progressing"
	// ToneInformational: worth knowing, never a failure.
	ToneInformational Tone = "informational"
	// ToneUnknown: the status is Unknown, or the portal does not know the
	// condition type, so it does not guess its polarity.
	ToneUnknown Tone = "unknown"
)

// conditionTones gives each condition type the operator writes the tone of
// its True and False statuses.
var conditionTones = map[string][2]Tone{
	conditionReady:              {ToneNormal, ToneAbnormal},
	conditionActive:             {ToneNormal, ToneInformational},
	"ModuleResolved":            {ToneNormal, ToneAbnormal},
	conditionStalled:            {ToneAbnormal, ToneNormal},
	conditionReconciling:        {ToneProgressing, ToneNormal},
	conditionDrifted:            {ToneInformational, ToneNormal},
	conditionContractsFulfilled: {ToneNormal, ToneInformational},
}

// ConditionTone returns how the condition c reads.
func ConditionTone(c Condition) Tone {
	t, ok := conditionTones[c.Type]
	if !ok {
		return ToneUnknown
	}
	switch c.Status {
	case metav1.ConditionTrue:
		return t[0]
	case metav1.ConditionFalse:
		return t[1]
	case metav1.ConditionUnknown:
	}
	return ToneUnknown
}
