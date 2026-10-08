package invariants

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Atom checks prove source and carrier shape only; semantic correspondence remains
// the mirror audit's responsibility, exactly as in authored_links.go.
func atomArtifacts(g *ontology.Graph, visit func(string, gate.AtomArtifact)) {
	_, snapshot, err := InvocationExecutionSnapshot(g)
	if err != nil || snapshot == nil {
		return
	}
	runs := snapshot.PackageRuns
	seen := map[string]bool{}
	for _, r := range g.Requirements {
		for _, entry := range r.VerifiedBy {
			if seen[entry] {
				continue
			}
			seen[entry] = true
			file, test, ok := gate.ParseFileColonSymbol(entry)
			if !ok {
				continue
			}
			for _, artifact := range recordedAtoms(file, test, runs) {
				visit(artifact.ReqID, artifact)
			}
		}
	}
}

func recordedAtoms(file, test string, runs map[string]gate.RecordingResult) []gate.AtomArtifact {
	key := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))
	run, exists := runs[key]
	if !exists || run.Skipped || run.Err != nil || run.CompileFailed {
		return nil
	}
	var artifacts []gate.AtomArtifact
	for _, raw := range run.Artifacts {
		var metadata struct {
			Test string `json:"test"`
		}
		if json.Unmarshal(raw.RawJSON, &metadata) != nil || (metadata.Test != test && !strings.HasPrefix(metadata.Test, test+"/")) {
			continue
		}
		if verdict := run.ForTest(metadata.Test); verdict.Err != nil || !verdict.Passed {
			continue
		}
		artifact, err := gate.DecodeAtomArtifact(raw.RawJSON)
		if err == nil && artifact.Verdict == "pass" && (artifact.Mode == "fact" || artifact.Mode == "holds" || artifact.Mode == "rule") {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}

func atomMethodAtomic(fn *ast.FuncDecl, relation, rule bool) bool {
	if fn == nil || fn.Body == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 0 || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	result := fn.Type.Results.List[0]
	if len(result.Names) > 1 {
		return false
	}
	if rule {
		return true
	}
	if relation {
		if typ, ok := result.Type.(*ast.Ident); ok && typ.Name == "bool" {
			return true
		}
		return false
	}
	if len(fn.Body.List) != 1 {
		return false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	return ok && len(ret.Results) == 1
}

func checkFactMethodHasPhrase(g *ontology.Graph) (out []Violation) {
	if !g.SelfExecutingAtoms {
		return nil
	}
	root := gate.SpecRootForGraph(g)
	view, index, err := InvocationSourceIndex(g)
	if view != nil && view.InvocationState != g.InvocationState {
		defer func() {
			if closeErr := CloseInvocation(view); closeErr != nil {
				out = append(out, Violation{Check: "execution_session_close", ID: root, Message: closeErr.Error()})
			}
		}()
	}
	g = view
	if err != nil {
		return []Violation{{Check: "check_fact_method_has_phrase", ID: root, Message: err.Error()}}
	}
	atomArtifacts(g, func(id string, a gate.AtomArtifact) {
		for _, step := range a.Steps {
			s, err := index.Resolve(step.Subject)
			if err != nil || strings.Trim(s.Phrase, " .!?;:…\t\r\n") == "" {
				out = append(out, Violation{Check: "check_fact_method_has_phrase", ID: id, Message: fmt.Sprintf("atom subject %q must resolve to a method with a non-empty first doc phrase (resolver: %v)", step.Subject, err)})
			}
		}
		if a.Title != "" {
			if _, err := index.DeriveClaim(a); err != nil {
				out = append(out, Violation{Check: "check_fact_method_has_phrase", ID: id, Message: err.Error()})
			}
		}
	})
	return out
}

func checkFactMethodAtomic(g *ontology.Graph) (out []Violation) {
	if !g.SelfExecutingAtoms {
		return nil
	}
	root := gate.SpecRootForGraph(g)
	view, index, err := InvocationSourceIndex(g)
	if view != nil && view.InvocationState != g.InvocationState {
		defer func() {
			if closeErr := CloseInvocation(view); closeErr != nil {
				out = append(out, Violation{Check: "execution_session_close", ID: root, Message: closeErr.Error()})
			}
		}()
	}
	g = view
	if err != nil {
		return []Violation{{Check: "check_fact_method_atomic", ID: root, Message: err.Error()}}
	}
	atomArtifacts(g, func(id string, a gate.AtomArtifact) {
		for i, step := range a.Steps {
			s, err := index.Resolve(step.Subject)
			if err != nil || !atomMethodAtomic(s.Method, a.Mode == "holds" && i == 0, a.Mode == "rule" && i == 0) {
				out = append(out, Violation{Check: "check_fact_method_atomic", ID: id, Message: fmt.Sprintf("atom subject %q must be a zero-argument single-result method; legacy value facts return one expression, Holds predicates return bool (resolver: %v)", step.Subject, err)})
			}
		}
	})
	return out
}

func factSubjectFailure(a gate.AtomArtifact) string {
	if a.Mode != "fact" {
		return ""
	}
	if len(a.Steps) != 1 || strings.TrimSpace(a.Steps[0].Subject) == "" {
		return "Fact must record exactly one non-empty method subject"
	}
	return ""
}

func checkOneSubjectPerFact(g *ontology.Graph) (out []Violation) {
	if !g.SelfExecutingAtoms {
		return nil
	}
	g, session, owned := invocationSession(g)
	if owned {
		defer func() {
			if err := session.Close(); err != nil {
				out = append(out, Violation{Check: "execution_session_close", ID: g.DomainDir, Message: err.Error()})
			}
		}()
	}
	atomArtifacts(g, func(id string, a gate.AtomArtifact) {
		if message := factSubjectFailure(a); message != "" {
			out = append(out, Violation{Check: "check_one_subject_per_fact", ID: id, Message: message})
		}
	})
	// Failed/invalid calls need a source check too: they cannot produce trusted
	// passing artifacts. Resolve import aliases rather than matching method names.
	root := gate.SpecRootForGraph(g)
	err := filepath.WalkDir(filepath.Join(root, "spec"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		_, f, err := session.SourceFile(path)
		if err != nil {
			return err
		}
		aliases := map[string]bool{}
		for _, imp := range f.Imports {
			pkg := strings.Trim(imp.Path.Value, "\"")
			if filepath.Base(pkg) != "hotamspec" {
				continue
			}
			name := "hotamspec"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			aliases[name] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := call.Fun
			if indexed, ok := fun.(*ast.IndexExpr); ok {
				fun = indexed.X
			}
			selector, ok := fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Fact" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[pkg.Name] {
				return true
			}
			valid := len(call.Args) >= 3
			if valid {
				_, closure := call.Args[1].(*ast.FuncLit)
				valid = !closure
			}
			for _, option := range call.Args[min(3, len(call.Args)):] {
				if _, literal := option.(*ast.BasicLit); literal {
					valid = false
				}
			}
			if !valid {
				out = append(out, Violation{Check: "check_one_subject_per_fact", ID: path, Message: "Fact requires (t, bound zero-argument method, want); closures and multiple subjects are not atom carriers"})
			}
			return true
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		out = append(out, Violation{Check: "check_one_subject_per_fact", ID: root, Message: fmt.Sprintf("cannot inspect Fact carriers: %v", err)})
	}
	return out
}

func freshAtomClaim(index *gate.AtomSourceIndex, runs map[string]gate.RecordingResult, requirement ontology.Requirement) (string, bool, error) {
	claim := ""
	found := false
	for _, entry := range requirement.VerifiedBy {
		file, test, ok := gate.ParseFileColonSymbol(entry)
		if !ok {
			return "", false, nil
		}
		matched := false
		for _, artifact := range recordedAtoms(file, test, runs) {
			if !atomMatchesRequirementIndexed(index, artifact, requirement) {
				continue
			}
			title, err := index.DeriveClaim(artifact)
			if err != nil {
				return "", false, err
			}
			if found && title != claim {
				return "", false, fmt.Errorf("atom %s has conflicting executed claims %q and %q across its verified_by carriers", requirement.ID, claim, title)
			}
			claim, found, matched = title, true, true
		}
		if !matched {
			return "", false, nil
		}
	}
	return claim, found, nil
}

func atomMatchesRequirementIndexed(index *gate.AtomSourceIndex, artifact gate.AtomArtifact, requirement ontology.Requirement) bool {
	if artifact.ReqID == requirement.ID {
		return true
	}
	if len(artifact.Steps) == 0 || len(requirement.ImplementedBy) == 0 {
		return false
	}
	source, err := index.Resolve(artifact.Steps[0].Subject)
	return err == nil && source.Link() == requirement.ImplementedBy[0]
}

var _ = All.MustRegister("check_fact_method_has_phrase", Invariant{Name: "check_fact_method_has_phrase", Canon: methodology.Domain, Claim: "atom subjects have doc phrases and no conflicting manual title", Rule: "In self_executing_atoms domains resolve each subject and derive doc plus value text.", Why: "A fact's text has one source, not a parallel authored title.", Check: checkFactMethodHasPhrase})
var _ = All.MustRegister("check_fact_method_atomic", Invariant{Name: "check_fact_method_atomic", Canon: methodology.Domain, Claim: "legacy value atoms return one expression; typed rule methods and bool relations may branch", Rule: "Every subject is a zero-argument single-result method; legacy value facts return one expression, Holds predicates return bool, and explicit rule methods may branch.", Why: "Structural atomicity is mechanical; meaning remains an audit concern.", Check: checkFactMethodAtomic})
var _ = All.MustRegister("check_one_subject_per_fact", Invariant{Name: "check_one_subject_per_fact", Canon: methodology.Domain, Claim: "each Fact has exactly one method subject", Rule: "Fact carriers record one subject and accept a bound method, not a closure.", Why: "Multiple facts belong in separate Fact calls or relation evidence.", Check: checkOneSubjectPerFact})
