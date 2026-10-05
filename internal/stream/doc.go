// Package stream serves live changes to clients as server-sent events.
//
// # Topics and streams
//
// A topic names what a client follows: the Platform, the instance list, one
// instance, package or registration, the events about one of them, or one
// container's log (ParseTopic documents the grammar). A browser tab holds one stream and
// attaches every topic its page needs to it, adding and removing topics while
// the stream stays open. In local mode the browser speaks HTTP/1.1 to
// loopback and has six connections per host for every tab and request, so
// streams per session are capped (two by default).
//
// Each topic starts with a snapshot of its current items and continues with
// upsert, delete and k8sevent messages carrying the same documents the read
// API serves, or, on a log topic, log and logend messages. Every snapshot and change carries an event id, and ids strictly
// increase along a stream. Ids count the stream's own events, so a gap never
// reveals an item left out for the reader or anything published elsewhere.
//
// A topic changes for everyone at once, but each subscriber's document is
// rendered for them. The stream remembers, per subscription, the document it
// last wrote (an upsert, delete or k8sevent item, or a snapshot's only item)
// and writes a later item only when it differs, so a change a subscriber
// cannot see, or a refresh that changed nothing, sends them no event and
// takes no id: they cannot tell when it happened. A reconnect keeps a
// remembered document when the client's Last-Event-ID is at or after the
// event it was written in, since the client holds it; otherwise it forgets
// it, and the topic's first item after the reconnect is written. Log lines
// are records, not documents, and are never compared.
//
// # The producer contract
//
// The read model and the log reader implement Producer, joined by a Mux, and
// call Broker.Publish. It says what
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
//   - A producer that is an Admitter decides, after every read of a topic
//     is allowed, whether the identity may follow it (a Pod log needs an
//     OPM inventory the reader may read to reach the Pod). A refusal closes
//     the topic with the code a denial gives; it runs again on reconnect.
//   - A producer that is a Follower is told when a stream newly subscribes
//     to a topic already active, on Open or Subscribe, after every check;
//     never on a reconnect, which resumes what the stream already holds.
//   - A session follows at most MaxLogTopicsPerSession log topics, and the
//     process serves at most MaxLogTopics distinct ones; both are checked
//     before any review.
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
//     review. Re-validating one message takes at most
//     Options.RevalidateTimeout, one decision lifetime by default: no
//     review starts after it, one running at it is canceled, and the topic
//     closes with upstream_unavailable. Every message is written right after
//     an in-memory pass confirms that every decision it used is unexpired.
//     A decision lives one TTL (30 s by default) from when its review
//     answers, so a revocation reaches the stream within the TTL plus one
//     review: about 35 s with the default 5 s review timeout, and on a
//     quiet topic one heartbeat later. An item that is now forbidden is
//     left out (a snapshot is written without it, an item event not at
//     all); a topic
//     denial or any other error closes the topic, so a snapshot never
//     arrives cut short by the topic's own grants.
//   - An authorization error is never a delivery: it closes the topic with
//     the code upstream_unavailable.
//
// # Resume and bounds
//
// Each topic keeps its recent changes in a ring buffer of RingSize entries;
// a log topic's ring also keeps at most LogRingBytes of item data. A stream
// that loses
// its connection stays registered for a resume window; a client that
// reconnects with the Last-Event-ID of that stream, in the same session, gets
// its topics back, re-authorized, with every change after that id the ring
// still holds. A topic whose gap the ring no longer covers gets a fresh
// snapshot, which replaces the client's state for it.
//
// A stream whose client falls behind, so its queue fills, loses its
// connection without delaying anyone else and stays resumable. A stream with
// no topics for the idle timeout is closed. A stream ends when the session
// that opened it expires (Session.Expires): its last message is an expired
// event, and no message whose snapshot, render or review is still running at
// that moment is written after it. It is discarded, not kept for resume, and
// a stream does not open for a session that has already expired. Session and
// process caps refuse a new stream only after the oldest disconnected stream
// in that scope has been discarded.
package stream
