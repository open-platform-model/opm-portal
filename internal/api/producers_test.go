package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// logStub serves log topics with one fixed message, standing in for the
// logs producer a mode routes through Config.Producers.
type logStub struct {
	activations atomic.Int32
}

func (*logStub) read(t stream.Topic) authz.Attributes {
	return authz.Attributes{
		Verb: "get", Resource: schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		Subresource: "log", Namespace: t.Namespace(), Name: t.Name(),
	}
}

func (l *logStub) Attributes(t stream.Topic) ([]authz.Attributes, bool) {
	if t.Kind() != stream.KindLog {
		return nil, false
	}
	return []authz.Attributes{l.read(t)}, true
}

func (l *logStub) Snapshot(_ context.Context, t stream.Topic) ([]stream.Item, error) {
	data, err := json.Marshal(map[string]any{"seq": 1, "type": "line", "container": t.Container(), "text": "hello"})
	if err != nil {
		return nil, err
	}
	return []stream.Item{{Event: stream.EventLog, Attrs: l.read(t), Data: data}}, nil
}

func (l *logStub) Activate(stream.Topic) func() {
	l.activations.Add(1)
	return func() {}
}

func TestProducersServeLogTopicsOnTheStream(t *testing.T) {
	stub := &logStub{}
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream, func(cfg *Config) {
		cfg.Producers = stream.Mux{stream.KindLog: stub}
	})
	if e.srv.Broker() == nil {
		t.Fatal("Broker() = nil")
	}
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "log:default/podinfo-0/podinfo,platform")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	m := c.next(t, func(ev sse, m message) bool {
		return ev.event == stream.EventSnapshot && m.Topic == "log:default/podinfo-0/podinfo"
	})
	if len(m.Items) != 1 || !strings.Contains(string(m.Items[0]), `"hello"`) {
		t.Fatalf("log snapshot items = %s", m.Items)
	}
	// The API's own topics still reach its own producer.
	c.next(t, func(ev sse, m message) bool { return ev.event == stream.EventSnapshot && m.Topic == "platform" })
	if stub.activations.Load() != 1 {
		t.Fatalf("log producer activations = %d; want 1", stub.activations.Load())
	}
}

func TestProducersCannotClaimTheAPIKinds(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, k := range ownKinds {
		cfg := e.srv.cfg
		cfg.Producers = stream.Mux{k: &logStub{}}
		if srv, err := New(cfg); err == nil {
			srv.Close()
			t.Errorf("New with a %q producer succeeded; want an error", k)
		}
	}
}
