# 03-ui-canvas: the redesigned web UI, as drawn

Status: Snapshot (2026-10-05)

## What this is

The eight board sources of the design canvas the owner reviewed for the web UI redesign
(OpenSpec change `redesign-web-ui`). The canvas itself is private:
https://claude.ai/artifact/EcxyWsn4Bg5ynpkCKoBXof. These files are copied from it unchanged so
the design survives without access to the canvas.

| Board | View |
| --- | --- |
| [Main.dc.html](Main.dc.html) | Platform: identity, status, Installed counts, Providers and Catalogs tabs, recent events |
| [Instances.dc.html](Instances.dc.html) | Installed: instances and packages in one list, with filters |
| [Instance.dc.html](Instance.dc.html) | An instance (podinfo): summary blocks, Graph, Resources, Events, Logs, YAML |
| [Package.dc.html](Package.dc.html) | A package (podinfo) |
| [CertManager.dc.html](CertManager.dc.html) | A large inventory (cert-manager, 42 objects): grouped graph |
| [Provider.dc.html](Provider.dc.html) | A provider instance (backup-provider) and its Provider tab |
| [ProviderPackage.dc.html](ProviderPackage.dc.html) | A package that ships a TransformerRegistration, refused |
| [Catalog.dc.html](Catalog.dc.html) | One catalog |

## Read these with care

- **Mock data.** Every name, count, version, message and timestamp on the boards is invented to
  show a layout. None of it is a measurement, and no portal decision may cite a board as
  evidence of what the controller writes; the controller's output is evidenced in
  [01-live-cluster-capture](../01-live-cluster-capture/) and in the controller source.
- **Not the shipped UI.** The boards show more than the change builds. What ships, and what was
  left out and why, is in the change's `proposal.md` and `design.md` and in
  [DESIGN.md](../../../DESIGN.md) D14 to D18. Notably the boards draw catalog definitions,
  descriptions, transformers and a Platform health block, which have no data source yet.
- **Not served by the portal, and not loadable as they are.** The files are the canvas's own
  format: they load a `support.js` runtime that is not copied here and fonts from Google Fonts,
  and they use inline style and script. The portal's pages load nothing from outside the binary
  and run under a strict Content-Security-Policy, so no markup or script here may be copied into
  `internal/ui`.
