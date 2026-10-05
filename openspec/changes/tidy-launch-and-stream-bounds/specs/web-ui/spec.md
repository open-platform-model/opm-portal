## ADDED Requirements

### Requirement: An expired session stops the page's stream

When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it does
not reconnect, and SHALL show in the live indicator that the session expired and the page must be
reloaded. The page SHALL keep showing what it last rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live indicator reads "session expired, reload"
- **AND** the page makes no further stream request
