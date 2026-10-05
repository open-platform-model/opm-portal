//go:build e2e

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// fixtureCluster returns the fixture cluster's kubeconfig and context that
// test/e2e/local.sh and test/e2e/m1.sh name, and skips the test without them.
func fixtureCluster(t *testing.T) (kubeconfig, kubeContext string) {
	t.Helper()
	kubeconfig, kubeContext = os.Getenv("OPM_PORTAL_E2E_KUBECONFIG"), os.Getenv("OPM_PORTAL_E2E_CONTEXT")
	if kubeconfig == "" || kubeContext == "" {
		t.Skip("OPM_PORTAL_E2E_KUBECONFIG and OPM_PORTAL_E2E_CONTEXT are not set: run task e2e:local or task e2e:m1")
	}
	return kubeconfig, kubeContext
}

// buildPortal builds the binary into a temporary directory.
func buildPortal(ctx context.Context, t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "opm-portal")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// TestLocalMode runs the built binary against the kind fixture cluster
// (task e2e:up) through test/e2e/local.sh, which names its kubeconfig and
// context in OPM_PORTAL_E2E_KUBECONFIG and OPM_PORTAL_E2E_CONTEXT.
func TestLocalMode(t *testing.T) {
	kubeconfig, kubeContext := fixtureCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	bin := buildPortal(ctx, t)
	p := startPortal(ctx, t, bin, "serve", "--kubeconfig", kubeconfig, "--context", kubeContext)
	base := p.launch.Scheme + "://" + p.launch.Host
	instances := base + "/api/v1alpha1/clusters/default/instances"

	// Without the session every read is refused before anything is read.
	plain := &http.Client{Timeout: 30 * time.Second}
	if res := get(ctx, t, plain, instances, nil); res.status != http.StatusUnauthorized || !strings.Contains(res.body, `"unauthenticated"`) {
		t.Fatalf("no session: %d %s; want 401 unauthenticated", res.status, res.body)
	}
	browser, cookie := launch(ctx, t, p.launch)
	// The token is spent.
	if res := get(ctx, t, plain, p.launch.String(), nil); res.status != http.StatusForbidden {
		t.Fatalf("second launch: %d; want 403", res.status)
	}
	readAsTheUser(ctx, t, browser, instances)
	refuseForeignRequests(ctx, t, browser, instances, p.launch.Port())
	// A podinfo container's log streams on the read API's stream.
	followLog(ctx, t, browser, base, logTopic(ctx, t, kubeconfig, kubeContext))

	// Interrupt with a stream open: the stream ends, the process stops
	// cleanly, and its output never held the token or the cookie.
	ended := holdStream(ctx, t, browser, base, "platform")
	p.stop(t)
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the open stream did not end at shutdown")
	}
	stderr := p.stderr.String()
	if strings.Contains(stderr, p.launch.Query().Get("token")) || strings.Contains(stderr, cookie) {
		t.Fatalf("stderr carries the token or the cookie:\n%s", stderr)
	}
	if !strings.Contains(stderr, "reading as the kubeconfig's user") {
		t.Fatalf("stderr does not log the identity:\n%s", stderr)
	}
}

// TestLocalModeNamespaces runs the binary as a ServiceAccount that may read
// the OPM kinds only in default, with --namespaces default: the namespace
// is served and the cluster-wide list is forbidden, not failed.
func TestLocalModeNamespaces(t *testing.T) {
	kubeconfig, kubeContext := fixtureCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	bin := buildPortal(ctx, t)
	scoped := namespaceReader(ctx, t, kubeconfig, kubeContext)
	p := startPortal(ctx, t, bin, "serve", "--kubeconfig", scoped, "--namespaces", "default")
	instances := p.launch.Scheme + "://" + p.launch.Host + "/api/v1alpha1/clusters/default/instances"
	browser, _ := launch(ctx, t, p.launch)
	res := get(ctx, t, browser, instances+"?namespace=default", nil)
	if res.status != http.StatusOK || !strings.Contains(res.body, `"access":"ok"`) || !strings.Contains(res.body, `"name":"podinfo"`) {
		t.Fatalf("namespace list: %d %.300s; want 200, access ok, podinfo", res.status, res.body)
	}
	res = get(ctx, t, browser, instances, nil)
	if res.status != http.StatusOK || !strings.Contains(res.body, `"access":"forbidden"`) {
		t.Fatalf("cluster-wide list: %d %.300s; want 200 with access forbidden", res.status, res.body)
	}
	p.stop(t)
	stderr := p.stderr.String()
	if !strings.Contains(stderr, "user=system:serviceaccount:default:"+scopedReader) {
		t.Fatalf("stderr does not name the ServiceAccount:\n%s", stderr)
	}
	if !strings.Contains(stderr, "cluster-scoped kind, so it reads as forbidden\" resource=platforms") || strings.Contains(stderr, "pass --namespaces") {
		t.Fatalf("stderr does not warn of the denied cluster-scoped kinds alone:\n%s", stderr)
	}

	// Without --namespaces the user is told the flag is the way in.
	p = startPortal(ctx, t, bin, "serve", "--kubeconfig", scoped)
	p.stop(t)
	if stderr := p.stderr.String(); !strings.Contains(stderr, "pass --namespaces with the namespaces you may read\" resource=moduleinstances") {
		t.Fatalf("stderr does not point a namespace-scoped user at --namespaces:\n%s", stderr)
	}
}

const scopedReader = "opm-portal-e2e-reader"

// namespaceReader creates a ServiceAccount allowed to get, list and watch
// ModuleInstances and ModulePackages in default only, and returns a
// kubeconfig holding a short-lived token for it. Everything it creates is
// deleted when the test ends.
func namespaceReader(ctx context.Context, t *testing.T, kubeconfig, kubeContext string) string {
	t.Helper()
	cfg := restConfig(t, kubeconfig, kubeContext)
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	const ns = "default"
	meta := metav1.ObjectMeta{Name: scopedReader, Namespace: ns}
	sa, err := cs.CoreV1().ServiceAccounts(ns).Create(ctx, &corev1.ServiceAccount{ObjectMeta: meta}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating the ServiceAccount: %v", err)
	}
	t.Cleanup(func() {
		bg := context.WithoutCancel(ctx)
		_ = cs.RbacV1().RoleBindings(ns).Delete(bg, scopedReader, metav1.DeleteOptions{})
		_ = cs.RbacV1().Roles(ns).Delete(bg, scopedReader, metav1.DeleteOptions{})
		_ = cs.CoreV1().ServiceAccounts(ns).Delete(bg, scopedReader, metav1.DeleteOptions{})
	})
	if _, err := cs.RbacV1().Roles(ns).Create(ctx, &rbacv1.Role{ObjectMeta: meta, Rules: []rbacv1.PolicyRule{{
		APIGroups: []string{"opmodel.dev"}, Resources: []string{"moduleinstances", "modulepackages"}, Verbs: []string{"get", "list", "watch"},
	}}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("creating the Role: %v", err)
	}
	if _, err := cs.RbacV1().RoleBindings(ns).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: meta,
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: scopedReader},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa.Name, Namespace: ns}},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("creating the RoleBinding: %v", err)
	}
	expiry := int64(3600)
	tok, err := cs.CoreV1().ServiceAccounts(ns).CreateToken(ctx, sa.Name, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expiry},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("requesting a token: %v", err)
	}
	out := clientcmdapi.NewConfig()
	out.Clusters["fixture"] = &clientcmdapi.Cluster{Server: cfg.Host, CertificateAuthorityData: cfg.CAData}
	out.AuthInfos[scopedReader] = &clientcmdapi.AuthInfo{Token: tok.Status.Token}
	out.Contexts["scoped"] = &clientcmdapi.Context{Cluster: "fixture", AuthInfo: scopedReader, Namespace: ns}
	out.CurrentContext = "scoped"
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*out, path); err != nil {
		t.Fatal(err)
	}
	return path
}

// restConfig loads the fixture cluster's client configuration, from the
// kubeconfig file alone.
func restConfig(t *testing.T, kubeconfig, kubeContext string) *rest.Config {
	t.Helper()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext}).ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// holdStream opens a stream on topic and returns a channel closed when the
// stream ends.
func holdStream(ctx context.Context, t *testing.T, browser *http.Client, base, topic string) <-chan struct{} {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1alpha1/clusters/default/stream?topics="+topic, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&http.Client{Jar: browser.Jar}).Do(req) //nolint:bodyclose // the reader goroutine closes it
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		t.Fatalf("stream %s: %d", topic, res.StatusCode)
	}
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(io.Discard, res.Body)
	}()
	return ended
}

// launch opens the launch link like a browser and returns a client holding
// the session, and the cookie's value.
func launch(ctx context.Context, t *testing.T, link *url.URL) (browser *http.Client, cookie string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	browser = &http.Client{Jar: jar, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	// The launch answers with the landing page itself, the Platform
	// (TestBrowserLaunch drives that in real browsers).
	if res := get(ctx, t, browser, link.String(), nil); res.status != http.StatusOK || !strings.Contains(res.body, "<title>Platform · OPM Portal</title>") {
		t.Fatalf("launch: %d %.300s; want 200 with the Platform page", res.status, res.body)
	}
	cookies := jar.Cookies(link)
	if len(cookies) != 1 || !strings.HasPrefix(cookies[0].Name, "opm-portal-") {
		t.Fatalf("launch cookies = %d; want the one session cookie", len(cookies))
	}
	return browser, cookies[0].Value
}

// readAsTheUser reads the instance list and the podinfo graph with the
// session.
func readAsTheUser(ctx context.Context, t *testing.T, browser *http.Client, instances string) {
	t.Helper()
	res := get(ctx, t, browser, instances, nil)
	if res.status != http.StatusOK || !strings.Contains(res.body, `"name":"podinfo"`) {
		t.Fatalf("instances: %d %.300s; want 200 with podinfo", res.status, res.body)
	}
	page := strings.Replace(instances, "/api/v1alpha1/clusters/default/instances", "/instances/default/podinfo", 1)
	if res := get(ctx, t, browser, page, nil); res.status != http.StatusOK || !strings.Contains(res.body, "podinfo-podinfo") {
		t.Fatalf("instance page: %d %.300s; want 200 with podinfo's objects", res.status, res.body)
	}
	res = get(ctx, t, browser, instances+"/default/podinfo/graph", nil)
	var graph struct {
		Kind  string            `json:"kind"`
		Nodes []json.RawMessage `json:"nodes"`
	}
	if res.status != http.StatusOK || json.Unmarshal([]byte(res.body), &graph) != nil || graph.Kind != "Graph" || len(graph.Nodes) == 0 {
		t.Fatalf("graph: %d %.300s; want 200 with a Graph", res.status, res.body)
	}
}

// refuseForeignRequests checks that a DNS-rebinding request (another Host)
// is refused, session or not, and a cross-site write before the read API.
func refuseForeignRequests(ctx context.Context, t *testing.T, browser *http.Client, instances, port string) {
	t.Helper()
	if res := get(ctx, t, browser, instances, map[string]string{"Host": "attacker.example:" + port}); res.status != http.StatusForbidden {
		t.Fatalf("foreign Host: %d; want 403", res.status)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, instances, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	post, err := browser.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = post.Body.Close()
	if post.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST: %d; want 403", post.StatusCode)
	}
}

type portalProcess struct {
	cmd    *exec.Cmd
	launch *url.URL
	stderr *syncBuffer
	done   chan error
}

func startPortal(ctx context.Context, t *testing.T, bin string, args ...string) *portalProcess {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &portalProcess{cmd: cmd, stderr: &syncBuffer{}, done: make(chan error, 1)}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
	})
	lines := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if i := strings.Index(sc.Text(), "http://"); i >= 0 {
				lines <- sc.Text()[i:]
			}
		}
		_, _ = io.Copy(io.Discard, stdout)
	}()
	select {
	case line := <-lines:
		u, err := url.Parse(strings.TrimSpace(line))
		if err != nil {
			t.Fatal(err)
		}
		p.launch = u
	case err := <-p.done:
		t.Fatalf("opm-portal exited before printing the launch link: %v\n%s", err, p.stderr.String())
	case <-time.After(90 * time.Second):
		t.Fatalf("no launch link within 90 s\n%s", p.stderr.String())
	}
	return p
}

func (p *portalProcess) stop(t *testing.T) {
	t.Helper()
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("opm-portal exited with %v\n%s", err, p.stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("opm-portal did not stop within 10 s of SIGINT\n%s", p.stderr.String())
	}
}

type result struct {
	status int
	body   string
}

func get(ctx context.Context, t *testing.T, c *http.Client, target string, header map[string]string) result {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{status: res.StatusCode, body: string(b)}
}

// logTopic returns the log topic of a podinfo Pod's first container.
func logTopic(ctx context.Context, t *testing.T, kubeconfig, kubeContext string) string {
	t.Helper()
	cs, err := kubernetes.NewForConfig(restConfig(t, kubeconfig, kubeContext))
	if err != nil {
		t.Fatal(err)
	}
	pods, err := cs.CoreV1().Pods("default").List(ctx, metav1.ListOptions{LabelSelector: "module-instance.opmodel.dev/name=podinfo"})
	if err != nil || len(pods.Items) == 0 {
		t.Fatalf("listing podinfo Pods: %v", err)
	}
	pod := pods.Items[0]
	return "log:default/" + pod.Name + "/" + pod.Spec.Containers[0].Name
}

// followLog opens a stream on topic and waits for a log line, in its
// snapshot or as a log message.
func followLog(ctx context.Context, t *testing.T, c *http.Client, base, topic string) {
	t.Helper()
	sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(sctx, http.MethodGet, base+"/api/v1alpha1/clusters/default/stream?topics="+url.QueryEscape(topic), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&http.Client{Jar: c.Jar}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("stream %s: %d %s", topic, res.StatusCode, b)
	}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	var event string
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			event = name
			continue
		}
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		switch event {
		case "closed":
			t.Fatalf("log topic closed: %s", data)
		case "log":
			return
		case "snapshot":
			if strings.Contains(data, `"type":"line"`) {
				return
			}
		}
	}
	t.Fatalf("no log line on %s: %v", topic, sc.Err())
}

// syncBuffer is a bytes.Buffer safe for the process's writer and the test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
