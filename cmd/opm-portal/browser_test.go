//go:build browser

package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/auth"
	"github.com/open-platform-model/opm-portal/internal/authz"
)

// playwrightImage pins the browsers; the Python package installed in it
// must match its version.
const (
	playwrightImage   = "mcr.microsoft.com/playwright/python:v1.63.0-noble"
	playwrightPackage = "playwright==1.63.0"
)

// TestBrowserLaunch opens the launch page openLaunch writes, as a file://
// page the way serve --open does, in real browsers run from the Playwright
// image (task test:browser). A browser withholds a SameSite=Strict cookie
// from a navigation a cross-site document started, which a Go client never
// does, so only a browser proves the launch lands with the session.
func TestBrowserLaunch(t *testing.T) {
	engine := os.Getenv("OPM_PORTAL_CONTAINER_ENGINE")
	if engine == "" {
		engine = "podman"
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "test", "browser", "launch.py"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, t *testing.T, browser, start, landing string, mounts ...string) error {
		args := make([]string, 0, 13+2*len(mounts))
		args = append(args, "run", "--rm", "-i", "--network", "host")
		for _, dir := range mounts {
			args = append(args, "-v", dir+":"+dir+":ro,Z")
		}
		args = append(args, playwrightImage, "bash", "-c",
			`pip install -q --root-user-action=ignore "$0" >/dev/null && python3 - "$@"`,
			playwrightPackage, browser, start, landing)
		cmd := exec.CommandContext(ctx, engine, args...)
		cmd.Stdin = bytes.NewReader(script)
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		return err
	}
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser+"/open", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			door := serveFrontDoor(t)
			cleanup, err := openLaunch(ctx, door.launch, func(ctx context.Context, path string) error {
				return run(ctx, t, browser, path, door.url, filepath.Dir(path))
			})
			defer cleanup()
			if err != nil {
				t.Fatalf("the launch page did not land on the landing page with the session: %v", err)
			}
		})
		t.Run(browser+"/link", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			door := serveFrontDoor(t)
			if err := run(ctx, t, browser, door.launch, door.url); err != nil {
				t.Fatalf("the launch link did not land on the landing page with the session: %v", err)
			}
		})
	}
}

type frontDoor struct {
	launch string // the one-time launch URL
	url    string // the landing page's URL
}

// serveFrontDoor serves local mode's front door on a free loopback port in
// front of a page that says whether the request carried the session.
func serveFrontDoor(t *testing.T) frontDoor {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	gate, err := auth.NewLocal(auth.LocalConfig{
		Identity: authz.Identity{Username: "browser-test"},
		Port:     port,
		Landing:  landing,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: gate.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if _, _, err := gate.Authenticate(r); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, "no session\n")
				return
			}
			_, _ = io.WriteString(w, "signed in\n")
		})),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return frontDoor{launch: gate.LaunchURL(), url: "http://" + ln.Addr().String() + landing}
}
