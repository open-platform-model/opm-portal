package stream

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

var instancesGVR = InstancesResource

func TestItemValidate(t *testing.T) {
	read := authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "apps", Name: "blog"}
	render := func(context.Context, authz.Identity) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	tests := []struct {
		name string
		item Item
		ok   bool
	}{
		{"upsert with data", Item{Event: EventUpsert, Attrs: read, Data: json.RawMessage(`{}`)}, true},
		{"delete with render", Item{Event: EventDelete, Attrs: read, Render: render}, true},
		{"k8sevent", Item{Event: EventK8sEvent, Attrs: read, Data: json.RawMessage(`{}`)}, true},
		{"snapshot is not an item event", Item{Event: EventSnapshot, Attrs: read, Data: json.RawMessage(`{}`)}, false},
		{"no event", Item{Attrs: read, Data: json.RawMessage(`{}`)}, false},
		{"no read", Item{Event: EventUpsert, Data: json.RawMessage(`{}`)}, false},
		{"no payload", Item{Event: EventUpsert, Attrs: read}, false},
		{"both payloads", Item{Event: EventUpsert, Attrs: read, Data: json.RawMessage(`{}`), Render: render}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.item.validate(); (err == nil) != tc.ok {
				t.Errorf("validate() = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
