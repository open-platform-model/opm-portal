package stream

// ringEntry is one published item and its sequence number.
type ringEntry struct {
	seq  uint64
	item Item
}

// ring keeps a topic's most recent items for resume, at most len(buf) of
// them and, past the newest one, at most maxBytes of item data. floor is the
// highest sequence number the ring cannot replay: the broker's sequence when
// the topic was created, raised each time an entry is evicted. A resume from
// sequence after is complete only when after >= floor.
type ring struct {
	buf      []ringEntry
	start    int // index of the oldest entry
	n        int
	bytes    int
	maxBytes int
	floor    uint64
}

func newRing(size, maxBytes int, floor uint64) *ring {
	return &ring{buf: make([]ringEntry, size), maxBytes: maxBytes, floor: floor}
}

// add appends e, evicting the oldest entries while the ring is full or over
// its byte bound. The newest entry is always kept. Sequence numbers must
// increase.
func (r *ring) add(e ringEntry) {
	if r.n == len(r.buf) {
		r.evict()
	}
	r.buf[(r.start+r.n)%len(r.buf)] = e
	r.n++
	r.bytes += len(e.item.Data)
	for r.n > 1 && r.bytes > r.maxBytes {
		r.evict()
	}
}

// evict drops the oldest entry and raises the floor to its sequence.
func (r *ring) evict() {
	old := &r.buf[r.start]
	r.floor = old.seq
	r.bytes -= len(old.item.Data)
	*old = ringEntry{}
	r.start = (r.start + 1) % len(r.buf)
	r.n--
}

// covers reports whether every entry after sequence after is still held.
func (r *ring) covers(after uint64) bool { return after >= r.floor }

// since returns the held entries with a sequence above after, oldest first.
func (r *ring) since(after uint64) []ringEntry {
	var out []ringEntry
	for i := range r.n {
		e := r.buf[(r.start+i)%len(r.buf)]
		if e.seq > after {
			out = append(out, e)
		}
	}
	return out
}
