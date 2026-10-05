## 1. Remove the launch page once its token is spent (internal/auth, cmd/opm-portal)

- [x] 1.1 `internal/auth`: `Local.Launched()` closes when the token is spent; test it closes on the first launch only and not on a refused one
- [x] 1.2 `cmd/opm-portal`: `openLaunch`'s cleanup runs once; `runLocal` removes the page when `Launched` closes, and at shutdown otherwise; test the page is gone after a launch while the portal serves
- [x] 1.3 `task check` green, then commit `fix(auth): remove the launch page once its token is spent`

## 2. Bound re-validation by time and pin the write guard (internal/stream, internal/authz)

- [x] 2.1 `internal/stream`: `Options.RevalidateTimeout` (default 30 s); `revalidate` runs under that deadline, `gateTopic` and `recheckParts` start no review after it; drop `revalidateRounds`
- [x] 2.2 `TestAMessageWhoseGrantsNeverSettleClosesTheTopic` asserts exactly 17 reviews
- [x] 2.3 `TestOnlySendWritesTopicData` pins every argument of each control event write and the bodies they carry
- [x] 2.4 Docs: the revocation bound as one decision lifetime plus one review (about 35 s) in `internal/authz` (`Check`, package doc) and `internal/stream` (package doc, `revalidate`)
- [x] 2.5 `task check` green, then commit `fix(stream): bound revalidation by one decision lifetime`

## 3. End a stream at session expiry (internal/stream, internal/auth, internal/api, internal/ui, cmd/opm-portal)

- [x] 3.1 `internal/stream`: `Session.Expires`; `Open` refuses an expired session; the writer ends the stream at expiry with an `expired` event and the stream is discarded; tests for both, and the guard covers the new control event
- [x] 3.2 `internal/auth`: `Authenticate` returns `Session{Identity, Key, Expires}`; `internal/api`: `Principal.Expires` reaches the stream; `cmd/opm-portal` wiring
- [x] 3.3 `internal/ui`: the layout closes the `EventSource` on `expired` and the script shows "session expired, reload"; `TestBrowserExpired` (Chromium, Firefox, WebKit) checks the text and that the page does not reconnect
- [x] 3.4 `openapi/v1alpha1.yaml` and `docs/site` name the `expired` event
- [x] 3.5 `task check`, `task docs:bundle:check`, `task test:browser` and `task e2e:local` (throwaway cluster `opm-portal-e2e-h2`, deleted after) green, then commit `fix(stream): end a stream when its session expires`
