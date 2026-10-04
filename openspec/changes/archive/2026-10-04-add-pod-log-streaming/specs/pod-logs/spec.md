## Purpose

Defines which Pods' logs a caller may follow on the change stream, how a log topic is authorized,
and how the portal bounds, marks and ends each log stream.

## ADDED Requirements

### Requirement: Logs are served only for Pods an OPM inventory reaches

A log topic SHALL attach only when its Pod is a runtime child of an inventory object the caller
may read. A topic for a Pod no inventory reaches, a Pod reached only through objects the caller
may not read, and a Pod that does not exist SHALL each be closed with the same `forbidden` code
a missing permission gives, and SHALL deliver nothing. No upstream log stream SHALL be opened for
a topic that did not attach. Source: 0030:D10:R1, 0030:D7:R1.

#### Scenario: A Pod of an instance

- **WHEN** a caller allowed `get pods/log` follows `log:default/<pod>/podinfo` for a Pod of the
  `podinfo` Deployment
- **THEN** the topic attaches and delivers a snapshot and then log lines

#### Scenario: A Pod outside every inventory

- **WHEN** a caller allowed `get pods/log` follows a log topic for a Pod no inventory reaches
- **THEN** the topic is closed with code `forbidden`, the same message a missing permission gives
- **AND** no upstream log stream is opened

### Requirement: Pod log access is checked before the stream starts and before each delivery

A log topic's read SHALL be `get` on the Pod's `log` subresource. It SHALL be authorized for the
caller before the reachability lookup and before any upstream stream opens, re-checked before
every delivery once the decision behind it has expired, and authorized again, with the
reachability, on every reconnect. A revoked permission SHALL close the topic with `forbidden`
within one decision lifetime plus one heartbeat interval, and no line SHALL be delivered after the
closing. The reading identity SHALL also be allowed the read before the upstream stream opens.
Source: 0030:D10:R2.

#### Scenario: No permission, no upstream read

- **WHEN** a caller who may not `get pods/log` follows a log topic
- **THEN** the topic is closed with code `forbidden`
- **AND** neither the Pod nor its logs are read

#### Scenario: Revoked mid-stream

- **WHEN** a caller following a log topic loses `get pods/log`
- **THEN** the topic is closed with code `forbidden` and no later line reaches the caller

#### Scenario: Reconnect re-checks

- **WHEN** a stream carrying a log topic reconnects within its resume window
- **THEN** the caller's `get pods/log` access and the Pod's reachability are checked again before
  any line is delivered

### Requirement: Log output is bounded by the portal and marked

The portal SHALL follow a container's log with timestamps and a capped initial tail, and SHALL
NOT use the API server's byte limit. A line longer than the line cap SHALL be cut and marked
`truncated` with the number of bytes cut. Lines beyond the per-topic line rate or byte rate SHALL
be dropped, and the next delivered message SHALL be a `rate-limited` marker carrying how many
lines were dropped. An initial tail larger than the tail byte cap SHALL skip ahead to live output,
announced by a `skipped` marker carrying how many lines were skipped. A stream SHALL never end
because of a byte limit. Log lines SHALL NOT appear in the portal's own logs. Source:
0030:D10:R3.

#### Scenario: An oversize line

- **WHEN** a container writes a line twice the line cap
- **THEN** the subscriber receives the first line-cap bytes marked `truncated`, with the count of
  bytes cut, and the following line intact

#### Scenario: A burst over the rate

- **WHEN** a container writes lines faster than the per-topic rate
- **THEN** the excess lines are dropped
- **AND** a `rate-limited` marker with the number dropped precedes the next delivered line

#### Scenario: A large initial tail

- **WHEN** the initial tail holds more bytes than the tail cap
- **THEN** the tail lines past the cap are skipped and a `skipped` marker with their count
  precedes the first live line

### Requirement: Containers are chosen explicitly and a stopped container ends its stream

A log topic SHALL name its container, which SHALL be one of the Pod's containers, init
containers or ephemeral containers; a container the Pod does not have SHALL end the topic with a
`logend` message whose reason says so. The `previous` form SHALL read the last terminated
instance of the container once, without following, so a crash-looping container's last output
can be read. When the followed container stops, or the previous output has been read, the topic
SHALL deliver a `logend` message with its reason. Source: 0030:D10:R4.

#### Scenario: A container that stops

- **WHEN** the followed container exits
- **THEN** the subscriber receives a `logend` message after the last line

#### Scenario: A crash-looping container

- **WHEN** a caller follows `log:apps/<pod>/server/previous`
- **THEN** the last terminated instance's output is delivered and then a `logend` message

#### Scenario: A container the Pod does not have

- **WHEN** a caller follows a log topic naming a container the Pod does not have
- **THEN** the topic delivers a `logend` message whose reason is that the container was not found

### Requirement: One upstream stream per log topic, closed with its last subscriber

All subscribers of one log topic SHALL share one upstream log stream. The upstream stream SHALL be
closed when the topic's last subscriber leaves, whether by detaching the topic, a denial, or the
stream ending. A subscriber that joins a topic already streaming SHALL receive the recent lines in
its snapshot; each log message SHALL carry a sequence number so a client can discard a line it
received both in a snapshot and as a later message.

#### Scenario: Two tabs, one upstream

- **WHEN** two sessions follow the same log topic
- **THEN** one upstream log stream is open

#### Scenario: The last subscriber leaves

- **WHEN** the only subscriber detaches a log topic
- **THEN** the upstream log stream is closed
