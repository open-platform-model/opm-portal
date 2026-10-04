package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Event names of the messages a stream carries.
const (
	EventOpen      = "open"
	EventSnapshot  = "snapshot"
	EventUpsert    = "upsert"
	EventDelete    = "delete"
	EventK8sEvent  = "k8sevent"
	EventLog       = "log"
	EventLogEnd    = "logend"
	EventClosed    = "closed"
	EventHeartbeat = "heartbeat"
)

// Item is one change on a topic, or one entry of a snapshot.
//
// Attrs is the read the item reveals, such as get on the ModuleInstance it
// describes on an instance topic, or list of the instance's namespace on a
// list topic. The broker delivers the item only to a subscriber allowed that
// read. The payload is Data when it reads the same for every allowed
// subscriber, or Render when it depends on the reader (for example a health
// roll-up that is partial for a reader who cannot see every object). Exactly
// one of the two is set. The payload is a read API document; the broker
// compacts it and never inspects it.
type Item struct {
	Event  string
	Attrs  authz.Attributes
	Data   json.RawMessage
	Render func(ctx context.Context, who authz.Identity) (json.RawMessage, error)
}

func (it Item) validate() error {
	switch it.Event {
	case EventUpsert, EventDelete, EventK8sEvent, EventLog, EventLogEnd:
	default:
		return fmt.Errorf("item event %q is not upsert, delete, k8sevent, log or logend", it.Event)
	}
	if it.Attrs.Verb == "" || it.Attrs.Resource.Resource == "" {
		// An item without the read it reveals could not be authorized.
		return errors.New("item names no read to authorize")
	}
	if (it.Data == nil) == (it.Render == nil) {
		return errors.New("item must set exactly one of Data and Render")
	}
	return nil
}
