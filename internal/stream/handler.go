package stream

import (
	"errors"
	"net/http"
	"strings"
)

// HandlerOptions tune NewHandler.
type HandlerOptions struct {
	// Error writes a refusal. status is the HTTP status the handler chose
	// for err. The default writes err's text as text/plain; the read API
	// replaces it with problem documents. Errors from this package never
	// carry a credential or an identity.
	Error func(w http.ResponseWriter, r *http.Request, status int, err error)
}

// NewHandler returns a handler that serves one stream per request: GET with
// a comma-separated topics query parameter and an optional Last-Event-ID
// header. session resolves the caller's Session; an error, or a session
// with no principal, is refused as unauthenticated before anything else
// happens. The handler mounts no route; the read API does.
func NewHandler(b *Broker, session func(*http.Request) (Session, error), opts HandlerOptions) http.Handler {
	if opts.Error == nil {
		opts.Error = func(w http.ResponseWriter, _ *http.Request, status int, err error) {
			http.Error(w, err.Error(), status)
		}
	}
	return &handler{b: b, session: session, fail: opts.Error}
}

type handler struct {
	b       *Broker
	session func(*http.Request) (Session, error)
	fail    func(w http.ResponseWriter, r *http.Request, status int, err error)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.fail(w, r, http.StatusMethodNotAllowed, errors.New("stream: method not allowed"))
		return
	}
	s, err := h.session(r)
	if err != nil || s.Key == "" || !s.Identity.Authenticated() {
		h.fail(w, r, http.StatusUnauthorized, ErrUnauthenticated)
		return
	}
	var topics []Topic
	for _, v := range r.URL.Query()["topics"] {
		for name := range strings.SplitSeq(v, ",") {
			if name == "" {
				continue
			}
			t, err := ParseTopic(name)
			if err != nil {
				h.fail(w, r, http.StatusBadRequest, err)
				return
			}
			topics = append(topics, t)
		}
	}
	st, err := h.b.Open(r.Context(), s, topics, r.Header.Get("Last-Event-ID"))
	if err != nil {
		h.fail(w, r, statusOf(err), err)
		return
	}
	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// The response has started, so why the stream ended can only be logged.
	if err := st.Serve(r.Context(), w); err != nil {
		h.b.log.Debug("stream ended", "stream", st.ID(), "reason", err)
	}
}

// statusOf maps the broker's errors to HTTP statuses. ErrNoStream comes
// only from Subscribe and Unsubscribe, which the read API's topic-change
// request calls.
func statusOf(err error) int {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, ErrTooManyStreams):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrTooManyTopics), errors.Is(err, ErrTopicNotServed):
		return http.StatusBadRequest
	case errors.Is(err, ErrNoStream):
		return http.StatusNotFound
	case errors.Is(err, ErrClosed):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}
