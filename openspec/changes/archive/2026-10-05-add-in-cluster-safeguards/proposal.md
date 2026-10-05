## Why

Two questions of enhancement 0030 must be settled before the portal runs in-cluster, where one
process serves many users:

- **0030:OQ8.** The operator's embedded kernel does not redact secret values in condition or
  event messages (no redaction code in the library's `main`). Local mode shows them, because the
  user's kubeconfig reads the same text with `kubectl`. In-cluster, the portal would show text
  the kernel may have copied a secret value into. The owner answered: in-cluster mode shows only
  reasons, never message text, until the kernel redacts.
- **0030:OQ20.** Change detection is per topic. When something a subscriber cannot see changes,
  the topic is rendered again for every subscriber and each one receives an event, so a
  subscriber learns when a hidden change happened (the content never leaks). The supervisor
  ruled: compare each subscriber's rendered document with the last one sent to that subscriber
  and send only on a difference, in every mode.

## What Changes

- **Stream, every mode.** The broker remembers, per subscription, the document it last wrote
  (an `upsert`, `delete` or `k8sevent` item, or a snapshot's only item) and writes a later item
  only when it differs. An unchanged re-render (a periodic refresh, or a change the subscriber
  cannot see) writes nothing, takes no event id, and so leaves no gap and no timing signal. A
  reconnect forgets the remembered document, so its first item is always written. Log lines are
  records, not documents, and are never compared.
- **Server mode.** `api.Config` gains a required `Mode`, `local` or `in-cluster`; `New` refuses
  any other value. `opm-portal serve` passes `local`.
- **In-cluster omits operator text.** In `in-cluster` mode every document, served by `GET` or on
  the stream, omits: condition messages (`conditions[]`, `reconcile.notes[]`), the reconcile
  message, history messages, a registration's `message` and `activeMessage`, every event
  `note`, the health message of an inventory object or graph node of an OPM kind (kstatus
  copies its `Ready` message), and, on an `Object` of an OPM kind, `status.conditions[].message`
  and `status.history[].message`. Reasons, states, `tone`, `meaning` and `nextStep` stay. The
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
- Enhancement link: carries the answers to 0030:OQ8 (owner) and 0030:OQ20 (supervisor ruling).
  Neither is yet recorded as a decision in 0030, so `enhancement.yaml` claims no decision.
