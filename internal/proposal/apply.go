package proposal

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
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

func Apply(graphPath string, today string, p Proposal) error {
	a, ok := p.(actor)
	if !ok {
		return fmt.Errorf("unsupported proposal type %T", p)
	}
	if err := a.validate(); err != nil {
		return err
	}

	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
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

	if err := loader.WriteGraph(graphPath, g); err != nil {
		return fmt.Errorf("write graph: %w", err)
	}
	note := p.Kind() + " " + p.TargetAnchor() + " applied"
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
