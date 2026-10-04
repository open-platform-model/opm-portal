package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestServeRefusesBeforeReadingTheKubeconfig(t *testing.T) {
	// The kubeconfig does not exist: reading it would fail with exit 1 and
	// a kubeconfig error, so exit 2 with the address error proves the
	// address was refused first.
	missing := filepath.Join(t.TempDir(), "no-such-kubeconfig")
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"all interfaces", []string{"--addr", "0.0.0.0:8080"}, "not a loopback address"},
		{"empty host", []string{"--addr", ":8080"}, "not a loopback address"},
		{"LAN address", []string{"--addr", "192.168.1.10:8080"}, "not a loopback address"},
		{"IPv6 any", []string{"--addr", "[::]:8080"}, "not a loopback address"},
		{"host name", []string{"--addr", "example.com:8080"}, "not a loopback address"},
		{"no port", []string{"--addr", "127.0.0.1"}, "not HOST:PORT"},
		{"bad port", []string{"--addr", "127.0.0.1:http"}, "no valid port"},
		{"unknown flag", []string{"--bogus"}, "flag provided but not defined"},
		{"positional argument", []string{"extra"}, "usage: opm-portal serve"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"serve", "--kubeconfig", missing}, tt.args...)
			if code := run(t.Context(), args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit code = %d; want %d (stderr %q)", code, exitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q; want it to contain %q", stderr.String(), tt.wantStderr)
			}
			if strings.Contains(stderr.String(), "kubeconfig:") || stdout.Len() != 0 {
				t.Fatalf("the kubeconfig was read or something was printed: stdout %q stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestServeFailsOnAMissingKubeconfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "no-such-kubeconfig")
	if code := run(t.Context(), []string{"serve", "--kubeconfig", missing}, &stdout, &stderr); code != exitFailure {
		t.Fatalf("exit code = %d; want %d (stderr %q)", code, exitFailure, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q; want nothing printed before startup succeeds", stdout.String())
	}
}

func TestServeHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"serve", "-h"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d; want 0", code)
	}
	for _, flag := range []string{"-kubeconfig", "-context", "-addr", "-namespaces", "-open"} {
		if !strings.Contains(stderr.String(), flag) {
			t.Errorf("help lacks %s: %q", flag, stderr.String())
		}
	}
}

func TestParseServe(t *testing.T) {
	o, err := parseServe([]string{"--context", "kind-x", "--namespaces", " team-a, ,team-b", "--open"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if o.addr != defaultAddr || o.context != "kind-x" || !o.open || !slices.Equal(o.namespaces, []string{"team-a", "team-b"}) {
		t.Fatalf("options = %+v", o)
	}
}

func TestLoopbackAddr(t *testing.T) {
	for in, want := range map[string]string{
		"127.0.0.1:0":           "127.0.0.1:0",
		"127.0.0.5:8080":        "127.0.0.5:8080",
		"localhost:8080":        "127.0.0.1:8080",
		"LOCALHOST:0":           "127.0.0.1:0",
		"[::1]:8080":            "[::1]:8080",
		"[::ffff:127.0.0.1]:80": "127.0.0.1:80",
	} {
		got, err := loopbackAddr(in)
		if err != nil || got != want {
			t.Errorf("loopbackAddr(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"[fe80::1%lo]:80", "[::]:0", "10.0.0.1:0", "localhost.example:0", "127.0.0.1:65536"} {
		if got, err := loopbackAddr(in); err == nil {
			t.Errorf("loopbackAddr(%q) = %q; want an error", in, got)
		}
	}
}

func TestBoundLoopback(t *testing.T) {
	if _, err := boundLoopback(&net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 80}); err == nil {
		t.Error("a non-loopback bound address was accepted")
	}
	if _, err := boundLoopback(&net.UnixAddr{Name: "/tmp/x", Net: "unix"}); err == nil {
		t.Error("a non-TCP bound address was accepted")
	}
	got, err := boundLoopback(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8123})
	if err != nil || got.String() != "127.0.0.1:8123" {
		t.Errorf("boundLoopback = %v, %v", got, err)
	}
}

// reviewing returns a fake clientset whose SelfSubjectReview answers with
// user, or fails with err.
func reviewing(user authenticationv1.UserInfo, err error) *fake.Clientset {
	cs := fake.NewClientset()
	cs.PrependReactor("create", "selfsubjectreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		if err != nil {
			return true, nil, err
		}
		return true, &authenticationv1.SelfSubjectReview{Status: authenticationv1.SelfSubjectReviewStatus{UserInfo: user}}, nil
	})
	return cs
}

func TestSelfIdentity(t *testing.T) {
	admin := authenticationv1.UserInfo{
		Username: "kubernetes-admin",
		UID:      "u-1",
		Groups:   []string{"kubeadm:cluster-admins", "system:authenticated"},
		Extra:    map[string]authenticationv1.ExtraValue{"scopes": {"a", "b"}},
	}
	cs := reviewing(admin, nil)
	id, err := selfIdentity(t.Context(), cs.AuthenticationV1().SelfSubjectReviews())
	if err != nil {
		t.Fatal(err)
	}
	if id.Username != admin.Username || id.UID != "u-1" || !slices.Equal(id.Groups, admin.Groups) || !slices.Equal(id.Extra["scopes"], []string{"a", "b"}) {
		t.Fatalf("identity = %+v", id)
	}
	if n := len(cs.Actions()); n != 1 {
		t.Fatalf("cluster calls = %d; want one SelfSubjectReview", n)
	}

	for name, user := range map[string]authenticationv1.UserInfo{
		"empty":     {Groups: []string{"system:authenticated"}},
		"blank":     {Username: "  "},
		"anonymous": {Username: "system:anonymous", Groups: []string{"system:unauthenticated"}},
	} {
		if id, err := selfIdentity(t.Context(), reviewing(user, nil).AuthenticationV1().SelfSubjectReviews()); err == nil {
			t.Errorf("%s: identity %+v accepted; want a refusal", name, id)
		}
	}
	if _, err := selfIdentity(t.Context(), reviewing(admin, errors.New("unauthorized")).AuthenticationV1().SelfSubjectReviews()); err == nil {
		t.Error("a failed review was accepted")
	}
}

func TestOpenLaunchKeepsTheTokenOffTheCommandLine(t *testing.T) {
	const url = "http://127.0.0.1:8123/launch?token=SECRET&x=<y>"
	var opened string
	cleanup, err := openLaunch(t.Context(), url, func(_ context.Context, path string) error {
		opened = path
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(opened, "SECRET") {
		t.Fatalf("the browser was given the token: %q", opened)
	}
	fi, err := os.Stat(opened)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("launch page mode = %v; want 0600", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(opened))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Errorf("launch page directory mode = %v; want 0700", di.Mode().Perm())
	}
	page, err := os.ReadFile(opened)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `url=http://127.0.0.1:8123/launch?token=SECRET&amp;x=&lt;y&gt;`) {
		t.Errorf("launch page = %s", page)
	}
	cleanup()
	if _, err := os.Stat(filepath.Dir(opened)); !os.IsNotExist(err) {
		t.Errorf("cleanup left the launch page directory: %v", err)
	}

	// A browser that cannot start still leaves a cleanup that removes the
	// page.
	cleanup, err = openLaunch(t.Context(), url, func(_ context.Context, path string) error {
		opened = path
		return errors.New("no browser")
	})
	if err == nil {
		t.Fatal("a failed browser start returned no error")
	}
	cleanup()
	if _, err := os.Stat(opened); !os.IsNotExist(err) {
		t.Errorf("cleanup after a failure left the page: %v", err)
	}
}
