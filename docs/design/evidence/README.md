# Design evidence

The measurements and research that [DESIGN.md](../../DESIGN.md) cites. They were gathered on
2026-10-04 for enhancement 0030 and moved here unchanged in substance when that entry was withdrawn
on 2026-10-05. Each file is a snapshot: new evidence goes in a new directory or file, never an edit
to an old one.

| Evidence | What it holds | Status |
| --- | --- | --- |
| [01-live-cluster-capture](01-live-cluster-capture/) | Live status, inventory, label and event shapes of a released operator, trimmed to secret-free samples: healthy, apply-failed, image-broken, CLI-owned, refused, accepted, active and removal-blocked states. Fourteen observations. | Concluded |
| [02-live-graph-spike](02-live-graph-spike/) | A throwaway status-only graph prototype run against the same cluster: sizes, latency, health and contract findings. | Concluded |
| [prior-art-and-access.md](prior-art-and-access.md) | Prior art (portals and dashboards), the access and identity model, and the operator surface, read from primary sources. | Snapshot |

Every sample has `managedFields`, the `kubectl.kubernetes.io/last-applied-configuration`
annotation and `spec.values` removed. The files under `inputs/` are what the capture applied; none
holds Secret data. The full snapshots (about 12 MB) were never committed.
