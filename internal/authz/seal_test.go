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
// new(authz.Grant), or declare a type from it; inside this package only
// issue may fill one in, and only Checker.Check may call issue.
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
func mint() Grant { return issue(Identity{}, Attributes{}) }`, true},
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
		ast.Inspect(decl, func(n ast.Node) bool {
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
		// Inside authz the zero Grant{} is how a denial returns "no grant";
		// a literal with fields is construction, allowed in issue alone.
		// Outside authz every literal is refused.
		allowed := s.inAuthz && (fnName == "issue" || len(n.Elts) == 0)
		if !allowed && s.isGrant(n.Type) {
			s.report(n, "Grant composite literal outside issue")
		}
	case *ast.CallExpr:
		s.visitCall(n, fnName)
	case *ast.TypeSpec:
		if !s.inAuthz && s.isGrant(n.Type) {
			s.report(n, "type declared from Grant")
		}
	}
}

func (s *grantScan) visitCall(n *ast.CallExpr, fnName string) {
	id, ok := n.Fun.(*ast.Ident)
	if !ok {
		return
	}
	if id.Name == "new" && len(n.Args) == 1 && s.isGrant(n.Args[0]) {
		s.report(n, "new(Grant)")
	}
	if s.inAuthz && !s.isTest && id.Name == "issue" && fnName != "(*Checker).Check" {
		s.report(n, "issue called outside Checker.Check")
	}
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
