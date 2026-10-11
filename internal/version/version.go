// Package version carries the portal's build-time version identity.
//
// The version is burned into source rather than injected at build time:
// release-please rewrites the annotated Version constant on every release PR
// (extra-files in release-please-config.json), so any build of a tagged commit
// (release binary, container image, go install) reports the tag's version
// with no ldflags or Dockerfile cooperation.
package version

import (
	"runtime/debug"
)

// Version is the portal's semantic version, without the leading "v".
// The trailing annotation is load-bearing: release-please's generic updater
// rewrites this line on every release PR. Do not detach the comment from the
// constant.
const Version = "0.1.0" // x-release-please-version

// Full returns "v" + Version, matching the release tags.
//
// When the binary was built from a VCS checkout that exposes build info, a
// "+g<short-revision>" build-metadata suffix is appended, with ".dirty" for a
// modified tree, as provenance for development builds. Consumers comparing
// versions must strip the "+" suffix (SemVer build metadata).
func Full() string {
	v := "v" + Version

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	return v + buildMetadata(info.Settings)
}

// buildMetadata returns the "+g<rev>[.dirty]" suffix for the given build
// settings, or "" when they carry no VCS revision.
func buildMetadata(settings []debug.BuildSetting) string {
	var revision string
	var dirty bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return ""
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	suffix := "+g" + revision
	if dirty {
		suffix += ".dirty"
	}
	return suffix
}
