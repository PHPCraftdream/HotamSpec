// lifecycle.go renders the Go transition-method layer described in
// GEN-CODE-CONTRACT.md §2's `lifecycle.transitions[]` row: one method per
// graph transition `event`, checking the receiver's current state against
// the transition's `src` and only mutating on a match. See
// docs/GEN-CODE-CONTRACT.md §0 (mirror principle) — the rendered method body
// is deliberately the most direct possible translation of (src, dst, event)
// so a human/LLM auditor can compare it against the graph without
// domain-specific reasoning.
package gocode

import (
	"fmt"
	"sort"
	"strings"
)

// RenderLifecycleFile renders the full lifecycle.go body for a domain: the
// ownership marker, package clause, one rendered transition method per
// EntityType (sorted by slug for determinism, contract §5), skipping
// EntityTypes with no lifecycle transitions (an EntityType may legally have
// zero transitions — e.g. a single-state lifecycle — in which case it simply
// contributes no methods, not an error).
func RenderLifecycleFile(packageName string, models []*entityModel) ([]byte, error) {
	sorted := make([]*entityModel, len(models))
	copy(sorted, models)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].src.Slug < sorted[j].src.Slug })

	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)
	b.WriteString("import \"fmt\"\n\n")
	b.WriteString(wrongStateErrorTypeSrc)
	b.WriteString("\n")

	any := false
	for _, m := range sorted {
		if len(m.transitions) == 0 {
			continue
		}
		if any {
			b.WriteString("\n")
		}
		any = true
		b.WriteString(renderEntityTransitions(m))
	}

	return []byte(b.String()), nil
}

// wrongStateErrorTypeSrc is the fixed WrongStateError type emitted once per
// lifecycle.go file (shared by every generated transition method in that
// file), rather than re-declared per EntityType. Plain ASCII "section 2"
// (not "§2"): this constant is written verbatim into generated output, and
// contract §1.1's zero-Cyrillic rule is enforced via a full non-ASCII grep.
const wrongStateErrorTypeSrc = `// WrongStateError reports that a lifecycle transition method was called
// while the receiver was not in the transition's required source state.
// Every generated transition method (GEN-CODE-CONTRACT.md section 2) returns this
// type on a state mismatch, so callers/tests can distinguish "illegal
// transition attempted" from other errors via errors.As.
type WrongStateError struct {
	Entity  string
	Event   string
	Want    string
	Got     string
}

func (e *WrongStateError) Error() string {
	return fmt.Sprintf("%s.%s: expected state %q, got %q", e.Entity, e.Event, e.Want, e.Got)
}
`

// renderEntityTransitions renders every transition method for one
// EntityType, in graph declaration order (not re-sorted — contract §0
// mirror: the method order should match the order a human reads
// lifecycle.transitions[] in the graph).
func renderEntityTransitions(m *entityModel) string {
	var b strings.Builder
	recv := strings.ToLower(m.structName[:1])

	for _, tr := range m.transitions {
		if tr.src.Why != "" {
			fmt.Fprintf(&b, "// Atom: %s.lifecycle.transition.%s - see requirements_audit.md\n", m.entitySlug, tr.eventValue)
		}
		if tr.src.Guard != "" {
			fmt.Fprintf(&b, "// TODO(gen-code): guard %q is declared on this transition but not\n", tr.src.Guard)
			b.WriteString("// yet enforced - GEN-CODE-CONTRACT.md section 2 hook point, no guard data\n")
			b.WriteString("// to formalize yet.\n")
		}
		if tr.src.GuardAssumption != nil && *tr.src.GuardAssumption != "" {
			fmt.Fprintf(&b, "// TODO(gen-code): guard_assumption %q is declared on this transition\n", *tr.src.GuardAssumption)
			b.WriteString("// but not yet enforced - GEN-CODE-CONTRACT.md section 2 hook point.\n")
		}
		fmt.Fprintf(&b, "func (%s *%s) %s() error {\n", recv, m.structName, tr.methodName)
		fmt.Fprintf(&b, "\tif %s.State != %s {\n", recv, tr.srcState.constant)
		fmt.Fprintf(&b, "\t\treturn &WrongStateError{Entity: %q, Event: %s, Want: string(%s), Got: string(%s.State)}\n",
			m.structName, tr.eventConst, tr.srcState.constant, recv)
		b.WriteString("\t}\n")
		fmt.Fprintf(&b, "\t%s.State = %s\n", recv, tr.dstState.constant)
		b.WriteString("\treturn nil\n")
		b.WriteString("}\n\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}
