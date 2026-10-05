## Why

Two open questions of the portal's design record (`docs/DESIGN.md`) must be settled before the
portal runs in-cluster, where one process serves many users:

- **portal:OQ8.** The operator's embedded kernel does not redact secret values in condition or
  event messages (no redaction code in the library's `main`). Local mode shows them, because the
  user's kubeconfig reads the same text with `kubectl`. In-cluster, the portal would show text
  the kernel may have copied a secret value into. The owner answered: in-cluster mode shows only
  reasons, never message text, until the kernel redacts. The question was about the operator's
  messages, so the supervisor ruled its scope: the kubelet's and other controllers' event notes,
  and rendered workloads' health messages, stay visible.
- **portal:OQ20.** Change detection is per topic. When something a subscriber cannot see changes,
  the topic is rendered again for every subscriber and each one receives an event, so a
  subscriber learns when a hidden change happened (the content never leaks). The supervisor
  ruled: compare each subscriber's rendered document with the last one sent to that subscriber
  and send only on a difference, in every mode.

## What Changes

- **Stream, every mode.** The broker remembers, per subscription, the document it last wrote
  (an `upsert`, `delete` or `k8sevent` item, or a snapshot's only item) and writes a later
  `upsert` or `k8sevent` item only when it differs; a `delete`, a snapshot and a closing are
  always written. An unchanged re-render (a periodic refresh, or a change the subscriber
  cannot see) writes nothing, takes no event id, and so leaves no gap and no timing signal. A
  reconnect keeps the remembered document when the client's `Last-Event-ID` shows it holds it, and
  otherwise forgets it, so the first item is written; a reconnect that falls back to a snapshot
  compares later items with it. Log lines are
  records, not documents, and are never compared.
- **Server mode.** `api.Config` gains a required `Mode`, `local` or `in-cluster`; `New` refuses
  any other value. `opm-portal serve` passes `local`.
- **In-cluster omits operator text.** In `in-cluster` mode every document, served by `GET` or on
  the stream, omits: condition messages (`conditions[]`, `reconcile.notes[]`), the reconcile
  message, history messages, a registration's `message` and `activeMessage`, the `note` of every
  event the operator reported, the health message of an inventory object or graph node of an OPM kind (kstatus
  copies its `Ready` message), and, on an `Object` of an OPM kind, `status.conditions[].message`
  and `status.history[].message`. Reasons, states, `tone`, `meaning` and `nextStep` stay, and so
  do the kubelet's and other controllers' event notes and rendered workloads' health messages. The
  pages read the API, so they show none of it either.
- **OpenAPI.** Each omitted field's description says it is absent in-cluster. Every one is
  already optional, so the contract change is descriptive only.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `change-stream`: a subscriber is written only a change to its own rendered document.
- `read-api`: the server runs in a declared mode; in-cluster it omits operator text.
- `web-ui`: in-cluster pages show no operator text.
- `portal-docs`: the security page and the read API reference say messages are shown as written
  in local mode only.

## Impact

- Packages: `internal/stream` (per-subscription comparison), `internal/api` (mode, omission, OpenAPI
  descriptions, goldens for both modes), `cmd/opm-portal` (passes `local`), `internal/ui` (goldens
  over an in-cluster server), `docs/site/`.
- API: additive (descriptions only; no field becomes required or changes type).
  `task api:breaking` stays green.
- Principle V: strengthened. No new read, no new verb, no new identity. In-cluster mode serves
  less; local mode serves what it did.
- Principle VII: the comparison keeps one byte slice per subscription, reset on reconnect; the
  omission is one function over the wire documents, applied where a document leaves the server.
- SemVer: MINOR after 1.0 (new behavior, additive contract); on the 0.x line a `feat`.
- Design record: carries the answers to portal:OQ8 (owner; portal:D8:R5/R6, scope by supervisor
  ruling under portal:D8) and portal:OQ20 (supervisor ruling; portal:D2:R7). Enhancement 0030 was
  withdrawn, so `enhancement.yaml` claims no decision.
