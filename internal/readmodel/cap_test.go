package readmodel

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCapText(t *testing.T) {
	three := strings.Repeat("€", 600) // each code point takes three bytes
	cases := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"empty", "", 5, ""},
		{"shorter", "abc", 5, "abc"},
		{"exactly the limit", "abcde", 5, "abcde"},
		{"one code point over", "abcdef", 5, "abcd" + ellipsis},
		{"much longer", strings.Repeat("a", 2000), 512, strings.Repeat("a", 511) + ellipsis},
		{"multi-byte at the limit", strings.Repeat("€", 5), 5, strings.Repeat("€", 5)},
		{"multi-byte over the limit", strings.Repeat("€", 6), 5, strings.Repeat("€", 4) + ellipsis},
		{"multi-byte boundary of a long text", three, 512, three[:511*3] + ellipsis},
		{"bytes over the limit, code points not", "ééé", 3, "ééé"},
		{"limit of one", "ab", 1, ellipsis},
		{"limit of zero", "ab", 0, ""},
		{"markup stays text", "<script>alert(1)</script>", 10, "<script>a" + ellipsis},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CapText(tc.in, tc.limit)
			if got != tc.want {
				t.Fatalf("CapText(%.20q..., %d) = %.40q..., want %.40q...", tc.in, tc.limit, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result is not valid UTF-8: %q", got)
			}
			if tc.limit > 0 && utf8.RuneCountInString(got) > tc.limit {
				t.Errorf("result holds %d code points, limit %d", utf8.RuneCountInString(got), tc.limit)
			}
		})
	}
}
