package stream

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// The ways a reader's packages access can change while a message is built.
var packageAccess = map[string]rule{
	// Still allowed: the expired grant is asked again and allowed.
	"allowed": allowNamespaces("team-a"),
	// Revoked: instances stay readable, packages do not.
	"revoked": func(a authorizationv1.ResourceAttributes) (bool, error) {
		return a.Namespace == "team-a" && a.Resource == "moduleinstances", nil
	},
	// Failing: the review of a package errors.
	"failing": func(a authorizationv1.ResourceAttributes) (bool, error) {
		if a.Resource == "modulepackages" {
			return false, errors.New("webhook timed out")
		}
		return a.Namespace == "team-a", nil
	},
}

// pkgItem is an item on instance topic team-a/blog that reveals another
// read, package team-a/pkg, so it is reviewed on its own.
// It carries render, or static data when render is nil.
func pkgItem(render func(context.Context, authz.Identity) (json.RawMessage, error)) Item {
	it := Item{
		Event:  EventUpsert,
		Attrs:  authz.Attributes{Verb: "get", Resource: packagesGVR, Namespace: "team-a", Name: "pkg"},
		Render: render,
	}
	if render == nil {
		it.Data = json.RawMessage(`{"name":"pkg","secret":"package-data"}`)
	}
	return it
}

// A snapshot item reviewed on its own whose decision expires before the
// write is asked again there: a revoked item is left out of the snapshot
// without a trace, and an authorization error closes the topic.
func TestASnapshotItemsOwnDecisionIsAskedAgainBeforeTheWrite(t *testing.T) {
	for _, access := range slices.Sorted(maps.Keys(packageAccess)) {
		t.Run(access, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
				e.policy.set("alice", allowNamespaces("team-a"))
				blog := mustTopic(t, "instance:team-a/blog")
				// Item a is reviewed at t=0; item b's render then changes
				// alice's access and outlives a's 30 s decision.
				e.prod.seed(blog, "a", pkgItem(nil))
				e.prod.seed(blog, "b", Item{
					Event: EventUpsert,
					Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "team-a", Name: "blog"},
					Render: func(context.Context, authz.Identity) (json.RawMessage, error) {
						e.policy.set("alice", packageAccess[access])
						time.Sleep(31 * time.Second)
						return json.RawMessage(`{"name":"blog"}`), nil
					},
				})
				sv := e.open(session("alice"), "instance:team-a/blog")
				time.Sleep(time.Minute)
				synctest.Wait()

				evs := sv.rec.take()
				if access == "failing" {
					if got, want := nonHeartbeats(evs), []string{"open()", "closed(instance:team-a/blog)upstream_unavailable"}; !slices.Equal(got, want) {
						t.Errorf("events = %v, want %v", got, want)
					}
				} else {
					want := `{"topic":"instance:team-a/blog","items":[{"name":"pkg","secret":"package-data"},{"name":"blog"}]}`
					if access == "revoked" {
						want = `{"topic":"instance:team-a/blog","items":[{"name":"blog"}]}`
					}
					if got := nonHeartbeats(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:team-a/blog)"}) || evs[1].Data != want {
						t.Errorf("events = %v, snapshot = %s; want %s", got, evs[1].Data, want)
					}
				}
				// The topic at open, a at t=0, then the topic and a again
				// before the write.
				if n := e.policy.count(); n != 4 {
					t.Errorf("%d reviews sent, want 4", n)
				}
			})
		})
	}
}

// A live item reviewed on its own whose render outlives its decision is
// asked again before the write: a revoked item is not written, and an
// authorization error closes the topic.
func TestALiveItemsOwnDecisionIsAskedAgainBeforeTheWrite(t *testing.T) {
	for _, access := range slices.Sorted(maps.Keys(packageAccess)) {
		t.Run(access, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
				e.policy.set("alice", allowNamespaces("team-a"))
				blog := mustTopic(t, "instance:team-a/blog")
				sv := e.open(session("alice"), "instance:team-a/blog")
				synctest.Wait()
				e.prod.upsert(t, blog, pkgItem(func(context.Context, authz.Identity) (json.RawMessage, error) {
					e.policy.set("alice", packageAccess[access])
					time.Sleep(31 * time.Second)
					return json.RawMessage(`{"name":"pkg","secret":"package-data"}`), nil
				}))
				time.Sleep(time.Minute)
				synctest.Wait()

				want := map[string][]string{
					"allowed": {"open()", "snapshot(instance:team-a/blog)", "upsert(instance:team-a/blog)"},
					"revoked": {"open()", "snapshot(instance:team-a/blog)"},
					"failing": {"open()", "snapshot(instance:team-a/blog)", "closed(instance:team-a/blog)upstream_unavailable"},
				}[access]
				if got := nonHeartbeats(sv.rec.take()); !slices.Equal(got, want) {
					t.Errorf("events = %v, want %v", got, want)
				}
				// The topic at open, the item at t=0, then the topic and the
				// item again before the write.
				if n := e.policy.count(); n != 4 {
					t.Errorf("%d reviews sent, want 4", n)
				}
			})
		})
	}
}

// Every write path is enumerated here, so no future path can write topic
// data without the re-validation in send. Only send asks for an event id,
// and the other writes carry only the stream's own control events.
func TestOnlySendWritesTopicData(t *testing.T) {
	allowed := map[string][]string{
		"eventID": {"send"},
		"event":   {"run", "heartbeat", "closed", "send"},
		"Write":   {"event"},
		// send re-validates, and only send.
		"revalidate": {"send"},
	}
	controlEvents := map[string]string{"run": "EventOpen", "heartbeat": "EventHeartbeat", "closed": "EventClosed"}

	found := map[string][]string{}
	// Where send calls revalidate, eventID and event, in source order.
	var inSend []string
	for _, c := range packageCalls(t) {
		if _, watched := allowed[c.name]; !watched {
			continue
		}
		// bytes.Buffer.Write builds the event; only the ResponseWriter's
		// Write sends it.
		if c.name == "Write" && !strings.HasSuffix(exprString(c.sel.X), ".w") {
			continue
		}
		found[c.name] = append(found[c.name], c.fn)
		if c.fn == "send" {
			inSend = append(inSend, c.name)
		}
		if want, ok := controlEvents[c.fn]; ok && c.name == "event" &&
			(len(c.call.Args) != 4 || exprString(c.call.Args[2]) != want) {
			t.Errorf("%s writes an event other than %s", c.fn, want)
		}
	}
	if want := []string{"revalidate", "eventID", "event"}; !slices.Equal(inSend, want) {
		t.Errorf("send calls %v, want %v: the grants are re-validated right before the event id and the write", inSend, want)
	}
	for name, want := range allowed {
		got := slices.Sorted(slices.Values(found[name]))
		if !slices.Equal(got, slices.Sorted(slices.Values(want))) {
			t.Errorf("%s is called from %v, want only %v: a new write path must go through send", name, got, want)
		}
	}
}

// methodCall is a call of a method or package function, x.name(...), made
// in function fn.
type methodCall struct {
	fn, name string
	sel      *ast.SelectorExpr
	call     *ast.CallExpr
}

// packageCalls lists every x.name(...) call in the package's non-test
// files, in source order.
func packageCalls(t *testing.T) []methodCall {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []methodCall
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						out = append(out, methodCall{fn: fn.Name.Name, name: sel.Sel.Name, sel: sel, call: call})
					}
				}
				return true
			})
		}
	}
	return out
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	}
	return ""
}
