package stream

// ringEntry is one published item and its sequence number.
type ringEntry struct {
	seq  uint64
	item Item
}

// ring keeps a topic's most recent items for resume. floor is the highest
// sequence number the ring cannot replay: the broker's sequence when the
// topic was created, raised each time an entry is overwritten. A resume
// from sequence after is complete only when after >= floor.
type ring struct {
	buf   []ringEntry
	start int // index of the oldest entry
	n     int
	floor uint64
}

func newRing(size int, floor uint64) *ring {
	return &ring{buf: make([]ringEntry, size), floor: floor}
}

// add appends e, overwriting the oldest entry when full. Sequence numbers
// must increase.
func (r *ring) add(e ringEntry) {
	if r.n < len(r.buf) {
		r.buf[(r.start+r.n)%len(r.buf)] = e
		r.n++
		return
	}
	r.floor = r.buf[r.start].seq
	r.buf[r.start] = e
	r.start = (r.start + 1) % len(r.buf)
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
