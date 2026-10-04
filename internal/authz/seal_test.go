package authz

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const authzImportPath = "github.com/open-platform-model/opm-portal/internal/authz"

// TestGrantFieldsAreUnexported: a struct with only unexported fields can be
// filled in only by its own package.
func TestGrantFieldsAreUnexported(t *testing.T) {
	typ := reflect.TypeFor[Grant]()
	if typ.NumField() == 0 {
		t.Fatal("Grant has no fields; the zero value would be indistinguishable from an issued grant")
	}
	for f := range typ.Fields() {
		if f.IsExported() {
			t.Errorf("Grant field %s is exported; another package could forge a grant", f.Name)
		}
	}
}

// TestForgingPackageDoesNotCompile builds testdata/forge, which tries to fill
// in a Grant and to call the unexported constructor from another package,
// and expects the compiler to refuse both.
func TestForgingPackageDoesNotCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go toolchain")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	cmd := exec.CommandContext(t.Context(), goBin, "build", "-o", os.DevNull, "./testdata/forge")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("testdata/forge compiled; another package can construct a Grant")
	}
	text := string(out)
	for _, want := range []string{"sealed", "issue"} {
		if !strings.Contains(text, want) {
			t.Errorf("compiler output does not name %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "forge.go") {
		t.Fatalf("build failed for a reason other than the forgery:\n%s", text)
	}
}

// TestNoGrantConstructionOutsideCheck scans every Go file in the module. No
// file outside this package may write a Grant composite literal, call
// new(authz.Grant), or declare a type from it. Inside this package only issue
// may fill one in (no filled-in Grant literal, grantData literal or
// new(grantData) elsewhere), and only Checker.Check may refer to issue, so it
// cannot be called or passed on as a function value elsewhere.
//
// An issued grant's data is reached through the field sealed, and the scan
// allowlists who may name it: issue and the read-only methods Valid,
// Identity, Attributes, Expires and Covers (sealedReaders). A sealed selector
// anywhere else is refused, whatever it is used for (copied, passed, returned,
// addressed, sliced), and no method may be declared on grantData. Inside the
// allowlisted functions, writes to or through sealed and copies of the sealed
// pointer are still refused. The scan's limits are in the archived change's
// design.md, Risks.
func TestNoGrantConstructionOutsideCheck(t *testing.T) {
	root := moduleRoot(t)
	var findings []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		inAuthz := filepath.Dir(rel) == filepath.Join("internal", "authz")
		findings = append(findings, scanGrantConstruction(t, rel, src, inAuthz)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	for _, f := range findings {
		t.Error(f)
	}
}

// TestScannerFindsForgeries keeps the module scan honest: it must flag each
// forgery pattern, or a passing scan proves nothing.
func TestScannerFindsForgeries(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		inAuthz bool
	}{
		{"literal outside", `package x
import "` + authzImportPath + `"
var g = authz.Grant{}`, false},
		{"renamed import", `package x
import az "` + authzImportPath + `"
func f() any { return &az.Grant{} }`, false},
		{"new outside", `package x
import "` + authzImportPath + `"
var g = new(authz.Grant)`, false},
		{"alias outside", `package x
import "` + authzImportPath + `"
type G = authz.Grant`, false},
		{"literal in authz outside issue", `package authz
func mint() Grant { return Grant{sealed: &grantData{}} }`, true},
		{"issue outside Check", `package authz
func mint() Grant { return issue(Identity{}, Attributes{}, time.Time{}, nil) }`, true},
		{"sealed field assignment", `package authz
func mint(d *grantData) Grant { var g Grant; g.sealed = d; return g }`, true},
		{"issued grant's expiry extended", `package authz
func extend(g Grant) { g.sealed.expires = time.Now().Add(time.Hour) }`, true},
		{"issued grant retargeted", `package authz
func retarget(g Grant, k string) { (g.sealed).idKey = k }`, true},
		{"issued grant's groups rewritten", `package authz
func regroup(g Grant) { g.sealed.identity.Groups[0] = "system:masters" }`, true},
		{"sealed pointer copied out", `package authz
func steal(g Grant) { d := g.sealed; d.expires = time.Time{} }`, true},
		{"grantData literal outside issue", `package authz
func mk() *grantData { return &grantData{} }`, true},
		{"issue as a function value", `package authz
var mintFn = issue`, true},
		{"sealed pointer declared with var", `package authz
func steal(g Grant) { var d = g.sealed; d.expires = time.Time{} }`, true},
		{"sealed pointer passed to a function", `package authz
func steal(g Grant) { mutate(g.sealed) }
func mutate(d *grantData) { d.idKey = "" }`, true},
		{"sealed pointer returned", `package authz
func leak(g Grant) *grantData { return g.sealed }`, true},
		{"address of a sealed field", `package authz
func steal(g Grant) { p := &g.sealed.expires; *p = time.Time{} }`, true},
		{"sealed slice aliased", `package authz
func regroup(g Grant) { gs := g.sealed.identity.Groups; gs[0] = "system:masters" }`, true},
		{"method on grantData", `package authz
func (d *grantData) extend() { d.expires = d.expires.Add(time.Hour) }`, true},
		{"write inside an allowlisted method", `package authz
func (g Grant) Covers(who Identity, req Attributes) error { g.sealed.expires = time.Time{}; return nil }`, true},
		{"new grantData outside issue", `package authz
func mk() *grantData { return new(grantData) }`, true},
		{"issue through a local variable", `package authz
func mint() Grant { f := issue; return f(Identity{}, Attributes{}, time.Time{}, nil) }`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scanGrantConstruction(t, "case.go", []byte(tc.src), tc.inAuthz); len(got) == 0 {
				t.Errorf("scanner missed the forgery in:\n%s", tc.src)
			}
		})
	}
}

func scanGrantConstruction(t *testing.T, name string, src []byte, inAuthz bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	s := grantScan{
		fset:    fset,
		inAuthz: inAuthz,
		isTest:  strings.HasSuffix(name, "_test.go"),
	}
	if !inAuthz {
		s.local = authzLocalName(file)
		if s.local == "" {
			return nil
		}
	}
	for _, decl := range file.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		fnName := funcName(fn)
		if inAuthz && receiverIs(fn, "grantData") {
			s.report(fn, "method declared on grantData")
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			if fn != nil && n == fn.Name {
				// The function's own name is a declaration, not a use.
				return true
			}
			s.visit(n, fnName)
			return true
		})
	}
	return s.findings
}

// grantScan looks for Grant construction in one file.
type grantScan struct {
	fset     *token.FileSet
	inAuthz  bool
	isTest   bool
	local    string // the file's name for the authz import, outside authz
	findings []string
}

func (s *grantScan) report(n ast.Node, what string) {
	s.findings = append(s.findings, s.fset.Position(n.Pos()).String()+": "+what)
}

// isGrant reports whether e names the Grant type.
func (s *grantScan) isGrant(e ast.Expr) bool {
	e = unparen(e)
	if s.inAuthz {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "Grant"
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Grant" {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == s.local
}

func (s *grantScan) visit(n ast.Node, fnName string) {
	switch n := n.(type) {
	case *ast.CompositeLit:
		s.visitLiteral(n, fnName)
	case *ast.AssignStmt:
		s.visitAssign(n)
	case *ast.IncDecStmt:
		if s.inAuthz && reachesSealed(n.X) {
			s.report(n, "assignment through a Grant's sealed data")
		}
	case *ast.CallExpr:
		s.visitNew(n, fnName)
	case *ast.SelectorExpr:
		if s.inAuthz && n.Sel.Name == "sealed" && !sealedReaders[fnName] {
			s.report(n, "a Grant's sealed data named outside issue and its read-only methods")
		}
	case *ast.Ident:
		s.visitIdent(n, fnName)
	case *ast.TypeSpec:
		if !s.inAuthz && s.isGrant(n.Type) {
			s.report(n, "type declared from Grant")
		}
	}
}

// sealedReaders are the only functions that may name a Grant's sealed field:
// issue fills it in, and the methods read it without letting it escape.
var sealedReaders = map[string]bool{
	"issue":            true,
	"Grant.Valid":      true,
	"Grant.Identity":   true,
	"Grant.Attributes": true,
	"Grant.Expires":    true,
	"Grant.Covers":     true,
}

// visitNew: new(Grant) anywhere outside authz, and new(grantData) inside it
// outside issue, is construction.
func (s *grantScan) visitNew(n *ast.CallExpr, fnName string) {
	if !isIdent(n.Fun, "new") || len(n.Args) != 1 {
		return
	}
	if s.isGrant(n.Args[0]) {
		s.report(n, "new(Grant)")
	}
	if s.inAuthz && fnName != "issue" && isIdent(n.Args[0], "grantData") {
		s.report(n, "new(grantData) outside issue")
	}
}

// visitIdent: any use of issue, called or not, outside Check is refused; a
// function value passed on would let its holder mint grants.
func (s *grantScan) visitIdent(n *ast.Ident, fnName string) {
	if s.inAuthz && !s.isTest && n.Name == "issue" && fnName != "(*Checker).Check" {
		s.report(n, "issue referred to outside Checker.Check")
	}
}

// visitLiteral: inside authz the zero Grant{} is how a denial returns "no
// grant"; a literal with fields, or any grantData literal, is construction,
// allowed in issue alone. Outside authz every Grant literal is refused.
func (s *grantScan) visitLiteral(n *ast.CompositeLit, fnName string) {
	allowed := s.inAuthz && (fnName == "issue" || len(n.Elts) == 0)
	if !allowed && s.isGrant(n.Type) {
		s.report(n, "Grant composite literal outside issue")
	}
	if s.inAuthz && fnName != "issue" && isIdent(n.Type, "grantData") {
		s.report(n, "grantData composite literal outside issue")
	}
}

// visitAssign: an issued grant's data is shared by pointer, so writing to
// sealed, or to anything reached through it (g.sealed.expires,
// g.sealed.identity.Groups[0]), would retarget or extend every copy of the
// grant. Copying the pointer out (d := g.sealed) is refused too, because a
// write through the copy would not show in this scan. issue builds the data
// with a literal and needs no assignment, so no function is exempt.
func (s *grantScan) visitAssign(n *ast.AssignStmt) {
	if !s.inAuthz {
		return
	}
	for _, lhs := range n.Lhs {
		if reachesSealed(lhs) {
			s.report(n, "assignment through a Grant's sealed data")
		}
	}
	for _, rhs := range n.Rhs {
		if sel, ok := unparen(rhs).(*ast.SelectorExpr); ok && sel.Sel.Name == "sealed" {
			s.report(n, "a Grant's sealed pointer copied into a variable")
		}
	}
}

// reachesSealed reports whether e is sealed, or a selector, index or
// dereference chain that passes through a selector named sealed.
func reachesSealed(e ast.Expr) bool {
	for {
		switch x := unparen(e).(type) {
		case *ast.SelectorExpr:
			if x.Sel.Name == "sealed" {
				return true
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return false
		}
	}
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := unparen(e).(*ast.Ident)
	return ok && id.Name == name
}

// receiverIs reports whether fn is a method on typ or *typ.
func receiverIs(fn *ast.FuncDecl, typ string) bool {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	return isIdent(recv, typ)
}

func authzLocalName(file *ast.File) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != authzImportPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "authz"
	}
	return ""
}

func funcName(fn *ast.FuncDecl) string {
	if fn == nil {
		return ""
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		if id, ok := star.X.(*ast.Ident); ok {
			return "(*" + id.Name + ")." + fn.Name.Name
		}
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's directory")
		}
		dir = parent
	}
}
