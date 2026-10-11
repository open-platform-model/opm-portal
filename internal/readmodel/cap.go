package readmodel

import "unicode/utf8"

// ellipsis ends a value CapText cut.
const ellipsis = "…"

// CapText returns s cut to at most limit Unicode code points. A string of at
// most limit code points comes back unchanged; a longer one comes back as
// its first limit-1 code points and an ellipsis, so the result never has
// more than limit code points and is always cut between code points. A cut
// can split a base character from a combining mark: the cap promises valid
// text, not whole grapheme clusters. A limit below 1 returns the empty
// string.
//
// The read model returns the cluster's text as it holds it. The code that
// converts text for serving cuts it, once, with this function, so every
// consumer of a served value holds the cut text.
func CapText(s string, limit int) string {
	if limit < 1 {
		return ""
	}
	if len(s) <= limit || utf8.RuneCountInString(s) <= limit {
		return s
	}
	end := 0
	for i := 0; i < limit-1; i++ {
		_, size := utf8.DecodeRuneInString(s[end:])
		end += size
	}
	return s[:end] + ellipsis
}
