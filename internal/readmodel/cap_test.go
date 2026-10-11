package readmodel

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCapText holds the cut to its definition: at most limit code points,
// the first limit-1 and an ellipsis for a longer string, never inside a code
// point.
func TestCapText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"empty", "", 5, ""},
		{"shorter than the limit", "abc", 5, "abc"},
		{"exactly the limit", "abcde", 5, "abcde"},
		{"one over the limit", "abcdef", 5, "abcd…"},
		{"multi-byte code points at the limit", "ååååå", 5, "ååååå"},
		{"multi-byte code point on the boundary", "ååååå" + "å", 5, "åååå…"},
		{"four-byte code point on the boundary", "ab😀cd😀e", 4, "ab😀…"},
		{"bytes over the limit, code points under", "ååå", 4, "ååå"},
		{"limit one", "abc", 1, "…"},
		{"no room", "abc", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CapText(tc.in, tc.limit)
			if got != tc.want {
				t.Fatalf("CapText(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result %q is not valid UTF-8", got)
			}
			if n := utf8.RuneCountInString(got); tc.limit >= 0 && n > tc.limit {
				t.Errorf("result has %d code points, limit %d", n, tc.limit)
			}
		})
	}

	long := strings.Repeat("é", 5000)
	got := CapText(long, 1024)
	if n := utf8.RuneCountInString(got); n != 1024 || !strings.HasSuffix(got, "…") {
		t.Errorf("5000 code points cut to %d code points, suffix ok %v", n, strings.HasSuffix(got, "…"))
	}
}
