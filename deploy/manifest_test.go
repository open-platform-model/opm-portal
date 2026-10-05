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

const kustomizationFile = "kustomization.yaml"

// shippedArgs are the container's arguments, exactly.
var shippedArgs = []string{"serve", "--addr", "127.0.0.1:8090"}

// portalSubject is the only subject the binding may name.
var portalSubject = rbacv1.Subject{Kind: "ServiceAccount", Name: "opm-portal", Namespace: "opm-portal"}

// object is one YAML document of the manifest.
type object struct {
	file string
	kind string
	raw  []byte
}

// manifestFiles returns every YAML file in deploy/, the kustomization
// included.
func manifestFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.yaml", "*.yml"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	slices.Sort(files)
	return files
}

// readObjects returns every object in the manifest files, the kustomization
// excepted.
func readObjects(t *testing.T, files []string) []object {
	t.Helper()
	var objects []object
	for _, f := range files {
		if f == kustomizationFile {
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
				t.Fatalf("%s holds a document with no kind", f)
			}
			objects = append(objects, object{file: f, kind: tm.Kind, raw: doc})
		}
	}
	if len(objects) == 0 {
		t.Fatal("no objects under deploy/")
	}
	return objects
}

// kustomizationKeys are the only fields the kustomization may carry: no
// patches, generators, images or remote bases that would change what the
// tests here read.
var kustomizationKeys = []string{"apiVersion", "kind", "resources"}

// checkKustomization refuses a kustomization with any other field, a
// resource that is not a file beside it, or a manifest file it does not
// list, so the files these tests read are exactly what kubectl applies.
func checkKustomization(raw []byte, files []string) error {
	var k map[string]any
	if err := yaml.Unmarshal(raw, &k); err != nil {
		return fmt.Errorf("%s: %w", kustomizationFile, err)
	}
	var errs []error
	for key := range k {
		if !slices.Contains(kustomizationKeys, key) {
			errs = append(errs, fmt.Errorf("%s carries the field %q; only %v are allowed", kustomizationFile, key, kustomizationKeys))
		}
	}
	list, _ := k["resources"].([]any)
	listed := map[string]bool{}
	for _, r := range list {
		name, ok := r.(string)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s lists a resource that is not a file name: %v", kustomizationFile, r))
		case name != filepath.Base(name) || strings.Contains(name, ":") || name == "..":
			errs = append(errs, fmt.Errorf("%s lists %q, which is not a file in deploy/", kustomizationFile, name))
		case !slices.Contains(files, name):
			errs = append(errs, fmt.Errorf("%s lists %q, which does not exist in deploy/", kustomizationFile, name))
		default:
			listed[name] = true
		}
	}
	for _, f := range files {
		if f != kustomizationFile && !listed[f] {
			errs = append(errs, fmt.Errorf("%s is not listed in %s, so the tests read a file kubectl does not apply", f, kustomizationFile))
		}
	}
	return errors.Join(errs...)
}

var readVerbs = []string{"get", "list", "watch"}

// checkRole refuses a role that could write, read Secrets, impersonate,
// reach into a node or Pod, or grant more than it names.
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
		errs = append(errs, checkResource(at, res, rule.Verbs)...)
	}
	return errs
}

// checkResource refuses a wildcard, Secrets, and every subresource but
// pods/log: proxy, exec, attach and portforward reach past the API into a
// node or a Pod, even with get.
func checkResource(at, res string, verbs []string) []error {
	base, sub, isSub := strings.Cut(res, "/")
	switch {
	case strings.Contains(res, "*"):
		return []error{fmt.Errorf("%s names the resource %q", at, res)}
	case base == "secrets":
		return []error{fmt.Errorf("%s grants %q: the portal never reads Secrets", at, res)}
	case isSub && res != "pods/log":
		return []error{fmt.Errorf("%s grants the subresource %q; only pods/log is allowed", at, base+"/"+sub)}
	case res == "pods/log" && !slices.Equal(verbs, []string{"get"}):
		return []error{fmt.Errorf("%s grants %v on pods/log; only get is needed", at, verbs)}
	}
	return nil
}

// exposing are the kinds that would open a network path to the portal.
var exposing = []string{"Service", "Ingress", "IngressClass", "Gateway", "HTTPRoute", "GRPCRoute", "TCPRoute", "TLSRoute", "Route"}

// checkObjects refuses any object that opens a network path, a binding that
// names another role or another subject, and a Namespace that does not
// enforce the restricted Pod Security Standard.
func checkObjects(objects []object) error {
	var errs []error
	for _, o := range objects {
		if slices.Contains(exposing, o.kind) {
			errs = append(errs, fmt.Errorf("%s holds kind %s: the portal is reached only through kubectl port-forward", o.file, o.kind))
		}
		switch o.kind {
		case "ClusterRoleBinding", "RoleBinding":
			var b rbacv1.ClusterRoleBinding
			if err := yaml.Unmarshal(o.raw, &b); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", o.file, err))
				continue
			}
			errs = append(errs, checkBinding(o.file, &b)...)
		case "Namespace":
			var ns corev1.Namespace
			if err := yaml.Unmarshal(o.raw, &ns); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", o.file, err))
				continue
			}
			if ns.Labels["pod-security.kubernetes.io/enforce"] != "restricted" {
				errs = append(errs, fmt.Errorf("%s: Namespace %s does not carry pod-security.kubernetes.io/enforce: restricted", o.file, ns.Name))
			}
		}
	}
	return errors.Join(errs...)
}

func checkBinding(file string, b *rbacv1.ClusterRoleBinding) []error {
	var errs []error
	if b.RoleRef.Name != "opm-portal-reader" {
		errs = append(errs, fmt.Errorf("%s binds %s %q; only opm-portal-reader is allowed", file, b.RoleRef.Kind, b.RoleRef.Name))
	}
	if !slices.Equal(b.Subjects, []rbacv1.Subject{portalSubject}) {
		errs = append(errs, fmt.Errorf("%s binds the subjects %+v; only the ServiceAccount opm-portal/opm-portal is allowed", file, b.Subjects))
	}
	return errs
}

// checkDeployment refuses a Deployment that binds beyond loopback, shares a
// host namespace, mounts a volume, carries a probe, or loosens the
// restricted security context.
func checkDeployment(d *appsv1.Deployment) error {
	var errs []error
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 1 {
		errs = append(errs, errors.New("the Deployment must run exactly one replica: the launch link and session live in one process"))
	}
	pod := &d.Spec.Template.Spec
	if pod.HostNetwork || pod.HostPID || pod.HostIPC {
		errs = append(errs, errors.New("the Pod shares a host namespace (hostNetwork, hostPID or hostIPC)"))
	}
	if len(pod.Volumes) > 0 {
		errs = append(errs, fmt.Errorf("the Pod mounts %d volumes; it needs none", len(pod.Volumes)))
	}
	errs = append(errs, checkPodSecurityContext(pod.SecurityContext)...)
	if len(pod.Containers) != 1 || len(pod.InitContainers) != 0 || len(pod.EphemeralContainers) != 0 {
		errs = append(errs, fmt.Errorf("the Pod has %d containers and %d init containers; want one container", len(pod.Containers), len(pod.InitContainers)))
	}
	for i := range pod.Containers {
		errs = append(errs, checkContainer(&pod.Containers[i])...)
	}
	return errors.Join(errs...)
}

func checkPodSecurityContext(sc *corev1.PodSecurityContext) []error {
	var errs []error
	if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		errs = append(errs, errors.New("the Pod does not set runAsNonRoot: true"))
	}
	if sc == nil || sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		errs = append(errs, errors.New("the Pod does not set the RuntimeDefault seccomp profile"))
	}
	return errs
}

func checkContainer(c *corev1.Container) []error {
	var errs []error
	at := "container " + c.Name
	if addr, ok := flagValue(c.Args, "--addr"); ok {
		if host, _, err := net.SplitHostPort(addr); err != nil || host != "127.0.0.1" {
			errs = append(errs, fmt.Errorf("%s binds %q; want 127.0.0.1", at, addr))
		}
	}
	if !slices.Equal(c.Args, shippedArgs) {
		errs = append(errs, fmt.Errorf("%s runs with args %q; want exactly %q", at, c.Args, shippedArgs))
	}
	if len(c.Command) > 0 {
		errs = append(errs, fmt.Errorf("%s overrides the image's command with %q", at, c.Command))
	}
	if len(c.Ports) > 0 {
		errs = append(errs, fmt.Errorf("%s declares ports; nothing reaches a loopback listener through them", at))
	}
	if len(c.VolumeMounts) > 0 {
		errs = append(errs, fmt.Errorf("%s mounts volumes", at))
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
	files := manifestFiles(t)
	raw, err := os.ReadFile(kustomizationFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkKustomization(raw, files); err != nil {
		t.Error(err)
	}
	objects := readObjects(t, files)
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
			if err := checkDeployment(&d); err != nil {
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

func TestCheckRoleRefuses(t *testing.T) {
	read := []string{"get", "list", "watch"}
	role := func(rules ...rbacv1.PolicyRule) rbacv1.ClusterRole {
		return rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Rules: rules}
	}
	sub := func(res string) rbacv1.ClusterRole {
		return role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{res}, Verbs: []string{"get"}})
	}
	tests := []struct {
		name string
		role rbacv1.ClusterRole
		want string
	}{
		{"a write verb", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}}), `verb "create"`},
		{"delete", role(rbacv1.PolicyRule{APIGroups: []string{"opmodel.dev"}, Resources: []string{"moduleinstances"}, Verbs: []string{"delete"}}), `verb "delete"`},
		{"secrets", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: read}), "never reads Secrets"},
		{"a secrets subresource", sub("secrets/status"), "never reads Secrets"},
		{"impersonate", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"users"}, Verbs: []string{"impersonate"}}), "impersonate"},
		{"a wildcard resource", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"*"}, Verbs: read}), `resource "*"`},
		{"a wildcard subresource", sub("pods/*"), `resource "pods/*"`},
		{"a wildcard group", role(rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"pods"}, Verbs: read}), `API group "*"`},
		{"a wildcard verb", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}}), "wildcard verb"},
		{"a non-resource URL", role(rbacv1.PolicyRule{NonResourceURLs: []string{"/metrics"}, Verbs: []string{"get"}}), "non-resource"},
		{"nodes/proxy", sub("nodes/proxy"), `subresource "nodes/proxy"`},
		{"pods/exec", sub("pods/exec"), `subresource "pods/exec"`},
		{"pods/attach", sub("pods/attach"), `subresource "pods/attach"`},
		{"pods/portforward", sub("pods/portforward"), `subresource "pods/portforward"`},
		{"pods/proxy", sub("pods/proxy"), `subresource "pods/proxy"`},
		{"services/proxy", sub("services/proxy"), `subresource "services/proxy"`},
		{"deployments/scale", sub("deployments/scale"), `subresource "deployments/scale"`},
		{"pods/log with list", role(rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: read}), "only get is needed"},
		{"an aggregation rule", func() rbacv1.ClusterRole {
			r := role()
			r.AggregationRule = &rbacv1.AggregationRule{}
			return r
		}(), "aggregation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRole(tt.role)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkRole() = %v; want an error naming %q", err, tt.want)
			}
		})
	}
}

func TestCheckObjectsRefuses(t *testing.T) {
	binding := func(role, subjects string) object {
		return object{file: "b.yaml", kind: "ClusterRoleBinding", raw: []byte("kind: ClusterRoleBinding\nroleRef: {kind: ClusterRole, name: " + role + "}\nsubjects: " + subjects + "\n")}
	}
	sa := "[{kind: ServiceAccount, name: opm-portal, namespace: opm-portal}]"
	tests := []struct {
		name string
		obj  object
		want string
	}{
		{"a Service", object{file: "svc.yaml", kind: "Service"}, "holds kind Service"},
		{"an Ingress", object{file: "ing.yaml", kind: "Ingress"}, "holds kind Ingress"},
		{"a binding to view", binding("view", sa), `"view"`},
		{"all authenticated users", binding("opm-portal-reader", "[{kind: Group, name: 'system:authenticated', apiGroup: rbac.authorization.k8s.io}]"), "only the ServiceAccount"},
		{"anonymous users", binding("opm-portal-reader", "[{kind: Group, name: 'system:unauthenticated', apiGroup: rbac.authorization.k8s.io}]"), "only the ServiceAccount"},
		{"a second subject", binding("opm-portal-reader", "[{kind: ServiceAccount, name: opm-portal, namespace: opm-portal}, {kind: User, name: alice}]"), "only the ServiceAccount"},
		{"a ServiceAccount elsewhere", binding("opm-portal-reader", "[{kind: ServiceAccount, name: opm-portal, namespace: default}]"), "only the ServiceAccount"},
		{"a Namespace without the restricted label", object{file: "ns.yaml", kind: "Namespace", raw: []byte("kind: Namespace\nmetadata: {name: opm-portal, labels: {pod-security.kubernetes.io/enforce: baseline}}\n")}, "enforce: restricted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkObjects([]object{tt.obj})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkObjects() = %v; want an error naming %q", err, tt.want)
			}
		})
	}
	if err := checkObjects([]object{binding("opm-portal-reader", sa)}); err != nil {
		t.Fatalf("checkObjects() refuses the shipped binding shape: %v", err)
	}
}

func TestCheckKustomizationRefuses(t *testing.T) {
	files := []string{"a.yaml", "b.yaml", kustomizationFile}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"a patch", "kind: Kustomization\nresources: [a.yaml, b.yaml]\npatches: []\n", `field "patches"`},
		{"an images override", "kind: Kustomization\nresources: [a.yaml, b.yaml]\nimages: []\n", `field "images"`},
		{"a remote base", "kind: Kustomization\nresources: [a.yaml, b.yaml, 'https://example.com/base']\n", "not a file in deploy/"},
		{"a parent directory", "kind: Kustomization\nresources: [a.yaml, b.yaml, ../other]\n", "not a file in deploy/"},
		{"a missing file", "kind: Kustomization\nresources: [a.yaml, b.yaml, c.yaml]\n", "does not exist"},
		{"an unlisted file", "kind: Kustomization\nresources: [a.yaml]\n", "b.yaml is not listed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKustomization([]byte(tt.raw), files)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkKustomization() = %v; want an error naming %q", err, tt.want)
			}
		})
	}
}

func TestCheckDeploymentRefuses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(d *appsv1.Deployment)
		want   string
	}{
		{"all interfaces", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{"serve", "--addr", "0.0.0.0:8090"}
		}, `binds "0.0.0.0:8090"`},
		{"no --addr", func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].Args = []string{"serve"} }, "want exactly"},
		{"an extra arg", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = append(slices.Clone(shippedArgs), "--namespaces", "default")
		}, "want exactly"},
		{"a command override", func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh"} }, "overrides the image's command"},
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
		{"host network", func(d *appsv1.Deployment) { d.Spec.Template.Spec.HostNetwork = true }, "host namespace"},
		{"host PID", func(d *appsv1.Deployment) { d.Spec.Template.Spec.HostPID = true }, "host namespace"},
		{"host IPC", func(d *appsv1.Deployment) { d.Spec.Template.Spec.HostIPC = true }, "host namespace"},
		{"a hostPath volume", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "logs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}}}
		}, "mounts 1 volumes"},
		{"a volume mount", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "logs", MountPath: "/logs"}}
		}, "mounts volumes"},
		{"two replicas", func(d *appsv1.Deployment) { d.Spec.Replicas = new(int32(2)) }, "one replica"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := shippedDeployment(t)
			if err := checkDeployment(&d); err != nil {
				t.Fatalf("the shipped Deployment fails before the change: %v", err)
			}
			tt.mutate(&d)
			err := checkDeployment(&d)
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
