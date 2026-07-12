package diagnose

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
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

type Signal struct {
	Source   string
	Priority int
	Target   string
	Message  string
}

func extractOpenQuestion(status string) string {
	s := strings.TrimPrefix(status, ontology.StatusOPENPrefix)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "()")
	s = strings.TrimSpace(s)
	return s
}

func DiagnoseSignals(g *ontology.Graph) []Signal {
	var out []Signal

	for _, f := range AllFindings(g) {
		priority := PReflection
		if f.Advisory {
			priority = PAdvisory
		}
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: priority,
			Target:   f.Target,
			Message:  f.Imperative,
		})
	}

	for _, v := range invariants.AllViolations(g) {
		out = append(out, Signal{
			Source:   "diagnose",
			Priority: PStructure,
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
				Target:   c.ID,
				Message: fmt.Sprintf(
					"conflict '%s' on axis '%s' is DETECTED with no steward movement; steward '%s' must ACKNOWLEDGE it",
					c.ID, c.Axis, c.Steward,
				),
			})
		} else if c.Lifecycle == ontology.ConflictACKNOWLEDGED {
			out = append(out, Signal{
				Source:   "diagnose",
				Priority: PConflictStalled,
				Target:   c.ID,
				Message: fmt.Sprintf(
					"conflict '%s' is ACKNOWLEDGED but undecided; steward '%s' must DECIDE (rationale) or set REVISIT_WHEN",
					c.ID, c.Steward,
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
			Target:   fmt.Sprintf("%s~%s", s.Left, s.Right),
			Message:  fmt.Sprintf("[HEURISTIC, entity-state conflict] %s", s.Hint),
		})
	}

	out = append(out, FreshnessSignals(g, todayISO())...)

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

func TopAction(g *ontology.Graph) string {
	signals := DiagnoseSignals(g)
	if len(signals) == 0 {
		return "none — graph clean"
	}
	return signals[0].Message
}
