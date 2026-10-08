package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// --- check_claim_matches_scenario -------------------------------------------
//
// Fixture shapes mirror spec_md_current_test.go's writeSpecMDFixtureModule /
// specMDFixtureGraph conventions exactly (a real, standalone temp Go module
// with a genuinely vendored hotamspec recorder — recordervendor.BodyForHash()
// — so gate.RunVerifiedByTestRecording actually records real artifacts, not
// an AST-only stub), since this check, like check_spec_md_current, pays the
// real go-test-execution cost rather than an AST-only one.

const claimScenarioFixtureImplSrc = `package model

type BrdPackage struct {
	Blockers int
}

func (p *BrdPackage) SignOff() error {
	if p.Blockers > 0 {
		return errHasBlockers
	}
	return nil
}

var errHasBlockers = stubErr{}

type stubErr struct{}

func (stubErr) Error() string { return "has blockers" }
`

func claimScenarioFixtureTestSrc(modulePath, reqID, title string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestBrdPackage_SignOff_RejectsBlockers(t *testing.T) {
	s := hotamspec.NewScenario(t, "` + reqID + `", "` + title + `")
	p := &BrdPackage{Blockers: 1}
	err := p.SignOff()
	s.Then("sign-off returns an error", err != nil)
}
`
}

// writeClaimScenarioFixtureModule mirrors spec_md_current_test.go's
// writeSpecMDFixtureModule, parameterized over the scenario's own title so
// mutation tests (below) can rewrite it between two graph reads.
func writeClaimScenarioFixtureModule(t *testing.T, reqID, title string) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	modulePath := "example.com/claimscenario"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(claimScenarioFixtureImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(claimScenarioFixtureTestSrc(modulePath, reqID, title)), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go: %v", err)
	}
	hotamspecDir := filepath.Join(root, "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.BodyForHash()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}
	return root
}

// claimScenarioFixtureGraph builds the graph the way loader.LoadGraph would
// populate DomainDir/Discipline (via a real manifest.json on disk), mirroring
// scenario_discipline_test.go's graphForDiscipline helper. discipline:"full"
// ALONE is no longer sufficient to activate check_claim_matches_scenario
// (task #388/W0.1) -- every call site that wants the check to actually run
// must use claimScenarioFixtureGraphWithAuthority(t, domainDir, discipline,
// loader.ClaimAuthorityScenario, r) instead. This helper always writes
// claim_authority absent (the "authored"/default case), so it now doubles as
// the NO-OP fixture for the new dual-gate no-op tests.
func claimScenarioFixtureGraph(t *testing.T, domainDir, discipline string, r ontology.Requirement) *ontology.Graph {
	t.Helper()
	return claimScenarioFixtureGraphWithAuthority(t, domainDir, discipline, "", r)
}

// claimScenarioFixtureGraphWithAuthority is claimScenarioFixtureGraph's
// superset: it additionally writes manifest.json's "claim_authority" field
// (task #388/W0.1) and populates g.ClaimAuthorityScenario the way
// loader.LoadGraph would (via loader.resolveClaimAuthorityScenario, exercised
// indirectly here through a real on-disk manifest read since that resolver is
// unexported outside package loader -- this test package reads it back the
// same way loader.LoadGraph itself does, by re-deriving from the literal).
func claimScenarioFixtureGraphWithAuthority(t *testing.T, domainDir, discipline, claimAuthority string, r ontology.Requirement) *ontology.Graph {
	t.Helper()
	manifest := `{"discipline": "` + discipline + `", "claim_authority": "` + claimAuthority + `"}`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	return &ontology.Graph{
		DomainDir:              domainDir,
		Discipline:             loader.ResolveDiscipline(graphPath),
		ClaimAuthorityScenario: claimAuthority == loader.ClaimAuthorityScenario,
		Requirements:           []ontology.Requirement{r},
	}
}

func claimScenarioReq(id, claim string, verifiedBy []string, enforceability string) ontology.Requirement {
	return ontology.Requirement{
		ID:             id,
		Claim:          claim,
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		Enforcement:    ontology.EnforcementENFORCED,
		Enforceability: enforceability,
		VerifiedBy:     verifiedBy,
	}
}

// TestCheckClaimMatchesScenario_NoOpWithoutDisciplineFull proves the
// domain-level honest no-op: a domain without discipline:"full" produces no
// violation even for a Claim that flatly disagrees with its scenario.
func TestCheckClaimMatchesScenario_NoOpWithoutDisciplineFull(t *testing.T) {
	root := writeClaimScenarioFixtureModule(t, "R-claim-scenario-1", "the real recorded title")
	r := claimScenarioReq("R-claim-scenario-1", "a completely different, stale claim", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraph(t, root, "", r)
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a non-discipline:full domain, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_GreenWhenClaimMatches proves the positive
// control: discipline:full + claim_authority:"scenario" (task #388/W0.1's
// dual-gate), Claim already equals the fresh derivation -> clean.
func TestCheckClaimMatchesScenario_GreenWhenClaimMatches(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-scenario e2e: discipline:full drives checkClaimMatchesScenario's freshDerivedClaim into a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
	title := "the real recorded title"
	root := writeClaimScenarioFixtureModule(t, "R-claim-scenario-2", title)
	r := claimScenarioReq("R-claim-scenario-2", title, []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraphWithAuthority(t, root, "full", loader.ClaimAuthorityScenario, r)
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	if !g.ClaimAuthorityScenario {
		t.Fatalf("test setup: expected ClaimAuthorityScenario=true")
	}
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations when Claim already matches the fresh derivation, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_FiresWhenClaimDiverges is the RED case:
// discipline:full + claim_authority:"scenario", committed Claim disagrees
// with what the verified_by test's CURRENT recorded scenario title says.
func TestCheckClaimMatchesScenario_FiresWhenClaimDiverges(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-scenario e2e: discipline:full drives checkClaimMatchesScenario's freshDerivedClaim into a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
	root := writeClaimScenarioFixtureModule(t, "R-claim-scenario-3", "the real recorded title")
	r := claimScenarioReq("R-claim-scenario-3", "a stale claim that no longer matches", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraphWithAuthority(t, root, "full", loader.ClaimAuthorityScenario, r)
	vs := runCheck(t, "check_claim_matches_scenario", g)
	if !hasViolationFor(vs, "R-claim-scenario-3") {
		t.Fatalf("expected a violation for a diverged Claim under discipline:full+claim_authority:scenario, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_NoOpWithDisciplineFullButNoClaimAuthority is
// the CORE regression fix this task exists for (task #388/W0.1, acceptance
// criterion (a)): a domain that is discipline:"full" but has NOT separately
// opted into claim_authority:"scenario" must stay an honest no-op, even for a
// Claim that flatly disagrees with its scenario -- exactly the shape of
// prat's and gpsm-sm's real manifests (discipline:"full" flipped long before
// claim_authority existed, never consenting to the exact-drift gate).
func TestCheckClaimMatchesScenario_NoOpWithDisciplineFullButNoClaimAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-scenario e2e: discipline:full drives checkClaimMatchesScenario's freshDerivedClaim into a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
	root := writeClaimScenarioFixtureModule(t, "R-claim-scenario-noauth", "the real recorded title")
	r := claimScenarioReq("R-claim-scenario-noauth", "a completely different, stale claim that would fire if compared", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraph(t, root, "full", r) // claim_authority absent -- the default
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	if g.ClaimAuthorityScenario {
		t.Fatalf("test setup: expected ClaimAuthorityScenario=false (absent key)")
	}
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for discipline:full WITHOUT claim_authority:scenario (task #388/W0.1), got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_InherentlyProseIsExempt proves the
// requirement-level exemption mirrors checkSettledRequiresScenario's own:
// a requirement tagged INHERENTLY_PROSE is never compared, even with a
// wildly stale Claim.
func TestCheckClaimMatchesScenario_InherentlyProseIsExempt(t *testing.T) {
	root := writeClaimScenarioFixtureModule(t, "R-claim-scenario-4", "the real recorded title")
	r := claimScenarioReq("R-claim-scenario-4", "a stale claim that no longer matches", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityINHERENTLY_PROSE)
	g := claimScenarioFixtureGraphWithAuthority(t, root, "full", loader.ClaimAuthorityScenario, r)
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an INHERENTLY_PROSE requirement, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_NoVerifiedByIsSkipped proves a requirement
// with no verified_by entries at all is out of scope for this check
// (check_settled_requires_scenario's own job to flag the missing carrier).
func TestCheckClaimMatchesScenario_NoVerifiedByIsSkipped(t *testing.T) {
	root := t.TempDir()
	r := claimScenarioReq("R-claim-scenario-5", "any claim at all", nil, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraphWithAuthority(t, root, "full", loader.ClaimAuthorityScenario, r)
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a requirement with no verified_by entries, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_MUTATION_StaleAfterTitleEditWithoutResync is
// the mutation probe the task's own verification step calls for: a Claim
// that matches its scenario goes RED the moment the underlying test's
// hotamspec.NewScenario title is edited WITHOUT re-deriving Claim (simulating
// "someone edited the verified_by test's description but forgot to re-run
// `hotam sync-domain`"), then GREEN again once Claim is re-derived to match.
func TestCheckClaimMatchesScenario_MUTATION_StaleAfterTitleEditWithoutResync(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-scenario e2e: discipline:full drives checkClaimMatchesScenario's freshDerivedClaim into repeated real go build/go test subprocesses via gate.RunVerifiedByTestRecording (FRESH, EDITED, RE-DERIVED passes); skipped in -short")
	}
	reqID := "R-claim-scenario-mutation"
	originalTitle := "the original recorded title"
	root := writeClaimScenarioFixtureModule(t, reqID, originalTitle)
	testPath := filepath.Join(root, "model", "impl_test.go")

	r := claimScenarioReq(reqID, originalTitle, []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	g := claimScenarioFixtureGraphWithAuthority(t, root, "full", loader.ClaimAuthorityScenario, r)

	// FRESH: Claim matches the currently-recorded title -- clean.
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("FRESH: expected no violations, got %v", vs)
	}

	// EDIT: rewrite the test's scenario title WITHOUT touching Claim --
	// simulates an author editing the verified_by test's narration and
	// forgetting to re-run `hotam sync-domain` to re-derive Claim.
	newTitle := "a completely reworded scenario title"
	if err := os.WriteFile(testPath, []byte(claimScenarioFixtureTestSrc("example.com/claimscenario", reqID, newTitle)), 0o644); err != nil {
		t.Fatalf("WriteFile test edit: %v", err)
	}
	vs := runCheck(t, "check_claim_matches_scenario", g)
	if !hasViolationFor(vs, reqID) {
		t.Fatalf("EDITED: expected a violation once the scenario title diverges from the stale Claim, got %v", vs)
	}

	// RE-DERIVE: mirror what `hotam sync-domain` would do -- re-derive Claim
	// via the same selfspec machinery, feed it back into the graph -- clean
	// again.
	session := gate.NewExecutionSession()
	defer func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	}()
	fresh, ok := freshDerivedClaim(session, root, false, r.VerifiedBy)
	if !ok {
		t.Fatalf("re-derivation: expected ok=true, got false")
	}
	if fresh != newTitle {
		t.Fatalf("re-derivation: got %q, want %q", fresh, newTitle)
	}
	g.Requirements[0].Claim = fresh
	if vs := runCheck(t, "check_claim_matches_scenario", g); len(vs) != 0 {
		t.Fatalf("RE-DERIVED: expected no violations after re-deriving Claim, got %v", vs)
	}
}

// TestCheckClaimMatchesScenario_MatchesSelfspecDerivation cross-checks this
// package's own freshDerivedClaim against internal/selfspec's exported
// DeriveClaimsFromScenarios on the SAME fixture -- proves the two
// independent (deliberately duplicated, see freshDerivedClaim's own doc
// comment) implementations can never silently disagree.
func TestCheckClaimMatchesScenario_MatchesSelfspecDerivation(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-scenario e2e: cross-checks freshDerivedClaim against selfspec.DeriveClaimsFromScenarios, each driving a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
	reqID := "R-claim-scenario-crosscheck"
	title := "cross-checked recorded title"
	root := writeClaimScenarioFixtureModule(t, reqID, title)
	verifiedBy := []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}

	session := gate.NewExecutionSession()
	defer func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	}()
	viaCheck, ok := freshDerivedClaim(session, root, false, verifiedBy)
	if !ok {
		t.Fatalf("freshDerivedClaim: expected ok=true")
	}

	reg := registryForSelfspecCrossCheck(reqID, verifiedBy)
	selfspec.DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	viaSelfspec, _ := reg.Get(reqID)

	if viaCheck != viaSelfspec.Claim {
		t.Fatalf("freshDerivedClaim = %q, selfspec.DeriveClaimsFromScenarios = %q -- the two derivations disagree", viaCheck, viaSelfspec.Claim)
	}
	if viaCheck != title {
		t.Fatalf("derived Claim = %q, want %q", viaCheck, title)
	}
}

// registryForSelfspecCrossCheck builds a minimal *registry.Registry[ontology.
// Requirement] carrying exactly one requirement (reqID, verifiedBy, a stale
// placeholder Claim distinct from any real scenario title so the cross-check
// above can prove derivation actually changed it), for
// TestCheckClaimMatchesScenario_MatchesSelfspecDerivation's own use.
func registryForSelfspecCrossCheck(reqID string, verifiedBy []string) *registry.Registry[ontology.Requirement] {
	reg := registry.New[ontology.Requirement]()
	reg.MustRegister(reqID, ontology.Requirement{
		ID:             reqID,
		Claim:          "placeholder claim before derivation",
		Enforceability: ontology.EnforceabilityENFORCEABLE,
		VerifiedBy:     verifiedBy,
	})
	return reg
}
