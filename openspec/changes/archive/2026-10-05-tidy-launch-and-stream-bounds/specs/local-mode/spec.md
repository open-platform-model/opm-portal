## MODIFIED Requirements

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
token on any process's command line. Source: 0030:D5:R3.

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
