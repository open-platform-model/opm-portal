// Package deploy holds the tests of the manifest that runs local mode in a
// Pod (portal:D13). The manifest itself is the YAML beside this file.
package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/open-platform-model/opm-portal/internal/version"
)

// object is one YAML document of the manifest.
type object struct {
	file string
	kind string
	raw  []byte
}

// readManifest returns every object in deploy/*.yaml, the kustomization
// excepted.
func readManifest(t *testing.T) []object {
	t.Helper()
	files, err := filepath.Glob("*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var objects []object
	for _, f := range files {
		if f == "kustomization.yaml" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for doc := range bytes.SplitSeq(b, []byte("\n---")) {
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}
			var tm metav1.TypeMeta
			if err := yaml.Unmarshal(doc, &tm); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if tm.Kind == "" {
				continue
			}
			objects = append(objects, object{file: f, kind: tm.Kind, raw: doc})
		}
	}
	if len(objects) == 0 {
		t.Fatal("no objects under deploy/")
	}
	return objects
}

var readVerbs = []string{"get", "list", "watch"}

// checkRole refuses a role that could write, read Secrets, impersonate, or
// grant more than it names.
func checkRole(r rbacv1.ClusterRole) error {
	var errs []error
	if r.AggregationRule != nil {
		errs = append(errs, fmt.Errorf("role %s has an aggregation rule", r.Name))
	}
	for i := range r.Rules {
		errs = append(errs, checkRule(fmt.Sprintf("role %s rule %d", r.Name, i), &r.Rules[i])...)
	}
	return errors.Join(errs...)
}

func checkRule(at string, rule *rbacv1.PolicyRule) []error {
	var errs []error
	if len(rule.NonResourceURLs) > 0 {
		errs = append(errs, fmt.Errorf("%s grants non-resource URLs %v", at, rule.NonResourceURLs))
	}
	if len(rule.ResourceNames) > 0 {
		errs = append(errs, fmt.Errorf("%s narrows by resource name; the role lists whole kinds", at))
	}
	for _, v := range rule.Verbs {
		switch {
		case v == "*":
			errs = append(errs, fmt.Errorf("%s grants the wildcard verb", at))
		case v == "impersonate":
			errs = append(errs, fmt.Errorf("%s grants impersonate", at))
		case !slices.Contains(readVerbs, v):
			errs = append(errs, fmt.Errorf("%s grants the verb %q; only get, list and watch are allowed", at, v))
		}
	}
	for _, g := range rule.APIGroups {
		if strings.Contains(g, "*") {
			errs = append(errs, fmt.Errorf("%s names the API group %q", at, g))
		}
	}
	for _, res := range rule.Resources {
		base, _, _ := strings.Cut(res, "/")
		switch {
		case strings.Contains(res, "*"):
			errs = append(errs, fmt.Errorf("%s names the resource %q", at, res))
		case base == "secrets":
			errs = append(errs, fmt.Errorf("%s grants %q: the portal never reads Secrets", at, res))
		case res == "pods/log" && !slices.Equal(rule.Verbs, []string{"get"}):
			errs = append(errs, fmt.Errorf("%s grants %v on pods/log; only get is needed", at, rule.Verbs))
		}
	}
	return errs
}

// exposing are the kinds that would open a network path to the portal.
var exposing = []string{"Service", "Ingress", "IngressClass", "Gateway", "HTTPRoute", "GRPCRoute", "TCPRoute", "TLSRoute", "Route"}

// checkObjects refuses any object that opens a network path, and a binding
// of the portal to a built-in role.
func checkObjects(objects []object) error {
	var errs []error
	for _, o := range objects {
		if slices.Contains(exposing, o.kind) {
			errs = append(errs, fmt.Errorf("%s holds kind %s: the portal is reached only through kubectl port-forward", o.file, o.kind))
		}
		if o.kind == "ClusterRoleBinding" || o.kind == "RoleBinding" {
			var b rbacv1.ClusterRoleBinding
			if err := yaml.Unmarshal(o.raw, &b); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", o.file, err))
				continue
			}
			if b.RoleRef.Name != "opm-portal-reader" {
				errs = append(errs, fmt.Errorf("%s binds %s %q; only opm-portal-reader is allowed", o.file, b.RoleRef.Kind, b.RoleRef.Name))
			}
		}
	}
	return errors.Join(errs...)
}

// checkDeployment refuses a Deployment that binds beyond loopback, carries a
// probe, or loosens the restricted security context.
func checkDeployment(d appsv1.Deployment) error {
	var errs []error
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 1 {
		errs = append(errs, errors.New("the Deployment must run exactly one replica: the launch link and session live in one process"))
	}
	pod := d.Spec.Template.Spec
	if pod.HostNetwork {
		errs = append(errs, errors.New("the Pod uses the host network"))
	}
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot {
		errs = append(errs, errors.New("the Pod does not set runAsNonRoot: true"))
	}
	if pod.SecurityContext == nil || pod.SecurityContext.SeccompProfile == nil || pod.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		errs = append(errs, errors.New("the Pod does not set the RuntimeDefault seccomp profile"))
	}
	if len(pod.Containers) != 1 || len(pod.InitContainers) != 0 {
		errs = append(errs, fmt.Errorf("the Pod has %d containers and %d init containers; want one container", len(pod.Containers), len(pod.InitContainers)))
	}
	for i := range pod.Containers {
		errs = append(errs, checkContainer(&pod.Containers[i])...)
	}
	return errors.Join(errs...)
}

func checkContainer(c *corev1.Container) []error {
	var errs []error
	at := "container " + c.Name
	if addr, ok := flagValue(c.Args, "--addr"); !ok {
		errs = append(errs, fmt.Errorf("%s does not pass --addr; the default port would change on every start", at))
	} else if host, _, err := net.SplitHostPort(addr); err != nil || host != "127.0.0.1" {
		errs = append(errs, fmt.Errorf("%s binds %q; want 127.0.0.1", at, addr))
	}
	if len(c.Args) == 0 || c.Args[0] != "serve" {
		errs = append(errs, fmt.Errorf("%s does not run serve", at))
	}
	if len(c.Ports) > 0 {
		errs = append(errs, fmt.Errorf("%s declares ports; nothing reaches a loopback listener through them", at))
	}
	if c.LivenessProbe != nil || c.ReadinessProbe != nil || c.StartupProbe != nil {
		errs = append(errs, fmt.Errorf("%s has a probe; a probe cannot reach a loopback listener", at))
	}
	errs = append(errs, checkSecurityContext(at, c.SecurityContext)...)
	if c.Resources.Requests.Memory().IsZero() || c.Resources.Limits.Memory().IsZero() {
		errs = append(errs, fmt.Errorf("%s does not set a memory request and limit", at))
	}
	return errs
}

func checkSecurityContext(at string, sc *corev1.SecurityContext) []error {
	if sc == nil {
		return []error{fmt.Errorf("%s has no securityContext", at)}
	}
	var errs []error
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		errs = append(errs, fmt.Errorf("%s does not set readOnlyRootFilesystem: true", at))
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		errs = append(errs, fmt.Errorf("%s does not set allowPrivilegeEscalation: false", at))
	}
	if sc.Privileged != nil && *sc.Privileged {
		errs = append(errs, fmt.Errorf("%s is privileged", at))
	}
	if sc.Capabilities == nil || !slices.Equal(sc.Capabilities.Drop, []corev1.Capability{"ALL"}) || len(sc.Capabilities.Add) > 0 {
		errs = append(errs, fmt.Errorf("%s does not drop ALL capabilities and add none", at))
	}
	return errs
}

// flagValue returns the value of name in args, as "--name value" or
// "--name=value".
func flagValue(args []string, name string) (string, bool) {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1], true
		}
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v, true
		}
	}
	return "", false
}

func TestManifest(t *testing.T) {
	objects := readManifest(t)
	kinds := make([]string, 0, len(objects))
	for _, o := range objects {
		kinds = append(kinds, o.kind)
		switch o.kind {
		case "ClusterRole", "Role":
			var r rbacv1.ClusterRole
			if err := yaml.UnmarshalStrict(o.raw, &r); err != nil {
				t.Fatalf("%s: %v", o.file, err)
			}
			if err := checkRole(r); err != nil {
				t.Errorf("%s: %v", o.file, err)
			}
		case "Deployment":
			var d appsv1.Deployment
			if err := yaml.UnmarshalStrict(o.raw, &d); err != nil {
				t.Fatalf("%s: %v", o.file, err)
			}
			if err := checkDeployment(d); err != nil {
				t.Errorf("%s: %v", o.file, err)
			}
		}
	}
	if err := checkObjects(objects); err != nil {
		t.Error(err)
	}
	slices.Sort(kinds)
	want := []string{"ClusterRole", "ClusterRoleBinding", "Deployment", "Namespace", "ServiceAccount"}
	if !slices.Equal(kinds, want) {
		t.Errorf("deploy/ holds %v; want exactly %v", kinds, want)
	}
}

// TestCheckRoleAllowsAReadRule shows the README's advice holds: a read rule
// for a provider kind added to the role passes.
func TestCheckRoleAllowsAReadRule(t *testing.T) {
	r := rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "opm-portal-reader"}, Rules: []rbacv1.PolicyRule{{
		APIGroups: []string{"cert-manager.io"}, Resources: []string{"certificates", "issuers", "clusterissuers"}, Verbs: []string{"get", "list", "watch"},
	}}}
	if err := checkRole(r); err != nil {
		t.Fatalf("checkRole() = %v; want a get, list and watch rule for a provider kind allowed", err)
	}
}

func TestChecksRefuse(t *testing.T) {
	read := []string{"get", "list", "watch"}
	role := func(rules ...rbacv1.PolicyRule) rbacv1.ClusterRole {
		return rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Rules: rules}
	}
	roles := []struct {
		name string
		role rbacv1.ClusterRole
		want string
	}{
		{"a write verb", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}}), `verb "create"`},
		{"delete", role(rbacv1.PolicyRule{APIGroups: []string{"opmodel.dev"}, Resources: []string{"moduleinstances"}, Verbs: []string{"delete"}}), `verb "delete"`},
		{"secrets", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: read}), "never reads Secrets"},
		{"a secrets subresource", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets/status"}, Verbs: []string{"get"}}), "never reads Secrets"},
		{"impersonate", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"users"}, Verbs: []string{"impersonate"}}), "impersonate"},
		{"a wildcard resource", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"*"}, Verbs: read}), `resource "*"`},
		{"a wildcard group", role(rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"pods"}, Verbs: read}), `API group "*"`},
		{"a wildcard verb", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}}), "wildcard verb"},
		{"a non-resource URL", role(rbacv1.PolicyRule{NonResourceURLs: []string{"/metrics"}, Verbs: []string{"get"}}), "non-resource"},
		{"an aggregation rule", func() rbacv1.ClusterRole {
			r := role()
			r.AggregationRule = &rbacv1.AggregationRule{}
			return r
		}(), "aggregation"},
	}
	for _, tt := range roles {
		t.Run("role/"+tt.name, func(t *testing.T) {
			err := checkRole(tt.role)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkRole() = %v; want an error naming %q", err, tt.want)
			}
		})
	}

	objects := []struct {
		name string
		obj  object
		want string
	}{
		{"a Service", object{file: "svc.yaml", kind: "Service"}, "holds kind Service"},
		{"an Ingress", object{file: "ing.yaml", kind: "Ingress"}, "holds kind Ingress"},
		{"a binding to view", object{file: "b.yaml", kind: "ClusterRoleBinding", raw: []byte("kind: ClusterRoleBinding\nroleRef: {kind: ClusterRole, name: view}\n")}, `"view"`},
	}
	for _, tt := range objects {
		t.Run("object/"+tt.name, func(t *testing.T) {
			err := checkObjects([]object{tt.obj})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkObjects() = %v; want an error naming %q", err, tt.want)
			}
		})
	}

	deployments := []struct {
		name   string
		mutate func(d *appsv1.Deployment)
		want   string
	}{
		{"all interfaces", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{"serve", "--addr", "0.0.0.0:8090"}
		}, `binds "0.0.0.0:8090"`},
		{"no --addr", func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].Args = []string{"serve"} }, "does not pass --addr"},
		{"writable root", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = nil
		}, "readOnlyRootFilesystem"},
		{"privilege escalation", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = new(true)
		}, "allowPrivilegeEscalation"},
		{"a capability kept", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Add = []corev1.Capability{"NET_ADMIN"}
		}, "capabilities"},
		{"root allowed", func(d *appsv1.Deployment) { d.Spec.Template.Spec.SecurityContext.RunAsNonRoot = nil }, "runAsNonRoot"},
		{"no seccomp", func(d *appsv1.Deployment) { d.Spec.Template.Spec.SecurityContext.SeccompProfile = nil }, "seccomp"},
		{"a probe", func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{} }, "probe"},
		{"host network", func(d *appsv1.Deployment) { d.Spec.Template.Spec.HostNetwork = true }, "host network"},
		{"two replicas", func(d *appsv1.Deployment) { d.Spec.Replicas = new(int32(2)) }, "one replica"},
	}
	for _, tt := range deployments {
		t.Run("deployment/"+tt.name, func(t *testing.T) {
			d := shippedDeployment(t)
			if err := checkDeployment(d); err != nil {
				t.Fatalf("the shipped Deployment fails before the change: %v", err)
			}
			tt.mutate(&d)
			err := checkDeployment(d)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkDeployment() = %v; want an error naming %q", err, tt.want)
			}
		})
	}
}

func shippedDeployment(t *testing.T) appsv1.Deployment {
	t.Helper()
	b, err := os.ReadFile("deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var d appsv1.Deployment
	if err := yaml.UnmarshalStrict(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// imageLine matches the image line release-please rewrites.
var imageLine = regexp.MustCompile(`^\s*image: ghcr\.io/open-platform-model/opm-portal:v(\S+) # x-release-please-version$`)

// TestImageTagFollowsTheVersion holds the image line to the release
// contract: release-please's generic updater rewrites only lines carrying
// the marker, so the marker must sit on the image line, and the tag must be
// the version internal/version carries, which the same release PR moves.
func TestImageTagFollowsTheVersion(t *testing.T) {
	b, err := os.ReadFile("deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	for line := range strings.SplitSeq(string(b), "\n") {
		if !strings.Contains(line, "x-release-please-version") {
			continue
		}
		m := imageLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("the x-release-please-version marker is on a line that is not the portal image: %q", line)
		}
		tags = append(tags, m[1])
	}
	if len(tags) != 1 {
		t.Fatalf("deployment.yaml has %d marked image lines; want 1", len(tags))
	}
	if tags[0] != version.Version {
		t.Fatalf("the image tag is v%s; internal/version is v%s", tags[0], version.Version)
	}
	d := shippedDeployment(t)
	if got, want := d.Spec.Template.Spec.Containers[0].Image, "ghcr.io/open-platform-model/opm-portal:v"+version.Version; got != want {
		t.Fatalf("the container image is %q; want %q", got, want)
	}
}
