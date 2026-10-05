## ADDED Requirements

### Requirement: A subscriber is written only a change to its own document

In every mode, the stream SHALL compare each `upsert` or `k8sevent` item it is about to write to
a subscriber with the document it last wrote to that subscriber on that topic, and SHALL write the
item only when the two differ. A `delete`, a snapshot and a `closed` event SHALL always be
written. A snapshot's only item, and a `delete`'s item, SHALL become the document later items are
compared with. An item that is not written SHALL take no event identifier. After a reconnect, the
document last written SHALL still be compared with when the client's `Last-Event-ID` is at or
after the event it was written in; otherwise the first item of the topic SHALL be written whatever
it holds. A reconnect that falls back to a snapshot SHALL write it, and later items SHALL be
compared with its document. Log lines SHALL never be compared. Source: supervisor ruling on
portal:OQ20 (portal:D2:R7); keeps portal:D7:R2.

#### Scenario: A change the subscriber cannot see

- **WHEN** alice and bob follow `instance:apps/blog`, and a Pod of it that only bob may read
  changes
- **THEN** bob receives an upsert and alice receives nothing: no event, and her next event's
  identifier follows her last one with no gap

#### Scenario: A periodic refresh with nothing changed

- **WHEN** a followed topic is rendered again and every subscriber's document is the one it
  was last sent
- **THEN** no subscriber receives an event

#### Scenario: A reconnect from the last event received

- **WHEN** a stream reconnects with the `Last-Event-ID` of its last event, and its topic's next
  item equals the last one written before the disconnect
- **THEN** the item is not written, and the next event's identifier follows that `Last-Event-ID`
  with no gap

#### Scenario: A reconnect from before the last document

- **WHEN** a stream reconnects with a `Last-Event-ID` before the event its topic's last document
  was written in
- **THEN** the topic's first item after the reconnect is written

#### Scenario: A delete after an upsert that carried its Removed document

- **WHEN** a subscriber was written an upsert carrying an object's `Removed` document, and the
  object's `delete` follows with the same document
- **THEN** the `delete` is written

#### Scenario: A reconnect that falls back to a snapshot

- **WHEN** a stream reconnects with a `Last-Event-ID` the stream can no longer replay from, and
  is written a snapshot of its topic
- **THEN** a later item equal to the snapshot's document is not written, and a different one is

#### Scenario: Equal log lines

- **WHEN** a log topic carries two equal lines
- **THEN** both are written
