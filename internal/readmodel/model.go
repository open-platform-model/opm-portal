package readmodel

import (
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Errors a read returns. None of their texts names an object or an identity,
// so a caller receives the same refusal whether or not the object exists.
var (
	// ErrNotCovered: the caller's grant does not cover the read. It wraps
	// authz.ErrNoGrant.
	ErrNotCovered = errors.New("read not covered by the caller's grant")
	// ErrNotFound: the read is covered and the object does not exist.
	ErrNotFound = errors.New("object not found")
	// ErrUnavailable: the read model does not hold the kind in the scope the
	// read needs, because the reader may not watch it or its cache has not
	// synced. It is never answered with an empty result.
	ErrUnavailable = errors.New("kind not available to the read model")
)

// Defaults for the zero fields of Config.
const (
	defaultIdleTimeout  = 5 * time.Minute
	defaultPollInterval = 30 * time.Second
	defaultPollWorkers  = 8
	defaultSyncTimeout  = 10 * time.Second
	defaultChildrenTTL  = 10 * time.Second

	// tunedQPS and tunedBurst replace client-go's defaults (5 and 10). A
	// cold instance read lists one resource per inventory kind, and the
	// capture measured 7-9 s for a per-request graph at QPS 5 (0030:D3:R9).
	tunedQPS   = 50
	tunedBurst = 100
)

// Config wires a Model to a cluster. Dynamic, Discovery and Authorizer are
// required, and Reader must name a principal.
type Config struct {
	// Dynamic is the client every cluster read goes through. It reads as
	// Reader.
	Dynamic dynamic.Interface
	// Discovery resolves inventory kinds to resources. It is cached for
	// the Model's lifetime and refreshed when a kind is not found.
	Discovery discovery.DiscoveryInterface
	// Authorizer issues every grant: the caller's for reads inside a view,
	// and Reader's before an informer, a poll or an on-demand list.
	Authorizer authz.Authorizer
	// Reader is the identity Dynamic authenticates as: the kubeconfig's
	// user in local mode.
	Reader authz.Identity
	// Namespaces limits the namespaced OPM kinds to these namespaces. Empty
	// means cluster-wide.
	Namespaces []string

	// IdleTimeout stops an inventory kind's watch after no read used it for
	// this long. Default 5 minutes.
	IdleTimeout time.Duration
	// PollInterval is how often an object that cannot be watched is read
	// again. Default 30 seconds.
	PollInterval time.Duration
	// PollWorkers bounds concurrent polls per kind. Default 8.
	PollWorkers int
	// SyncTimeout bounds the wait for a new informer's first list. Default
	// 10 seconds.
	SyncTimeout time.Duration
	// ChildrenTTL is how long an on-demand list of runtime children is
	// reused when no one holds interest in the namespace. Default 10 seconds.
	ChildrenTTL time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

// withDefaults returns c with every zero tuning field at its default.
func (c Config) withDefaults() Config {
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = defaultIdleTimeout
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.PollWorkers <= 0 {
		c.PollWorkers = defaultPollWorkers
	}
	if c.SyncTimeout <= 0 {
		c.SyncTimeout = defaultSyncTimeout
	}
	if c.ChildrenTTL <= 0 {
		c.ChildrenTTL = defaultChildrenTTL
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

func (c Config) validate() error {
	switch {
	case c.Dynamic == nil:
		return errors.New("read model: no dynamic client")
	case c.Discovery == nil:
		return errors.New("read model: no discovery client")
	case c.Authorizer == nil:
		return errors.New("read model: no authorizer")
	case !c.Reader.Authenticated():
		return errors.New("read model: the reader identity has no username")
	}
	return nil
}

// Model is the portal's read model: the cache, watches, on-demand reads and
// joins every view is built from. Every exported read takes the caller's
// identity and a grant, and refuses before looking anything up unless the
// grant covers the read (0030:D7).
type Model struct {
	cfg   Config
	kinds *kindResolver

	mu      sync.Mutex
	started bool
	stopped bool
	opm     map[schema.GroupVersionResource]*opmKind
	done    chan struct{}
}

// New returns a Model for cfg. It reads nothing until Start.
func New(cfg Config) (*Model, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg = cfg.withDefaults()
	return &Model{
		cfg:   cfg,
		kinds: newKindResolver(cfg.Discovery),
		opm:   map[schema.GroupVersionResource]*opmKind{},
		done:  make(chan struct{}),
	}, nil
}

// TuneConfig raises client-go's request rate for the reading client: QPS 50
// and burst 100 where cfg leaves them at zero, which client-go reads as 5
// and 10. Explicit values are kept.
func TuneConfig(cfg *rest.Config) {
	if cfg == nil {
		return
	}
	if cfg.QPS == 0 {
		cfg.QPS = tunedQPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = tunedBurst
	}
}
