package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api"
	"github.com/open-platform-model/opm-portal/internal/auth"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

func TestConfigSource(t *testing.T) {
	complete := func(current string) clientcmdapi.Config {
		return clientcmdapi.Config{
			CurrentContext: current,
			Clusters: map[string]*clientcmdapi.Cluster{
				"dev-cluster":  {Server: "https://127.0.0.1:6443"},
				"prod-cluster": {Server: "https://prod.example:6443"},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{"u": {Token: "secret-token"}},
			Contexts: map[string]*clientcmdapi.Context{
				"kind-dev": {Cluster: "dev-cluster", AuthInfo: "u"},
				"prod":     {Cluster: "prod-cluster", AuthInfo: "u"},
			},
		}
	}
	inPod := api.Connection{Source: v1.SourceInCluster}
	dev := api.Connection{Source: v1.SourceKubeconfig, Context: "kind-dev", ClusterEntry: "dev-cluster"}
	prod := api.Connection{Source: v1.SourceKubeconfig, Context: "prod", ClusterEntry: "prod-cluster"}
	tests := []struct {
		name      string
		raw       clientcmdapi.Config
		asked     string
		inCluster bool
		want      api.Connection
		wantLog   string
	}{
		{"no kubeconfig in a Pod", clientcmdapi.Config{}, "", true, inPod, "source=in-cluster"},
		{"contexts but no current context, in a Pod", complete(""), "", true, inPod, "source=in-cluster"},
		{"the current context, in a Pod", complete("kind-dev"), "", true, dev, "context=kind-dev"},
		{"the current context", complete("kind-dev"), "", false, dev, "context=kind-dev"},
		{"the context asked for", complete("kind-dev"), "prod", false, prod, "context=prod"},
		{"the context asked for, in a Pod", complete(""), "prod", true, prod, "context=prod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configSource(tt.raw, tt.asked, tt.inCluster)
			if got != tt.want {
				t.Fatalf("configSource() = %+v; want %+v", got, tt.want)
			}
			if attr := sourceAttr(got); attr.String() != tt.wantLog {
				t.Errorf("log attribute = %s; want %s", attr, tt.wantLog)
			}
			// The user entry's name, the server URL and the token are
			// nowhere in it.
			named := fmt.Sprintf("%+v", got)
			if strings.Contains(named, "https://") || strings.Contains(named, "secret-token") || got.Context == "u" || got.ClusterEntry == "u" {
				t.Fatalf("the connection names a server, user entry or credential: %s", named)
			}
		})
	}
}

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

// TestServeFailsOnAMissingKubeconfig also shows that port 0 still binds a
// free port: serve gets past the listen to the kubeconfig.
func TestServeFailsOnAMissingKubeconfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "no-such-kubeconfig")
	if code := run(t.Context(), []string{"serve", "--kubeconfig", missing, "--addr", "127.0.0.1:0"}, &stdout, &stderr); code != exitFailure {
		t.Fatalf("exit code = %d; want %d (stderr %q)", code, exitFailure, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q; want nothing printed before startup succeeds", stdout.String())
	}
	if !strings.Contains(stderr.String(), "loading the kubeconfig") {
		t.Fatalf("stderr = %q; want the kubeconfig error", stderr.String())
	}
}

// TestServeRefusesATakenPort: an address another process holds stops serve
// with exit 1 and a message naming it and --addr, before the kubeconfig is
// read, so no request reaches a cluster.
func TestServeRefusesATakenPort(t *testing.T) {
	var lc net.ListenConfig
	// The default port: held here, or already held by another process,
	// which takes it just the same.
	if held, err := lc.Listen(t.Context(), "tcp", defaultAddr); err == nil {
		t.Cleanup(func() { _ = held.Close() })
	} else if !errors.Is(err, syscall.EADDRINUSE) {
		t.Skipf("cannot hold %s: %v", defaultAddr, err)
	}
	other, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	missing := filepath.Join(t.TempDir(), "no-such-kubeconfig")
	for _, tt := range []struct {
		name string
		args []string
		addr string
	}{
		{"the default address", nil, defaultAddr},
		{"an address passed with --addr", []string{"--addr", other.Addr().String()}, other.Addr().String()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"serve", "--kubeconfig", missing}, tt.args...)
			if code := run(t.Context(), args, &stdout, &stderr); code != exitFailure {
				t.Fatalf("exit code = %d; want %d (stderr %q)", code, exitFailure, stderr.String())
			}
			if want := tt.addr + " is in use; pass --addr 127.0.0.1:<port> to use another port"; !strings.Contains(stderr.String(), want) {
				t.Fatalf("stderr = %q; want it to contain %q", stderr.String(), want)
			}
			if strings.Contains(stderr.String(), "kubeconfig") || stdout.Len() != 0 {
				t.Fatalf("the kubeconfig was read or a link was printed: stdout %q stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestListenOnPortZeroPicksAFreePort(t *testing.T) {
	ln, bound, err := listen(t.Context(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if bound.Port() == 0 || !bound.Addr().IsLoopback() {
		t.Fatalf("bound = %s; want a loopback address and a chosen port", bound)
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
	if defaultAddr != "127.0.0.1:7878" {
		t.Fatalf("defaultAddr = %q; want the fixed 127.0.0.1:7878", defaultAddr)
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

// The --open page leaves the disk as soon as a browser spends its token,
// while the portal keeps serving.
func TestTheLaunchPageGoesOnceTheTokenIsSpent(t *testing.T) {
	gate, err := auth.NewLocal(auth.LocalConfig{Identity: authz.Identity{Username: "alice"}, Port: 8123})
	if err != nil {
		t.Fatal(err)
	}
	var opened string
	cleanup, err := openLaunch(t.Context(), gate.LaunchURL(), func(_ context.Context, path string) error {
		opened = path
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	removed := make(chan struct{})
	go func() {
		removeWhenLaunched(t.Context(), gate.Launched(), cleanup)
		close(removed)
	}()
	if _, err := os.Stat(opened); err != nil {
		t.Fatalf("the launch page is gone before the launch: %v", err)
	}

	h := gate.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, gate.LaunchURL(), http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("launch = %d; want 200", rec.Code)
	}
	select {
	case <-removed:
	case <-time.After(5 * time.Second):
		t.Fatal("the launch page was not removed after the launch")
	}
	if _, err := os.Stat(filepath.Dir(opened)); !os.IsNotExist(err) {
		t.Errorf("the launch page directory is still there: %v", err)
	}
	// Shutdown's cleanup runs again without harm.
	cleanup()
}

func TestLogDeniedNamesTheFlagWhereItHelps(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	logDenied(log, []readmodel.DeniedScope{
		{Resource: "moduleinstances", Namespaced: true},
		{Resource: "modulepackages", Namespaced: true, Namespace: "team-b"},
		{Resource: "transformerregistrations"},
	})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("logged %d lines, want 3:\n%s", len(lines), buf.String())
	}
	for i, want := range []struct {
		has   []string
		names bool // names --namespaces as the way out
	}{
		{[]string{"level=WARN", "cluster-wide", "resource=moduleinstances"}, true},
		{[]string{"level=WARN", "resource=modulepackages", "namespace=team-b"}, false},
		{[]string{"level=WARN", "cluster-scoped", "resource=transformerregistrations"}, false},
	} {
		for _, s := range want.has {
			if !strings.Contains(lines[i], s) {
				t.Errorf("line %d = %s; want %q", i, lines[i], s)
			}
		}
		if got := strings.Contains(lines[i], "pass --namespaces"); got != want.names {
			t.Errorf("line %d names the flag = %v, want %v: %s", i, got, want.names, lines[i])
		}
	}
	buf.Reset()
	logDenied(log, nil)
	if buf.Len() != 0 {
		t.Errorf("logged with nothing denied: %s", buf.String())
	}
}
