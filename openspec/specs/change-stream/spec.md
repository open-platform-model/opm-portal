# change-stream Specification

## Purpose
Defines how a client receives live changes from the portal over one server-sent-events stream:
which topics exist, who may subscribe to them, what each subscriber receives, and how a stream
resumes, stays alive and is bounded.

## Requirements

### Requirement: One stream per tab carries many topics

A stream SHALL carry every topic attached to it, multiplexed, with each message naming its topic.
A client SHALL be able to attach and detach topics while the stream stays open. The first message
on a stream SHALL name the stream's identifier, which only the session that opened it, under the
identity that opened it, can use to change its topics. A stream SHALL carry at most a configured
number of topics; a topic whose closed message has not been sent yet counts once towards it, and
detaching such a topic SHALL drop its unsent closed message.

#### Scenario: Adding a topic to an open stream

- **WHEN** a client with an open stream attaches topic `instance:apps/web`
- **THEN** the same stream delivers a snapshot of `instance:apps/web` and then its changes
- **AND** no second connection is opened

#### Scenario: Removing a topic from an open stream

- **WHEN** a client detaches topic `instance:apps/blog` from its stream
- **THEN** the stream delivers no further message for `instance:apps/blog`
- **AND** messages for the stream's other topics keep arriving

#### Scenario: Another session cannot change a stream

- **WHEN** a session other than the one that opened stream S asks to attach a topic to S
- **THEN** the request is refused exactly as for a stream that does not exist

#### Scenario: Another identity in the same session cannot change a stream

- **WHEN** the session that opened stream S now names a different identity and asks to attach a
  topic to S
- **THEN** the request is refused exactly as for a stream that does not exist
- **AND** no authorization review is sent

#### Scenario: Asking again for a denied topic counts it once

- **WHEN** a stream at one topic below its cap has a closed message for topic T not yet sent
- **AND** the client attaches T again
- **THEN** the request is not refused as too many topics

#### Scenario: A detached topic's unsent closing is dropped

- **WHEN** a client detaches a topic whose closed message its stream has not sent yet
- **THEN** the stream never sends that closed message

### Requirement: Every subscription and every delivery is authorized for the subscriber

The portal SHALL authorize a topic for the stream's identity, on the reads the topic stands for,
before attaching it, and SHALL re-check that authorization before delivering each message once
the decision behind it has expired. Every message SHALL be written right after an in-memory check
confirms that every decision it used is unexpired; decisions are cached for at most 30 s, so
revocation reaches the stream within that TTL. The list topics `instances` and `instances:<namespace>`
SHALL follow the read model's list rule, as a `GET` list does: `instances` SHALL require a
cluster-wide `list` grant on instances and `instances:<namespace>` a `list` grant on that
namespace, a reader without it SHALL be refused the topic with the same denial the `GET` list
gives, and the topic SHALL carry only items within the namespace scope of that grant, with no
authorization review per item. On any other topic each change SHALL be delivered only to
subscribers allowed to read the object it reveals. Items a subscriber may not read SHALL be left
out without a trace or a count, and a snapshot SHALL contain only items the subscriber may read.
A denied topic SHALL be closed on the stream with a `forbidden` code (or `unauthenticated` when
the authorizer does not serve the stream's identity) and SHALL deliver nothing; a closed message
not written before its connection ends, including one for a topic denied while the stream is
disconnected, SHALL be sent first thing on the stream's next connection; an authorization error
SHALL close the topic with an `upstream_unavailable` code and SHALL deliver nothing. A stream
SHALL NOT open for an unauthenticated identity, and no authorization review SHALL be sent for
one. Source: 0030:D7:R2, 0030:D5:R5, 0030:D6:R4.

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
  `upstream_unavailable` after a bounded number of rounds

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
- **THEN** within one authorization decision lifetime plus one heartbeat interval the topic is
  closed with code `forbidden`
- **AND** no change published after the closing is delivered for it

#### Scenario: An authorization error delivers nothing

- **WHEN** the access review for a subscriber fails or times out
- **THEN** the topic is closed with code `upstream_unavailable`
- **AND** no snapshot or change is delivered for it

#### Scenario: An unauthenticated identity opens no stream

- **WHEN** a request with an empty or anonymous identity asks to open a stream
- **THEN** it is refused as unauthenticated
- **AND** no authorization review is sent

### Requirement: Each topic starts with a snapshot and continues with changes

The first message for a topic on a stream SHALL be a snapshot carrying the topic's current items
the subscriber may read, in the same document shapes as the read API's `GET`s. Every later
message for the topic SHALL be an `upsert`, a `delete` or an event item in those shapes. No change
made after the snapshot was taken SHALL be missed; a change MAY repeat state the snapshot already
carried, and clients apply upserts and deletes idempotently. Every snapshot and change SHALL carry
an event identifier, and identifiers SHALL strictly increase along a stream. Identifiers SHALL
count only the events delivered on that stream, so their gaps reveal nothing about items left out
for the subscriber, other topics or other streams. Source: 0030:D2:R5, 0030:D7:R2.

#### Scenario: A change racing the snapshot is not lost

- **WHEN** an instance changes while a new subscriber's snapshot is being taken
- **THEN** the subscriber receives the change in the snapshot, after it as an upsert, or both
- **AND** never neither

#### Scenario: Identifiers reveal nothing left out

- **WHEN** topic `instance:apps/blog` publishes five changes, of which three reveal a Pod the
  subscriber may read and two reveal a Pod it may not, while other streams follow the same and
  other topics
- **THEN** the subscriber receives the three changes it may read, with consecutive identifiers
  following the snapshot's

### Requirement: A reconnecting stream resumes or re-reads

When a client reconnects with the `Last-Event-ID` of the stream it lost, within the resume window
and in the same session, the portal SHALL reattach that stream's topics, re-authorize each, and
deliver every change after that identifier that its buffer still holds. When the buffer no longer
covers the gap, or the client had not yet received a topic's snapshot, the portal SHALL send that
topic a fresh snapshot, which replaces the client's state for the topic. An identifier from
another session, another process lifetime, or past the resume window SHALL open a new stream with
fresh snapshots. Source: 0030:D2:R5.

#### Scenario: Resume within the buffer

- **WHEN** a stream drops after event N and three changes are published before it reconnects with
  `Last-Event-ID` N
- **THEN** the reconnected stream delivers exactly those three changes, in order, and no snapshot

#### Scenario: Resume past the buffer

- **WHEN** more changes were published while the stream was away than the topic's buffer holds
- **THEN** the reconnected stream delivers a fresh snapshot for that topic

#### Scenario: An identifier from another session

- **WHEN** a client reconnects with a `Last-Event-ID` issued to a different session
- **THEN** it gets a new stream with fresh snapshots for the topics it asked for
- **AND** none of the other session's topics

### Requirement: Streams are kept alive and bounded

A stream SHALL send a heartbeat at a fixed interval while it is open. A stream whose client cannot
keep up, so its queue of undelivered messages fills, SHALL be closed without delaying any other
subscriber, and SHALL stay resumable for the resume window. A stream with no topics for the idle
timeout SHALL be closed. The portal SHALL refuse a new stream as too many streams when the
session already holds its cap (two by default, so a browser keeps HTTP/1.1 connections for
ordinary requests) or the process holds its cap, after first discarding the oldest disconnected
stream in that scope.

#### Scenario: Heartbeats on a quiet stream

- **WHEN** a stream's topics publish nothing for three heartbeat intervals
- **THEN** the client receives three heartbeats

#### Scenario: A slow consumer is evicted

- **WHEN** a client stops reading while its topic keeps publishing past its queue size
- **THEN** its stream is closed
- **AND** another subscriber of the same topic receives every change without delay
- **AND** the evicted client can resume with its last event identifier

#### Scenario: A third tab in local mode

- **WHEN** a session with two open streams opens a third
- **THEN** the third is refused as too many streams
- **AND** the first two keep streaming

#### Scenario: A reloaded tab is not refused

- **WHEN** a session at its cap loses one stream's connection and opens a new stream
- **THEN** the new stream opens and the disconnected one is discarded

### Requirement: Topics follow a fixed grammar

The portal SHALL accept exactly these topic names on a stream: `platform`, `instances`,
`instances:<namespace>`, `instance:<namespace>/<name>`, `package:<namespace>/<name>`,
`registration:<name>`, `events:<object topic>`, where an object topic is `platform`,
`instance:…`, `package:…` or `registration:…`, and `log:<namespace>/<pod>/<container>` with an
optional trailing `/previous` segment that asks for the previous container's logs. Namespaces and
containers SHALL be DNS-1123 labels and names and Pods DNS-1123 subdomains. Any other name, a
wildcard, an empty segment, or a fourth log segment other than `previous` SHALL be refused as a
bad request before the stream opens.

#### Scenario: A well-formed topic is accepted

- **WHEN** a client opens a stream with topics `instance:apps/blog` and `events:instance:apps/blog`
- **THEN** the stream opens with both topics attached

#### Scenario: A malformed topic is refused

- **WHEN** a client opens a stream with topic `instance:apps` or `instance:Apps/blog` or `pods:*`
- **THEN** the request is refused as a bad request
- **AND** no stream is opened and no authorization review is sent

#### Scenario: A log topic is served

- **WHEN** a client opens a stream with topic `log:apps/blog-0/server` or
  `log:apps/blog-0/server/previous`
- **THEN** the topic is authorized and attached like any other topic

#### Scenario: A malformed log topic is refused

- **WHEN** a client opens a stream with topic `log:apps/blog-0/server/current` or `log:apps/blog-0`
- **THEN** the request is refused as a bad request and no authorization review is sent

### Requirement: Log topics are capped per session and per process

A session SHALL follow at most four log topics across all of its streams, and the process SHALL
serve at most fifty distinct log topics at once. A request that would exceed either cap SHALL be
refused as too many topics before any authorization review is sent, and the topics already
attached SHALL keep streaming. A log topic another subscriber already follows SHALL NOT count
again towards the process cap.

#### Scenario: A fifth log topic in one session

- **WHEN** a session follows four log topics and attaches a fifth
- **THEN** the request is refused as too many topics
- **AND** the four log topics keep streaming

#### Scenario: A shared log topic counts once in the process

- **WHEN** the process serves fifty log topics and another session attaches one of them
- **THEN** the topic attaches
