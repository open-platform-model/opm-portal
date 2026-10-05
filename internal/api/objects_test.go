package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// TestObjectOnlyWhenReached (portal:D7:R4): an object no inventory reaches,
// a kind the cluster does not serve, and a Secret all get the forbidden
// problem a forbidden caller gets, and a Secret is never asked about or
// read.
func TestObjectOnlyWhenReached(t *testing.T) {
	forbiddenBody := newEnv(t, loadF1(t), denyAll).get(t, base+"/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo").body

	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, path := range []string{
		base + "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=cert-manager&name=cert-manager-webhook",
		base + "/instances/default/podinfo/object?group=example.com&kind=Widget&namespace=default&name=w",
		base + "/instances/default/podinfo/object?kind=Secret&namespace=default&name=podinfo",
		base + "/packages/pkg/podinfo/object?group=apps&kind=Deployment&namespace=cert-manager&name=cert-manager-webhook",
	} {
		res := e.get(t, path)
		if res.status != http.StatusForbidden || !bytes.Equal(res.body, forbiddenBody) {
			t.Errorf("GET %s = %d %s, want the forbidden problem", path, res.status, res.body)
		}
	}
	for _, a := range e.az.callerAsked() {
		if a.Resource.Resource == "secrets" {
			t.Errorf("a Secret was reviewed: %+v", a)
		}
	}
	for _, line := range readmodeltest.Actions(e.client) {
		if strings.Contains(line, "secrets") || strings.HasPrefix(line, "get deployments cert-manager") {
			t.Errorf("an object outside the request's reach was read: %s", line)
		}
	}
}

// TestObjectAuthorizesEveryReadFirst: get on the owner, then get on the
// object, both before anything is read; a denied object get reads nothing.
func TestObjectAuthorizesEveryReadFirst(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("deployments"))
	path := base + "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo"
	expectProblem(t, e.get(t, path), http.StatusForbidden, v1.CodeForbidden)
	asked := e.az.callerAsked()
	got := make([]string, 0, len(asked))
	for _, a := range asked {
		got = append(got, a.Verb+" "+a.Resource.Resource+" "+a.Namespace+"/"+a.Name)
	}
	if want := []string{"get moduleinstances default/podinfo", "get deployments default/podinfo-podinfo"}; !slices.Equal(got, want) {
		t.Errorf("checks = %v, want %v", got, want)
	}
	for _, line := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(line, "get deployments") {
			t.Errorf("the object was read for a caller who may not get it: %s", line)
		}
	}
}

// TestObjectNeedsAName: the object resource names one object.
func TestObjectNeedsAName(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	expectProblem(t, e.get(t, base+"/instances/default/podinfo/object"), http.StatusBadRequest, v1.CodeBadRequest)
}

// openEvent reads the first event of c, the open event, and returns the
// stream id it carries.
func openEvent(t *testing.T, c *client) string {
	t.Helper()
	select {
	case ev := <-c.events:
		if ev.event != stream.EventOpen {
			t.Fatalf("first event %q, want open", ev.event)
		}
		var open struct {
			Stream string `json:"stream"`
		}
		if err := json.Unmarshal(ev.data, &open); err != nil || open.Stream == "" {
			t.Fatalf("open event %s: %v", ev.data, err)
		}
		return open.Stream
	case <-time.After(5 * time.Second):
		t.Fatal("no open event")
	}
	return ""
}

func postTopics(t *testing.T, ts *httptest.Server, id, contentType, body string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+base+"/stream/"+id+"/topics", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	return readResponse(t, res)
}

// TestTopicChangeMovesAStream: a stream on platform is moved to podinfo's
// instance topic by one request.
func TestTopicChangeMovesAStream(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "platform")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	id := openEvent(t, c)
	c.next(t, func(ev sse, m message) bool { return ev.event == stream.EventSnapshot && m.Topic == "platform" })

	got := postTopics(t, ts, id, "application/json", `{"add":["instance:default/podinfo"],"remove":["platform"]}`)
	if got.status != http.StatusNoContent {
		t.Fatalf("topic change = %d %s", got.status, got.body)
	}
	m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
	if m.Topic != "instance:default/podinfo" {
		t.Errorf("snapshot of %q, want the instance topic", m.Topic)
	}
}

// TestTopicChangeRefusals: a form body, a bad topic, another session's
// stream, and a GET are refused, and none changes the stream.
func TestTopicChangeRefusals(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "platform")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	id := openEvent(t, c)

	expectProblem(t, postTopics(t, ts, id, "application/x-www-form-urlencoded", "add=instance%3Adefault%2Fpodinfo"), http.StatusBadRequest, v1.CodeBadRequest)
	expectProblem(t, postTopics(t, ts, id, "application/json", `{"add":["nope:x"]}`), http.StatusBadRequest, v1.CodeBadRequest)
	expectProblem(t, postTopics(t, ts, id, "application/json", `{"add":[],"extra":1}`), http.StatusBadRequest, v1.CodeBadRequest)
	expectProblem(t, postTopics(t, ts, "no-such-stream", "application/json", `{"add":["platform"]}`), http.StatusNotFound, v1.CodeNotFound)

	get := e.get(t, base+"/stream/"+id+"/topics")
	expectProblem(t, get, http.StatusMethodNotAllowed, v1.CodeMethodNotAllowed)
	if allow := get.header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want POST", allow)
	}

	e.principal = Principal{Identity: e.principal.Identity, Session: "session-other"}
	expectProblem(t, postTopics(t, ts, id, "application/json", `{"add":["instance:default/podinfo"]}`), http.StatusNotFound, v1.CodeNotFound)
}
