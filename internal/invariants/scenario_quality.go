// scenario_quality.go implements check_scenario_quality (task #397/W1.5): a
// POST-HOC QUALITY gate over a requirement's already-recorded scenario
// artifact(s), sitting on top of check_settled_requires_scenario's cheap,
// AST-only "does a scenario exist at all" signal
// (internal/invariants/scenario_discipline.go's anyVerifiedByEntryHasScenario,
// gate.ResolveSpecTest's HasScenario). A test that does
// `s := hotamspec.NewScenario(t, "R-x", ""); s.Then("", true)` formally
// satisfies check_settled_requires_scenario/check_model_complete's "has a
// scenario" bar with an empty title, an empty Then description, and (if it
// also has a Given/When) no guarantee they are recorded in a sensible order.
// This check closes that gap by evaluating the REAL recorded Artifact against
// four quality rules (see checkScenarioQuality's own doc comment for the
// exact rules).
//
// OPT-IN TRIGGER: scenario_authority:"quality" (loader.ScenarioAuthorityQuality,
// internal/loader/loader.go) -- its own, brand-new, INDEPENDENT trigger, NOT
// co-gated with discipline:"full"/claim_authority:"scenario"/
// public_surface_authority:"linked" -- see R-opt-in-trigger-owns-its-own-
// obligations (internal/selfspec/requirements_authoredspec.go) and this
// package's own scenario_authority_ratchet.go for the mechanical embodiment
// of that law for this check's own trigger.
//
// PERFORMANCE: the invocation owns both the immutable atom snapshot and shared
// compiled artifacts. Manual scenario recording still executes afresh; no
// recording verdict survives this invocation's Close.
//
// AST PREFILTER (zero execution cost for the common case): for each
// verified_by entry, anyVerifiedByEntryHasScenario-style AST prefiltering
// (gate.ResolveSpecTest's HasScenario) skips any entry with NO scenario
// constructor call at all -- an honest no-op, mirroring
// checkSettledRequiresScenario's own "nothing to check" boundary. Only
// entries that DO carry a scenario constructor call pay the real `go test`
// price this check needs to inspect the ACTUAL recorded steps.
package invariants

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// scenarioQualityArtifact is this file's own local decode target for one
// gate.RecordedArtifact's RawJSON bytes -- the same "equivalent local shape
// rather than widening gate's public API" precedent
// internal/selfspec/claim_derive.go's own scenarioArtifact establishes (see
// that type's doc comment). Unlike scenarioArtifact (which only needs
// ReqID/Title/Verdict), this check additionally needs the Steps themselves
// (Kind/Desc, in call order) to evaluate rules 3 and 4 below --
// internal/gate/spec_build.go's own unexported specArtifact/specArtifactStep
// is the reference shape mirrored field-for-field, but cannot be imported
// directly (different package, unexported).
type scenarioQualityArtifact struct {
	ReqID   string                        `json:"req_id"`
	Test    string                        `json:"test"`
	Title   string                        `json:"title"`
	Steps   []scenarioQualityArtifactStep `json:"steps"`
	Verdict string                        `json:"verdict"`
}

// scenarioQualityArtifactStep mirrors internal/recorder/canon's own
// ArtifactStep JSON shape (Kind/Desc) -- only the two fields this check's
// four rules actually inspect; Values/Passed are irrelevant to scenario
// quality and deliberately not decoded here.
type scenarioQualityArtifactStep struct {
	Kind string `json:"kind"`
	Desc string `json:"desc"`
}

// The literal StepKind JSON string values, confirmed against
// internal/recorder/canon/hotamspec.go's own StepGiven/StepWhen/StepThen
// consts (StepKind is a defined string type, so its JSON rendering is the
// literal Go constant value: "given"/"when"/"then"/"value") -- duplicated
// here as plain string literals (not an import of the canon package's typed
// consts) for the same reason scenarioQualityArtifact itself is a local
// decode target: this package never imports internal/recorder/canon for
// production code (only this package's own *_test.go files do, to build a
// REAL vendored-recorder fixture -- see scenario_quality_test.go).
const (
	stepKindGiven = "given"
	stepKindWhen  = "when"
	stepKindThen  = "then"
)

// checkScenarioQuality is check_scenario_quality's Check function.
//
// SCOPE: activates ONLY when g.ScenarioAuthorityQuality is true
// (manifest.json's "scenario_authority": "quality" opt-in) -- deliberately
// NOT gated on g.Discipline, matching public_surface_authority:"linked"'s own
// precedent (loader.PublicSurfaceAuthorityLinked's doc comment), not
// claim_authority:"scenario"'s (which uniquely requires discipline:"full" as
// a co-requirement). A domain that has not opted in is a pure HONEST NO-OP,
// regardless of how many SETTLED requirements it has or how bare/malformed
// their recorded scenarios are.
//
// For every SETTLED requirement, scope is its verified_by entries. An entry
// with NO scenario constructor call at all (AST prefilter, gate.ResolveSpecTest's
// HasScenario) is skipped -- check_settled_requires_scenario is the check
// responsible for flagging "no scenario carrier at all"; this check's own
// concern begins only once a scenario exists to evaluate.
//
// For each scenario-carrying entry, gate.RunVerifiedByTestRecording is called
// and every resulting PASS-verdict artifact (a Scenario constructed inside a
// test whose OTHER assertions failed produces a "fail"-verdict artifact that
// must never be trusted as proof of anything -- mirrors
// scenarioArtifactTitleIfPass's own precedent, internal/selfspec/
// claim_derive.go) is evaluated against four rules:
//
//  1. Non-empty title: strings.TrimSpace(Title) != "".
//  2. Exact requirement ID match: Artifact.ReqID == the citing requirement's
//     own ID -- a scenario recorded under a DIFFERENT reqID than the
//     requirement whose verified_by cites this test is a mismatch (a
//     copy-paste error, or a scenario meant for a different requirement).
//  3. At least one Then step: at least one Steps entry with Kind == "then".
//  4. Ordering, behavioral vs declarative: an artifact with at least one
//     Kind == "when" step is BEHAVIORAL; zero When steps means DECLARATIVE/
//     INVARIANT (the absence of a When call IS the explicit "no action step"
//     signal -- no separate marker API, per this task's own scope boundary:
//     internal/recorder/canon/hotamspec.go is never touched). A BEHAVIORAL
//     artifact must hold: max(index of any "given" step) < min(index of any
//     "when" step) < min(index of any "then" step) -- every Given precedes
//     the first When, and the first When precedes the first Then. This is
//     deliberately NOT "every When precedes every Then" (max(when) <
//     min(then)): a scenario with two legitimate When/Then cycles ("When A,
//     Then B, When C, Then D" -- a common, valid BDD shape for a sequence of
//     actions) has a When step AFTER a Then step and must not be rejected for
//     it. "value" steps are ignored entirely for this rule (orthogonal
//     narration). A DECLARATIVE artifact (zero When steps) is exempt from
//     this rule entirely -- rules 1-3 still apply to it.
//
// COMPLIANCE BAR PER REQUIREMENT (mirrors anyVerifiedByEntryHasScenario's own
// OR-across-entries semantics, and check_model_complete's own anyScenario
// OR-semantics): a requirement is COMPLIANT iff AT LEAST ONE of its
// verified_by entries' pass-verdict artifacts satisfies ALL FOUR rules. A
// requirement where every scenario-carrying artifact fails at least one rule
// (or produces zero pass-verdict artifacts at all despite carrying a
// scenario per the AST prefilter -- e.g. the test currently fails) is a
// violation, naming the requirement and listing every checked artifact with
// which specific rule(s) it failed.
func checkScenarioQuality(g *ontology.Graph) (out []Violation) {
	if !g.ScenarioAuthorityQuality {
		// Honest no-op -- see this check's own doc comment / loader.
		// ScenarioAuthorityQuality's doc comment for why this trigger is
		// independent of g.Discipline.
		return nil
	}
	specRoot := gate.SpecRootForGraph(g)
	g, session, owned := invocationSession(g)
	if owned {
		defer func() {
			if err := session.Close(); err != nil {
				out = append(out, Violation{Check: "execution_session_close", ID: g.DomainDir, Message: err.Error()})
			}
		}()
	}
	var atomIndex *gate.AtomSourceIndex
	var atomRuns map[string]gate.RecordingResult
	needAtomSnapshot := false
	if g.SelfExecutingAtoms {
		for _, r := range g.Requirements {
			for _, e := range parseSpecEntries(r.VerifiedBy) {
				if e.ok && strings.HasPrefix(e.symbol, "Test") && fileInAtomPackages(e.file, g.SelfExecutingAtomPackages) {
					needAtomSnapshot = true
					break
				}
			}
			if needAtomSnapshot {
				break
			}
		}
	}
	if g.SelfExecutingAtoms && (len(g.SelfExecutingAtomPackages) == 0 || needAtomSnapshot) {
		_, snapshot, err := InvocationExecutionSnapshot(g)
		if err != nil {
			return []Violation{{Check: "check_scenario_quality", ID: specRoot, Message: err.Error()}}
		}
		if snapshot.SourceErr != nil {
			return []Violation{{Check: "check_scenario_quality", ID: specRoot, Message: snapshot.SourceErr.Error()}}
		}
		atomIndex, atomRuns = snapshot.SourceIndex, snapshot.PackageRuns
	}
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		entries := parseSpecEntries(r.VerifiedBy)
		var scenarioEntries []specFileEntry
		for _, e := range entries {
			if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				continue
			}
			result, err := resolveSpecTestForGraph(g, specRoot, e.file, e.symbol, g.AtomRecorderImportPath)
			if err != nil || !result.Found || !result.HasScenario {
				continue
			}
			scenarioEntries = append(scenarioEntries, e)
		}
		if len(scenarioEntries) == 0 {
			// No scenario-carrying verified_by entry at all -- not this
			// check's concern (check_settled_requires_scenario already
			// covers "has no scenario at all" under its own trigger).
			continue
		}

		compliant := false
		var diagnostics []string
		for _, e := range scenarioEntries {
			var result gate.RecordingResult
			if g.SelfExecutingAtoms && (len(g.SelfExecutingAtomPackages) == 0 || fileInAtomPackages(e.file, g.SelfExecutingAtomPackages)) {
				key := filepath.ToSlash(filepath.Dir(filepath.FromSlash(e.file)))
				var exists bool
				result, exists = atomRuns[key]
				if !exists {
					result.Err = fmt.Errorf("scenario package %q missing from shared execution snapshot", key)
				} else {
					result.TestRunResult = result.ForTest(e.symbol)
				}
			} else {
				result = session.RunVerifiedByTestRecording(specRoot, e.file, e.symbol, "")
			}
			if result.Skipped || result.Err != nil || result.CompileFailed || !result.Passed {
				diagnostics = append(diagnostics, fmt.Sprintf(
					"%s: could not be executed to inspect its recorded scenario (skipped=%v, err=%v, compileFailed=%v, passed=%v)",
					e.raw, result.Skipped, result.Err, result.CompileFailed, result.Passed))
				continue
			}
			if len(result.Artifacts) == 0 {
				diagnostics = append(diagnostics, fmt.Sprintf("%s: executed and passed but recorded no scenario artifact", e.raw))
				continue
			}
			for _, art := range result.Artifacts {
				parsed, ok := decodeScenarioQualityArtifact(art.RawJSON)
				if !ok {
					diagnostics = append(diagnostics, fmt.Sprintf("%s: artifact %s could not be decoded", e.raw, art.FileName))
					continue
				}
				if g.SelfExecutingAtoms && parsed.Test != e.symbol && !strings.HasPrefix(parsed.Test, e.symbol+"/") {
					continue
				}
				if parsed.Verdict != "pass" {
					// Never trusted as proof of anything -- mirrors
					// scenarioArtifactTitleIfPass's own precedent.
					diagnostics = append(diagnostics, fmt.Sprintf("%s: artifact %s has verdict %q, not \"pass\" -- skipped", e.raw, art.FileName, parsed.Verdict))
					continue
				}
				if g.SelfExecutingAtoms {
					atom, err := gate.DecodeAtomArtifact(art.RawJSON)
					if err == nil && (atom.Mode == "fact" || atom.Mode == "holds") {
						_, err = atomIndex.DeriveClaim(atom)
						if err == nil && !atomMatchesRequirementIndexed(atomIndex, atom, r) {
							err = fmt.Errorf("atom req_id %q does not match citing requirement %q or its primary implemented_by", atom.ReqID, r.ID)
						}
						if err == nil && factSubjectFailure(atom) == "" {
							compliant = true
							break
						}
						diagnostics = append(diagnostics, fmt.Sprintf("%s: invalid atom artifact: %v %s", e.raw, err, factSubjectFailure(atom)))
						continue
					}
				}
				failed := scenarioQualityFailures(parsed, r.ID)
				if len(failed) == 0 {
					compliant = true
					break
				}
				diagnostics = append(diagnostics, fmt.Sprintf("%s: artifact %s (title=%q) failed: %s", e.raw, art.FileName, parsed.Title, strings.Join(failed, "; ")))
			}
			if compliant {
				break
			}
		}
		if compliant {
			continue
		}
		out = append(out, Violation{
			Check: "check_scenario_quality",
			ID:    r.ID,
			Message: fmt.Sprintf(
				"scenario_authority:\"quality\" requires at least one verified_by entry's pass-verdict scenario "+
					"artifact to satisfy all four quality rules (non-empty title, exact requirement-ID match, at "+
					"least one Then step, and -- for a behavioral scenario carrying a When step -- a strict "+
					"Given-before-When-before-Then order): %s carries a scenario (AST-confirmed) but no checked "+
					"artifact qualified -- %s",
				r.ID, strings.Join(diagnostics, " | ")),
		})
	}
	return out
}

// decodeScenarioQualityArtifact decodes raw (one gate.RecordedArtifact.RawJSON)
// into scenarioQualityArtifact. A malformed/undecodable artifact
// (structurally impossible for a genuine recorder-produced file, but handled
// defensively exactly as internal/selfspec/claim_derive.go's own
// scenarioArtifactTitleIfPass does) returns ok=false.
func decodeScenarioQualityArtifact(raw []byte) (scenarioQualityArtifact, bool) {
	var parsed scenarioQualityArtifact
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return scenarioQualityArtifact{}, false
	}
	return parsed, true
}

// scenarioQualityFailures evaluates art against the four quality rules for a
// requirement whose verified_by cites it (wantReqID is that requirement's own
// ID). Returns a list of human-readable failure descriptions -- empty iff art
// satisfies all four rules.
func scenarioQualityFailures(art scenarioQualityArtifact, wantReqID string) []string {
	var failed []string

	if strings.TrimSpace(art.Title) == "" {
		failed = append(failed, "rule 1 (non-empty title): title is empty")
	}

	if art.ReqID != wantReqID {
		failed = append(failed, fmt.Sprintf("rule 2 (exact requirement-ID match): artifact req_id %q does not match citing requirement %q", art.ReqID, wantReqID))
	}

	hasThen := false
	for _, st := range art.Steps {
		if st.Kind == stepKindThen {
			hasThen = true
			break
		}
	}
	if !hasThen {
		failed = append(failed, "rule 3 (at least one Then step): no step with kind \"then\"")
	}

	// Rule 4: ordering, behavioral vs declarative.
	maxGiven, hasGiven := -1, false
	minWhen, hasWhen := -1, false
	minThen, hasThenIdx := -1, false
	for i, st := range art.Steps {
		switch st.Kind {
		case stepKindGiven:
			hasGiven = true
			if i > maxGiven {
				maxGiven = i
			}
		case stepKindWhen:
			if !hasWhen || i < minWhen {
				minWhen = i
			}
			hasWhen = true
		case stepKindThen:
			if !hasThenIdx || i < minThen {
				minThen = i
			}
			hasThenIdx = true
		}
	}
	if hasWhen {
		// BEHAVIORAL: every Given must precede the FIRST When, and the first
		// When must precede the FIRST Then -- deliberately not "every When
		// precedes every Then" (max(when) < min(then)), which would reject
		// legitimate multi-cycle scenarios like "When A, Then B, When C, Then
		// D" (see checkScenarioQuality's own doc comment, rule 4).
		orderOK := true
		if hasGiven && !(maxGiven < minWhen) {
			orderOK = false
		}
		if hasThenIdx && !(minWhen < minThen) {
			orderOK = false
		}
		if !orderOK {
			failed = append(failed, "rule 4 (behavioral ordering): steps are not in strict Given-before-When-before-Then order")
		}
	}
	// DECLARATIVE (zero When steps): exempt from rule 4 entirely.

	return failed
}

var _ = All.MustRegister("check_scenario_quality", Invariant{
	Name:  "check_scenario_quality",
	Canon: methodology.Requirement,
	Claim: "in a domain whose manifest.json declares scenario_authority:\"quality\" (its OWN, independent opt-in " +
		"trigger, task #397/W1.5), every SETTLED requirement that carries a scenario (AST-confirmed via " +
		"gate.ResolveSpecTest's HasScenario on at least one verified_by entry) must have AT LEAST ONE verified_by " +
		"entry whose pass-verdict recorded scenario artifact is MEANINGFUL: a non-empty title, an exact match " +
		"between the artifact's own req_id and the citing requirement's ID, at least one Then step, and -- for a " +
		"behavioral scenario (one that records a When step) -- a strict Given-before-When-before-Then step order. A " +
		"domain that has not opted into scenario_authority:\"quality\" is an honest no-op; a requirement with no " +
		"scenario-carrying verified_by entry at all is check_settled_requires_scenario's own concern, not this " +
		"check's.",
	Rule: "IF g.ScenarioAuthorityQuality is false (manifest.json's \"scenario_authority\" is absent or not literally " +
		"\"quality\"), this check is a pure HONEST NO-OP -- independent of g.Discipline/g.ClaimAuthorityScenario/" +
		"g.PublicSurfaceAuthorityLinked. OTHERWISE, for EVERY Requirement r with Status == SETTLED: collect r's " +
		"verified_by entries that AST-resolve (gate.ResolveSpecTest) to a real Test* function whose body calls " +
		"hotamspec.NewScenario(...) anywhere (HasScenario) -- entries that do not carry a scenario constructor call " +
		"at all are skipped (zero execution cost, not this check's concern). If r has zero such entries, skip r " +
		"entirely (honest no-op for this check specifically). OTHERWISE, for each scenario-carrying entry, call " +
		"gate.RunVerifiedByTestRecording(specRoot, file, testName, \"\") and, for every resulting artifact whose own " +
		"recorded verdict is exactly \"pass\", evaluate FOUR rules: (1) strings.TrimSpace(Title) != \"\"; (2) " +
		"Artifact.ReqID == r.ID exactly; (3) at least one Steps entry has Kind == \"then\"; (4) if the artifact has " +
		"at least one Kind == \"when\" step (BEHAVIORAL), the maximum step-index among Kind == \"given\" entries " +
		"must be STRICTLY LESS than the minimum step-index among Kind == \"when\" entries, AND that minimum \"when\" " +
		"index must be STRICTLY LESS than the minimum step-index among Kind == \"then\" entries -- every Given precedes " +
		"the FIRST When, and the first When precedes the FIRST Then only (deliberately NOT max(when) < min(then), " +
		"which would reject a legitimate multi-cycle scenario like \"When A, Then B, When C, Then D\"; Kind == " +
		"\"value\" steps are ignored for this rule) -- an artifact with ZERO When steps is " +
		"DECLARATIVE/INVARIANT and is exempt from rule 4 entirely (rules 1-3 still apply). r is COMPLIANT iff AT " +
		"LEAST ONE checked pass-verdict artifact across ALL its scenario-carrying verified_by entries satisfies ALL " +
		"FOUR rules (OR-across-entries, mirroring anyVerifiedByEntryHasScenario's and check_model_complete's own " +
		"anyScenario semantics). A non-compliant r fires ONE violation naming r.ID and listing every checked " +
		"artifact plus which specific rule(s) it failed (or, for an entry that could not even be executed/decoded, " +
		"why).",
	Why: "task #397 (W1.5): check_settled_requires_scenario/check_model_complete's 'has a scenario' bar is purely " +
		"AST-based (does the test body call hotamspec.NewScenario anywhere) -- a test that does `s := " +
		"hotamspec.NewScenario(t, \"R-x\", \"\"); s.Then(\"\", true)` formally satisfies that bar with an empty " +
		"title, an empty Then description, and (if it also has a Given/When) no guarantee they are recorded in a " +
		"sensible order. This directly undermines the methodology's core claim -- 'a claim cannot exist without a " +
		"GREEN and MEANINGFUL proof beside it', not merely without a scenario-constructor call. " +
		"check_scenario_quality closes that gap by inspecting the REAL recorded Artifact (via " +
		"gate.RunVerifiedByTestRecording, the same call shape check_claim_matches_scenario/DeriveClaimsFromScenarios " +
		"already use -- never a bespoke AST parser reading Given/When/Then call literals, which would be fragile " +
		"against dynamic titles, helper-function indirection, and loops, and would duplicate work the engine's own " +
		"recorder+record-mode machinery already does better). OWN OPT-IN TRIGGER (R-opt-in-trigger-owns-its-own-" +
		"obligations, internal/selfspec/requirements_authoredspec.go): check_settled_requires_scenario/" +
		"check_model_complete (discipline:\"full\") and check_claim_matches_scenario/" +
		"check_public_surface_linked_or_marked (claim_authority:\"scenario\"/public_surface_authority:\"linked\", " +
		"their own separate triggers) already have real consumer domains relying on their CURRENT obligation sets -- " +
		"tightening any of those triggers' existing 'has a scenario' bar would silently break them overnight, " +
		"structurally the same harm task #369 caused and task #388 fixed. scenario_authority:\"quality\" " +
		"(loader.ScenarioAuthorityQuality) is therefore a brand new, wholly independent trigger, NOT co-gated with " +
		"discipline:\"full\" (mirroring public_surface_authority:\"linked\"'s own precedent, not claim_authority:" +
		"\"scenario\"'s, which uniquely requires discipline:\"full\" as a co-requirement) -- see " +
		"loader.ScenarioAuthorityQuality's own doc comment. check_scenario_authority_ratchet (scenario_authority_" +
		"ratchet.go) makes that trigger's own one-way-door promise mechanical, symmetric with " +
		"check_public_surface_authority_ratchet/check_claim_authority_ratchet/check_discipline_ratchet. This wave " +
		"deliberately opts NO real domain into scenario_authority:\"quality\" -- domains/hotam-spec-self, " +
		"domains/hotam-dev, and everything under PRAT-hotam stay honest no-ops for this check.",
	Check: checkScenarioQuality,
})
