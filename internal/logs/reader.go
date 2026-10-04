package logs

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"time"
)

// Message types and the markers and end reasons they carry. Clients treat
// an unknown value as unknown (the read API's enums are open).
const (
	TypeLine   = "line"
	TypeMarker = "marker"
	TypeEnd    = "end"

	// MarkerTruncated is set on a line cut at the line cap; Cut says how
	// many bytes were cut.
	MarkerTruncated = "truncated"
	// MarkerRateLimited reports, before the next delivered line or at the
	// end, how many lines were dropped over the rate.
	MarkerRateLimited = "rate-limited"
	// MarkerSkipped reports how many initial-tail lines past the tail cap
	// were skipped.
	MarkerSkipped = "skipped"

	// ReasonContainerStopped: the followed container stopped.
	ReasonContainerStopped = "container_stopped"
	// ReasonCompleted: the previous container's output was read to its end.
	ReasonCompleted = "completed"
	// ReasonUpstreamClosed: the API server ended the stream while the
	// container still runs; following again starts a new tail.
	ReasonUpstreamClosed = "upstream_closed"
	// ReasonContainerNotFound: the Pod has no container of that name.
	ReasonContainerNotFound = "container_not_found"
	// ReasonNoPrevious: the container has no previous, terminated instance.
	ReasonNoPrevious = "no_previous"
	// ReasonPodNotFound: the Pod is gone.
	ReasonPodNotFound = "pod_not_found"
	// ReasonUnavailable: the log could not be read.
	ReasonUnavailable = "unavailable"
)

// Message is the document a log or logend stream item carries. Seq
// increases along one upstream stream; a line a subscriber receives both in
// its snapshot and as a later message has the same Seq. A line with empty
// text carries no text field.
type Message struct {
	Seq       uint64     `json:"seq"`
	Type      string     `json:"type"`
	Container string     `json:"container"`
	Time      *time.Time `json:"time,omitempty"`
	Text      string     `json:"text,omitempty"`
	Marker    string     `json:"marker,omitempty"`
	Cut       int        `json:"cut,omitempty"`
	Dropped   int64      `json:"dropped,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}

// rawLine is one line read from the upstream stream.
type rawLine struct {
	time time.Time // zero when the line carries no timestamp
	text string
	cut  int // bytes cut past the line cap
}

// lineReader reads lines of at most max bytes. A longer line is returned
// cut, and the rest is discarded as it is read, so a writer that never
// writes a newline cannot grow memory past the buffer.
type lineReader struct {
	r   *bufio.Reader
	max int
}

func newLineReader(r io.Reader, maxBytes int) *lineReader {
	return &lineReader{r: bufio.NewReaderSize(r, maxBytes), max: maxBytes}
}

// next returns the next line, or io.EOF (or the read error) once no line is
// left. A last line without a newline is returned before io.EOF.
func (lr *lineReader) next() (rawLine, error) {
	chunk, err := lr.r.ReadSlice('\n')
	switch {
	case err == nil:
		return parseLine(bytes.TrimRight(chunk, "\r\n"), 0), nil
	case errors.Is(err, bufio.ErrBufferFull):
		head := bytes.Clone(chunk)
		cut, err := lr.discard()
		line := parseLine(head, cut)
		if err != nil && !errors.Is(err, io.EOF) {
			return line, err
		}
		return line, nil
	case len(chunk) > 0:
		// A last line without a newline; the error comes on the next call.
		return parseLine(bytes.TrimRight(chunk, "\r"), 0), nil
	}
	return rawLine{}, err
}

// discard reads past the rest of an oversize line, returning how many bytes
// it dropped (not counting the newline).
func (lr *lineReader) discard() (int, error) {
	n := 0
	for {
		chunk, err := lr.r.ReadSlice('\n')
		switch {
		case err == nil:
			return n + len(bytes.TrimRight(chunk, "\r\n")), nil
		case errors.Is(err, bufio.ErrBufferFull):
			n += len(chunk)
		default:
			return n + len(chunk), err
		}
	}
}

// parseLine splits the timestamp the API server prefixes when asked for
// timestamps from the text.
func parseLine(b []byte, cut int) rawLine {
	if ts, rest, ok := bytes.Cut(b, []byte(" ")); ok {
		if t, err := time.Parse(time.RFC3339Nano, string(ts)); err == nil {
			return rawLine{time: t, text: string(rest), cut: cut}
		}
	}
	return rawLine{text: string(b), cut: cut}
}

// bucket is a token bucket: rate tokens per second up to burst.
type bucket struct {
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newBucket(rate float64, burst int, now time.Time) *bucket {
	return &bucket{rate: rate, burst: float64(burst), tokens: float64(burst), last: now}
}

func (b *bucket) refill(now time.Time) {
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
	}
	b.last = now
}

// limiter admits a line only when both its line and byte buckets allow it.
type limiter struct {
	lines, bytes *bucket
}

func (l *limiter) allow(now time.Time, n int) bool {
	l.lines.refill(now)
	l.bytes.refill(now)
	cost := min(float64(n), l.bytes.burst)
	if l.lines.tokens < 1 || l.bytes.tokens < cost {
		return false
	}
	l.lines.tokens--
	l.bytes.tokens -= cost
	return true
}
