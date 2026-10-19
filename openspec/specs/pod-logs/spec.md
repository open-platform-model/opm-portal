# pod-logs Specification

## Purpose
Defines which Pods' logs a caller may follow on the change stream, how a log topic is authorized,
and how the portal bounds, marks and ends each log stream.

## Requirements

### Requirement: A log topic attaches only for a Pod an inventory reaches
A log topic SHALL attach only when its Pod is a runtime child of an inventory object the caller may
read, in the inventory of a ModuleInstance or ModulePackage the caller may get. Source:
portal:D10:R1, portal:D7:R1.

#### Scenario: A Pod of an instance

- **WHEN** a caller allowed `get pods/log` follows `log:default/<pod>/podinfo` for a Pod of the
  `podinfo` Deployment
- **THEN** the topic attaches and delivers a snapshot and then log lines

### Requirement: A log topic for a Pod no inventory reaches is closed as forbidden
A log topic for a Pod no inventory reaches, for a Pod reached only through objects the caller may
not read (the inventory object or its owner), or for a Pod that does not exist SHALL be closed with
the same `forbidden` code a missing permission gives, and SHALL deliver nothing. Source:
portal:D10:R1, portal:D7:R1.

#### Scenario: A Pod outside every inventory

- **WHEN** a caller allowed `get pods/log` follows a log topic for a Pod no inventory reaches
- **THEN** the topic is closed with code `forbidden`, the same message a missing permission gives
- **AND** no upstream log stream is opened

#### Scenario: A Pod whose owning instance the caller may not read

- **WHEN** a caller allowed `get pods/log` and the Deployment, but not `get` on the `podinfo`
  ModuleInstance, follows a log topic for one of the Deployment's Pods
- **THEN** the topic is closed with code `forbidden`, the same message a missing permission gives

### Requirement: No upstream log stream opens for a log topic that did not attach
No upstream log stream SHALL be opened for a log topic that did not attach. Source: portal:D10:R1,
portal:D7:R1.

#### Scenario: A topic that did not attach

- **WHEN** a log topic did not attach
- **THEN** no upstream log stream is opened

### Requirement: A log topic's read is authorized before the lookup and before the upstream stream opens
A log topic's read SHALL be `get` on the Pod's `log` subresource. It SHALL be authorized for the
caller before the reachability lookup and before any upstream stream opens. The reading identity
SHALL also be allowed the read before the upstream stream opens. Source: portal:D10:R2.

#### Scenario: No permission, no upstream read

- **WHEN** a caller who may not `get pods/log` follows a log topic
- **THEN** the topic is closed with code `forbidden`
- **AND** neither the Pod nor its logs are read

### Requirement: A log topic's authorization is renewed before delivery and on reconnect
A log topic's read, `get` on the Pod's `log` subresource, SHALL be re-checked before every delivery
once the decision behind it has expired, and SHALL be authorized again, with the reachability, on
every reconnect. Source: portal:D10:R2.

#### Scenario: Reconnect re-checks

- **WHEN** a stream carrying a log topic reconnects within its resume window
- **THEN** the caller's `get pods/log` access and the Pod's reachability are checked again before
  any line is delivered

#### Scenario: An expired decision

- **WHEN** the decision behind a log topic's authorization has expired
- **THEN** the read is re-checked before the next delivery

### Requirement: A revoked permission closes a log topic within one decision lifetime plus one heartbeat
A revoked permission SHALL close a log topic with `forbidden` within one decision lifetime plus one
heartbeat interval, and no line SHALL be delivered after the closing. Source: portal:D10:R2.

#### Scenario: Revoked mid-stream

- **WHEN** a caller following a log topic loses `get pods/log`
- **THEN** the topic is closed with code `forbidden` and no later line reaches the caller

### Requirement: A log follows with timestamps, a capped initial tail and no byte limit of the API server
The portal SHALL follow a container's log with timestamps and a capped initial tail, and SHALL NOT
use the API server's byte limit. A log stream SHALL never end because of a byte limit. Source:
portal:D10:R3.

#### Scenario: A followed container

- **WHEN** the portal follows a container's log
- **THEN** it follows with timestamps and a capped initial tail, without the API server's byte limit, and the stream does not end because of a byte limit

### Requirement: A log line longer than the line cap is cut and marked truncated
A log line longer than the line cap SHALL be cut and marked `truncated` with the number of bytes
cut. Source: portal:D10:R3.

#### Scenario: An oversize line

- **WHEN** a container writes a line twice the line cap
- **THEN** the subscriber receives the first line-cap bytes marked `truncated`, with the count of
  bytes cut, and the following line intact

### Requirement: Log lines beyond the rate are dropped and counted by a rate-limited marker
Log lines beyond the per-topic line rate or byte rate SHALL be dropped, and a `rate-limited` marker
carrying how many lines were dropped SHALL precede the next delivered line, or SHALL be sent within
a short delay when no line follows. Source: portal:D10:R3.

#### Scenario: A burst over the rate

- **WHEN** a container writes lines faster than the per-topic rate
- **THEN** the excess lines are dropped
- **AND** a `rate-limited` marker with the number dropped precedes the next delivered line

#### Scenario: A burst followed by silence

- **WHEN** a container writes lines faster than the per-topic rate and then writes nothing
- **THEN** a `rate-limited` marker with the number dropped is delivered within the marker delay

### Requirement: The initial tail of a log ends at its first live line and is delivered once it ends
The initial tail SHALL be at most the requested tail lines, ending at the first line stamped after
the stream opened. The initial tail SHALL be delivered once it ends, and no later
than the marker delay after its latest line when the container is quiet, the delay of requirement
"Log lines beyond the rate are dropped and counted by a rate-limited marker". Source: portal:D10:R3.

#### Scenario: A quiet container's tail

- **WHEN** a followed container's initial tail has been read and the container writes nothing
- **THEN** the tail is delivered within the marker delay

### Requirement: An initial tail larger than the tail byte cap keeps its newest lines and announces the skipped ones
An initial tail larger than the tail byte cap SHALL keep its newest lines within the cap and skip
the older ones, announced by a `skipped` marker carrying how many lines were skipped, delivered
before the kept lines. Source: portal:D10:R3.

#### Scenario: A large initial tail

- **WHEN** the initial tail holds more bytes than the tail cap
- **THEN** a `skipped` marker with the count of the oldest tail lines that do not fit is delivered
- **AND** the newest tail lines within the cap follow it, oldest first, before the first live line

### Requirement: Clock skew leaves a log line bounded and counted
The stamps of log lines are the node's clock and the stream's opening is the portal's, so clock skew
can count a tail line as live or an early live line as tail; either way the line SHALL stay bounded
and, when dropped, counted. Source: portal:D10:R3.

#### Scenario: Clock skew

- **WHEN** clock skew counts a tail line as live or an early live line as tail
- **THEN** the line stays bounded and, when dropped, is counted

### Requirement: Log lines never appear in the portal's own logs
Log lines SHALL NOT appear in the portal's own logs. Source: portal:D10:R3.

#### Scenario: The portal's own logs

- **WHEN** the portal follows a container's log
- **THEN** no log line appears in the portal's own logs

### Requirement: A log topic names one container of the Pod
A log topic SHALL name its container, which SHALL be one of the Pod's containers, init containers or
ephemeral containers. Source: portal:D10:R4.

#### Scenario: A named container

- **WHEN** a caller follows a log topic
- **THEN** the topic names its container, one of the Pod's containers, init containers or ephemeral containers

### Requirement: A container the Pod does not have ends the log topic
A log topic that names a container the Pod does not have SHALL end with a `logend` message whose
reason says the Pod does not have it. Source: portal:D10:R4.

#### Scenario: A container the Pod does not have

- **WHEN** a caller follows a log topic naming a container the Pod does not have
- **THEN** the topic delivers a `logend` message whose reason is that the container was not found

### Requirement: The previous form reads the last terminated instance once
The `previous` form of a log topic SHALL read the last terminated instance of the container once,
without following, so a crash-looping container's last output can be read. Source: portal:D10:R4.

#### Scenario: A crash-looping container

- **WHEN** a caller follows `log:apps/<pod>/server/previous`
- **THEN** the last terminated instance's output is delivered and then a `logend` message

### Requirement: A stopped container ends its log topic with a logend message
When the followed container of a log topic stops, or the previous output has been read, the topic
SHALL deliver a `logend` message with its reason. Source: portal:D10:R4.

#### Scenario: A container that stops

- **WHEN** the followed container exits
- **THEN** the subscriber receives a `logend` message after the last line

### Requirement: A container that has not started ends a live log topic as waiting
A live log topic for a container that has not started yet SHALL end with a `logend` whose reason
says it is waiting, distinct from a read failure. Source: portal:D10:R4.

#### Scenario: A container that has not started

- **WHEN** a caller follows a log topic for a container still waiting to start
- **THEN** the topic delivers a `logend` message whose reason is `container_waiting`

### Requirement: All subscribers of one log topic share one upstream log stream
All subscribers of one log topic SHALL share one upstream log stream.

#### Scenario: Two tabs, one upstream

- **WHEN** two sessions follow the same log topic
- **THEN** one upstream log stream is open

### Requirement: The upstream log stream closes when the log topic's last subscriber leaves
The upstream log stream of a log topic SHALL be closed when the topic's last subscriber leaves,
whether by detaching the topic, a denial, or the stream ending.

#### Scenario: The last subscriber leaves

- **WHEN** the only subscriber detaches a log topic
- **THEN** the upstream log stream is closed

### Requirement: A subscriber that joins a streaming log topic receives the recent lines, numbered
A subscriber that joins a log topic already streaming SHALL receive the recent lines in its
snapshot, bounded by count and by bytes; each log message SHALL carry a sequence number, increasing
along the topic, so a client can discard a line it received both in a snapshot and as a later
message.

#### Scenario: A subscriber joins a streaming topic

- **WHEN** a subscriber joins a log topic already streaming
- **THEN** it receives the recent lines in its snapshot, bounded by count and by bytes, and each log message carries a sequence number, increasing along the topic

### Requirement: A new subscription after the read ended starts a new read
A new subscription to a log topic whose read has ended, while others still hold it, SHALL start a
new read with a fresh tail, which every subscriber of the topic receives after the earlier `logend`.

#### Scenario: Following again after the end

- **WHEN** two sessions follow a log topic, its read ends with a `logend`, and one of them detaches
  and follows the topic again
- **THEN** a new upstream log stream opens and both sessions receive its lines

### Requirement: A reconnect that resumes a stream starts no new read
A reconnect that resumes a stream SHALL NOT start a new read: for a resumed subscription the
`logend` stays the end, and only unsubscribing and subscribing again follows the container afresh.

#### Scenario: A reconnect after the end

- **WHEN** the only subscriber of a log topic receives its `logend`, loses its connection and
  reconnects with its `Last-Event-ID`
- **THEN** no new upstream log stream opens and no line is delivered on the resumed topic
