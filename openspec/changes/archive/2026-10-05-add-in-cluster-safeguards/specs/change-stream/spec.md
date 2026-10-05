## ADDED Requirements

### Requirement: A subscriber is written only a change to its own document

In every mode, the stream SHALL compare each document item it is about to write to a
subscriber (an `upsert`, `delete` or `k8sevent` item) with the document it last wrote to that
subscriber on that topic, and SHALL write the item only when the two differ. A snapshot SHALL
always be written, and its only item SHALL become the document later items are compared with. An
item that is not written SHALL take no event identifier. After a reconnect, the document last
written SHALL still be compared with when the client's `Last-Event-ID` is at or after the event
it was written in; otherwise the first item of the topic SHALL be written whatever it holds. Log lines SHALL never be compared. Source:
supervisor ruling on 0030:OQ20; keeps 0030:D7:R2.

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

#### Scenario: Equal log lines

- **WHEN** a log topic carries two equal lines
- **THEN** both are written
