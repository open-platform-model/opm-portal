# Fixture cluster, captures and goldens

Covers the throwaway kind fixture cluster, the committed cluster captures and the golden files.

## The fixture cluster (`test/e2e/`)

`task e2e:up` creates the kind cluster `opm-portal-e2e` with the released operator and fixture set F1. `task e2e:down` deletes it.

- The provider is podman by default. Set `E2E_PROVIDER=docker` to use docker. `e2e:up` records the choice in `.e2e/provider`, and capture and down read it from there.
- Needs kubectl, curl and the kind release that `test/e2e/versions.env` pins.
- The kubeconfig is `.e2e/kubeconfig`. The scripts never use another context.
- `E2E_CLUSTER=opm-portal-e2e-<suffix>` runs a second cluster. Its state is in `.e2e/clusters/<name>/`.
- One F1 fixture, the refused claim, is refused on purpose. Do not "fix" it.

Tests that run against the cluster (all need `task e2e:up` first, all use build tag `e2e`):

| Task | What it runs | What it proves |
| --- | --- | --- |
| `task e2e:local` | Builds the binary, runs the local-mode test | Launch, reads with and without the session, a foreign `Host`, a cross-site `POST`, a pod log on the stream, a clean `SIGINT` |
| `task e2e:m1` | Builds the binary, runs the milestone 1 tests | The milestone 1 views through the read API and the stream: both claims, Applied and Health apart, the CLI-owned instance, a namespace-scoped reader with locked objects, and a scripted image break that must read Degraded within 10 s of the cluster while Applied stays |
| `task e2e:pod` | Builds the image with the e2e provider, loads it into the cluster, applies `deploy/` through the `test/e2e/pod/` overlay (local image, `imagePullPolicy: Never`), runs the Pod test | The Pod log names the ServiceAccount, a launch through `kubectl port-forward 8090:8090` lists the instances, a forward from 8091 is refused |
| `task e2e:dump` | Dumps OPM objects (no values), Pods, events and the operator log | Reading a failed run. `DIR=<directory>` sets the target; the default is `.e2e/dump/<cluster>` |

`e2e:m1` patches podinfo's image tag and reverts it before it ends. `e2e:pod` needs local ports 8090 and 8091 free.

The nightly `E2E` workflow runs `e2e:up`, `e2e:capture`, `e2e:local`, `e2e:m1` and `e2e:pod` on docker kind. It runs `test:browser` in a parallel `Browser` job. On failure it uploads the test logs, the cluster dump and the browser screenshots. Dispatch it on a branch with `gh workflow run` on the E2E workflow with `--ref <branch>`.

## Captures (`testdata/clusters/`)

`task e2e:capture` snapshots the cluster into `testdata/clusters/f1/`. It needs yq and jq. The files are generated: never hand-edit one.

`task e2e:capture:check` first tests its own checker, then checks the captures. It refuses any file under `testdata/clusters/`, at any depth, that holds one of these, or that does not parse:

- a Secret,
- `managedFields`,
- the `kubectl.kubernetes.io/last-applied-configuration` annotation,
- `spec.values`.

The rule exists because a capture is committed and public. `task check` runs this gate. Moving a pin in `test/e2e/versions.env` means you must recapture.

## Golden files

| Suite | Update command | Directory |
| --- | --- | --- |
| Read API | `go test ./internal/api -update` | `internal/api/testdata/golden/` |
| UI | `go test ./internal/ui -update` | `internal/ui/testdata/golden/` (each page's `<main>`, or a fragment) |

Read the diff before you commit. A golden that changes without a reason in your change is a regression, not an update.

## Pitfalls

- Running `e2e:m1`, `e2e:local` or `e2e:pod` before `e2e:up`: they need the fixture cluster. Run `e2e:up` first.
- Committing a capture after a pin move without recapturing: the nightly run compares the verdicts and versions in the capture metadata with the committed capture and fails on a difference.
- Updating goldens with `-update` and committing without reading the diff.
- Linting only untagged code. Lint the tagged tests too: `golangci-lint run --build-tags e2e,browser ./cmd/...`. `task vet` already builds them with `-tags e2e,browser`.

## Check before you finish

- [ ] `task e2e:capture:check` passes after any change under `testdata/clusters/`.
- [ ] You read the diff of every golden file you updated.
- [ ] You ran `task e2e:down` for any cluster you started, including a suffixed one.
- [ ] A new fixture is a capture of the cluster, not hand-written YAML.
