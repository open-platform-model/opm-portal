## Purpose

Defines how the portal runs on a user's machine against their kubeconfig: which address it
listens on, how a browser is admitted, which requests are refused before any cluster call, which
identity every read is made as, and what the process writes and how it stops.

## ADDED Requirements

### Requirement: Local mode listens on loopback only

`opm-portal serve` SHALL listen on the address `--addr` names, `127.0.0.1:0` by default. It SHALL
refuse, with exit code 2 and before it reads the kubeconfig, an address whose host is empty, a
name other than `localhost`, or an IP address outside the loopback ranges, and SHALL check the
bound address again after listening. Source: 0030:D5:R2.

#### Scenario: Default address

- **WHEN** a user runs `opm-portal serve` with no `--addr`
- **THEN** the portal listens on a free port of `127.0.0.1`

#### Scenario: A non-loopback address

- **WHEN** a user runs `opm-portal serve --addr 0.0.0.0:8080`, `--addr :8080`,
  `--addr 192.168.1.10:8080` or `--addr example.com:8080`
- **THEN** standard error says the address is not loopback
- **AND** the exit code is 2 and the kubeconfig is not read

### Requirement: Local mode reads as the kubeconfig's identity and fails closed

At startup the portal SHALL learn the kubeconfig's identity with one SelfSubjectReview and SHALL
refuse to start, with exit code 1 and before it listens for requests or reads anything else, when
that review fails or names an empty, blank or anonymous user. Every read SHALL be preceded by a
SelfSubjectAccessReview for that identity and the read's exact attributes, and every client SHALL
be built from the kubeconfig with no credential of the portal's own. In local mode the only
create requests the portal sends SHALL be `selfsubjectreviews` and `selfsubjectaccessreviews`.
Source: 0030:D5:R1/R6/R7, 0030:D6:R2.

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
user who may not list them cluster-wide can still read them in the namespaces they may.
Source: 0030:D5:R5.

#### Scenario: A namespace-scoped user

- **WHEN** a user who may list ModuleInstances only in `team-a` runs
  `opm-portal serve --namespaces team-a`
- **THEN** the instance list of `team-a` is served

### Requirement: A browser is admitted only through a one-time launch token

At startup the portal SHALL print one launch URL on standard output carrying a random token, and
SHALL write the token nowhere else except the private launch page `--open` writes (mode `0600`
in a directory only the user may enter), which shutdown removes. A `GET /launch` carrying that
token SHALL spend it, set a session cookie and answer `200` with a page, naming no token, that
moves the browser on to the landing page through a meta refresh and a link. It SHALL NOT
redirect: a browser treats a redirect as part of the navigation that reached `/launch`, and when
that navigation started from the `--open` page (a `file://` document) it withholds the new
`SameSite=Strict` cookie from the landing request. A missing, wrong or already-spent token SHALL
be refused with `403` and the same body for each. A request that does not carry a live session
SHALL be refused before any authorization review or read: the read API answers it `401` with
code `unauthenticated`. `--open` SHALL open the launch URL in the default browser without putting
the token on any process's command line. Source: 0030:D5:R3.

#### Scenario: Launch and read

- **WHEN** a browser opens the printed launch URL and then requests
  `/api/v1alpha1/clusters/default/instances`
- **THEN** the launch answers `200` with a session cookie and a page that refreshes to the
  landing page
- **AND** the instance list is served with that cookie

#### Scenario: Launch from the --open page

- **WHEN** Chromium, Firefox or WebKit opens the `file://` page `--open` writes
- **THEN** the browser ends on the landing page with the session
- **AND** a reload of that page still carries the session

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
loopback IP `<ip>` and the port it listens on. Source: 0030:D5:R4.

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
`Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Resource-Policy: same-origin`.

#### Scenario: A refused host

- **WHEN** a request is refused for its `Host`
- **THEN** the refusal carries every security header

### Requirement: Local mode stops cleanly on a signal

On `SIGINT` or `SIGTERM` the portal SHALL end every open stream, stop accepting requests, finish
the requests in flight within five seconds, stop its watches, remove any file `--open` wrote, and
exit 0.

#### Scenario: Interrupt with an open stream

- **WHEN** a client holds an open stream and the portal receives `SIGINT`
- **THEN** the stream ends and the process exits 0 within five seconds
