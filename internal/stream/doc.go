// Package stream serves live changes to clients as server-sent events.
//
// # Topics and streams
//
// A topic names what a client follows: the Platform, the instance list, one
// instance, package or registration, or the events about one of them
// (ParseTopic documents the grammar). A browser tab holds one stream and
// attaches every topic its page needs to it, adding and removing topics while
// the stream stays open. In local mode the browser speaks HTTP/1.1 to
// loopback and has six connections per host for every tab and request, so
// streams per session are capped (two by default).
//
// Each topic starts with a snapshot of its current items and continues with
// upsert, delete and k8sevent messages carrying the same documents the read
// API serves. Every snapshot and change carries an event id, and ids strictly
// increase along a stream. Ids count the stream's own events, so a gap never
// reveals an item left out for the reader or anything published elsewhere.
//
// # The producer contract
//
// The read model implements Producer and calls Broker.Publish. It says what
// following a topic takes (Producer.Attributes), returns a topic's current
// items (Producer.Snapshot), and starts and stops watching a topic when the
// broker activates and releases it. A producer MUST update the state Snapshot
// reads before it publishes the change: the broker registers a subscriber
// before it asks for the snapshot, so a change then reaches the subscriber in
// the snapshot, as a later message, or both, and never neither. Clients
// apply upserts and deletes idempotently.
//
// # Authorization
//
// The broker reads nothing from the cluster; it decides who receives what
// through internal/authz, for the identity of the session that opened the
// stream.
//
//   - A stream does not open for an unauthenticated identity, and no review
//     is sent for one.
//   - A topic attaches only after every read the producer names for it is
//     allowed. A denied topic is not registered, so it starts no watch, and
//     the stream gets a closed message for it. A list topic needs the list
//     grant a GET list needs, cluster-wide for "instances" and on the
//     namespace for "instances:<ns>", and is refused with the same denial
//     (0030:D7:R2). A producer that names any other read for a list topic
//     does not serve it.
//   - A closed message stays pending until a connection writes it, so one
//     queued on a connection that ends is the next connection's first
//     message.
//   - Only the session and identity that opened a stream can change its
//     topics or resume it.
//   - Before each delivery and on each heartbeat, the topic's grants are
//     checked with Grant.Covers; an expired grant is checked again, and a
//     denial closes the topic.
//   - Each item is delivered only when the subscriber may read Item.Attrs.
//     An item within the scope of the topic's reads is delivered under the
//     topic's grants. On a list topic an item outside the list grant's scope
//     is a producer fault and is left out without a review, so a list
//     carries only items within its grant's scope and costs no review per
//     item. On an object topic an item outside the topic's reads is reviewed
//     on its own. A forbidden item is left out without a trace.
//   - All grants a message used are re-validated in one place before the
//     write: every snapshot and item carries the topic's grants and each
//     item's own, and the single writer of topic data asks again for any
//     that expired during a slow snapshot, render or review, then checks
//     them all with Grant.Covers in memory, repeating until a pass needs no
//     review (at most three rounds of reviews, or the topic closes with
//     upstream_unavailable). Every message is written right after an
//     in-memory pass confirms that every decision it used is unexpired;
//     decisions are cached for at most 30 s, so revocation reaches the
//     stream within that TTL. An item that is now forbidden is left out (a
//     snapshot is written without it, an item event not at all); a topic
//     denial or any other error closes the topic, so a snapshot never
//     arrives cut short by the topic's own grants.
//   - An authorization error is never a delivery: it closes the topic with
//     the code upstream_unavailable.
//
// # Resume and bounds
//
// Each topic keeps its recent changes in a ring buffer. A stream that loses
// its connection stays registered for a resume window; a client that
// reconnects with the Last-Event-ID of that stream, in the same session, gets
// its topics back, re-authorized, with every change after that id the ring
// still holds. A topic whose gap the ring no longer covers gets a fresh
// snapshot, which replaces the client's state for it.
//
// A stream whose client falls behind, so its queue fills, loses its
// connection without delaying anyone else and stays resumable. A stream with
// no topics for the idle timeout is closed. Session and process caps refuse
// a new stream only after the oldest disconnected stream in that scope has
// been discarded.
package stream
