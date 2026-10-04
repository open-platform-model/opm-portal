package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// sse is one server-sent event.
type sse struct {
	event string
	data  []byte
}

// message is the body of a snapshot or item event.
type message struct {
	Topic string            `json:"topic"`
	Items []json.RawMessage `json:"items"`
	Item  json.RawMessage   `json:"item"`
	Code  string            `json:"code"`
}

// client is an open stream read in the background.
type client struct {
	events chan sse
	cancel context.CancelFunc
}

func fastStream(cfg *Config) {
	cfg.Coalesce = 20 * time.Millisecond
	cfg.Stream.HeartbeatInterval = time.Hour
}

// openStream opens a stream on topics through a real HTTP server, so events
// are flushed as they would be to a browser. It returns the response when
// the stream did not open.
func openStream(t *testing.T, ts *httptest.Server, topics string) (*client, *response) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+base+"/stream?topics="+topics, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		defer cancel()
		refused := readResponse(t, res)
		return nil, &refused
	}
	c := &client{events: make(chan sse, 64), cancel: cancel}
	t.Cleanup(func() { cancel(); _ = res.Body.Close() })
	go func() {
		defer close(c.events)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<22)
		var cur sse
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if cur.event != "" {
					c.events <- cur
				}
				cur = sse{}
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = []byte(strings.TrimPrefix(line, "data: "))
			}
		}
	}()
	return c, nil
}

// next returns the next event other than open and heartbeat that match
// accepts, failing after a timeout.
func (c *client) next(t *testing.T, accept func(sse, message) bool) message {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-c.events:
			if !ok {
				t.Fatal("the stream ended")
			}
			if ev.event == stream.EventOpen || ev.event == stream.EventHeartbeat {
				continue
			}
			var m message
			if err := json.Unmarshal(ev.data, &m); err != nil {
				t.Fatalf("event %s: %v", ev.event, err)
			}
			if accept(ev, m) {
				return m
			}
		case <-timeout:
			t.Fatal("no matching event")
		}
	}
}

func compact(t *testing.T, raw []byte) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestSnapshotsAreTheGETDocuments (0030:D2:R5).
func TestSnapshotsAreTheGETDocuments(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	topics := map[string]string{
		"platform":                        base + "/platform",
		"instances:default":               base + "/instances?namespace=default",
		"instance:default/podinfo":        base + "/instances/default/podinfo",
		"package:pkg/podinfo":             base + "/packages/pkg/podinfo",
		"events:instance:default/podinfo": base + "/instances/default/podinfo/events",
		"events:registration:default.refused-claim-fixture": base + "/platform/registrations/default.refused-claim-fixture/events",
	}
	names := make([]string, 0, len(topics))
	for name := range topics {
		names = append(names, name)
	}
	c, res := openStream(t, ts, strings.Join(names, ","))
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	for range topics {
		m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
		path, ok := topics[m.Topic]
		if !ok || len(m.Items) != 1 {
			t.Fatalf("snapshot %s with %d items", m.Topic, len(m.Items))
		}
		want := compact(t, e.get(t, path).body)
		if got := compact(t, m.Items[0]); got != want {
			t.Errorf("snapshot of %s differs from GET %s:\n%s\n%s", m.Topic, path, got, want)
		}
	}
}

// TestInstanceTopicFollowsAPodBreaking: a Pod that starts waiting on
// ErrImagePull reaches the instance topic as a Degraded upsert while the
// instance stays Applied (0030:D3:R3).
func TestInstanceTopicFollowsAPodBreaking(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "instance:default/podinfo")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })

	pods := podsGVR
	breakPod := func() error {
		pod, err := e.client.Resource(pods).Namespace("default").Get(t.Context(), "podinfo-podinfo-d9585d794-4lg6h", metav1.GetOptions{})
		if err != nil {
			return err
		}
		statuses, _, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
		cs := statuses[0].(map[string]any)
		cs["ready"] = false
		cs["state"] = map[string]any{"waiting": map[string]any{"reason": "ErrImagePull", "message": "manifest unknown"}}
		statuses[0] = cs
		if err := unstructured.SetNestedSlice(pod.Object, statuses, "status", "containerStatuses"); err != nil {
			return err
		}
		_, err = e.client.Resource(pods).Namespace("default").Update(t.Context(), pod, metav1.UpdateOptions{})
		return err
	}
	// The children watch starts with the topic; retry the break until the
	// update reaches it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := breakPod(); err != nil {
			t.Fatal(err)
		}
		var doc v1.Instance
		got := false
		select {
		case ev := <-c.events:
			var m message
			if json.Unmarshal(ev.data, &m) == nil && ev.event == stream.EventUpsert && json.Unmarshal(m.Item, &doc) == nil {
				got = doc.Health.State == "Degraded"
			}
		case <-time.After(200 * time.Millisecond):
		}
		if got {
			if doc.Reconcile.State != "Applied" {
				t.Errorf("reconcile = %q, want Applied", doc.Reconcile.State)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no Degraded upsert")
		}
	}
}

func TestDeletedInstanceIsRemoved(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "instance:default/podinfo,instance:default/nothing")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	snapshots := map[string]json.RawMessage{}
	for len(snapshots) < 2 {
		m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
		snapshots[m.Topic] = m.Items[0]
	}
	var gone v1.Removed
	if err := json.Unmarshal(snapshots["instance:default/nothing"], &gone); err != nil || gone.Kind != v1.KindRemoved || gone.Ref.Name != "nothing" {
		t.Errorf("snapshot of a missing instance = %s", snapshots["instance:default/nothing"])
	}
	if err := e.client.Resource(instancesGVR).Namespace("default").Delete(t.Context(), "podinfo", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventDelete })
	if err := json.Unmarshal(m.Item, &gone); err != nil || gone.Kind != v1.KindRemoved || gone.Ref.Name != "podinfo" || gone.Ref.Kind != kindModuleInstance {
		t.Errorf("delete item = %s", m.Item)
	}
}

func TestForbiddenTopicCloses(t *testing.T) {
	denyDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Namespace != "default"
	}
	e := newEnv(t, loadF1(t), denyDefault, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "instance:default/podinfo,instance:web/web")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventClosed })
	if m.Topic != "instance:default/podinfo" || m.Code != v1.CodeForbidden {
		t.Errorf("closed = %+v", m)
	}
	c.next(t, func(ev sse, m message) bool { return ev.event == stream.EventSnapshot && m.Topic == "instance:web/web" })
}

func TestStreamRefusals(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream, func(cfg *Config) { cfg.Stream.MaxStreamsPerSession = 1 })
	ts := newHTTPServer(t, e)
	for topics, code := range map[string]string{
		"registration:default.backup-provider": v1.CodeBadRequest,
		"instance:Default/x":                   v1.CodeBadRequest,
		"log:default/p/c":                      v1.CodeBadRequest,
	} {
		_, res := openStream(t, ts, topics)
		if res == nil {
			t.Fatalf("%s: stream opened", topics)
		}
		expectProblem(t, *res, http.StatusBadRequest, code)
	}
	if c, res := openStream(t, ts, "platform"); c == nil {
		t.Fatalf("first stream refused: %d %s", res.status, res.body)
	}
	_, res := openStream(t, ts, "platform")
	if res == nil {
		t.Fatal("a second stream opened past the session cap")
	}
	expectProblem(t, *res, http.StatusTooManyRequests, v1.CodeTooManyStreams)
}

func readResponse(t *testing.T, res *http.Response) response {
	t.Helper()
	defer func() { _ = res.Body.Close() }()
	var b bytes.Buffer
	if _, err := b.ReadFrom(res.Body); err != nil {
		t.Fatal(err)
	}
	return response{status: res.StatusCode, header: res.Header, body: b.Bytes()}
}

// newHTTPServer serves e over HTTP. It closes after the streams the test
// opened (cleanups run last-registered first), which it waits for.
func newHTTPServer(t *testing.T, e *env) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(e.srv)
	t.Cleanup(ts.Close)
	return ts
}
