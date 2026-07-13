package query

import (
	"fmt"
	"strings"
)

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func lineIfSet(label, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s\n", label, value)
}

// FormatRequirementCard renders a RequirementCard as compact human-readable
// text for `hotam req show` without --json.
func FormatRequirementCard(c RequirementCard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]\n", c.ID, c.Status)
	fmt.Fprintf(&b, "claim: %s\n", c.Claim)
	b.WriteString(lineIfSet("why", c.Why))
	fmt.Fprintf(&b, "owner: %s\n", c.Owner)
	fmt.Fprintf(&b, "enforcement: %s (%s)\n", c.Enforcement, c.Enforceability)
	fmt.Fprintf(&b, "enforced_by: %s\n", joinOrDash(c.EnforcedBy))
	fmt.Fprintf(&b, "assumptions: %s\n", joinOrDash(c.Assumptions))
	if len(c.Relations) > 0 {
		var rels []string
		for _, r := range c.Relations {
			rels = append(rels, fmt.Sprintf("%s->%s", r.Kind, r.Target))
		}
		fmt.Fprintf(&b, "relations: %s\n", strings.Join(rels, ", "))
	} else {
		b.WriteString("relations: -\n")
	}
	b.WriteString(lineIfSet("m_tag", c.MTag))
	b.WriteString(lineIfSet("created_at", c.CreatedAt))
	b.WriteString(lineIfSet("settled_at", c.SettledAt))
	b.WriteString(lineIfSet("last_reviewed_at", c.LastReviewedAt))
	b.WriteString(lineIfSet("review_after", c.ReviewAfter))
	if len(c.Evidence) > 0 {
		fmt.Fprintf(&b, "evidence: %s\n", joinOrDash(c.Evidence))
	}
	if len(c.SourceRefs) > 0 {
		fmt.Fprintf(&b, "source_refs: %s\n", joinOrDash(c.SourceRefs))
	}
	if len(c.History) > 0 {
		b.WriteString("history:\n")
		for _, h := range c.History {
			fmt.Fprintf(&b, "  - %s %s (%s)\n", h.At, h.Summary, h.DecidedBy)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatConflictCard renders a ConflictCard as compact human-readable text.
func FormatConflictCard(c ConflictCard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]\n", c.ID, c.Lifecycle)
	fmt.Fprintf(&b, "axis: %s\n", c.Axis)
	fmt.Fprintf(&b, "context: %s\n", c.Context)
	fmt.Fprintf(&b, "members: %s\n", joinOrDash(c.Members))
	fmt.Fprintf(&b, "steward: %s\n", c.Steward)
	if c.SharedAssumption != nil {
		fmt.Fprintf(&b, "shared_assumption: %s\n", *c.SharedAssumption)
	}
	if len(c.Derived) > 0 {
		fmt.Fprintf(&b, "derived: %s\n", joinOrDash(c.Derived))
	}
	b.WriteString(lineIfSet("revisit_marker", c.RevisitMarker))
	b.WriteString(lineIfSet("decided_by", c.DecidedBy))
	b.WriteString(lineIfSet("decided_at", c.DecidedAt))
	if c.Signoff != nil {
		fmt.Fprintf(&b, "signoff: %s on %s\n", c.Signoff.DecidedBy, c.Signoff.Date)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatAssumptionCard renders an AssumptionCard as compact human-readable text.
func FormatAssumptionCard(c AssumptionCard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]\n", c.ID, c.Status)
	fmt.Fprintf(&b, "statement: %s\n", c.Statement)
	fmt.Fprintf(&b, "owner: %s\n", c.Owner)
	if c.MachineCheck != nil {
		fmt.Fprintf(&b, "machine_check: %s\n", *c.MachineCheck)
	}
	b.WriteString(lineIfSet("created_at", c.CreatedAt))
	b.WriteString(lineIfSet("decided_at", c.DecidedAt))
	if c.Signoff != nil {
		fmt.Fprintf(&b, "signoff: %s on %s\n", c.Signoff.DecidedBy, c.Signoff.Date)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatShow dispatches to the right formatter for whatever Show returned.
func FormatShow(v any) (string, error) {
	switch c := v.(type) {
	case RequirementCard:
		return FormatRequirementCard(c), nil
	case ConflictCard:
		return FormatConflictCard(c), nil
	case AssumptionCard:
		return FormatAssumptionCard(c), nil
	default:
		return "", fmt.Errorf("query: unknown card type %T", v)
	}
}

// FormatList renders list/search rows as one line each: "id [status] summary".
func FormatList(items []ListItem) string {
	if len(items) == 0 {
		return "none"
	}
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s [%s] %s", it.ID, it.Status, it.Summary)
	}
	return b.String()
}

// FormatSearch renders search results as one line each, ranked order preserved.
func FormatSearch(items []SearchResult) string {
	if len(items) == 0 {
		return "none"
	}
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s [%s] %s", it.ID, it.Status, it.Summary)
	}
	return b.String()
}

// FormatRelated renders neighbor refs as one line each: "id (rel_kind)".
func FormatRelated(refs []NeighborRef) string {
	if len(refs) == 0 {
		return "none"
	}
	var b strings.Builder
	for i, r := range refs {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s (%s)", r.ID, r.RelKind)
	}
	return b.String()
}

// FormatContext renders a ContextCard as a compact multi-section text block.
func FormatContext(c ContextCard) string {
	var b strings.Builder
	b.WriteString(FormatRequirementCard(c.Requirement))
	b.WriteString("\n\nneighbors (relations):\n")
	b.WriteString(indent(FormatRelated(c.Relations)))
	b.WriteString("\n\nassumptions (full text):\n")
	if len(c.Assumptions) == 0 {
		b.WriteString("  none")
	} else {
		for i, a := range c.Assumptions {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "  %s [%s] %s", a.ID, a.Status, a.Statement)
		}
	}
	b.WriteString("\n\nconflicts (member of):\n")
	if len(c.Conflicts) == 0 {
		b.WriteString("  none")
	} else {
		for i, cf := range c.Conflicts {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "  %s [%s] axis=%s members=%s", cf.ID, cf.Lifecycle, cf.Axis, joinOrDash(cf.Members))
		}
	}
	b.WriteString("\n\nshares assumption with:\n")
	b.WriteString(indent(FormatRelated(c.SharedAssumptionWith)))
	return b.String()
}

// FormatBrief renders a BriefCard as a compact multi-section text block,
// reusing the existing per-card formatters (FormatRequirementCard etc.) and
// the section layout established by FormatContext. A freshness line appears
// only for a Requirement anchor.
func FormatBrief(b BriefCard) string {
	var sb strings.Builder
	switch b.Kind {
	case KindRequirement:
		if b.Requirement != nil {
			sb.WriteString(FormatRequirementCard(*b.Requirement))
		}
	case KindConflict:
		if b.Conflict != nil {
			sb.WriteString(FormatConflictCard(*b.Conflict))
		}
	case KindAssumption:
		if b.Assumption != nil {
			sb.WriteString(FormatAssumptionCard(*b.Assumption))
		}
	}

	if b.Freshness != nil {
		sb.WriteString("\n\nfreshness: ")
		sb.WriteString(string(b.Freshness.Status))
		if b.Freshness.OverdueDays > 0 {
			fmt.Fprintf(&sb, " (%dd overdue)", b.Freshness.OverdueDays)
		}
	}

	sb.WriteString("\n\nneighbors:\n")
	sb.WriteString(indent(FormatRelated(b.Neighbors)))

	if len(b.Assumptions) > 0 {
		sb.WriteString("\n\nassumptions (full text):\n")
		for i, a := range b.Assumptions {
			if i > 0 {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "  %s [%s] %s", a.ID, a.Status, a.Statement)
		}
	}
	if len(b.Conflicts) > 0 {
		sb.WriteString("\n\nconflicts (member of):\n")
		for i, cf := range b.Conflicts {
			if i > 0 {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "  %s [%s] axis=%s members=%s", cf.ID, cf.Lifecycle, cf.Axis, joinOrDash(cf.Members))
		}
	}
	if len(b.SharedAssumptionWith) > 0 {
		sb.WriteString("\n\nshares assumption with:\n")
		sb.WriteString(indent(FormatRelated(b.SharedAssumptionWith)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
