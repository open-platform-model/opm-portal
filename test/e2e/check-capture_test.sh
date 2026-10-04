#!/usr/bin/env bash
# check-capture_test.sh: run check-capture.sh against scratch capture directories, one case per
# directory, and fail unless each case gets the exit status it should. Needs only yq; never
# contacts a cluster.
set -euo pipefail

CHECK=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-capture.sh
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT

failures=0
cases=0

# expect <pass|fail> <name> <file> <<content: write content to <name>/<file>, run the check on
# <name>, and compare its exit status with the expectation.
expect() {
  local want=$1 name=$2 file=$3 dir got
  dir=$SCRATCH/$name
  mkdir -p "$(dirname "$dir/$file")"
  cat >"$dir/$file"
  cases=$((cases + 1))
  if "$CHECK" "$dir" >/dev/null 2>"$SCRATCH/$name.err"; then got=pass; else got=fail; fi
  if [ "$got" != "$want" ]; then
    failures=$((failures + 1))
    echo "FAIL $name: want $want, got $got" >&2
    sed 's/^/  /' "$SCRATCH/$name.err" >&2
  fi
}

expect pass clean-list f1/objects.yaml <<'EOF'
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ConfigMap
    metadata: {name: a, namespace: default}
    data: {k: v}
  - apiVersion: opmodel.dev/v1alpha1
    kind: ModuleInstance
    metadata: {name: web, namespace: web}
    spec: {module: {path: opmodel.dev/modules/web_app@v1}}
    status:
      inventory:
        entries:
          - {group: "", v: v1, kind: Secret, namespace: web, name: web-tls}
EOF

expect pass event-about-a-secret f1/events.yaml <<'EOF'
apiVersion: v1
kind: List
items:
  - apiVersion: events.k8s.io/v1
    kind: Event
    metadata: {name: e1, namespace: default}
    reason: Issued
    regarding: {apiVersion: v1, kind: Secret, namespace: default, name: tls}
EOF

expect fail bare-secret f1/s.yaml <<'EOF'
apiVersion: v1
kind: Secret
metadata: {name: s, namespace: default}
data: {a: eA==}
EOF

expect fail secret-in-list f1/objects.yml <<'EOF'
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: Secret
    metadata: {name: s, namespace: default}
EOF

expect fail secret-second-document f1/multi.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: {name: a, namespace: default}
---
apiVersion: v1
kind: Secret
metadata: {name: s, namespace: default}
EOF

expect fail secret-top-level-json-array f1/nested/arr.json <<'EOF'
[{"apiVersion": "v1", "kind": "Secret", "metadata": {"name": "s", "namespace": "default"}}]
EOF

expect fail secret-top-level-yaml-sequence f1/seq.yaml <<'EOF'
- apiVersion: v1
  kind: Secret
  metadata: {name: s, namespace: default}
EOF

expect fail secret-in-nested-list f1/n.yaml <<'EOF'
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: List
    items:
      - apiVersion: v1
        kind: Secret
        metadata: {name: s, namespace: default}
EOF

expect fail secret-with-items-key f1/items.yaml <<'EOF'
kind: Secret
items: []
data: {a: eA==}
EOF

expect fail secret-without-metadata f1/nometa.yaml <<'EOF'
apiVersion: v1
kind: List
items:
  - kind: Secret
    stringData: {a: x}
EOF

expect fail secret-hidden-file f1/.hidden <<'EOF'
apiVersion: v1
kind: Secret
metadata: {name: s, namespace: default}
EOF

mkdir -p "$SCRATCH/markdown-ignored/f1"
printf 'apiVersion: v1\nkind: List\nitems: []\n' >"$SCRATCH/markdown-ignored/f1/objects.yaml"
expect pass markdown-ignored f1/README.md <<'EOF'
kind: Secret
metadata: {name: s}
EOF

expect fail instance-values f1/moduleinstances.yaml <<'EOF'
apiVersion: v1
kind: List
items:
  - apiVersion: opmodel.dev/v1alpha1
    kind: ModuleInstance
    metadata: {name: web, namespace: web}
    spec: {values: {password: hunter2}}
EOF

expect fail package-values-in-nested-list f1/modulepackages.yaml <<'EOF'
- apiVersion: v1
  kind: List
  items:
    - apiVersion: opmodel.dev/v1alpha1
      kind: ModulePackage
      metadata: {name: p, namespace: pkg}
      spec: {values: {a: b}}
EOF

expect fail managed-fields f1/mf.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: a
  namespace: default
  managedFields: [{manager: kubectl}]
EOF

expect fail last-applied f1/la.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: a
  namespace: default
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{"data":{"k":"v"}}'
EOF

expect fail crd-schema f1/crd.yaml <<'EOF'
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: {name: things.example.com}
spec:
  versions:
    - name: v1
      schema: {openAPIV3Schema: {type: object}}
EOF

expect fail unparseable f1/broken.yaml <<'EOF'
kind: [unclosed
EOF

if [ "$failures" -gt 0 ]; then
  echo "check-capture_test: $failures of $cases cases failed" >&2
  exit 1
fi
echo "check-capture_test: ok ($cases cases)"
