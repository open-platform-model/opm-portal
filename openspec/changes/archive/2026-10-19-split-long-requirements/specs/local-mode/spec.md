## REMOVED Requirements

### Requirement: Local mode listens on loopback only
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Local mode listens on the address --addr names, 127.0.0.1:7878 by default", "Local mode refuses an address that is not loopback", "Local mode binds the address before it reads the kubeconfig".

### Requirement: Local mode reads as the kubeconfig's identity and fails closed
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Local mode learns the kubeconfig's identity at startup and reads only after a review", "Local mode creates only two kinds of review".

### Requirement: A browser is admitted only through a one-time launch token
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The portal prints one launch URL at startup and removes the launch page when the token is spent", "A GET /launch spends the token", "A GET /launch from a browser that holds the live session answers the same way and does not redirect", "A missing, wrong or spent token is refused with the same body", "A request without a live session is refused before any review or read", "--open opens the launch URL without the token on a command line".

### Requirement: Local mode runs unchanged in a Pod with in-cluster credentials
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "In a Pod local mode binds loopback and fails closed on an empty or anonymous identity", "In a Pod local mode prints the launch URL and applies the launch token and the browser protections", "In a Pod local mode reads as the ServiceAccount and logs source in-cluster", "In a Pod local mode is reached only through port-forward to the bound port".

## ADDED Requirements

### Requirement: Local mode listens on the address --addr names, 127.0.0.1:7878 by default
`opm-portal serve` SHALL listen on the address `--addr` names, `127.0.0.1:7878` by default, so the
browser origin, and the theme and filters stored for it, stay the same across restarts. `--addr
127.0.0.1:0` SHALL still pick a free port. Source: portal:D5:R2, portal:D14:R6.

#### Scenario: Default address

- **WHEN** a user runs `opm-portal serve` with no `--addr`
- **THEN** the portal listens on `127.0.0.1:7878` and the printed launch link names that port

#### Scenario: The Pod keeps its own address

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090`
- **THEN** the portal listens on `127.0.0.1:8090`, not the default

### Requirement: Local mode refuses an address that is not loopback
`opm-portal serve` SHALL refuse, with exit code 2 and before it reads the kubeconfig, an address
whose host is empty, a name other than `localhost`, or an IP address outside the loopback ranges,
and SHALL check the bound address again after listening. Source: portal:D5:R2, portal:D14:R6.

#### Scenario: A non-loopback address

- **WHEN** a user runs `opm-portal serve --addr 0.0.0.0:8080`, `--addr :8080`,
  `--addr 192.168.1.10:8080` or `--addr example.com:8080`
- **THEN** standard error says the address is not loopback
- **AND** the exit code is 2 and the kubeconfig is not read

### Requirement: Local mode binds the address before it reads the kubeconfig
`opm-portal serve` SHALL bind the address before it reads the kubeconfig or makes any cluster call;
when the address is in use it SHALL exit 1 with a message naming the address and saying to pass
`--addr` for another port. Source: portal:D5:R2, portal:D14:R6.

#### Scenario: The default port is taken

- **WHEN** another process listens on `127.0.0.1:7878` and a user runs `opm-portal serve` with no
  `--addr`
- **THEN** standard error says `127.0.0.1:7878` is in use and to pass `--addr` for another port
- **AND** the exit code is 1, no launch link is printed, and no request reaches the cluster

### Requirement: Local mode learns the kubeconfig's identity at startup and reads only after a review
At startup the portal SHALL learn the kubeconfig's identity with one SelfSubjectReview and SHALL
refuse to start, with exit code 1 and before it listens for requests or reads anything else, when
that review fails or names an empty, blank or anonymous user. Every read SHALL be preceded by a
SelfSubjectAccessReview for that identity and the read's exact attributes, and every client SHALL be
built from the kubeconfig with no credential of the portal's own. Source: portal:D5:R1/R6/R7,
portal:D6:R2.

#### Scenario: An anonymous kubeconfig

- **WHEN** the kubeconfig's SelfSubjectReview answers with username `system:anonymous` or an
  empty username
- **THEN** the portal exits 1 with a message that the kubeconfig names no user
- **AND** it never listens and sends no access review

#### Scenario: The fixture cluster's admin

- **WHEN** the portal starts with the kind fixture cluster's kubeconfig
- **THEN** standard error logs the username the SelfSubjectReview returned
- **AND** the instance list it serves holds the fixture cluster's instances

### Requirement: Local mode creates only two kinds of review
In local mode the only create requests the portal sends SHALL be `selfsubjectreviews` and
`selfsubjectaccessreviews`. Source: portal:D5:R1/R6/R7, portal:D6:R2.

#### Scenario: Only review objects are created

- **WHEN** the portal runs in local mode
- **THEN** the only create requests the portal sends SHALL be `selfsubjectreviews` and `selfsubjectaccessreviews`

### Requirement: The portal prints one launch URL at startup and removes the launch page when the token is spent
At startup the portal SHALL print one launch URL on standard output carrying a random token, and
SHALL write the token nowhere else except the private launch page `--open` writes (mode `0600` in a
directory only the user may enter). The portal SHALL remove that page and its directory as soon as
the token is spent, through either the page or the printed link, and at shutdown when the token was
never spent. Source: portal:D5:R3.

#### Scenario: The launch page is removed once the token is spent

- **WHEN** `--open` wrote the launch page and a browser then spends the token
- **THEN** the page and its directory are gone while the portal keeps serving

### Requirement: A GET /launch spends the token
At startup the portal SHALL print one launch URL on standard output carrying a random token. A `GET
/launch` carrying that token SHALL spend it, set a session cookie and answer with the UI's landing
page itself, rendered under the new session, whose script then moves once, from the portal's own
origin, to the landing page's address, so a reload carries the session and the spent token leaves
the history. Source: portal:D5:R3.

#### Scenario: Launch and read

- **WHEN** a browser opens the printed launch URL and then requests
  `/api/v1alpha1/clusters/default/instances`
- **THEN** the launch answers `200` with a session cookie and the landing page
- **AND** the instance list is served with that cookie

### Requirement: A GET /launch from a browser that holds the live session answers the same way and does not redirect
A `GET /launch` from a browser that already holds the live session SHALL answer the same way as
listed in requirement "A GET /launch spends the token", whatever its token. A `GET /launch` SHALL
NOT redirect: a browser treats a redirect as part of the navigation that reached `/launch`, and when
that navigation started from the `--open` page (a `file://` document) it withholds the new
`SameSite=Strict` cookie from the landing request. Source: portal:D5:R3.

#### Scenario: Launch from the --open page

- **WHEN** Chromium, Firefox or WebKit opens the `file://` page `--open` writes
- **THEN** the browser ends on the landing page with the session
- **AND** a reload of that page still carries the session

### Requirement: A missing, wrong or spent token is refused with the same body
A missing, wrong or already-spent token SHALL be refused with `403` and the same body for each.
Source: portal:D5:R3.

#### Scenario: The token is used twice

- **WHEN** a second client without the session opens the launch URL after the first one did
- **THEN** the response is `403` and no cookie is set

### Requirement: A request without a live session is refused before any review or read
A request that does not carry a live session SHALL be refused before any authorization review or
read: the read API answers it `401` with code `unauthenticated`. Source: portal:D5:R3.

#### Scenario: A request without the session

- **WHEN** a client requests `/api/v1alpha1/clusters/default/instances` with no cookie, or a
  cookie that is not the session's
- **THEN** the response is `401` with code `unauthenticated`
- **AND** no access review is sent

### Requirement: --open opens the launch URL without the token on a command line
`--open` SHALL open the launch URL in the default browser without putting the token on any process's
command line. Source: portal:D5:R3.

#### Scenario: The browser is opened with the launch URL

- **WHEN** `--open` is given
- **THEN** it SHALL open the launch URL in the default browser without putting the token on any process's command line

### Requirement: In a Pod local mode binds loopback and fails closed on an empty or anonymous identity
When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration, the
portal SHALL run local mode unchanged: it SHALL bind only the loopback address `--addr` names, SHALL
learn its identity, the Pod's ServiceAccount, with one SelfSubjectReview and fail closed on an empty
or anonymous one. Source: portal:D13:R1/R2, portal:D5:R3/R4.

#### Scenario: An identity the cluster does not name

- **WHEN** the in-cluster SelfSubjectReview fails or names no user
- **THEN** the container exits 1 before it listens, and the Pod restarts without ever printing a
  launch URL

### Requirement: In a Pod local mode prints the launch URL and applies the launch token and the browser protections
When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration, the
portal SHALL run local mode unchanged: it SHALL print the launch URL on standard output, which
becomes the container log, and SHALL apply the launch token, the session cookie, the `Host`
allowlist, the cross-origin refusal and the security headers exactly as on a user's machine. Source:
portal:D13:R1/R2, portal:D5:R3/R4.

#### Scenario: The Pod starts in local mode

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090` with no
  kubeconfig in the Pod
- **THEN** the container log names the user `system:serviceaccount:opm-portal:opm-portal` with
  `source=in-cluster`
- **AND** the container log holds one `Open this link once to sign in:
  http://127.0.0.1:8090/launch?token=...` line

#### Scenario: The token is spent

- **WHEN** a second browser opens the launch URL after the first one did
- **THEN** the response is `403`
- **AND** after `kubectl -n opm-portal rollout restart deploy/opm-portal` the new Pod's log holds
  a new launch URL that admits a browser

### Requirement: In a Pod local mode reads as the ServiceAccount and logs source in-cluster
When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration,
every read SHALL be made as the Pod's ServiceAccount after a SelfSubjectAccessReview for it. When `opm-portal serve` finds no kubeconfig and client-go
loads the in-cluster configuration, the startup log line naming the identity SHALL carry
`source=in-cluster` in place of an empty context name. Source: portal:D13:R1/R2, portal:D5:R3/R4.

#### Scenario: The startup log line

- **WHEN** the portal starts in a Pod
- **THEN** the startup log line naming the identity SHALL carry `source=in-cluster` in place of an empty context name

### Requirement: In a Pod local mode is reached only through port-forward to the bound port
When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration,
a browser reaches the portal only through `kubectl port-forward` to the bound port, and the `Host`
allowlist SHALL admit only the bound port, so a forward from any other local port is refused.
Source: portal:D13:R1/R2, portal:D5:R3/R4.

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
