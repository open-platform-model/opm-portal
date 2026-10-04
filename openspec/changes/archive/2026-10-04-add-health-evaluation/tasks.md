## 1. Object health

- [x] 1.1 Copy the captured objects into `internal/health/testdata/` (podinfo phases 3, 4 and 5; cert-manager phase 3; the CLI-owned web instance; the experiment 01 operator samples; the cert-manager ModuleInstance and the Flux-less ModulePackage), with `managedFields`, annotations, `spec.values` and CRD schemas removed; verify `grep` finds no `kind: Secret`, no `caBundle`, no `values:` and no `last-applied` in the directory
- [x] 1.2 Add `github.com/fluxcd/cli-utils` at the version opm-operator's `go.mod` pins (v1.2.2) and `k8s.io/apimachinery`; verify `go mod tidy` is stable
- [x] 1.3 Add `internal/health` with `State`, `ObjectHealth` and `Object(u)`: kstatus mapping, a kstatus error as Unknown, and the Pod rule for the five waiting reasons over containers and init containers; verify with table tests over every captured object (healthy phase 3 and cert-manager objects Healthy, phase 4 Deployment Progressing, phase 4 broken Pod Degraded with `ImagePullBackOff`, phase 5 Deployment Degraded) and synthetic Pods for each reason
- [x] 1.4 `task check` green, then commit `chore(health): evaluate object health with kstatus and the pod rule`

## 2. Roll-up

- [x] 2.1 Add `Ref`, `Access`, `Entry`, `Input`, `Result`, `Summary`, `Counts` and `Evaluate(in)`: per-entry health (absent object Missing, Secret withheld whatever was passed, non-ok access excluded), worst-of per component and instance, `Partial`, oldest `EvaluatedAt`, `Live`
- [x] 2.2 Propagate the Pod rule: walk each degraded Pod's controller owner chain by UID through children and inventory objects (eight hops at most) and mark the inventory object reached Degraded; verify with the phase 4 capture (instance Degraded within the same evaluation, Deployment reason `ImagePullBackOff`) and a Pod whose chain reaches nothing
- [x] 2.3 Verify the roll-up with tests built from captured inventories: cert-manager (42 entries, Healthy), podinfo phases 3/4/5 (Healthy, Degraded, Degraded), the CLI-owned web instance (Healthy), plus forbidden, not readable, Secret, empty inventory, missing object and polled-entry cases
- [x] 2.4 `task check` green, then commit `chore(health): roll object health up to components and the instance`

## 3. Applied axis and registration verdicts

- [x] 3.1 Add `AppliedState`, `Applied`, `Note` and `ReadApplied(u)` with the first-match order of the design; verify against every captured ModuleInstance, ModulePackage and Platform sample (podinfo healthy and broken Applied, cert-manager apply-failed Failed and retrying, CLI-owned ManagedExternally, Flux-less package Failed `SourceNotReady`, fresh Platform Applied with an `UnfulfilledContracts` note), plus synthetic Stalled, Suspended, Reconciling, Drifted and unknown-kind cases, and a test that changing `failureCounters` changes nothing
- [x] 3.2 Add `Verdict`, `Registration` and `ReadRegistration(u)`; verify against every captured TransformerRegistration (refusals Refused, rendered beta.5 claim Refused `CatalogUnresolved`, accepted-active Accepted, removal-blocked RemovalBlocked with accepted and active true)
- [x] 3.3 `task check` green, then commit `chore(health): read the applied axis and registration verdicts`

## 4. Reason explanations

- [x] 4.1 Add `Explanation` and `Explain(reason)` with a row for every condition reason the operator writes (meaning, and a next step where a person can act); no row cites an enhancement
- [x] 4.2 Add the copied operator reason list with a comment pointing at opm-operator `internal/status/conditions.go`, and a test that fails on a reason without a row or a row without a reason; add a test, skipped unless `OPM_OPERATOR_SRC` names an operator checkout, that parses the `*Reason` constants from that file and compares them with the copy; verify both pass, the second with `OPM_OPERATOR_SRC` pointing at the local opm-operator checkout
- [x] 4.3 `task check` green, then commit `chore(health): explain every operator condition reason`

## 5. Review fixes

- [x] 5.1 Add `Input.ChildrenAccess`: when the children could not be read, skip the Pod rule, mark readable workloads that can own Pods `ChildrenUnread`, and make their component and the instance partial; verify with the phase 4 capture and the children forbidden, not readable and unset
- [x] 5.2 Follow only owner references marked controller; verify a Pod with a plain owner reference to the inventory Deployment changes nothing
- [x] 5.3 `task check` green, then commit `chore(health): mark health partial when children are unread`
- [x] 5.4 Read a `Ready=True` whose `observedGeneration` is older than the object's generation as Reconciling; verify with the podinfo capture one generation ahead and a condition without `observedGeneration`
- [x] 5.5 Pin the applied state of the refused and removal-blocked registrations (both Stalled) and state that the verdict decides how a registration is shown; verify with both captures
- [x] 5.6 `task check` green, then commit `chore(health): read a stale ready as reconciling`
