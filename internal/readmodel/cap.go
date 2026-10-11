package readmodel

import "unicode/utf8"

// ellipsis ends a text CapText cut.
const ellipsis = "…"

// CapText bounds a text the cluster wrote before the portal serves it: a
// string of at most limit Unicode code points comes back unchanged, a longer
// one as its first limit-1 code points and an ellipsis, so the result never
// holds more than limit. It always cuts between code points, so valid text
// stays valid; it does not keep grapheme clusters whole, so a combining mark
// may lose its base character at the cut. A limit below 1 returns "".
//
// The caller applies it where the text is converted for the wire, after any
// fold that compares whole texts, so the cap never changes what folds
// together (portal:D9:R3).
func CapText(s string, limit int) string {
	if limit < 1 {
		return ""
	}
	// A byte count at most limit cannot hold more than limit code points.
	if len(s) <= limit {
		return s
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	cut, seen := 0, 0
	for i := range s {
		if seen == limit-1 {
			cut = i
			break
		}
		seen++
	}
	return s[:cut] + ellipsis
}
