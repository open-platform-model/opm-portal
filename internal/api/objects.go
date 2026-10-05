package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// maxTopicChangeBytes bounds a topic-change body, well above the longest
// valid list the per-stream topic cap admits.
const maxTopicChangeBytes = 64 << 10

func (s *Server) instanceObject(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, instanceOwner)
	if err != nil {
		return nil, err
	}
	return s.ownerObject(ctx, p.Identity, o, r)
}

func (s *Server) packageObject(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, packageOwner)
	if err != nil {
		return nil, err
	}
	return s.ownerObject(ctx, p.Identity, o, r)
}

// ownerObject serves one object the owner's inventory reaches, for a YAML
// view. Every read is authorized before any lookup, in the order the events
// of one object are: get on the owner, then get on the object. A core
// Secret is refused before anything is asked about it, and a kind the
// cluster does not serve or an object the owner does not reach gets the
// forbidden problem a forbidden caller gets (0030:D7:R4, 0030:D8:R1).
func (s *Server) ownerObject(ctx context.Context, who authz.Identity, o owner, r *http.Request) (v1.Object, error) {
	about, named, err := regardingQuery(r)
	if err != nil {
		return v1.Object{}, err
	}
	if !named {
		return v1.Object{}, badRequest("an object is named by kind and name, with group and namespace where it has them")
	}
	if about.Group == "" && about.Kind == "Secret" {
		return v1.Object{}, forbidden()
	}
	og, err := s.authorize(ctx, who, o.get())
	if err != nil {
		return v1.Object{}, err
	}
	kind, err := s.cfg.Model.ResolveKind(about.Group, about.Kind)
	if err != nil {
		return v1.Object{}, forbidden()
	}
	if !kind.Namespaced {
		about.Namespace = ""
	}
	about.Version = kind.Resource.Version
	g, err := s.authorize(ctx, who, authz.Attributes{Verb: verbGet, Resource: kind.Resource, Namespace: about.Namespace, Name: about.Name})
	if err != nil {
		return v1.Object{}, err
	}
	if err := s.ownerReaches(ctx, who, og[0], o, about, true); err != nil {
		return v1.Object{}, err
	}
	obj, err := s.cfg.Model.Object(ctx, who, g[0], kind, about)
	if err != nil {
		return v1.Object{}, err
	}
	return v1.Object{TypeMeta: meta(v1.KindObject), Ref: objectRef(about), Object: obj}, nil
}

// changeTopics adds and removes topics on one of the session's open
// streams. It reads nothing from the cluster: added topics are authorized
// by the broker as at open, and a denied one is closed on the stream. The
// body must be JSON, so a cross-site form cannot send it without a CORS
// preflight the portal never grants; the front door's cross-origin check
// refuses a cross-origin POST before it gets here.
func (s *Server) changeTopics(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r.Context())
	if !ok {
		writeProblem(w, r, s.log, &apiError{status: http.StatusUnauthorized, code: v1.CodeUnauthenticated, detail: detailUnauthenticated})
		return
	}
	change, err := topicChange(r)
	if err != nil {
		writeProblem(w, r, s.log, err)
		return
	}
	session := stream.Session{Key: p.Session, Identity: p.Identity, Expires: p.Expires}
	id := r.PathValue("stream")
	if len(change.add) > 0 {
		if err := s.broker.Subscribe(r.Context(), session, id, change.add...); err != nil {
			writeProblem(w, r, s.log, err)
			return
		}
	}
	if len(change.remove) > 0 {
		if err := s.broker.Unsubscribe(session, id, change.remove...); err != nil {
			writeProblem(w, r, s.log, err)
			return
		}
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

type parsedChange struct {
	add, remove []stream.Topic
}

// topicChange reads and parses a topic-change body.
func topicChange(r *http.Request) (parsedChange, error) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return parsedChange{}, badRequest("a topic change is a JSON body (application/json)")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxTopicChangeBytes))
	dec.DisallowUnknownFields()
	var body v1.TopicChange
	if err := dec.Decode(&body); err != nil {
		return parsedChange{}, badRequest("a topic change is {\"add\": [topic...], \"remove\": [topic...]}")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return parsedChange{}, badRequest("a topic change is one JSON object")
	}
	var out parsedChange
	for _, list := range []struct {
		names []string
		into  *[]stream.Topic
	}{{body.Add, &out.add}, {body.Remove, &out.remove}} {
		for _, name := range list.names {
			t, err := stream.ParseTopic(name)
			if err != nil {
				return parsedChange{}, err
			}
			*list.into = append(*list.into, t)
		}
	}
	return out, nil
}
