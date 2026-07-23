// orientation_faq_assert.go evaluates an orientation_faq entry's optional
// "assert" contract (loader.OrientationFAQAssert) against LIVE graph state,
// closing the gap check_orientation_faq_answered's pure keyword-matching
// signal cannot: a keyword phrase's lexical PRESENCE in the crystal proves
// nothing about whether the phrase is still semantically TRUE relative to
// the graph's current state (task #321/R3-semantic-faq — see
// loader.OrientationFAQAssert's doc comment for the real "27 of 32" bug this
// closes). Dispatches on Assert.Kind to the matching internal/graphfacts
// tally function (graphfacts, NOT internal/query: internal/query is a
// PERIPHERY consumer package per internal/selfcheck/imports_test.go's
// R-core-periphery-import-ratchet, and this package — internal/invariants —
// is CORE; a core package must never import a periphery package, so the
// live-fact tallies live in internal/graphfacts instead, a package in
// neither the core nor periphery set, importable from both sides of that
// one-way arrow), then checks the live (count, total) pair against the
// entry's declared Expect and/or Phrase — fail-closed on every malformed or
// unrecognized shape, matching this package's existing "the check, not the
// loader, diagnoses" convention (see orientation_faq.go's own doc comment).
package invariants

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/graphfacts"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// evalOrientationAssert evaluates e.Assert (which MUST be non-nil — callers
// check that before invoking this) against g's live state, and against
// crystalText/linkText for a declared Phrase (crystalText is always the
// crystal's lowercased text; linkText is the linked file's lowercased
// content when e.Link is declared, else ""). Returns nil when the assert is
// satisfied, or one or more Violations naming e.Question describing exactly
// what failed.
func evalOrientationAssert(g *ontology.Graph, e loader.OrientationFAQEntry, crystalText, linkText string) []Violation {
	a := e.Assert
	if a == nil {
		return nil
	}

	var count, total int
	switch a.Kind {
	case "gate_signoff_count":
		if strings.TrimSpace(a.Stage) == "" {
			return []Violation{assertViolation(e, "assert kind \"gate_signoff_count\" requires a non-empty \"stage\"")}
		}
		var order []string
		if g.DomainDir != "" {
			order = loader.ResolveGateStageOrder(filepath.Join(g.DomainDir, "graph.json"))
		}
		if !stringInSlice(order, a.Stage) {
			return []Violation{assertViolation(e, fmt.Sprintf(
				"assert kind \"gate_signoff_count\" names stage %q, which is not present in the domain's declared gate_stage_order %v",
				a.Stage, order))}
		}

		// Multi-pipeline-run guard (task #330): a stage that has recorded
		// GateSignoffs from more than one distinct pipeline_run cannot be
		// tallied honestly without knowing WHICH run this assert means —
		// fail closed rather than silently conflating runs.
		if strings.TrimSpace(a.PipelineRun) == "" {
			if runs := graphfacts.PipelineRunsAtStage(g, order, a.Stage); len(runs) > 1 {
				return []Violation{assertViolation(e, fmt.Sprintf(
					"assert kind \"gate_signoff_count\" targets stage %q, which has signoffs from %d distinct pipeline_runs %v — declare \"pipeline_run\" on this assert to disambiguate",
					a.Stage, len(runs), runs))}
			}
		}

		tally := graphfacts.GateSignoffTally(g, order, a.Stage, a.PipelineRun)

		switch a.State {
		case "", ontology.GateSignoffStateSigned:
			count = tally.Signed
		case ontology.GateSignoffStateDeferred:
			count = tally.Deferred
		default:
			return []Violation{assertViolation(e, fmt.Sprintf(
				"assert kind \"gate_signoff_count\" declares unrecognized \"state\" %q — must be \"\", \"SIGNED\", or \"DEFERRED\"",
				a.State))}
		}

		if spec := loader.ResolveGateCohort(filepath.Join(g.DomainDir, "graph.json")); spec != nil {
			member, violation := gateCohortMemberPredicate(e, g, spec)
			if violation != nil {
				return []Violation{*violation}
			}
			total = graphfacts.CohortCount(g, member)
		} else {
			// No gate_cohort declared — today's semantics, byte-identical:
			// the denominator is whatever this stage has actually recorded
			// (Signed+Deferred), blind to any Requirement never evaluated
			// at this stage at all.
			total = tally.Signed + tally.Deferred
		}

	case "conflict_count_by_lifecycle":
		c, t, err := graphfacts.ConflictLifecycleTally(g, a.LifecycleClass)
		if err != nil {
			return []Violation{assertViolation(e, fmt.Sprintf("assert kind \"conflict_count_by_lifecycle\": %v", err))}
		}
		count, total = c, t

	case "requirement_count_by_status":
		c, t, err := graphfacts.RequirementStatusTally(g, a.Status, a.Enforcement)
		if err != nil {
			return []Violation{assertViolation(e, fmt.Sprintf("assert kind \"requirement_count_by_status\": %v", err))}
		}
		count, total = c, t

	default:
		return []Violation{assertViolation(e, fmt.Sprintf("assert declares unknown kind %q — must be one of \"gate_signoff_count\", \"conflict_count_by_lifecycle\", \"requirement_count_by_status\"", a.Kind))}
	}

	hasExpect := len(a.Expect) > 0
	hasPhrase := strings.TrimSpace(a.Phrase) != ""
	if !hasExpect && !hasPhrase {
		return []Violation{assertViolation(e, "assert declares no checkable predicate (neither \"expect\" nor \"phrase\" is present)")}
	}

	var out []Violation

	if hasExpect {
		ok, reason := evalExpect(a.Expect, count, total)
		if reason != "" {
			out = append(out, assertViolation(e, fmt.Sprintf("assert \"expect\" is malformed: %s", reason)))
		} else if !ok {
			out = append(out, assertViolation(e, fmt.Sprintf(
				"assert live state (count=%d, total=%d) does not satisfy the declared \"expect\"", count, total)))
		}
	}

	if hasPhrase {
		phrase := strings.NewReplacer(
			"{count}", strconv.Itoa(count),
			"{total}", strconv.Itoa(total),
		).Replace(a.Phrase)
		phraseLower := strings.ToLower(phrase)
		inCrystal := strings.Contains(crystalText, phraseLower)
		inLink := linkText != "" && strings.Contains(linkText, phraseLower)
		if !inCrystal && !inLink {
			where := "the crystal"
			if linkText != "" {
				where = "the crystal or its linked file"
			}
			out = append(out, assertViolation(e, fmt.Sprintf(
				"live-substituted phrase %q (count=%d, total=%d) is not present in %s — the crystal's static text has drifted from the graph's live state",
				phrase, count, total, where)))
		}
	}

	return out
}

// assertViolation builds a single Violation naming e.Question, the uniform
// shape every assert-eval failure uses.
func assertViolation(e loader.OrientationFAQEntry, reason string) Violation {
	return Violation{
		Check: "check_orientation_faq_answered",
		ID:    e.Question,
		Message: fmt.Sprintf(
			"orientation_faq question %q declares an \"assert\" that failed: %s",
			e.Question, reason),
	}
}

// evalExpect parses and evaluates raw (an OrientationFAQAssert.Expect JSON
// value) against the live (count, total) pair. ok is the predicate result
// when parsing succeeds (reason == ""); reason is a non-empty, human
// -readable explanation of a parse/shape failure (fail-closed — ok is
// always false when reason != "").
func evalExpect(raw json.RawMessage, count, total int) (ok bool, reason string) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		switch asString {
		case "all":
			return count == total, ""
		case "none":
			return count == 0, ""
		default:
			return false, fmt.Sprintf("unrecognized string value %q (must be \"all\" or \"none\")", asString)
		}
	}

	var asObject struct {
		Op    string `json:"op"`
		Value *int   `json:"value"`
	}
	if err := json.Unmarshal(raw, &asObject); err != nil {
		return false, fmt.Sprintf("not a recognized shape (neither the string \"all\"/\"none\" nor {\"op\":...,\"value\":...}): %v", err)
	}
	if asObject.Value == nil {
		return false, "object form requires a numeric \"value\" field"
	}
	switch asObject.Op {
	case "gte":
		return count >= *asObject.Value, ""
	case "eq":
		return count == *asObject.Value, ""
	default:
		return false, fmt.Sprintf("unrecognized \"op\" %q (must be \"gte\" or \"eq\")", asObject.Op)
	}
}

func stringInSlice(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// gateCohortMemberPredicate validates spec against g's live Requirements
// (fail-closed — never silently no-ops a malformed spec, matching this
// package's own "the check, not the loader, diagnoses" convention already
// established for gate_stage_order/orientation_faq) and, when valid, returns
// a membership predicate for graphfacts.CohortCount: r is a cohort member
// when r.Status matches spec.Statuses (mirroring
// graphfacts.RequirementStatusTally's own exact-match + OPEN-prefix
// matching rule — reused, not reinvented) AND r.ID is not named in
// spec.Exclude.
//
// Two validation failures fail closed with a named violation instead of
// silently proceeding:
//
//   - Any spec.Exclude id that does not match a real g.Requirements id (a
//     typo, a since-renamed/removed requirement).
//   - Any spec.Statuses entry that is not a recognized status value
//     (ontology.StatusDRAFT/StatusSETTLED/StatusREJECTED/StatusOPENPrefix).
func gateCohortMemberPredicate(e loader.OrientationFAQEntry, g *ontology.Graph, spec *loader.GateCohortSpec) (func(r ontology.Requirement) bool, *Violation) {
	knownIDs := make(map[string]struct{}, len(g.Requirements))
	for _, r := range g.Requirements {
		knownIDs[r.ID] = struct{}{}
	}
	for _, id := range spec.Exclude {
		if _, ok := knownIDs[id]; !ok {
			v := assertViolation(e, fmt.Sprintf(
				"assert kind \"gate_signoff_count\" declares a gate_cohort \"exclude\" entry %q, which does not match any Requirement id in this domain's graph",
				id))
			return nil, &v
		}
	}

	for _, status := range spec.Statuses {
		if status != ontology.StatusDRAFT && status != ontology.StatusSETTLED &&
			status != ontology.StatusREJECTED && status != ontology.StatusOPENPrefix {
			v := assertViolation(e, fmt.Sprintf(
				"assert kind \"gate_signoff_count\" declares a gate_cohort \"statuses\" entry %q, which is not a recognized requirement status",
				status))
			return nil, &v
		}
	}

	excluded := make(map[string]struct{}, len(spec.Exclude))
	for _, id := range spec.Exclude {
		excluded[id] = struct{}{}
	}

	return func(r ontology.Requirement) bool {
		if _, isExcluded := excluded[r.ID]; isExcluded {
			return false
		}
		for _, status := range spec.Statuses {
			if status == ontology.StatusOPENPrefix {
				if r.IsOpen() {
					return true
				}
				continue
			}
			if r.Status == status {
				return true
			}
		}
		return false
	}, nil
}
