package health

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// operatorConditionReasons is a copy of the reasons opm-operator writes into
// conditions, from internal/status/conditions.go (constants ending in Reason)
// at opm-operator main 8d34b6b. The operator keeps them in an internal
// package, so they cannot be imported. Update this list with the table.
var operatorConditionReasons = []string{
	"Suspended", "ResolutionFailed", "RenderFailed", "SkewRefused", "DuplicateIdentities",
	"ApplyFailed", "PruneFailed", "ImpersonationFailed", "DeletionSAMissing",
	"ReconciliationSucceeded", "DriftDetected", "ManagedExternally", "SelfManagementRefused",
	"Generated", "GenerateFailed", "BuildFailed",
	"ContractCollisions", "OverSubscribedContracts", "ComparablePredicates",
	"UnfulfilledContracts", "ContractsFulfilled", "NoContractsDefined",
	"SourceNotReady", "FetchFailed", "PathNotFound", "InstanceFileNotFound", "UnsupportedKind",
	"DependenciesNotReady", "PlatformNotReady",
	"Accepted", "CatalogUnresolved", "CatalogWrongKind", "ProvidesMismatch", "ProviderMismatch",
	"ProviderInventoryPending", "BuildIncompatible", "ProviderReady", "ProviderNotReady",
	"ContractSubscribed", "ContractClaimed", "DuplicateClaim", "DependentsRemain",
}

// operatorLiteralReasons are condition reasons the operator writes as string
// literals rather than constants: "Progressing" in its MarkReconciling calls
// (internal/reconcile) and "ModuleResolved" in MarkModuleResolved.
var operatorLiteralReasons = []string{"Progressing", "ModuleResolved"}

// operatorEventOnlyReasons are reason constants the operator only uses on
// events, never on a condition; they need no row.
// OrphanedOnDeletion is declared among the condition reasons but is only
// emitted as an event when a deletion orphans its objects.
var operatorEventOnlyReasons = []string{"Applied", "Pruned", "Resumed", "NoOp", "RenderWarning", "OrphanedOnDeletion"}

func TestExplain_EveryOperatorReasonHasARow(t *testing.T) {
	want := map[string]bool{}
	for _, r := range append(append([]string{}, operatorConditionReasons...), operatorLiteralReasons...) {
		want[r] = true
		e, ok := Explain(r)
		if !ok {
			t.Errorf("operator reason %q has no explanation", r)
			continue
		}
		if e.Meaning == "" {
			t.Errorf("reason %q has an empty meaning", r)
		}
	}
	for r := range reasonExplanations {
		if !want[r] {
			t.Errorf("explanation for %q names no operator condition reason", r)
		}
	}
}

func TestExplain_TextCitesNoEnhancement(t *testing.T) {
	citation := regexp.MustCompile(`\b\d{4}:D\d+|\bD\d+\b|enhancement`)
	for r, e := range reasonExplanations {
		if citation.MatchString(e.Meaning) || citation.MatchString(e.NextStep) {
			t.Errorf("explanation for %q cites an enhancement", r)
		}
	}
}

func TestExplain_KnownAndUnknown(t *testing.T) {
	e, ok := Explain("ProvidesMismatch")
	if !ok || e.Meaning == "" || e.NextStep == "" {
		t.Fatalf("ProvidesMismatch: %+v %v", e, ok)
	}
	if _, ok := Explain("SomethingTheOperatorNeverWrites"); ok {
		t.Fatal("unknown reason explained")
	}
}

// TestOperatorReasonsMatchSource compares the copied lists with the reason
// constants of an opm-operator checkout. It runs only when OPM_OPERATOR_SRC
// names that checkout, because CI has no sibling repo.
func TestOperatorReasonsMatchSource(t *testing.T) {
	root := os.Getenv("OPM_OPERATOR_SRC")
	if root == "" {
		t.Skip("set OPM_OPERATOR_SRC to an opm-operator checkout to compare reason constants")
	}
	path := filepath.Join(root, "internal", "status", "conditions.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var source []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if !strings.HasSuffix(name.Name, "Reason") || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquoting %s: %v", name.Name, err)
				}
				source = append(source, v)
			}
		}
	}
	copied := append(append([]string{}, operatorConditionReasons...), operatorEventOnlyReasons...)
	sort.Strings(source)
	sort.Strings(copied)
	if strings.Join(source, ",") != strings.Join(copied, ",") {
		t.Fatalf("reason constants differ from the copy\nsource: %v\ncopy:   %v", source, copied)
	}
}
