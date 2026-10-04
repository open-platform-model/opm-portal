## Purpose

Defines how a client receives live changes from the portal over one server-sent-events stream:
which topics exist, who may subscribe to them, what each subscriber receives, and how a stream
resumes, stays alive and is bounded.

## ADDED Requirements

### Requirement: Topics have a fixed grammar

The portal SHALL accept exactly these topic names on a stream: `platform`, `instances`,
`instances:<namespace>`, `instance:<namespace>/<name>`, `package:<namespace>/<name>`,
`registration:<name>`, and `events:<object topic>`, where an object topic is `platform`,
`instance:…`, `package:…` or `registration:…`. Namespaces SHALL be DNS-1123 labels and names
DNS-1123 subdomains. The `log:<namespace>/<pod>/<container>` form SHALL be recognised and
refused as not yet served. Any other name, a wildcard, or an empty segment SHALL be refused as a
bad request before the stream opens.

#### Scenario: A well-formed topic is accepted

- **WHEN** a client opens a stream with topics `instance:apps/blog` and `events:instance:apps/blog`
- **THEN** the stream opens with both topics attached

#### Scenario: A malformed topic is refused

- **WHEN** a client opens a stream with topic `instance:apps` or `instance:Apps/blog` or `pods:*`
- **THEN** the request is refused as a bad request
- **AND** no stream is opened and no authorization review is sent

#### Scenario: A log topic is reserved

- **WHEN** a client opens a stream with topic `log:apps/blog-0/server`
- **THEN** the request is refused as a bad request that names the topic as not served

### Requirement: One stream per tab carries many topics

A stream SHALL carry every topic attached to it, multiplexed, with each message naming its topic.
A client SHALL be able to attach and detach topics while the stream stays open. The first message
on a stream SHALL name the stream's identifier, which only the session that opened it can use to
change its topics. A stream SHALL carry at most a configured number of topics.

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

### Requirement: Every subscription and every delivery is authorized for the subscriber

The portal SHALL authorize a topic for the stream's identity, on the reads the topic stands for,
before attaching it, and SHALL re-check that authorization before delivering each message once
the decision behind it has expired. Each change SHALL be delivered only to subscribers allowed to
read the object it reveals; items a subscriber may not read SHALL be left out without a trace,
and a snapshot SHALL contain only items the subscriber may read. A denied topic SHALL be closed on
the stream with a `forbidden` code (or `unauthenticated` when the authorizer does not serve the
stream's identity) and SHALL deliver nothing; an authorization error SHALL close the topic with an
`upstream_unavailable` code and SHALL deliver nothing. A stream SHALL NOT open
for an unauthenticated identity, and no authorization review SHALL be sent for one. Source:
0030:D7:R2, 0030:D6:R4.

#### Scenario: A subscriber receives only what it may read

- **WHEN** topic `instances` publishes changes for instances in namespaces `team-a` and `team-b`
- **AND** subscriber A may read instances in `team-a` only, and subscriber B in both
- **THEN** A receives the `team-a` changes and nothing about `team-b`
- **AND** B receives both

#### Scenario: A forbidden topic is closed, not served

- **WHEN** a client may not read instance `apps/blog` and attaches `instance:apps/blog`
- **THEN** the stream delivers a closed message for that topic with code `forbidden`
- **AND** never a snapshot or change for it
- **AND** the message reads the same whether or not `apps/blog` exists

#### Scenario: A revoked permission stops delivery

- **WHEN** a subscriber's permission to read `apps/blog` is revoked while its stream is open
- **THEN** within one authorization decision lifetime the topic is closed with code `forbidden`
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
an event identifier, and identifiers SHALL strictly increase along a stream. Source: 0030:D2:R5.

#### Scenario: A change racing the snapshot is not lost

- **WHEN** an instance changes while a new subscriber's snapshot is being taken
- **THEN** the subscriber receives the change in the snapshot, after it as an upsert, or both
- **AND** never neither

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
