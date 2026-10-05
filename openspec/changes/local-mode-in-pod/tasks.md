## 1. Design record and rules (docs/DESIGN.md, AGENTS.md, CONSTITUTION.md, openspec/config.yaml)

- [ ] 1.1 `docs/DESIGN.md`: add portal:D13 (Kind, Decision, Requirements R1 to R4, Alternatives, Rationale, Source); amend D5's decision text, R1 and alternatives in place to point at D13; update the numbering header and the Summary/Operations lines that say nothing is installed
- [ ] 1.2 `AGENTS.md` Security Rules, `CONSTITUTION.md` Principle V and `openspec/config.yaml` Principle V: one-line carve-out that a Pod running local mode reads as its configured ServiceAccount per portal:D13, and the no-fallback rule still binds in-cluster mode
- [ ] 1.3 `task check` green, then commit `docs(design): let local mode run in a Pod as a test tool (portal:D13)`

## 2. Name the in-cluster source at startup (cmd/opm-portal)

- [ ] 2.1 `cmd/opm-portal/serve.go`: `configSource(raw, contextName) slog.Attr`; `loadKubeconfig` returns it; the identity log line carries `source=in-cluster` when the raw config holds no context, `context=<name>` otherwise
- [ ] 2.2 `cmd/opm-portal/serve_test.go`: table test for `configSource` (no context → `source=in-cluster`; a named context → `context=<name>`; a current context in the raw config → that name)
- [ ] 2.3 `task check` green, then commit `feat(local): name the in-cluster config source in the startup log`

## 3. The manifest, its role and its tests (deploy/, release-please-config.json)

- [ ] 3.1 `deploy/`: `kustomization.yaml`, `namespace.yaml`, `serviceaccount.yaml`, `clusterrole.yaml` (the explicit rule list in design.md), `clusterrolebinding.yaml`, `deployment.yaml` (1 replica, `serve --addr 127.0.0.1:8090`, restricted security context, requests and limits, no probes, image line with the release-please marker naming `v` + `internal/version`)
- [ ] 3.2 `release-please-config.json`: add `{"type": "generic", "path": "deploy/deployment.yaml"}` to `extra-files`
- [ ] 3.3 `deploy/manifest_test.go`: parse every `deploy/*.yaml`; `checkRole`, `checkObjects`, `checkDeployment` over the shipped files and over a table of denied inputs that must each fail; the image tag equals `"v" + version.Version` on the marked line
- [ ] 3.4 `task check` green (and `kubectl kustomize deploy` renders), then commit `feat(deploy): ship a manifest that runs local mode in a Pod`

## 4. End to end on the fixture cluster (test/e2e, cmd/opm-portal, Taskfile.yml, .github/workflows/e2e.yml)

- [ ] 4.1 `test/e2e/pod/kustomization.yaml`: overlay of `../../../deploy` with image `localhost/opm-portal:e2e` and `imagePullPolicy: Never`
- [ ] 4.2 `test/e2e/pod.sh` and `task e2e:pod`: build the image with `$E2E_PROVIDER`, save it, `kind load image-archive`, apply the overlay, wait for the rollout, run `TestPod`
- [ ] 4.3 `cmd/opm-portal/pod_e2e_test.go` (build tag `e2e`): the Pod log names the ServiceAccount with `source=in-cluster` and holds the launch URL; `kubectl port-forward 8090:8090` launches and lists the same instances as the cluster; a launch through `8091:8090` gets `403`
- [ ] 4.4 `.github/workflows/e2e.yml`: run `task e2e:pod` after `e2e:m1`; `actionlint` green
- [ ] 4.5 Run against a throwaway cluster (`task e2e:up`, `task e2e:pod`, `task e2e:down`)
- [ ] 4.6 `task check` green, then commit `test(e2e): run the portal image in a Pod on the fixture cluster`

## 5. Docs and roadmap (README.md, AGENTS.md, ROADMAP.md)

- [ ] 5.1 README "Try it in a cluster": apply from a release tag, read the token from the log, `port-forward 8090:8090`, open the link, rollout restart for a new token, the trust model in two sentences, delete
- [ ] 5.2 `AGENTS.md`: `deploy/` in the layout, `task e2e:pod` in the commands and the nightly list
- [ ] 5.3 `ROADMAP.md`: one line under Now for this change, and "Last updated"
- [ ] 5.4 `task check` green, then commit `docs: explain how to try the portal in a cluster`
