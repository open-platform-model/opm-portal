## MODIFIED Requirements

### Requirement: Every subscription and every delivery is authorized for the subscriber

The portal SHALL authorize a topic for the stream's identity, on the reads the topic stands for,
before attaching it, and SHALL re-check that authorization before delivering each message once
the decision behind it has expired. Every message SHALL be written right after an in-memory check
confirms that every decision it used is unexpired. A decision is cached for at most its lifetime
(30 s by default), counted from when its review answers, so a revocation reaches the stream within
one decision lifetime plus one review (about 35 s with the defaults). Re-validating one message
SHALL take at most one decision lifetime (30 s by default): a message whose decisions have not
all been confirmed by then SHALL close its topic with code `upstream_unavailable`, and a review
still running at that deadline SHALL be cancelled. The list topics `instances` and
`instances:<namespace>` SHALL follow the read model's list rule, as a `GET` list does:
`instances` SHALL require a cluster-wide `list` grant on instances and `instances:<namespace>` a
`list` grant on that namespace, a reader without it SHALL be refused the topic with the same
denial the `GET` list gives, and the topic SHALL carry only items within the namespace scope of
that grant, with no authorization review per item. On any other topic each change SHALL be
delivered only to subscribers allowed to read the object it reveals. Items a subscriber may not
read SHALL be left out without a trace or a count, and a snapshot SHALL contain only items the
subscriber may read. A denied topic SHALL be closed on the stream with a `forbidden` code (or
`unauthenticated` when the authorizer does not serve the stream's identity) and SHALL deliver
nothing; a closed message not written before its connection ends, including one for a topic
denied while the stream is disconnected, SHALL be sent first thing on the stream's next
connection; an authorization error SHALL close the topic with an `upstream_unavailable` code and
SHALL deliver nothing. A stream SHALL NOT open for an unauthenticated identity, and no
authorization review SHALL be sent for one. Source: 0030:D7:R2, 0030:D5:R5, 0030:D6:R4.

#### Scenario: A subscriber receives only what it may read

- **WHEN** topic `instances` publishes changes for instances in namespaces `team-a` and `team-b`
- **AND** subscriber A may list instances in `team-a` only, and subscriber B cluster-wide
- **THEN** A is refused `instances` with code `forbidden`, the denial a `GET` list of every
  namespace gives A
- **AND** A follows `instances:team-a` and receives the `team-a` changes and nothing about
  `team-b`
- **AND** B follows `instances` and receives both

#### Scenario: A list topic sends no review per item

- **WHEN** a reader follows a list topic its `list` grant covers and the topic carries many items
- **THEN** the only authorization review sent is the topic's own `list` read

#### Scenario: A decision that expires while a message is built is asked again before the write

- **WHEN** a subscriber follows `instance:apps/blog`, whose snapshot carries an item revealing a
  package that is reviewed on its own
- **AND** the subscriber's access to that package is revoked and the item's decision expires
  before the snapshot is written
- **THEN** the snapshot is written without that item and with no trace of it
- **AND** had the item's review failed with an error instead, the topic would be closed with code
  `upstream_unavailable` and no snapshot written

#### Scenario: A message is written only right after every decision it used is confirmed

- **WHEN** a message's grants are asked again before the write and one review is slow enough that
  another grant the message used, which still covered moments before, expires meanwhile
- **THEN** that grant is asked again too, and the message is written only right after a check,
  with no review in between, confirms that every decision it used is unexpired
- **AND** a topic revoked meanwhile is closed with code `forbidden` and the message is not written
- **AND** an item revoked meanwhile is left out without a trace
- **AND** a message whose decisions keep expiring this way closes its topic with code
  `upstream_unavailable` once one decision lifetime has passed since its re-validation began

#### Scenario: Re-validation sends no review past its deadline

- **WHEN** a message carries eight parts reviewed on their own, each review takes 4.5 s, and
  every decision the message used has expired when its re-validation begins
- **THEN** the topic's review and the parts' reviews started within the first 30 s are sent, and
  no review after
- **AND** the topic is closed with code `upstream_unavailable` and nothing is written for it

#### Scenario: A reader allowed single names is refused the list topic

- **WHEN** subscriber A may read instance `team-b/two` by name but may not list namespace `team-b`
- **AND** A attaches `instances:team-b`
- **THEN** the topic is closed with code `forbidden` and delivers nothing

#### Scenario: A forbidden topic is closed, not served

- **WHEN** a client may not read instance `apps/blog` and attaches `instance:apps/blog`
- **THEN** the stream delivers a closed message for that topic with code `forbidden`
- **AND** never a snapshot or change for it
- **AND** the message reads the same whether or not `apps/blog` exists

#### Scenario: A topic denied while the stream is away is closed on reconnect

- **WHEN** a client attaches `instance:apps/blog`, which it may not read, while its stream is
  disconnected
- **AND** the stream reconnects within the resume window
- **THEN** the reconnected stream delivers a closed message for that topic with code `forbidden`
  before anything else

#### Scenario: A closing lost with its connection is sent on the next

- **WHEN** a closed message for a topic is queued on a stream whose connection ends, by eviction
  or disconnect, before it is written
- **AND** the stream reconnects within the resume window
- **THEN** the reconnected stream delivers that closed message before anything else

#### Scenario: A revoked permission stops delivery

- **WHEN** a subscriber's permission to read `apps/blog` is revoked while its stream is open
- **THEN** within one authorization decision lifetime plus one review plus one heartbeat
  interval the topic is closed with code `forbidden`
- **AND** no change published after the closing is delivered for it

#### Scenario: An authorization error delivers nothing

- **WHEN** the access review for a subscriber fails or times out
- **THEN** the topic is closed with code `upstream_unavailable`
- **AND** no snapshot or change is delivered for it

#### Scenario: An unauthenticated identity opens no stream

- **WHEN** a request with an empty or anonymous identity asks to open a stream
- **THEN** it is refused as unauthenticated
- **AND** no authorization review is sent

## ADDED Requirements

### Requirement: A stream ends when its session expires

A stream SHALL end when the session that opened it expires. Its last message SHALL be an
`expired` event carrying `{"code":"unauthenticated"}`, after which the connection SHALL end and
the stream SHALL be discarded rather than kept for resume, so no data reaches the client after
the session's end. A stream SHALL NOT open for a session that has already expired, and no
authorization review SHALL be sent for one. A session with no expiry opens streams that never
end this way. Control events (`open`, `heartbeat`, `closed`, `expired`) SHALL carry only the
stream's own fields, never topic data.

#### Scenario: A session expires under an open stream

- **WHEN** a stream is open on a topic and its session expires
- **THEN** the stream's next and last message is an `expired` event with code `unauthenticated`
- **AND** the connection ends
- **AND** a reconnect with that stream's `Last-Event-ID` resumes nothing

#### Scenario: An expired session opens no stream

- **WHEN** a request whose session has already expired asks to open a stream
- **THEN** it is refused as unauthenticated
- **AND** no authorization review is sent
