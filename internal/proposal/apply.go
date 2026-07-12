package proposal

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func errNotFound(label, id string) error {
	return fmt.Errorf("%s %q not found in the graph. No changes made.", label, id)
}

func errDuplicate(kind, id string) error {
	return fmt.Errorf("%s %q already exists — a duplicate is a re-declaration, not a new node.", kind, id)
}

func errNotDeclared(label, id string) error {
	return fmt.Errorf("%s %q is not declared in the graph.", label, id)
}

func errStewardOwnsMember(steward, member string) error {
	return fmt.Errorf("steward %q owns member %q — the steward must not own any member.", steward, member)
}

func errTooFewMembers(conflictID string, count int) error {
	return fmt.Errorf(
		"ConflictMemberUpdate on %q would leave %d distinct member(s); "+
			"requires >= 2. Refusing to write.", conflictID, count)
}

// applyToGraph validates and mutates g in place, then verifies the proposal
// introduces no new invariant violations relative to the graph's pre-mutation
// state. It performs no disk I/O — Apply and ApplyBatch own load/write so a
// batch can apply N proposals to one in-memory graph and write exactly once
// (all-or-nothing). The "before" baseline is captured from g's current state,
// so in a batch each proposal is checked against the state left by the
// previous one — exactly mirroring N sequential single Applies.
func applyToGraph(g *ontology.Graph, today string, p Proposal) error {
	a, ok := p.(actor)
	if !ok {
		return fmt.Errorf("unsupported proposal type %T", p)
	}
	if err := a.validate(); err != nil {
		return err
	}

	before := indexViolations(invariants.AllViolations(g))

	if err := a.mutate(g, today); err != nil {
		return err
	}

	after := invariants.AllViolations(g)
	newViolations := newViolationsSince(before, after)
	if len(newViolations) > 0 {
		return fmt.Errorf(
			"proposal %s %s would introduce %d new invariant violation(s):\n%s",
			p.Kind(), p.TargetAnchor(), len(newViolations), formatViolations(newViolations))
	}
	return nil
}

func Apply(graphPath string, today string, p Proposal) error {
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	if err := applyToGraph(g, today, p); err != nil {
		return err
	}

	if err := loader.WriteGraph(graphPath, g); err != nil {
		return fmt.Errorf("write graph: %w", err)
	}
	note := p.Kind() + " " + p.TargetAnchor() + " applied"
	if err := loader.WriteLock(graphPath, note); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	return nil
}

// ApplyBatch applies a sequence of proposals to a single graph atomically: it
// loads the graph once, applies every proposal to the same in-memory graph
// (each checked against a rolling invariant baseline via applyToGraph —
// proposal i must not introduce violations relative to the state after
// proposal i-1, exactly mirroring N sequential Apply calls), and writes the
// graph to disk exactly once, only if every proposal succeeded. If any
// proposal fails validation, mutation, or introduces new invariant
// violations, ApplyBatch returns an error naming the offending proposal and
// the graph on disk (graph.json + graph.lock) is left untouched.
func ApplyBatch(graphPath string, today string, ps []Proposal) error {
	if len(ps) == 0 {
		return fmt.Errorf("batch is empty — supply at least one proposal")
	}

	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	for i, p := range ps {
		if err := applyToGraph(g, today, p); err != nil {
			return fmt.Errorf("batch proposal %d of %d (%s %s): %w",
				i+1, len(ps), p.Kind(), p.TargetAnchor(), err)
		}
	}

	if err := loader.WriteGraph(graphPath, g); err != nil {
		return fmt.Errorf("write graph: %w", err)
	}
	note := fmt.Sprintf("batch of %d proposals applied", len(ps))
	if err := loader.WriteLock(graphPath, note); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	return nil
}

func violationKey(v invariants.Violation) string {
	return v.Check + "\x00" + v.ID
}

func indexViolations(vs []invariants.Violation) map[string]struct{} {
	out := make(map[string]struct{}, len(vs))
	for _, v := range vs {
		out[violationKey(v)] = struct{}{}
	}
	return out
}

func newViolationsSince(before map[string]struct{}, after []invariants.Violation) []invariants.Violation {
	var fresh []invariants.Violation
	for _, v := range after {
		if _, ok := before[violationKey(v)]; !ok {
			fresh = append(fresh, v)
		}
	}
	return fresh
}

func formatViolations(vs []invariants.Violation) string {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "  - %s %s: %s\n", v.Check, v.ID, v.Message)
	}
	return b.String()
}
