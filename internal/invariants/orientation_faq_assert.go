// orientation_faq_assert.go evaluates an orientation_faq entry's optional
// "assert" contract (loader.OrientationFAQAssert) against LIVE graph state,
// closing the gap check_orientation_faq_answered's pure keyword-matching
// signal cannot: a keyword phrase's lexical PRESENCE in the crystal proves
// nothing about whether the phrase is still semantically TRUE relative to
// the graph's current state (task #321/R3-semantic-faq — see
// loader.OrientationFAQAssert's doc comment for the real "27 of 32" bug this
// closes). Dispatches on Assert.Kind to the matching internal/query tally
// function, then checks the live (count, total) pair against the entry's
// declared Expect and/or Phrase — fail-closed on every malformed or
// unrecognized shape, matching this package's existing "the check, not the
// loader, diagnoses" convention (see orientation_faq.go's own doc comment).
package invariants

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/query"
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
		tally := query.GateSignoffTally(g, order, a.Stage)
		count, total = tally.Signed, tally.Signed+tally.Deferred

	case "conflict_count_by_lifecycle":
		c, t, err := query.ConflictLifecycleTally(g, a.LifecycleClass)
		if err != nil {
			return []Violation{assertViolation(e, fmt.Sprintf("assert kind \"conflict_count_by_lifecycle\": %v", err))}
		}
		count, total = c, t

	case "requirement_count_by_status":
		c, t, err := query.RequirementStatusTally(g, a.Status, a.Enforcement)
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
