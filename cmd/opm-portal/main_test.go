package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/version"
)

func TestRun(t *testing.T) {
	versionLine := "opm-portal " + version.Full() + "\n"
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no arguments", args: nil, wantCode: exitOK, wantStdout: versionLine},
		{name: "version subcommand", args: []string{"version"}, wantCode: exitOK, wantStdout: versionLine},
		{name: "version flag", args: []string{"--version"}, wantCode: exitOK, wantStdout: versionLine},
		{name: "unknown argument", args: []string{"start"}, wantCode: exitUsage, wantStderr: "usage: opm-portal"},
		{name: "extra argument", args: []string{"version", "extra"}, wantCode: exitUsage, wantStderr: "usage: opm-portal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d; want %d", code, tt.wantCode)
			}
			if stdout.String() != tt.wantStdout {
				t.Fatalf("stdout = %q; want %q", stdout.String(), tt.wantStdout)
			}
			switch {
			case tt.wantStderr == "" && stderr.Len() != 0:
				t.Fatalf("stderr = %q; want it empty", stderr.String())
			case !strings.HasPrefix(stderr.String(), tt.wantStderr):
				t.Fatalf("stderr = %q; want prefix %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestVersionLineShape(t *testing.T) {
	var stdout bytes.Buffer
	run(t.Context(), nil, &stdout, &bytes.Buffer{})
	if !strings.HasPrefix(stdout.String(), "opm-portal v") {
		t.Fatalf("stdout = %q; want a line starting with %q", stdout.String(), "opm-portal v")
	}
}
