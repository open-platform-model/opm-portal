package version

import (
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
)

// semverRe matches the constant's expected shape: MAJOR.MINOR.PATCH with an
// optional pre-release, no leading "v", no build metadata.
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

func TestVersionIsSemver(t *testing.T) {
	if !semverRe.MatchString(Version) {
		t.Fatalf("Version %q is not a bare semver (expected e.g. 0.1.0)", Version)
	}
}

// TestReleasePleaseAnnotationOnConstLine guards the release automation
// contract: the generic updater rewrites only lines annotated with
// x-release-please-version, so the annotation must sit on the line of the
// Version constant. A reformat that detaches it would freeze the version.
func TestReleasePleaseAnnotationOnConstLine(t *testing.T) {
	src, err := os.ReadFile("version.go")
	if err != nil {
		t.Fatalf("reading version.go: %v", err)
	}
	for line := range strings.SplitSeq(string(src), "\n") {
		if !strings.Contains(line, "x-release-please-version") {
			continue
		}
		if strings.Contains(line, "const Version = ") && strings.Contains(line, `"`+Version+`"`) {
			return
		}
		t.Fatalf("x-release-please-version annotation on a line without the Version constant: %q", line)
	}
	t.Fatal("no line in version.go carries the x-release-please-version annotation")
}

func TestFullPrefix(t *testing.T) {
	full := Full()
	want := "v" + Version
	if full != want && !strings.HasPrefix(full, want+"+g") {
		t.Fatalf("Full() = %q; want %q or %q with a +g<rev>[.dirty] suffix", full, want, want)
	}
}

func TestBuildMetadata(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{name: "no vcs info", settings: nil, want: ""},
		{
			name:     "clean tree",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.modified", Value: "false"}},
			want:     "+g0123456",
		},
		{
			name:     "modified tree",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.modified", Value: "true"}},
			want:     "+g0123456.dirty",
		},
		{
			name:     "short revision kept whole",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}},
			want:     "+gabc",
		},
		{
			name:     "dirty without revision",
			settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildMetadata(tt.settings); got != tt.want {
				t.Fatalf("buildMetadata() = %q; want %q", got, tt.want)
			}
		})
	}
}
