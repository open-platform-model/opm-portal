//go:build e2e

package main

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

const (
	podNamespace = "opm-portal"
	podIdentity  = "system:serviceaccount:opm-portal:opm-portal"
	// podPort is the port the shipped Deployment binds on the Pod's loopback.
	podPort = "8090"
)

var launchLine = regexp.MustCompile(`Open this link once to sign in: (http://127\.0\.0\.1:8090/launch\?token=\S+)`)

// TestPod reads through the portal image test/e2e/pod.sh deployed into the
// fixture cluster from deploy/: local mode in a Pod, as its ServiceAccount,
// reached only through kubectl port-forward (portal:D13).
func TestPod(t *testing.T) {
	kubeconfig, kubeContext := fixtureCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	kubectl := func(args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "kubectl", append([]string{"--kubeconfig", kubeconfig, "--context", kubeContext, "-n", podNamespace}, args...)...)
	}

	podLog := waitForLaunchLine(ctx, t, kubectl)
	if !strings.Contains(podLog, "user="+podIdentity) || !strings.Contains(podLog, "source=in-cluster") {
		t.Fatalf("the Pod log does not name %s with source=in-cluster:\n%s", podIdentity, podLog)
	}
	link, err := url.Parse(launchLine.FindStringSubmatch(podLog)[1])
	if err != nil {
		t.Fatal(err)
	}

	// A forward from another local port reaches the Pod, but the Host
	// check refuses it before the token is looked at.
	other := portForward(ctx, t, kubectl, "8091")
	wrongPort := *link
	wrongPort.Host = "127.0.0.1:8091"
	if res := get(ctx, t, &http.Client{Timeout: 30 * time.Second}, wrongPort.String(), nil); res.status != http.StatusForbidden {
		t.Fatalf("launch through 8091:%s: %d; want 403", podPort, res.status)
	}
	other()

	stop := portForward(ctx, t, kubectl, podPort)
	defer stop()
	browser, _ := launch(ctx, t, link)
	var list v1.InstanceList
	getJSON(ctx, t, browser, "http://127.0.0.1:"+podPort+"/api/v1alpha1/clusters/default/instances", &list)
	if list.Access != v1.AccessOK {
		t.Fatalf("instance list access = %q; want ok", list.Access)
	}
	got := make([]string, 0, len(list.Items))
	for _, it := range list.Items {
		got = append(got, it.Ref.Namespace+"/"+it.Ref.Name)
	}
	want := clusterInstances(ctx, t, kubeconfig, kubeContext)
	slices.Sort(got)
	if !slices.Equal(got, want) || len(want) == 0 {
		t.Fatalf("instances through the Pod = %v; the fixture cluster holds %v", got, want)
	}
	t.Logf("read as %s through port-forward %s:%s: %v", podIdentity, podPort, podPort, got)
	// The token is spent: a second browser is refused.
	if res := get(ctx, t, &http.Client{Timeout: 30 * time.Second}, link.String(), nil); res.status != http.StatusForbidden {
		t.Fatalf("second launch: %d; want 403", res.status)
	}
}

// waitForLaunchLine returns the Pod's log once it holds the launch link.
func waitForLaunchLine(ctx context.Context, t *testing.T, kubectl func(...string) *exec.Cmd) string {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		out, err := kubectl("logs", "deploy/opm-portal").CombinedOutput()
		if err == nil && launchLine.Match(out) {
			return string(out)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no launch link in the Pod log within 90 s: %v\n%s", err, out)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

// portForward forwards 127.0.0.1:local to the Pod's podPort and returns
// once kubectl says it is forwarding; the returned func stops it.
func portForward(ctx context.Context, t *testing.T, kubectl func(...string) *exec.Cmd, local string) (stop func()) {
	t.Helper()
	cmd := kubectl("port-forward", "--address", "127.0.0.1", "deploy/opm-portal", local+":"+podPort)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &syncBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop = func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	t.Cleanup(stop)
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stdout)
		seen := false
		for sc.Scan() {
			if !seen && strings.HasPrefix(sc.Text(), "Forwarding from 127.0.0.1:"+local) {
				close(ready)
				seen = true
			}
		}
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatalf("kubectl port-forward %s:%s: %v\n%s", local, podPort, ctx.Err(), stderr.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("kubectl port-forward %s:%s did not start within 30 s\n%s", local, podPort, stderr.String())
	}
	return stop
}

// clusterInstances lists the fixture cluster's ModuleInstances as
// namespace/name, sorted, with the fixture cluster's own kubeconfig.
func clusterInstances(ctx context.Context, t *testing.T, kubeconfig, kubeContext string) []string {
	t.Helper()
	dyn, err := dynamic.NewForConfig(restConfig(t, kubeconfig, kubeContext))
	if err != nil {
		t.Fatal(err)
	}
	list, err := dyn.Resource(instanceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing ModuleInstances: %v", err)
	}
	names := make([]string, 0, len(list.Items))
	for _, it := range list.Items {
		names = append(names, it.GetNamespace()+"/"+it.GetName())
	}
	slices.Sort(names)
	return names
}
