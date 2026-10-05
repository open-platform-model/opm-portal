# local-mode Specification

## Purpose
Defines how the portal runs on a user's machine against their kubeconfig: which address it
listens on, how a browser is admitted, which requests are refused before any cluster call, which
identity every read is made as, and what the process writes and how it stops; and how the same
mode runs in a Pod with in-cluster credentials.

## Requirements

### Requirement: Local mode listens on loopback only

`opm-portal serve` SHALL listen on the address `--addr` names, `127.0.0.1:7878` by default, so the
browser origin, and the theme and filters stored for it, stay the same across restarts. It SHALL
refuse, with exit code 2 and before it reads the kubeconfig, an address whose host is empty, a
name other than `localhost`, or an IP address outside the loopback ranges, and SHALL check the
bound address again after listening. It SHALL bind the address before it reads the kubeconfig or
makes any cluster call; when the address is in use it SHALL exit 1 with a message naming the
address and saying to pass `--addr` for another port. `--addr 127.0.0.1:0` SHALL still
pick a free port. Source: portal:D5:R2, portal:D14:R6.

#### Scenario: Default address

- **WHEN** a user runs `opm-portal serve` with no `--addr`
- **THEN** the portal listens on `127.0.0.1:7878` and the printed launch link names that port

#### Scenario: A non-loopback address

- **WHEN** a user runs `opm-portal serve --addr 0.0.0.0:8080`, `--addr :8080`,
  `--addr 192.168.1.10:8080` or `--addr example.com:8080`
- **THEN** standard error says the address is not loopback
- **AND** the exit code is 2 and the kubeconfig is not read

#### Scenario: The default port is taken

- **WHEN** another process listens on `127.0.0.1:7878` and a user runs `opm-portal serve` with no
  `--addr`
- **THEN** standard error says `127.0.0.1:7878` is in use and to pass `--addr` for another port
- **AND** the exit code is 1, no launch link is printed, and no request reaches the cluster

#### Scenario: The Pod keeps its own address

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090`
- **THEN** the portal listens on `127.0.0.1:8090`, not the default

### Requirement: Local mode reads as the kubeconfig's identity and fails closed

At startup the portal SHALL learn the kubeconfig's identity with one SelfSubjectReview and SHALL
refuse to start, with exit code 1 and before it listens for requests or reads anything else, when
that review fails or names an empty, blank or anonymous user. Every read SHALL be preceded by a
SelfSubjectAccessReview for that identity and the read's exact attributes, and every client SHALL
be built from the kubeconfig with no credential of the portal's own. In local mode the only
create requests the portal sends SHALL be `selfsubjectreviews` and `selfsubjectaccessreviews`.
Source: portal:D5:R1/R6/R7, portal:D6:R2.

#### Scenario: An anonymous kubeconfig

- **WHEN** the kubeconfig's SelfSubjectReview answers with username `system:anonymous` or an
  empty username
- **THEN** the portal exits 1 with a message that the kubeconfig names no user
- **AND** it never listens and sends no access review

#### Scenario: The fixture cluster's admin

- **WHEN** the portal starts with the kind fixture cluster's kubeconfig
- **THEN** standard error logs the username the SelfSubjectReview returned
- **AND** the instance list it serves holds the fixture cluster's instances

### Requirement: Namespaced reads can be limited for users without cluster-wide access

`--namespaces` SHALL limit the read model's namespaced OPM kinds to the listed namespaces, so a
user who may not list them cluster-wide can still read them in the namespaces they may. At
startup the portal SHALL log one warning on standard error for each OPM kind, and each namespace
`--namespaces` names, that the identity may not list and watch; a warning for a namespaced kind
read cluster-wide SHALL name `--namespaces`. Source: portal:D5:R5.

#### Scenario: A namespace-scoped user

- **WHEN** a user who may list ModuleInstances only in `team-a` runs
  `opm-portal serve --namespaces team-a`
- **THEN** the instance list of `team-a` is served

#### Scenario: A namespace-scoped user without --namespaces

- **WHEN** a user who may list ModuleInstances only in `team-a` runs `opm-portal serve`
- **THEN** standard error warns that the user may not list and watch `moduleinstances`
  cluster-wide and names `--namespaces`
- **AND** the cluster-wide instance list answers with access `forbidden`

### Requirement: A browser is admitted only through a one-time launch token

At startup the portal SHALL print one launch URL on standard output carrying a random token, and
SHALL write the token nowhere else except the private launch page `--open` writes (mode `0600`
in a directory only the user may enter). The portal SHALL remove that page and its directory as
soon as the token is spent, through either the page or the printed link, and at shutdown when
the token was never spent. A `GET /launch` carrying that token SHALL spend it, set a session
cookie and answer with the UI's landing page itself, rendered under the new session, whose script
then moves once, from the portal's own origin, to the landing page's address, so a reload carries
the session and the spent token leaves the history. A `GET /launch` from a browser that already
holds the live session SHALL answer the same way, whatever its token. It SHALL NOT redirect: a
browser treats a redirect as part of the navigation that reached `/launch`, and when that
navigation started from the `--open` page (a `file://` document) it withholds the new
`SameSite=Strict` cookie from the landing request. A missing, wrong or already-spent token SHALL
be refused with `403` and the same body for each. A request that does not carry a live session
SHALL be refused before any authorization review or read: the read API answers it `401` with code
`unauthenticated`. `--open` SHALL open the launch URL in the default browser without putting the
token on any process's command line. Source: portal:D5:R3.

#### Scenario: Launch and read

- **WHEN** a browser opens the printed launch URL and then requests
  `/api/v1alpha1/clusters/default/instances`
- **THEN** the launch answers `200` with a session cookie and the landing page
- **AND** the instance list is served with that cookie

#### Scenario: Launch from the --open page

- **WHEN** Chromium, Firefox or WebKit opens the `file://` page `--open` writes
- **THEN** the browser ends on the landing page with the session
- **AND** a reload of that page still carries the session

#### Scenario: The launch page is removed once the token is spent

- **WHEN** `--open` wrote the launch page and a browser then spends the token
- **THEN** the page and its directory are gone while the portal keeps serving

#### Scenario: A request without the session

- **WHEN** a client requests `/api/v1alpha1/clusters/default/instances` with no cookie, or a
  cookie that is not the session's
- **THEN** the response is `401` with code `unauthenticated`
- **AND** no access review is sent

#### Scenario: The token is used twice

- **WHEN** a second client without the session opens the launch URL after the first one did
- **THEN** the response is `403` and no cookie is set

### Requirement: The session cookie is host-only, HttpOnly and SameSite=Strict

The session cookie SHALL be named `opm-portal-<port>`, SHALL carry `Path=/`, `HttpOnly`,
`SameSite=Strict` and a `Max-Age` equal to the session's lifetime, and SHALL carry no `Domain`.
Local mode serves plain HTTP, so the cookie SHALL carry no `Secure` flag and no `__Host-` prefix.
Only a digest of the cookie value SHALL be kept, and neither the value nor the launch token SHALL
appear in a log line, an error or an API response.

#### Scenario: Cookie flags

- **WHEN** a launch succeeds on port 8123
- **THEN** the `Set-Cookie` header names `opm-portal-8123` with `Path=/`, `HttpOnly`,
  `SameSite=Strict` and `Max-Age`, and no `Domain` or `Secure`

### Requirement: Requests for another host are refused

The portal SHALL refuse with `403`, before any other step, every request whose `Host` header is
not exactly `127.0.0.1:<port>`, `localhost:<port>`, `[::1]:<port>` or `<ip>:<port>` for the
loopback IP `<ip>` and the port it listens on. Source: portal:D5:R4.

#### Scenario: DNS rebinding

- **WHEN** a request carrying a valid session cookie arrives with `Host: attacker.example:8123`
- **THEN** the response is `403`
- **AND** no access review is sent

#### Scenario: Loopback names

- **WHEN** requests arrive with `Host` `127.0.0.1:8123`, `localhost:8123` and `[::1]:8123`
- **THEN** each passes the host check

#### Scenario: The loopback IP --addr names

- **WHEN** the portal listens on `127.0.0.2:8123` and a request arrives with `Host`
  `127.0.0.2:8123`
- **THEN** it passes the host check
- **AND** a request with `Host` `127.0.0.3:8123` is refused with `403`

### Requirement: Cross-origin writes are refused

A request with a method other than `GET`, `HEAD` or `OPTIONS` that the browser marks as coming
from another origin (`Sec-Fetch-Site` or `Origin`) SHALL be refused with `403` before it reaches
the read API.

#### Scenario: A cross-site POST

- **WHEN** a `POST` arrives with `Sec-Fetch-Site: cross-site`
- **THEN** the response is `403`

#### Scenario: A same-origin POST

- **WHEN** a `POST` arrives with `Sec-Fetch-Site: same-origin` and the session
- **THEN** the read API answers it `405` with code `method_not_allowed`

### Requirement: Every response carries the security headers

Every response, refusals included, SHALL carry
`Content-Security-Policy: default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`,
`X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`,
`Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Resource-Policy: same-origin`. A UI
page SHALL replace only the Content-Security-Policy, with the page policy the `web-ui`
capability states, which allows nothing beyond `'self'`.

#### Scenario: A refused host

- **WHEN** a request is refused for its `Host`
- **THEN** the refusal carries every security header

#### Scenario: A page

- **WHEN** the landing page is served
- **THEN** it carries the page policy and every other security header unchanged

### Requirement: Local mode stops cleanly on a signal

On `SIGINT` or `SIGTERM` the portal SHALL end every open stream, stop accepting requests, finish
the requests in flight within five seconds, stop its watches, remove any file `--open` wrote, and
exit 0.

#### Scenario: Interrupt with an open stream

- **WHEN** a client holds an open stream and the portal receives `SIGINT`
- **THEN** the stream ends and the process exits 0 within five seconds

### Requirement: Local mode runs unchanged in a Pod with in-cluster credentials

When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration, the
portal SHALL run local mode unchanged: it SHALL bind only the loopback address `--addr` names,
SHALL learn its identity, the Pod's ServiceAccount, with one SelfSubjectReview and fail closed on
an empty or anonymous one, SHALL print the launch URL on standard output, which becomes the
container log, and SHALL apply the launch token, the session cookie, the `Host` allowlist, the
cross-origin refusal and the security headers exactly as on a user's machine. Every read SHALL be
made as that ServiceAccount after a SelfSubjectAccessReview for it. The startup log line naming
the identity SHALL carry `source=in-cluster` in place of an empty context name. A browser reaches
the portal only through `kubectl port-forward` to the bound port, and the `Host` allowlist SHALL
admit only the bound port, so a forward from any other local port is refused. Source:
portal:D13:R1/R2, portal:D5:R3/R4.

#### Scenario: The Pod starts in local mode

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090` with no
  kubeconfig in the Pod
- **THEN** the container log names the user `system:serviceaccount:opm-portal:opm-portal` with
  `source=in-cluster`
- **AND** the container log holds one `Open this link once to sign in:
  http://127.0.0.1:8090/launch?token=...` line

#### Scenario: Port-forward to the bound port

- **WHEN** a user runs `kubectl -n opm-portal port-forward deploy/opm-portal 8090:8090` and opens
  the launch URL from the Pod log
- **THEN** the launch answers `200` with a session cookie
- **AND** `/api/v1alpha1/clusters/default/instances` lists the instances the ServiceAccount may
  read

#### Scenario: Port-forward from another local port

- **WHEN** a user forwards local port 8091 to the Pod's port 8090 and requests
  `http://127.0.0.1:8091/launch?token=...`
- **THEN** the response is `403`, the token is not spent, and no cookie is set

#### Scenario: The token is spent

- **WHEN** a second browser opens the launch URL after the first one did
- **THEN** the response is `403`
- **AND** after `kubectl -n opm-portal rollout restart deploy/opm-portal` the new Pod's log holds
  a new launch URL that admits a browser

#### Scenario: An identity the cluster does not name

- **WHEN** the in-cluster SelfSubjectReview fails or names no user
- **THEN** the container exits 1 before it listens, and the Pod restarts without ever printing a
  launch URL

### Requirement: Local mode names its connection to the read API

`opm-portal serve` SHALL give the read API the name of the kubeconfig context it loaded and the
name of that context's cluster entry, or, when client-go fell back to the in-cluster
configuration, the `in-cluster` source with no context. It SHALL pass no server URL, user entry or
credential. Source: portal:D18:R1.

#### Scenario: A named context

- **WHEN** `opm-portal serve --context kind-opm-portal-e2e` starts
- **THEN** the read API's `Cluster` document names context `kind-opm-portal-e2e` and that
  context's cluster entry

#### Scenario: In a Pod

- **WHEN** the shipped Deployment starts with no kubeconfig
- **THEN** the `Cluster` document has `source: in-cluster` and no context
