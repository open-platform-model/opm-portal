package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// authorized for, the session that owns its streams, and when that session
// ends, which ends its streams; the zero time never does. Session is never
// logged.
type Principal struct {
	Identity authz.Identity
	Session  string
	Expires  time.Time
}

// Config wires a Server.
type Config struct {
	// Mode is where the portal runs: ModeLocal or ModeInCluster. Required,
	// so a caller that forgets it does not get the mode that serves more.
	// In ModeInCluster no document carries text the operator wrote.
	Mode Mode
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
	// Producers serves topic kinds the read API does not produce itself,
	// such as stream.KindLog, on the same stream. A producer that publishes
	// is given Server.Broker. An entry for one of the API's own kinds is
	// refused.
	Producers stream.Mux
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
	// Connection names what the read model reads the cluster with, for the
	// Cluster document. Required: its Source is v1.SourceKubeconfig or
	// v1.SourceInCluster.
	Connection Connection
}

// Connection names a mode's client configuration by its kubeconfig names
// only: never a server URL, a user entry or a credential (portal:D18:R1).
type Connection struct {
	// Source is v1.SourceKubeconfig or v1.SourceInCluster.
	Source string
	// Context and ClusterEntry are the kubeconfig context loaded and the
	// name of its cluster entry; ignored when Source is in-cluster.
	Context      string
	ClusterEntry string
}

func (c Connection) valid() bool {
	return c.Source == v1.SourceKubeconfig || c.Source == v1.SourceInCluster
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
// returns (nil for the stream and the topic change), and how it is served.
// The OpenAPI contract test reads this table.
type route struct {
	pattern string
	doc     any
	serve   func(s *Server, ctx context.Context, p Principal, r *http.Request) (any, error)
}

var routes = []route{
	{"/clusters/{cluster}", v1.Cluster{}, (*Server).getCluster},
	{"/clusters/{cluster}/instances", v1.InstanceList{}, (*Server).listInstances},
	{"/clusters/{cluster}/instances/{namespace}/{name}", v1.Instance{}, (*Server).getInstance},
	{"/clusters/{cluster}/instances/{namespace}/{name}/graph", v1.Graph{}, (*Server).instanceGraph},
	{"/clusters/{cluster}/instances/{namespace}/{name}/events", v1.EventList{}, (*Server).instanceEvents},
	{"/clusters/{cluster}/instances/{namespace}/{name}/object", v1.Object{}, (*Server).instanceObject},
	{"/clusters/{cluster}/packages", v1.PackageList{}, (*Server).listPackages},
	{"/clusters/{cluster}/packages/{namespace}/{name}", v1.Package{}, (*Server).getPackage},
	{"/clusters/{cluster}/packages/{namespace}/{name}/graph", v1.Graph{}, (*Server).packageGraph},
	{"/clusters/{cluster}/packages/{namespace}/{name}/events", v1.EventList{}, (*Server).packageEvents},
	{"/clusters/{cluster}/packages/{namespace}/{name}/object", v1.Object{}, (*Server).packageObject},
	{"/clusters/{cluster}/platform", v1.Platform{}, (*Server).getPlatform},
	{"/clusters/{cluster}/platform/graph", v1.Graph{}, (*Server).platformGraph},
	{"/clusters/{cluster}/platform/events", v1.EventList{}, (*Server).platformEvents},
	{"/clusters/{cluster}/platform/registrations/{name}/events", v1.EventList{}, (*Server).registrationEvents},
	{streamPattern, nil, nil},
	{topicsPattern, nil, nil},
}

const (
	streamPattern = "/clusters/{cluster}/stream"
	// topicsPattern is the one resource that takes POST: it changes the
	// topics of an open stream and reads nothing.
	topicsPattern = "/clusters/{cluster}/stream/{stream}/topics"
)

// New returns a Server over cfg. Model, Authorizer and Authenticate are
// required.
func New(cfg Config) (*Server, error) {
	switch {
	case !cfg.Mode.valid():
		return nil, fmt.Errorf("read api: mode %q is neither %q nor %q", cfg.Mode, ModeLocal, ModeInCluster)
	case cfg.Model == nil:
		return nil, errors.New("read api: no read model")
	case cfg.Authorizer == nil:
		return nil, errors.New("read api: no authorizer")
	case cfg.Authenticate == nil:
		return nil, errors.New("read api: no authenticator")
	case !cfg.Connection.valid():
		return nil, fmt.Errorf("read api: connection source %q is neither %q nor %q", cfg.Connection.Source, v1.SourceKubeconfig, v1.SourceInCluster)
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
	routed := make(stream.Mux, len(ownKinds)+len(cfg.Producers))
	for _, k := range ownKinds {
		routed[k] = s.producer
	}
	for k, p := range cfg.Producers {
		if _, own := routed[k]; own {
			return nil, fmt.Errorf("read api: the read api produces %q topics itself", k)
		}
		if p != nil {
			routed[k] = p
		}
	}
	s.broker = stream.New(routed, cfg.Authorizer, cfg.Stream)
	s.producer.start(s.broker)
	streamHandler := stream.NewHandler(s.broker, func(r *http.Request) (stream.Session, error) {
		p, ok := principalFrom(r.Context())
		if !ok {
			return stream.Session{}, stream.ErrUnauthenticated
		}
		return stream.Session{Key: p.Session, Identity: p.Identity, Expires: p.Expires}, nil
	}, stream.HandlerOptions{Error: func(w http.ResponseWriter, r *http.Request, _ int, err error) {
		writeProblem(w, r, s.log, err)
	}})

	s.mount(streamHandler)
	return s, nil
}

// mount registers every route of the table, and the not-found answer for
// every other path.
func (s *Server) mount(streamHandler http.Handler) {
	for _, rt := range routes {
		switch rt.pattern {
		case streamPattern:
			s.mux.HandleFunc(Prefix+rt.pattern, s.guard(streamHandler.ServeHTTP))
			continue
		case topicsPattern:
			s.mux.HandleFunc(Prefix+rt.pattern, s.guardMethod(http.MethodPost, s.changeTopics))
			continue
		}
		s.mux.HandleFunc(Prefix+rt.pattern, s.guard(s.document(rt.serve)))
	}
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, r, s.log, notFound(detailUnknownPath))
	})
}

// ownKinds are the topic kinds the read API's own producer serves.
var ownKinds = []stream.Kind{
	stream.KindPlatform, stream.KindInstances, stream.KindInstance,
	stream.KindPackage, stream.KindRegistration, stream.KindEvents,
}

// Broker returns the change stream's broker, for a producer in
// Config.Producers that publishes to it.
func (s *Server) Broker() *stream.Broker { return s.broker }

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
// handler runs, so no review is sent for it (portal:D6:R2).
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
	return s.guardMethod(http.MethodGet, next)
}

// guardMethod refuses a method other than method and a cluster the portal
// does not serve, before any review.
func (s *Server) guardMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeProblem(w, r, s.log, &apiError{status: http.StatusMethodNotAllowed, code: v1.CodeMethodNotAllowed, detail: detailMethod, allow: method})
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
		if err == nil {
			doc, err = s.forMode(doc)
		}
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

// forMode returns doc as the server's mode serves it. Every document leaves
// the server through here: the GET handlers' and the change stream's.
func (s *Server) forMode(doc any) (any, error) {
	if s.cfg.Mode == ModeInCluster {
		return omitOperatorText(doc)
	}
	return doc, nil
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
