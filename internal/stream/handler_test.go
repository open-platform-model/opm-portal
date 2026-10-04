package stream

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// handlerEnv serves a broker over real HTTP. It runs outside a synctest
// bubble, so nothing in it depends on time: heartbeats are an hour apart.
type handlerEnv struct {
	*env
	srv *httptest.Server
}

func newHandlerEnv(t *testing.T, opts Options) *handlerEnv {
	t.Helper()
	opts.HeartbeatInterval = time.Hour
	p := &policy{rules: map[string]rule{}}
	prod := newFakeProducer()
	b := New(prod, newAuthz(t, p, "alice", "bob"), opts)
	prod.b = b
	h := NewHandler(b, func(r *http.Request) (Session, error) {
		name := r.Header.Get("X-Test-User")
		if name == "" {
			return Session{}, errors.New("no session")
		}
		return session(name), nil
	}, HandlerOptions{})
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		b.Close()
		srv.Close()
	})
	return &handlerEnv{env: &env{t: t, policy: p, prod: prod, b: b}, srv: srv}
}

// get sends a stream request; it returns the response with its body open.
func (h *handlerEnv) get(ctx context.Context, user, topics, lastID string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.srv.URL+"?topics="+url.QueryEscape(topics), http.NoBody)
	if err != nil {
		h.t.Fatal(err)
	}
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	return resp
}

// sseReader reads events off a live response body.
type sseReader struct {
	r *bufio.Reader
}

func (s *sseReader) next(t *testing.T) sse {
	t.Helper()
	var block strings.Builder
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading the stream: %v", err)
		}
		if line == "\n" {
			if block.Len() == 0 {
				continue
			}
			return parseSSE(block.String())[0]
		}
		block.WriteString(line)
	}
}

func TestHandlerServesAStream(t *testing.T) {
	h := newHandlerEnv(t, Options{})
	h.policy.set("alice", allowNamespaces("apps"))
	blog := mustTopic(t, "instance:apps/blog")
	h.prod.upsert(t, blog, instItem("apps", "blog", 1))

	ctx, cancel := context.WithCancel(context.Background())
	resp := h.get(ctx, "alice", "instance:apps/blog,events:instance:apps/blog", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for k, want := range map[string]string{
		"Content-Type":           "text/event-stream",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	rd := &sseReader{r: bufio.NewReader(resp.Body)}
	open := rd.next(t)
	if open.Event != EventOpen || open.Retry != "3000" {
		t.Fatalf("first event %+v", open)
	}
	snaps := map[string]bool{}
	var last string
	for range 2 {
		ev := rd.next(t)
		if ev.Event != EventSnapshot {
			t.Fatalf("event %+v, want a snapshot", ev)
		}
		snaps[ev.topic()], last = true, ev.ID
	}
	if !snaps["instance:apps/blog"] || !snaps["events:instance:apps/blog"] {
		t.Errorf("snapshots for %v", snaps)
	}

	// Drop the connection, publish, and resume over HTTP.
	// Whether or not the broker has seen the old connection go, the
	// reconnect resumes it: a still-attached stream is taken over.
	cancel()
	_, _ = io.Copy(io.Discard, resp.Body)
	h.prod.upsert(t, blog, instItem("apps", "blog", 2))
	resp2 := h.get(context.Background(), "alice", "instance:apps/blog", last)
	defer resp2.Body.Close()
	rd2 := &sseReader{r: bufio.NewReader(resp2.Body)}
	if ev := rd2.next(t); ev.Event != EventOpen || ev.Data != open.Data {
		t.Fatalf("resumed stream opened as %+v, want %+v", ev, open)
	}
	if ev := rd2.next(t); ev.Event != EventUpsert || ev.versions(t)[0] != 2 {
		t.Fatalf("resumed with %+v", ev)
	}
}

func TestHandlerRefusals(t *testing.T) {
	h := newHandlerEnv(t, Options{})
	h.policy.set("alice", allowNamespaces("apps"))
	tests := []struct {
		name, user, topics string
		status             int
		body               string
	}{
		{"no session", "", "instance:apps/blog", http.StatusUnauthorized, "unauthenticated"},
		{"bad topic", "alice", "instance:apps", http.StatusBadRequest, "invalid topic"},
		{"reserved log topic", "alice", "log:apps/blog-0/server", http.StatusBadRequest, "not served"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := h.get(context.Background(), tc.user, tc.topics, "")
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tc.status || !strings.Contains(string(body), tc.body) {
				t.Errorf("status %d body %q, want %d containing %q", resp.StatusCode, body, tc.status, tc.body)
			}
		})
	}
	if n := h.policy.count(); n != 0 {
		t.Errorf("%d reviews sent for refused requests", n)
	}

	t.Run("method", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.srv.URL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != http.MethodGet {
			t.Errorf("POST: status %d, Allow %q", resp.StatusCode, resp.Header.Get("Allow"))
		}
	})

	t.Run("third stream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for range 2 {
			resp := h.get(ctx, "alice", "instance:apps/blog", "")
			t.Cleanup(func() { _ = resp.Body.Close() })
			(&sseReader{r: bufio.NewReader(resp.Body)}).next(t)
		}
		resp := h.get(ctx, "alice", "instance:apps/blog", "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("third stream: status %d", resp.StatusCode)
		}
	})
}

func TestStatusOf(t *testing.T) {
	for err, want := range map[error]int{
		ErrUnauthenticated:          http.StatusUnauthorized,
		ErrTooManyStreams:           http.StatusTooManyRequests,
		ErrTooManyTopics:            http.StatusBadRequest,
		ErrTopicNotServed:           http.StatusBadRequest,
		ErrNoStream:                 http.StatusNotFound,
		ErrClosed:                   http.StatusServiceUnavailable,
		errors.New("something new"): http.StatusInternalServerError,
	} {
		if got := statusOf(err); got != want {
			t.Errorf("statusOf(%v) = %d, want %d", err, got, want)
		}
	}
}
