package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	authenticationv1client "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-platform-model/opm-portal/internal/api"
	"github.com/open-platform-model/opm-portal/internal/auth"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/logs"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

const (
	defaultAddr     = "127.0.0.1:0"
	identityTimeout = 10 * time.Second
	shutdownTimeout = 5 * time.Second
)

// landing is where a browser goes after its launch, until the portal has
// pages of its own.
const landing = api.Prefix + "/clusters/" + api.DefaultCluster + "/instances"

// serveOptions are the serve subcommand's flags.
type serveOptions struct {
	kubeconfig string
	context    string
	addr       string
	namespaces []string
	open       bool
}

// parseServe parses the serve flags. Help and errors are written to stderr.
func parseServe(args []string, stderr io.Writer) (serveOptions, error) {
	var o serveOptions
	var namespaces string
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: opm-portal serve [--kubeconfig PATH] [--context NAME] [--addr HOST:PORT] [--namespaces NS,...] [--open]")
		fs.PrintDefaults()
	}
	fs.StringVar(&o.kubeconfig, "kubeconfig", "", "kubeconfig to read the cluster with (default: $KUBECONFIG, then ~/.kube/config)")
	fs.StringVar(&o.context, "context", "", "kubeconfig context to use (default: the current context)")
	fs.StringVar(&o.addr, "addr", defaultAddr, "loopback address to listen on; port 0 picks a free port")
	fs.StringVar(&namespaces, "namespaces", "", "comma-separated namespaces to read ModuleInstances and ModulePackages in (default: all)")
	fs.BoolVar(&o.open, "open", false, "open the launch link in the default browser")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	for ns := range strings.SplitSeq(namespaces, ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			o.namespaces = append(o.namespaces, ns)
		}
	}
	return o, nil
}

// loopbackAddr checks that addr names a loopback address and returns it in
// the form to listen on. "localhost" listens on 127.0.0.1. An empty host,
// any other name and any other IP are refused (0030:D5:R2).
func loopbackAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("--addr %q is not HOST:PORT: %w", addr, err)
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", fmt.Errorf("--addr %q has no valid port", addr)
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.Unmap().IsLoopback() || ip.Zone() != "" {
		return "", fmt.Errorf("--addr %q is not a loopback address: local mode listens only on 127.0.0.1, ::1 or localhost", addr)
	}
	return net.JoinHostPort(ip.Unmap().String(), port), nil
}

// boundLoopback returns the IP and port a listener is bound to, and refuses
// one that is not loopback.
func boundLoopback(a net.Addr) (netip.AddrPort, error) {
	tcp, ok := a.(*net.TCPAddr)
	if !ok {
		return netip.AddrPort{}, fmt.Errorf("listening on %s: not a TCP address", a)
	}
	ap := tcp.AddrPort()
	if !ap.Addr().Unmap().IsLoopback() {
		return netip.AddrPort{}, fmt.Errorf("listening on %s: not a loopback address", ap)
	}
	return netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port()), nil
}

// serve runs local mode until ctx is done.
func serve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	o, err := parseServe(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		return exitUsage
	}
	addr, err := loopbackAddr(o.addr)
	if err != nil {
		fmt.Fprintln(stderr, "opm-portal serve:", err)
		return exitUsage
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	if err := runLocal(ctx, o, addr, stdout, log); err != nil {
		log.Error("local mode stopped", "error", err)
		return exitFailure
	}
	return exitOK
}

// loadKubeconfig returns the REST config of the kubeconfig and context o
// names, and the context's name. Errors carry client-go's message and the
// path, never the file's content.
func loadKubeconfig(o serveOptions) (*rest.Config, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if o.kubeconfig != "" {
		rules.ExplicitPath = o.kubeconfig
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: o.context})
	raw, err := cc.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("loading the kubeconfig: %w", err)
	}
	name := o.context
	if name == "" {
		name = raw.CurrentContext
	}
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("loading the kubeconfig: %w", err)
	}
	return cfg, name, nil
}

// selfIdentity asks the cluster who the kubeconfig authenticates as, with
// one SelfSubjectReview. An identity that names no one (empty, blank or
// anonymous) is refused, so nothing is ever read on its behalf
// (0030:D6:R2).
func selfIdentity(ctx context.Context, reviews authenticationv1client.SelfSubjectReviewInterface) (authz.Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	res, err := reviews.Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return authz.Identity{}, fmt.Errorf("asking the cluster who the kubeconfig authenticates as: %w", err)
	}
	if res == nil {
		return authz.Identity{}, errors.New("asking the cluster who the kubeconfig authenticates as: empty response")
	}
	u := res.Status.UserInfo
	id := authz.Identity{Username: u.Username, UID: u.UID, Groups: u.Groups}
	if len(u.Extra) > 0 {
		id.Extra = make(map[string][]string, len(u.Extra))
		for k, v := range u.Extra {
			id.Extra[k] = []string(v)
		}
	}
	if !id.Authenticated() {
		return authz.Identity{}, errors.New("the kubeconfig authenticates as no user (empty or anonymous); refusing to start")
	}
	return id, nil
}

// runLocal wires local mode and serves until ctx is done.
func runLocal(ctx context.Context, o serveOptions, addr string, stdout io.Writer, log *slog.Logger) error {
	c, err := connect(ctx, o, log)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	defer func() { _ = ln.Close() }()
	bound, err := boundLoopback(ln.Addr())
	if err != nil {
		return err
	}
	p, err := build(ctx, c, o, bound, log)
	if err != nil {
		return err
	}
	defer p.close()

	httpSrv := &http.Server{
		Handler:           p.gate.Handler(p.api),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	served := make(chan error, 1)
	go func() { served <- httpSrv.Serve(ln) }()

	log.Info("serving", "addr", bound.String())
	launchURL := p.gate.LaunchURL()
	fmt.Fprintf(stdout, "Open this link once to sign in: %s\n", launchURL)
	if o.open {
		cleanup, err := openLaunch(ctx, launchURL, startBrowser)
		if err != nil {
			log.Warn("could not open a browser; open the printed link instead", "error", err)
		}
		defer cleanup()
	}

	select {
	case <-ctx.Done():
	case err := <-served:
		return fmt.Errorf("serving: %w", err)
	}
	log.Info("shutting down")
	// Closing the read API first ends every stream, so Shutdown is not
	// left waiting on a long-lived server-sent-events response.
	p.api.Close()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	return nil
}

// cluster is what local mode reads with: the clients, built from the one
// tuned REST config, the kubeconfig's identity and its authorizer.
type cluster struct {
	cs      *kubernetes.Clientset
	dyn     dynamic.Interface
	self    authz.Identity
	checker *authz.Checker
}

// connect loads the kubeconfig and learns its identity.
func connect(ctx context.Context, o serveOptions, log *slog.Logger) (cluster, error) {
	restCfg, contextName, err := loadKubeconfig(o)
	if err != nil {
		return cluster{}, err
	}
	// Every client is built from this one config, so the access reviews
	// sent before each read run at the same rate as the reads.
	readmodel.TuneConfig(restCfg)
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return cluster{}, fmt.Errorf("building the cluster client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return cluster{}, fmt.Errorf("building the dynamic client: %w", err)
	}
	self, err := selfIdentity(ctx, cs.AuthenticationV1().SelfSubjectReviews())
	if err != nil {
		return cluster{}, err
	}
	log.Info("reading as the kubeconfig's user", "user", self.Username, "context", contextName)
	checker, err := authz.NewLocal(cs.AuthorizationV1().SelfSubjectAccessReviews(), self, authz.Options{})
	if err != nil {
		return cluster{}, err
	}
	return cluster{cs: cs, dyn: dyn, self: self, checker: checker}, nil
}

// portal is local mode's started parts.
type portal struct {
	model *readmodel.Model
	api   *api.Server
	gate  *auth.Local
}

func (p *portal) close() {
	p.api.Close()
	p.model.Stop()
}

// build starts the read model and wires the logs producer, the read API
// and the front door for the address the portal is bound to.
func build(ctx context.Context, c cluster, o serveOptions, bound netip.AddrPort, log *slog.Logger) (*portal, error) {
	model, err := readmodel.New(readmodel.Config{
		Dynamic:    c.dyn,
		Discovery:  c.cs.Discovery(),
		Authorizer: c.checker,
		Reader:     c.self,
		Namespaces: o.namespaces,
	})
	if err != nil {
		return nil, err
	}
	if err := model.Start(ctx); err != nil {
		return nil, err
	}
	logDenied(log, model.Denied())
	p, err := wire(c, model, bound, log)
	if err != nil {
		model.Stop()
		return nil, err
	}
	return p, nil
}

// logDenied warns once per OPM kind scope the kubeconfig's user may not
// list and watch, so a user whose pages answer forbidden learns why, and,
// for a namespaced kind read cluster-wide, that --namespaces can narrow it
// to namespaces they may read (0030:D5:R5).
func logDenied(log *slog.Logger, denied []readmodel.DeniedScope) {
	for _, d := range denied {
		switch {
		case d.Namespaced && d.Namespace == "":
			log.Warn("the kubeconfig's user may not list and watch this kind cluster-wide, so it reads as forbidden; "+
				"pass --namespaces with the namespaces you may read", "resource", d.Resource)
		case d.Namespaced:
			log.Warn("the kubeconfig's user may not list and watch this kind in a namespace --namespaces names, so it reads as forbidden there",
				"resource", d.Resource, "namespace", d.Namespace)
		default:
			log.Warn("the kubeconfig's user may not list and watch this cluster-scoped kind, so it reads as forbidden",
				"resource", d.Resource)
		}
	}
}

func wire(c cluster, model *readmodel.Model, bound netip.AddrPort, log *slog.Logger) (*portal, error) {
	logsProducer, err := logs.New(logs.Config{
		Source:     logs.ClientSource(c.cs),
		Reach:      model,
		Authorizer: c.checker,
		Reader:     c.self,
		Options:    logs.Options{Logger: log},
	})
	if err != nil {
		return nil, err
	}
	gate, err := auth.NewLocal(auth.LocalConfig{
		Identity: c.self,
		Host:     bound.Addr().String(),
		Port:     int(bound.Port()),
		Landing:  landing,
		Logger:   log,
	})
	if err != nil {
		return nil, err
	}
	srv, err := api.New(api.Config{
		Model:      model,
		Authorizer: c.checker,
		Reader:     c.self,
		Authenticate: func(r *http.Request) (api.Principal, error) {
			id, session, err := gate.Authenticate(r)
			return api.Principal{Identity: id, Session: session}, err
		},
		Producers: stream.Mux{stream.KindLog: logsProducer},
		Logger:    log,
	})
	if err != nil {
		return nil, err
	}
	logsProducer.SetPublisher(srv.Broker())
	return &portal{model: model, api: srv, gate: gate}, nil
}

// openLaunch opens url in a browser without putting it on a command line,
// where any local user could read it from the process list: it writes a
// redirect page into a fresh private directory and opens that file. The
// returned cleanup removes the directory; it is safe to call after an
// error.
func openLaunch(ctx context.Context, url string, start func(ctx context.Context, path string) error) (cleanup func(), err error) {
	cleanup = func() {}
	dir, err := os.MkdirTemp("", "opm-portal-launch-")
	if err != nil {
		return cleanup, fmt.Errorf("creating the launch page directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	if err := os.Chmod(dir, 0o700); err != nil {
		return cleanup, fmt.Errorf("securing the launch page directory: %w", err)
	}
	u := html.EscapeString(url)
	page := `<!doctype html><meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
		`<meta http-equiv="refresh" content="0;url=` + u + `"><title>opm-portal</title>` +
		`<p><a href="` + u + `">Open opm-portal</a></p>` + "\n"
	path := filepath.Join(dir, "launch.html")
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		return cleanup, fmt.Errorf("writing the launch page: %w", err)
	}
	if err := start(ctx, path); err != nil {
		return cleanup, fmt.Errorf("starting the browser: %w", err)
	}
	return cleanup, nil
}

// startBrowser opens path with the platform's opener and does not wait for
// the browser.
func startBrowser(ctx context.Context, path string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	// The argument is the launch page's path, never the token.
	// The browser outlives the portal's context: the opener may become the
	// browser process, which shutting down must not kill.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), opener, path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Debug("the browser opener exited", "error", err)
		}
	}()
	return nil
}
