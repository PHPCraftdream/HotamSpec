package proposal

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type snapshot struct {
	Claim          string
	Owner          string
	Status         string
	Why            string
	Assumptions    []string
	Enforcement    string
	EnforcedBy     []string
	Relations      []ontology.Relation
	Enforceability string
	MTag           string
	Summary        string
	CreatedAt      string
	SettledAt      string
	LastReviewedAt string
	ReviewAfter    string
	Evidence       []string
	SourceRefs     []string
	BlockedOn      string
}

func snapshotFrom(r ontology.Requirement) snapshot {
	rel := make([]ontology.Relation, len(r.Relations))
	copy(rel, r.Relations)
	ass := append([]string(nil), r.Assumptions...)
	eb := append([]string(nil), r.EnforcedBy...)
	ev := append([]string(nil), r.Evidence...)
	sr := append([]string(nil), r.SourceRefs...)
	return snapshot{
		Claim:          r.Claim,
		Owner:          r.Owner,
		Status:         r.Status,
		Why:            r.Why,
		Assumptions:    ass,
		Enforcement:    r.Enforcement,
		EnforcedBy:     eb,
		Relations:      rel,
		Enforceability: r.Enforceability,
		MTag:           r.MTag,
		Summary:        r.Summary,
		CreatedAt:      r.CreatedAt,
		SettledAt:      r.SettledAt,
		LastReviewedAt: r.LastReviewedAt,
		ReviewAfter:    r.ReviewAfter,
		Evidence:       ev,
		SourceRefs:     sr,
		BlockedOn:      r.BlockedOn,
	}
}

func abbrev(s string, limit int) string {
	text := strings.Join(strings.Fields(s), " ")
	if len(text) > limit {
		text = text[:limit-1] + "…"
	}
	return text
}

func abbrevSlice(in []string) string {
	parts := make([]string, len(in))
	for i, s := range in {
		parts[i] = s
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func normDefaultEnforcement(v string) string {
	if v == "" {
		return ontology.EnforcementPROSE
	}
	return v
}

func normDefaultEnforceability(v string) string {
	if v == "" {
		return ontology.EnforceabilityENFORCEABLE
	}
	return v
}

func summarizeFieldDiff(old snapshot, applied snapshot) string {
	type fieldDiff struct {
		name string
		oldS string
		newS string
	}
	diffs := []fieldDiff{
		{"claim", old.Claim, applied.Claim},
		{"owner", old.Owner, applied.Owner},
		{"status", old.Status, applied.Status},
		{"why", old.Why, applied.Why},
		{"assumptions", sliceKey(old.Assumptions), sliceKey(applied.Assumptions)},
		{"enforcement", normDefaultEnforcement(old.Enforcement), normDefaultEnforcement(applied.Enforcement)},
		{"enforced_by", sliceKey(old.EnforcedBy), sliceKey(applied.EnforcedBy)},
		{"relations", relationKey(old.Relations), relationKey(applied.Relations)},
		{"enforceability", normDefaultEnforceability(old.Enforceability), normDefaultEnforceability(applied.Enforceability)},
		{"m_tag", old.MTag, applied.MTag},
		{"summary", old.Summary, applied.Summary},
		{"settled_at", old.SettledAt, applied.SettledAt},
		{"last_reviewed_at", old.LastReviewedAt, applied.LastReviewedAt},
		{"review_after", old.ReviewAfter, applied.ReviewAfter},
		{"evidence", sliceKey(old.Evidence), sliceKey(applied.Evidence)},
		{"source_refs", sliceKey(old.SourceRefs), sliceKey(applied.SourceRefs)},
		{"blocked_on", old.BlockedOn, applied.BlockedOn},
	}
	var parts []string
	for _, d := range diffs {
		if d.oldS == d.newS {
			continue
		}
		parts = append(parts, d.name+": "+abbrev(d.oldS, 150)+"→"+abbrev(d.newS, 150))
	}
	return strings.Join(parts, "; ")
}

func sliceKey(in []string) string {
	if len(in) == 0 {
		return abbrevSlice(nil)
	}
	return abbrevSlice(in)
}

func relationKey(in []ontology.Relation) string {
	if len(in) == 0 {
		return "[]"
	}
	parts := make([]string, len(in))
	for i, r := range in {
		parts[i] = "(" + r.Kind + ", " + r.Target + ")"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
