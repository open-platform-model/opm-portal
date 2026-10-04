package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// Prefix is the path every read API resource lives under.
const Prefix = "/api/v1alpha1"

// DefaultCluster is the only cluster milestone 1 serves. Paths carry the
// cluster so more can be added without a new route set.
const DefaultCluster = "default"

// Principal is who a request is made by: the identity every read is
// authorized for, and the session that owns its streams. Session is never
// logged.
type Principal struct {
	Identity authz.Identity
	Session  string
}

// Config wires a Server.
type Config struct {
	// Model is the started read model every document is built from.
	Model *readmodel.Model
	// Authorizer authorizes every read for the request's identity.
	Authorizer authz.Authorizer
	// Authenticate resolves a request's principal. It is the mode's seam:
	// local mode answers from its session cookie. An error, an identity
	// that names no principal, or an empty session is refused with 401
	// before anything else happens.
	Authenticate func(*http.Request) (Principal, error)
	// Reader is the identity the read model reads as. The stream producer
	// holds runtime-children watches as it while a topic is followed.
	Reader authz.Identity
	// Stream tunes the change stream's broker.
	Stream stream.Options
	// Coalesce is how long the producer gathers changes before it publishes
	// them. Default 250 ms.
	Coalesce time.Duration
	// Refresh is how often a followed topic is rendered again whatever the
	// change feed says, for events and polled objects. Default 30 s.
	Refresh time.Duration
	// Logger receives operational logs. Default: discarded.
	Logger *slog.Logger
}

// Server serves the read API. It is an http.Handler; Close stops its
// change stream.
type Server struct {
	cfg      Config
	log      *slog.Logger
	mux      *http.ServeMux
	producer *producer
	broker   *stream.Broker
}

// route is one resource: its pattern below Prefix, the wire type its 200
// returns (nil for the stream), and how it is served. The OpenAPI contract
// test reads this table.
type route struct {
	pattern string
	doc     any
	serve   func(s *Server, ctx context.Context, p Principal, r *http.Request) (any, error)
}

var routes = []route{
	{"/clusters/{cluster}/instances", v1.InstanceList{}, (*Server).listInstances},
	{"/clusters/{cluster}/instances/{namespace}/{name}", v1.Instance{}, (*Server).getInstance},
	{"/clusters/{cluster}/instances/{namespace}/{name}/graph", v1.Graph{}, (*Server).instanceGraph},
	{"/clusters/{cluster}/instances/{namespace}/{name}/events", v1.EventList{}, (*Server).instanceEvents},
	{"/clusters/{cluster}/packages", v1.PackageList{}, (*Server).listPackages},
	{"/clusters/{cluster}/packages/{namespace}/{name}", v1.Package{}, (*Server).getPackage},
	{"/clusters/{cluster}/packages/{namespace}/{name}/graph", v1.Graph{}, (*Server).packageGraph},
	{"/clusters/{cluster}/packages/{namespace}/{name}/events", v1.EventList{}, (*Server).packageEvents},
	{"/clusters/{cluster}/platform", v1.Platform{}, (*Server).getPlatform},
	{"/clusters/{cluster}/platform/graph", v1.Graph{}, (*Server).platformGraph},
	{"/clusters/{cluster}/platform/events", v1.EventList{}, (*Server).platformEvents},
	{"/clusters/{cluster}/platform/registrations/{name}/events", v1.EventList{}, (*Server).registrationEvents},
	{streamPattern, nil, nil},
}

const streamPattern = "/clusters/{cluster}/stream"

// New returns a Server over cfg. Model, Authorizer and Authenticate are
// required.
func New(cfg Config) (*Server, error) {
	switch {
	case cfg.Model == nil:
		return nil, errors.New("read api: no read model")
	case cfg.Authorizer == nil:
		return nil, errors.New("read api: no authorizer")
	case cfg.Authenticate == nil:
		return nil, errors.New("read api: no authenticator")
	}
	if cfg.Coalesce <= 0 {
		cfg.Coalesce = 250 * time.Millisecond
	}
	if cfg.Refresh <= 0 {
		cfg.Refresh = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Stream.Logger == nil {
		cfg.Stream.Logger = cfg.Logger
	}
	s := &Server{cfg: cfg, log: cfg.Logger, mux: http.NewServeMux()}
	s.producer = newProducer(s)
	s.broker = stream.New(s.producer, cfg.Authorizer, cfg.Stream)
	s.producer.start(s.broker)
	streamHandler := stream.NewHandler(s.broker, func(r *http.Request) (stream.Session, error) {
		p, ok := principalFrom(r.Context())
		if !ok {
			return stream.Session{}, stream.ErrUnauthenticated
		}
		return stream.Session{Key: p.Session, Identity: p.Identity}, nil
	}, stream.HandlerOptions{Error: func(w http.ResponseWriter, r *http.Request, _ int, err error) {
		writeProblem(w, r, s.log, err)
	}})

	for _, rt := range routes {
		if rt.pattern == streamPattern {
			s.mux.HandleFunc(Prefix+rt.pattern, s.guard(streamHandler.ServeHTTP))
			continue
		}
		s.mux.HandleFunc(Prefix+rt.pattern, s.guard(s.document(rt.serve)))
	}
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, r, s.log, notFound(detailUnknownPath))
	})
	return s, nil
}

// Close stops the change stream: every stream ends and the producer stops.
func (s *Server) Close() {
	s.broker.Close()
	s.producer.stop()
}

type principalKey struct{}

func principalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// ServeHTTP authenticates the request, then routes it. No principal, an
// identity that names no one, or an empty session is refused before any
// handler runs, so no review is sent for it (0030:D6:R2).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := s.cfg.Authenticate(r)
	if err != nil || !p.Identity.Authenticated() || strings.TrimSpace(p.Session) == "" {
		writeProblem(w, r, s.log, &apiError{status: http.StatusUnauthorized, code: v1.CodeUnauthenticated, detail: detailUnauthenticated})
		return
	}
	s.mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
}

// guard refuses a method other than GET and a cluster the portal does not
// serve, before any review.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, r, s.log, &apiError{status: http.StatusMethodNotAllowed, code: v1.CodeMethodNotAllowed, detail: detailMethod})
			return
		}
		if r.PathValue("cluster") != DefaultCluster {
			writeProblem(w, r, s.log, notFound(detailUnknownCluster))
			return
		}
		next(w, r)
	}
}

// document runs a resource and writes its document or its problem.
func (s *Server) document(serve func(*Server, context.Context, Principal, *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			writeProblem(w, r, s.log, &apiError{status: http.StatusUnauthorized, code: v1.CodeUnauthenticated, detail: detailUnauthenticated})
			return
		}
		doc, err := serve(s, r.Context(), p, r)
		if err != nil {
			writeProblem(w, r, s.log, err)
			return
		}
		body, err := json.Marshal(doc)
		if err != nil {
			writeProblem(w, r, s.log, err)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body); err != nil {
			s.log.Debug("writing a document", "path", r.URL.Path, "error", err)
		}
	}
}

// authorize checks every read for who, in order, and returns their grants.
// The first refusal is returned as is; classify turns it into the problem.
func (s *Server) authorize(ctx context.Context, who authz.Identity, reads ...authz.Attributes) ([]authz.Grant, error) {
	grants := make([]authz.Grant, 0, len(reads))
	for _, a := range reads {
		g, err := s.cfg.Authorizer.Check(ctx, who, a)
		if err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, nil
}
