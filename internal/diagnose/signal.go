package diagnose

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const (
	PReflection      = 0
	PStructure       = 1
	PDriftFallout    = 2
	PConflictStalled = 3
	POpenItem        = 4
	PLatentConnector = 5
	PRuntime         = 6
	PAdvisory        = 7
)

const UncertainAgingMinDependents = 5

// Signal is one actionable item produced by DiagnoseSignals.
//
// Check names the predicate/source that produced this signal (e.g. an
// invariant check name, a reflection condition, or a fixed producer label).
// It is the grouping key the what-now renderer uses to collapse several
// identical-kind signals affecting different nodes into one line, so every
// producer MUST set it.
type Signal struct {
	Source   string `json:"source"`
	Priority int    `json:"priority"`
	Check    string `json:"check"`
	Target   string `json:"target"`
	Message  string `json:"message"`
}

func extractOpenQuestion(status string) string {
	s := strings.TrimPrefix(status, ontology.StatusOPENPrefix)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "()")
	s = strings.TrimSpace(s)
	return s
}

// DiagnoseSignals derives the full prioritized signal list for a graph as of
// today (YYYY-MM-DD). today is threaded through explicitly (rather than
// computed internally via time.Now()) so callers — and ultimately the
// generated docs/gen/*.md + root CLAUDE.md/AGENTS.md/GEMINI.md crystals that
// embed FreshnessSignals' OVERDUE/NEVER-REVIEWED messages — are reproducible
// and byte-identical when regenerated twice with the same today (R-...
// idempotency; see gen-spec's --today flag). Every caller not itself
// exposing a --today flag defaults to time.Now().Format("2006-01-02").
//
// This is a thin wrapper over DiagnoseSignalsWithViolations, computing its
// own invariants.AllViolations(g) pass — the right default for every
// existing caller (hotam what-now/status, internal/generator's doc
// projections), all of which want signals derived from a FRESH violation
// scan of g. See DiagnoseSignalsWithViolations's own doc comment for the one
// caller that must NOT take this path (check_domain_claude_md_current,
// internal/invariants/claude_md_current.go), which supplies an
// already-computed violation set instead to avoid re-entering
// invariants.AllViolations while it is itself still running.
func DiagnoseSignals(g *ontology.Graph, today string) []Signal {
	return DiagnoseSignalsWithViolations(g, today, invariants.AllViolations(g))
}

// DiagnoseSignalsExcludingDiskProjection is DiagnoseSignals' EXCLUDING-variant
// sibling: it computes signals from
// invariants.AllViolationsExcludingDiskProjection(g) instead of the full
// invariants.AllViolations(g) — i.e. every ComparesOnDiskProjection check
// (check_spec_md_current, check_domain_claude_md_current) is left out of the
// violation set feeding this signal list.
//
// The ONLY caller is internal/generator's domainPulse, for a SIBLING
// domain's pulse line inside a DOMAIN-MAP block render (see
// invariants.AllViolationsExcludingDiskProjection's own doc comment for the
// full rationale: a sibling pulse line has never needed
// check_spec_md_current/check_domain_claude_md_current's byte-comparison
// staleness signal, and calling the FULL AllViolations there created real,
// observed mutual recursion between sibling domains once
// check_domain_claude_md_current existed — confirmed via a captured
// goroutine stack trace). The ACTIVE domain's own LIVE-STATE block (this
// function's sibling, DiagnoseSignalsWithViolations via BuildLiveState) is
// unaffected — it always receives the full, precise violation set, either
// computed fresh (BuildLiveState's own plain path) or supplied via
// ViolationsOverride when check_domain_claude_md_current itself is the
// caller.
func DiagnoseSignalsExcludingDiskProjection(g *ontology.Graph, today string) []Signal {
	return DiagnoseSignalsWithViolations(g, today, invariants.AllViolationsExcludingDiskProjection(g))
}

// DiagnoseSignalsWithViolations is DiagnoseSignals' core, parameterized over
// an already-computed violations slice instead of calling
// invariants.AllViolations(g) itself. It exists for exactly one reason:
// internal/invariants/claude_md_current.go's check_domain_claude_md_current
// needs a fresh render of the domain's CLAUDE.md (via
// internal/generator.RenderClaudeMDFromTemplate, which embeds this same
// signal list in its LIVE-STATE block and, when the active domain also
// appears in its own DOMAIN-MAP, in that domain's pulse line too) to compare
// against the committed file — but that check itself runs FROM INSIDE
// invariants.AllViolations(g)'s own fan-out. If the render path called plain
// DiagnoseSignals (which calls invariants.AllViolations(g) again, for the
// SAME g), the result would be unbounded same-process recursion: AllViolations
// -> check_domain_claude_md_current -> render -> DiagnoseSignals ->
// AllViolations -> check_domain_claude_md_current -> ... (a genuinely
// different, harder problem than check_spec_md_current's own recursion
// concern, which is bounded by a subprocess boundary — see
// internal/gate/test_exec.go's recursionGuardEnv doc comment — because
// go test spawns a NEW process, whereas this is all in-process).
//
// Passing the ALREADY-COMPUTED violations list (the very set
// invariants.AllViolations(g) is in the middle of assembling when this
// check's own Check func runs) breaks the cycle: the render this check
// verifies against is computed from the same violation snapshot every OTHER
// check in the same AllViolations pass sees, so the comparison stays
// meaningful (not a stale or partial view) while never calling back into
// AllViolations for g itself.
func DiagnoseSignalsWithViolations(g *ontology.Graph, today string, violations []invariants.Violation) []Signal {
	var out []Signal

	for _, f := range AllFindings(g) {
		priority := PReflection
		if f.Advisory {
			priority = PAdvisory
		}
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: priority,
			Check:    f.Condition,
			Target:   f.Target,
			Message:  f.Imperative,
		})
	}

	for _, v := range violations {
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PStructure,
			Check:    v.Check,
			Target:   v.ID,
			Message:  fmt.Sprintf("[%s] %s", v.Check, v.Message),
		})
	}

	for _, a := range ontology.DeadAssumptions(g) {
		depReqs := ontology.RequirementsOnAssumption(g, a.ID)
		depCons := ontology.ConflictsOnAssumption(g, a.ID)
		if len(depReqs) == 0 && len(depCons) == 0 {
			continue
		}
		for _, r := range depReqs {
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: PDriftFallout,
				Check:    "dead_assumption_fallout_req",
				Target:   r.ID,
				Message: fmt.Sprintf(
					"assumption '%s' is DEAD (%s); revisit requirement '%s' which rests on it",
					a.ID, pyRepr(a.Statement), r.ID,
				),
			})
		}
		for _, c := range depCons {
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: PDriftFallout,
				Check:    "dead_assumption_fallout_conflict",
				Target:   c.ID,
				Message: fmt.Sprintf(
					"assumption '%s' is DEAD; revive conflict cluster '%s' whose shared_assumption was '%s'",
					a.ID, c.ID, a.ID,
				),
			})
		}
	}

	for _, c := range g.Conflicts {
		if c.Lifecycle == ontology.ConflictDETECTED {
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: PConflictStalled,
				Check:    "conflict_detected_stalled",
				Target:   c.ID,
				Message: fmt.Sprintf(
					"conflict '%s' on axis '%s' is DETECTED with no resolver movement; resolver '%s' must ACKNOWLEDGE it",
					c.ID, c.Axis, c.Resolver,
				),
			})
		} else if c.Lifecycle == ontology.ConflictACKNOWLEDGED {
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: PConflictStalled,
				Check:    "conflict_acknowledged_stalled",
				Target:   c.ID,
				Message: fmt.Sprintf(
					"conflict '%s' is ACKNOWLEDGED but undecided; resolver '%s' must DECIDE (rationale) or set REVISIT_WHEN",
					c.ID, c.Resolver,
				),
			})
		}
	}

	for _, r := range g.Requirements {
		if r.IsOpen() {
			question := extractOpenQuestion(r.Status)
			if question == "" {
				question = "(no question stated)"
			}
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: POpenItem,
				Check:    "open_requirement",
				Target:   r.ID,
				Message: fmt.Sprintf(
					"OPEN requirement '%s' (owner '%s') awaits a decision: %s",
					r.ID, r.Owner, question,
				),
			})
		}
	}

	for _, c := range g.Conflicts {
		if c.IsHeld() {
			for _, v := range c.Variants {
				out = append(out, Signal{
					Source:   "diagnose",
					Priority: POpenItem,
					Check:    "held_variant_choice",
					Target:   c.ID,
					Message: fmt.Sprintf(
						"choose a variant: '%s' — %s",
						v.ID, c.Axis,
					),
				})
			}
		}
	}

	for _, a := range ontology.UncertainAssumptions(g) {
		depReqs := ontology.RequirementsOnAssumption(g, a.ID)
		if len(depReqs) < UncertainAgingMinDependents {
			continue
		}
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: POpenItem,
			Check:    "uncertain_assumption_aging",
			Target:   a.ID,
			Message: fmt.Sprintf(
				"review assumption '%s' (%s): still UNCERTAIN with %d dependent requirements — resolve the doubt (transition to DEAD or re-affirm HOLDS) or it drifts",
				a.ID, pyRepr(a.Statement), len(depReqs),
			),
		})
	}

	for _, cl := range ontology.LatentConnectorClusters(g) {
		sig := strings.Join(cl.Assumptions, ", ")
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PLatentConnector,
			Check:    "latent_connector_cluster",
			Target:   strings.Join(cl.Assumptions, ","),
			Message: fmt.Sprintf(
				"[HEURISTIC, for AI review] assumption(s) %s shared by %d requirements (%s) with no mediating Conflict node — review the cluster as ONE item: consider splitting the assumption or materializing a connector (%d pair(s); detail: docs/gen/TENSIONS.md)",
				sig, len(cl.Requirements), strings.Join(cl.Requirements, ", "), len(cl.Pairs),
			),
		})
	}

	for _, s := range ontology.EntityStateConflictSuspects(g) {
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PLatentConnector,
			Check:    "entity_state_suspect",
			Target:   fmt.Sprintf("%s~%s", s.Left, s.Right),
			Message:  fmt.Sprintf("[HEURISTIC, entity-state conflict] %s", s.Hint),
		})
	}

	out = append(out, FreshnessSignals(g, today)...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Message < out[j].Message
	})

	return out
}

func TopAction(g *ontology.Graph, today string) string {
	signals := DiagnoseSignals(g, today)
	if len(signals) == 0 {
		return "none — graph clean"
	}
	return signals[0].Message
}
