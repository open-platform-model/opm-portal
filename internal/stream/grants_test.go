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
	"strconv"
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

// slowPackages returns a rule that allows team-a, refuses the packages in
// revoked, and answers every other package review after delay, inside the
// 5 s review timeout.
func slowPackages(delay time.Duration, revoked ...string) rule {
	return func(a authorizationv1.ResourceAttributes) (bool, error) {
		if a.Resource != "modulepackages" {
			return a.Namespace == "team-a", nil
		}
		if slices.Contains(revoked, a.Name) {
			return false, nil
		}
		time.Sleep(delay)
		return a.Namespace == "team-a", nil
	}
}

// pkgNamed is a static item on instance topic team-a/blog revealing package
// team-a/name, so it is reviewed on its own.
func pkgNamed(name string) Item {
	return Item{
		Event: EventUpsert,
		Attrs: authz.Attributes{Verb: "get", Resource: packagesGVR, Namespace: "team-a", Name: name},
		Data:  json.RawMessage(`{"name":"` + name + `"}`),
	}
}

// A topic grant that covered when revalidate looked at it, and expired
// during a slow review of an item, is asked again before the write: the
// topic was revoked meanwhile, so it closes and no snapshot is written.
func TestATopicGrantThatExpiresDuringRevalidateIsAskedAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
		e.policy.set("alice", allowNamespaces("team-a"))
		// t=0: another stream caches alice's pkg decision until t=30.
		e.open(Session{Key: "session-other", Identity: user("alice")}, "package:team-a/pkg")
		time.Sleep(5 * time.Second)
		blog := mustTopic(t, "instance:team-a/blog")
		e.prod.seed(blog, "a", pkgItem(nil))
		e.prod.seed(blog, "b", Item{
			Event: EventUpsert,
			Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "team-a", Name: "blog"},
			Render: func(context.Context, authz.Identity) (json.RawMessage, error) {
				// Instances are revoked; packages stay allowed but take
				// 4.5 s to review. The render ends at t=31.
				e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
					if a.Resource == "modulepackages" {
						time.Sleep(4500 * time.Millisecond)
						return true, nil
					}
					return false, nil
				})
				time.Sleep(26 * time.Second)
				return json.RawMessage(`{"name":"blog","secret":"instance-data"}`), nil
			},
		})
		// t=5: the topic's grant lasts until t=35. At t=31 pkg is asked
		// again until t=35.5, past the topic's grant.
		sv := e.open(session("alice"), "instance:team-a/blog")
		time.Sleep(time.Minute)
		synctest.Wait()

		if got, want := nonHeartbeats(sv.rec.take()), []string{"open()", "closed(instance:team-a/blog)forbidden"}; !slices.Equal(got, want) {
			t.Errorf("events = %v, want %v", got, want)
		}
	})
}

// An item grant that covered when revalidate looked at it, and expired
// during a slow review of another item, is asked again before the write:
// the item was revoked meanwhile, so the snapshot is written without it.
func TestAnItemGrantThatExpiresDuringRevalidateIsAskedAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
		e.policy.set("alice", allowNamespaces("team-a"))
		// t=0: another stream caches alice's pkg2 decision until t=30.
		e.open(Session{Key: "session-other", Identity: user("alice")}, "package:team-a/pkg2")
		time.Sleep(5 * time.Second)
		blog := mustTopic(t, "instance:team-a/blog")
		e.prod.seed(blog, "a", pkgNamed("pkg1"))
		e.prod.seed(blog, "b", pkgNamed("pkg2"))
		e.prod.seed(blog, "c", Item{
			Event: EventUpsert,
			Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "team-a", Name: "blog"},
			Render: func(context.Context, authz.Identity) (json.RawMessage, error) {
				// pkg1 is revoked, pkg2 takes 4.5 s to review. The render
				// ends at t=31.
				e.policy.set("alice", slowPackages(4500*time.Millisecond, "pkg1"))
				time.Sleep(26 * time.Second)
				return json.RawMessage(`{"name":"blog"}`), nil
			},
		})
		// t=5: the topic's and pkg1's grants last until t=35, the cached
		// pkg2 decision until t=30. At t=31 pkg2 is asked again until
		// t=35.5, past pkg1's grant.
		sv := e.open(session("alice"), "instance:team-a/blog")
		time.Sleep(time.Minute)
		synctest.Wait()

		evs := sv.rec.take()
		want := `{"topic":"instance:team-a/blog","items":[{"name":"pkg2"},{"name":"blog"}]}`
		if got := nonHeartbeats(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:team-a/blog)"}) || evs[1].Data != want {
			t.Errorf("events = %v, snapshot = %s; want %s", got, evs[1].Data, want)
		}
	})
}

// A message whose grants keep expiring while the others are asked again
// does not settle: after revalidateRounds rounds of reviews its topic is
// closed with upstream_unavailable and nothing is written.
func TestAMessageWhoseGrantsNeverSettleClosesTheTopic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
		e.policy.set("alice", allowNamespaces("team-a"))
		blog := mustTopic(t, "instance:team-a/blog")
		// Eight package reviews of 4.5 s take 36 s, longer than a
		// decision lasts, so each round leaves the earliest expired.
		for i := range 8 {
			e.prod.seed(blog, "a"+strconv.Itoa(i), pkgNamed("pkg"+strconv.Itoa(i)))
		}
		e.prod.seed(blog, "b", Item{
			Event: EventUpsert,
			Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "team-a", Name: "blog"},
			Render: func(context.Context, authz.Identity) (json.RawMessage, error) {
				e.policy.set("alice", slowPackages(4500*time.Millisecond))
				time.Sleep(31 * time.Second)
				return json.RawMessage(`{"name":"blog"}`), nil
			},
		})
		sv := e.open(session("alice"), "instance:team-a/blog")
		time.Sleep(5 * time.Minute)
		synctest.Wait()

		if got, want := nonHeartbeats(sv.rec.take()), []string{"open()", "closed(instance:team-a/blog)upstream_unavailable"}; !slices.Equal(got, want) {
			t.Errorf("events = %v, want %v", got, want)
		}
	})
}

// Every write path is enumerated here, so no future path can write topic
// data without the re-validation in send. Only send asks for an event id,
// the other writes carry only the stream's own control events, and only
// event touches the response writer.
func TestOnlySendWritesTopicData(t *testing.T) {
	allowed := map[string][]string{
		"eventID": {"send"},
		"event":   {"run", "heartbeat", "closed", "send"},
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

	// Every use of an http.ResponseWriter, whether the writer's field w or
	// a parameter, is listed here: a Write, an fmt.Fprint or an
	// io.WriteString on it anywhere else is a write path that bypasses send.
	wantUses := []string{
		"NewHandler: argument of http.Error",
		"Serve: argument of http.NewResponseController",
		"Serve: value of field w",
		"ServeHTTP: argument of h.fail",
		"ServeHTTP: argument of h.fail",
		"ServeHTTP: argument of h.fail",
		"ServeHTTP: argument of h.fail",
		"ServeHTTP: argument of st.Serve",
		"ServeHTTP: w.Header",
		"ServeHTTP: w.Header",
		"ServeHTTP: w.WriteHeader",
		"event: wr.w.Write",
	}
	if got := responseWriterUses(t); !slices.Equal(got, wantUses) {
		t.Errorf("the response writer is used as\n%s\nwant\n%s\nonly event writes to it: a new write path must go through send",
			strings.Join(got, "\n"), strings.Join(wantUses, "\n"))
	}
}

// responseWriterUses lists, sorted, every use in the package's non-test
// files of the writer's field w and of every parameter typed
// http.ResponseWriter, as "function: how it is used".
func responseWriterUses(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, fn := range packageFuncs(t) {
		params := writerParams(fn)
		var stack []ast.Node
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, n)
			if use := writerUse(n, parent, params); use != nil {
				out = append(out, fn.Name.Name+": "+describeUse(use, parent, stack))
			}
			return true
		})
	}
	slices.Sort(out)
	return out
}

// writerParams returns the names of fn's parameters typed
// http.ResponseWriter, including those of the function literals inside it.
func writerParams(fn *ast.FuncDecl) map[string]bool {
	params := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if ft, ok := n.(*ast.FuncType); ok && ft.Params != nil {
			for _, f := range ft.Params.List {
				if exprString(f.Type) == "http.ResponseWriter" {
					for _, name := range f.Names {
						params[name.Name] = true
					}
				}
			}
		}
		return true
	})
	return params
}

// writerUse returns n when it is a use of the writer's field w or of one of
// params, whose parent node is parent, and nil otherwise.
func writerUse(n, parent ast.Node, params map[string]bool) ast.Expr {
	switch x := n.(type) {
	case *ast.SelectorExpr:
		if x.Sel.Name == "w" {
			return x
		}
	case *ast.Ident:
		if !params[x.Name] {
			return nil
		}
		switch p := parent.(type) {
		case *ast.SelectorExpr:
			if p.Sel == x {
				return nil // a field or method named like the parameter
			}
		case *ast.KeyValueExpr:
			if p.Key == x {
				return nil // a composite literal's key
			}
		case *ast.Field:
			return nil // a function literal's parameter
		}
		return x
	}
	return nil
}

// describeUse says how the expression use, whose parent node is parent, is
// used. stack ends with use itself.
func describeUse(use ast.Expr, parent ast.Node, stack []ast.Node) string {
	switch p := parent.(type) {
	case *ast.CallExpr:
		if slices.Contains(p.Args, use) {
			return "argument of " + exprString(p.Fun)
		}
	case *ast.SelectorExpr:
		if p.X == use {
			name := exprString(p)
			if len(stack) >= 3 {
				if call, ok := stack[len(stack)-3].(*ast.CallExpr); ok && call.Fun == p {
					return name
				}
			}
			return name + " (not called)"
		}
	case *ast.KeyValueExpr:
		if p.Value == use {
			return "value of field " + exprString(p.Key)
		}
	}
	return "other use of " + exprString(use)
}

// methodCall is a call of a method or package function, x.name(...), made
// in function fn.
type methodCall struct {
	fn, name string
	call     *ast.CallExpr
}

// packageFuncs parses the package's non-test files and returns every
// function declaration with a body, in source order.
func packageFuncs(t *testing.T) []*ast.FuncDecl {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []*ast.FuncDecl
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				out = append(out, fn)
			}
		}
	}
	return out
}

// packageCalls lists every x.name(...) call in the package's non-test
// files, in source order.
func packageCalls(t *testing.T) []methodCall {
	t.Helper()
	var out []methodCall
	for _, fn := range packageFuncs(t) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					out = append(out, methodCall{fn: fn.Name.Name, name: sel.Sel.Name, call: call})
				}
			}
			return true
		})
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
