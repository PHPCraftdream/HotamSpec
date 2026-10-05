package selfspec

import (
	"reflect"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// FieldDiff is one structural field that differs between a registry entry
// and its corresponding graph node — the atomic unit StructuralFieldDiffs
// returns. Old/New are `any` (not string) so a caller can both render a
// human-readable diff (fmt.Sprintf("%v", ...)) AND — on a future step —
// hash or otherwise inspect the FULL, un-truncated value: truncation for
// display is the caller's concern (see StructuralFieldDiffs' own doc
// comment), never this type's or this function's.
type FieldDiff struct {
	Field string
	Old   any
	New   any
}

// StructuralFieldDiffs is the SINGLE SOURCE OF TRUTH for the full list of
// structural fields that differ between reg (a selfspec.Requirements entry)
// and graph (the corresponding node in a live ontology.Graph) — exactly the
// fields MergeIntoGraph (merge.go) replaces wholesale from the registry:
// Claim, Owner, Status, Why, Assumptions, Relations, Enforcement,
// EnforcedBy, MTag, Enforceability, Summary, CreatedAt, SettledAt,
// SourceRefs, DeclOrder, BlockedOn, ImplementedBy, VerifiedBy, SourceLinks,
// Coverage, ClaimTexts, AtomKind, Cases, ClauseLinks, Strength,
// Applicability, and Precedence.
//
// Deliberately excluded: the EVENT fields MergeIntoGraph passes through
// untouched — History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence.
// Those legitimately differ between two otherwise-identical Requirement
// values (a registry entry never carries them at all), so comparing them
// here would produce permanent, meaningless noise.
//
// This supersedes internal/invariants' unexported firstStructuralFieldDiff
// (selfspec_shadow.go), which stops at the FIRST differing field and is
// missing CreatedAt/DeclOrder from its comparison set even though
// MergeIntoGraph replaces both. StructuralFieldDiffs returns the FULL list —
// every differing field, not just the first — and is the field list itself,
// reused by name at three call sites across the next RAC-B steps (a dry-run
// render, a live invariant, and a history-summary generator): all three must
// agree on exactly which fields count as "structural," and this function is
// where that agreement lives.
//
// Values are returned WHOLE, never truncated or otherwise summarized —
// truncating a long Claim/Why/Summary for a terminal render, or hashing the
// full value for a future staleness fingerprint, are both callers'
// decisions, made from the complete value FieldDiff.Old/New carries. This
// function only detects and reports difference; it never abbreviates.
//
// Returns an empty (nil) slice when reg and graph agree on every structural
// field.
func StructuralFieldDiffs(reg, graph ontology.Requirement) []FieldDiff {
	type fieldValue struct {
		name       string
		regValue   any
		graphValue any
	}
	fields := []fieldValue{
		{"Claim", reg.Claim, graph.Claim},
		{"Owner", reg.Owner, graph.Owner},
		{"Status", reg.Status, graph.Status},
		{"Why", reg.Why, graph.Why},
		{"Assumptions", reg.Assumptions, graph.Assumptions},
		{"Relations", reg.Relations, graph.Relations},
		{"Enforcement", reg.Enforcement, graph.Enforcement},
		{"EnforcedBy", reg.EnforcedBy, graph.EnforcedBy},
		{"MTag", reg.MTag, graph.MTag},
		{"Enforceability", reg.Enforceability, graph.Enforceability},
		{"Summary", reg.Summary, graph.Summary},
		{"CreatedAt", reg.CreatedAt, graph.CreatedAt},
		{"SettledAt", reg.SettledAt, graph.SettledAt},
		{"SourceRefs", reg.SourceRefs, graph.SourceRefs},
		{"DeclOrder", reg.DeclOrder, graph.DeclOrder},
		{"BlockedOn", reg.BlockedOn, graph.BlockedOn},
		{"ImplementedBy", reg.ImplementedBy, graph.ImplementedBy},
		{"VerifiedBy", reg.VerifiedBy, graph.VerifiedBy},
		{"SourceLinks", reg.SourceLinks, graph.SourceLinks},
		{"Coverage", reg.Coverage, graph.Coverage},
		{"ClaimTexts", reg.ClaimTexts, graph.ClaimTexts},
		{"AtomKind", reg.AtomKind, graph.AtomKind},
		{"Cases", reg.Cases, graph.Cases},
		{"ClauseLinks", reg.ClauseLinks, graph.ClauseLinks},
		{"Strength", reg.Strength, graph.Strength},
		{"Applicability", reg.Applicability, graph.Applicability},
		{"Precedence", reg.Precedence, graph.Precedence},
	}
	var out []FieldDiff
	for _, f := range fields {
		var equal bool
		if f.name == "Cases" {
			equal = ontology.EqualCaseDefinitions(reg.Cases, graph.Cases)
		} else {
			equal = reflect.DeepEqual(f.regValue, f.graphValue)
		}
		if !equal {
			out = append(out, FieldDiff{Field: f.name, Old: f.graphValue, New: f.regValue})
		}
	}
	return out
}
