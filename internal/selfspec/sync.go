package selfspec

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// SyncKind classifies one SyncReport entry: whether SyncGraph created a
// brand-new graph node for a registry ID that had none (SyncKindAdded), or
// replaced structural fields on an already-existing node whose structural
// fields disagreed with the registry (SyncKindChanged). Typed constants,
// not bare strings, so a caller (the future `hotam sync-self` CLI render,
// task #349) can switch on Kind without risking a typo'd literal.
type SyncKind string

const (
	SyncKindAdded   SyncKind = "ADDED"
	SyncKindChanged SyncKind = "CHANGED"
)

// SyncReportEntry is one requirement SyncGraph actually created or changed.
// FieldDiffs is always empty for SyncKindAdded (a brand-new node has no
// "old" value to diff against — every structural field is, by construction,
// new) and always non-empty for SyncKindChanged (SyncGraph only records a
// CHANGED entry when StructuralFieldDiffs found at least one differing
// field — see SyncGraph's own doc comment).
type SyncReportEntry struct {
	ID         string
	Kind       SyncKind
	FieldDiffs []FieldDiff
}

// SyncReport is SyncGraph's full result: every requirement it actually
// created or changed, in registry iteration order. A requirement whose
// graph node already matched the registry on every structural field is NOT
// included — SyncReport lists only real, actionable sync activity.
type SyncReport struct {
	Entries []SyncReportEntry
}

// SyncGraph is Phase B's authority-flip primitive (task #346/RAC-B, this
// step's task #348/RAC-B1): unlike MergeIntoGraph (merge.go), which errors
// on a registered ID absent from g, SyncGraph CREATES a node for it instead.
// For every ID in Requirements.All():
//
//   - Absent from g.Requirements: SyncGraph APPENDS a new node built from
//     the registry's structural fields, with every EVENT field empty except
//     a single seed HistoryEntry (At: today, Summary: "created via
//     sync-self") recording the creation itself — an empty History would
//     leave the node's own origin unrecorded, breaking the append-only
//     History discipline internal/proposal/mutate.go already upholds for
//     ordinary proposal-landed nodes (see that package's history.go). The
//     entry carries no DecidedBy/Signoff — CREATE via sync-self is a
//     mechanical mirroring action, not a human decision to attribute.
//
//   - Present in both: the SAME structural-field replacement MergeIntoGraph
//     performs (event fields pass through untouched), but ADDITIONALLY: if
//     StructuralFieldDiffs(reg, existing) is non-empty, SyncGraph APPENDS one
//     HistoryEntry summarizing exactly what changed ("field Claim: <old> ->
//     <new>; field Owner: <old> -> <new>; ..."), because MergeIntoGraph
//     itself deliberately never writes History (it is a read-mostly mirror
//     check's engine, not a landing path) — SyncGraph IS a landing path, so
//     it must uphold the same history-on-mutation discipline
//     internal/proposal/mutate.go's ProposedRequirement.mutate already does
//     for ordinary proposals.
//
//   - Present in graph only (no matching registry ID): left COMPLETELY
//     untouched. Deletion is out of scope for this phase — such a node is
//     drift a future boot invariant (task #351/RAC-B4) will catch, not
//     something this function acts on.
//
// A requirement that already matches the registry on every structural field
// is touched not at all (no field replacement — a no-op replacement is
// still observably a no-op since it copies identical values back — and,
// importantly, no History entry) and does not appear in the returned
// SyncReport.
//
// SyncGraph mutates g IN PLACE and returns a report of every requirement it
// actually created or changed. It never touches disk — writing the result
// through internal/loader.WriteGraph is the caller's job (the future `hotam
// sync-self` CLI, task #349).
func SyncGraph(g *ontology.Graph, today string) (*SyncReport, error) {
	if g == nil {
		return nil, fmt.Errorf("selfspec: SyncGraph: nil graph")
	}

	indexByID := make(map[string]int, len(g.Requirements))
	for i, r := range g.Requirements {
		indexByID[r.ID] = i
	}

	report := &SyncReport{}

	for _, id := range registeredIDsSorted() {
		reg, ok := Requirements.Get(id)
		if !ok {
			// unreachable: id came from Requirements itself.
			continue
		}

		idx, found := indexByID[id]
		if !found {
			created := *reg // structural fields from the registry
			created.LastReviewedAt = ""
			created.ReviewAfter = ""
			created.Evidence = nil
			created.GateSignoffs = nil
			created.History = []ontology.HistoryEntry{
				{At: today, Summary: "created via sync-self"},
			}
			g.Requirements = append(g.Requirements, created)
			indexByID[id] = len(g.Requirements) - 1
			report.Entries = append(report.Entries, SyncReportEntry{
				ID:   id,
				Kind: SyncKindAdded,
			})
			continue
		}

		existing := g.Requirements[idx]
		diffs := StructuralFieldDiffs(*reg, existing)
		if len(diffs) == 0 {
			continue
		}

		merged := *reg // copy: structural fields from the registry
		// Event fields pass through untouched from the graph's existing node.
		merged.LastReviewedAt = existing.LastReviewedAt
		merged.ReviewAfter = existing.ReviewAfter
		merged.Evidence = existing.Evidence
		merged.History = append(existing.History, ontology.HistoryEntry{
			At:      today,
			Summary: summarizeSyncFieldDiffs(diffs),
		})
		merged.GateSignoffs = existing.GateSignoffs
		g.Requirements[idx] = merged

		report.Entries = append(report.Entries, SyncReportEntry{
			ID:         id,
			Kind:       SyncKindChanged,
			FieldDiffs: diffs,
		})
	}

	return report, nil
}

// summarizeSyncFieldDiffs renders diffs (StructuralFieldDiffs' result) into
// the single human-readable History summary line SyncGraph appends for a
// CHANGED requirement — "field Claim: <old> -> <new>; field Owner: <old> ->
// <new>; ...", one clause per differing field, in StructuralFieldDiffs'
// own field order (stable, since that function always builds its field list
// in the same order).
func summarizeSyncFieldDiffs(diffs []FieldDiff) string {
	parts := make([]string, 0, len(diffs))
	for _, d := range diffs {
		parts = append(parts, fmt.Sprintf("field %s: %v -> %v", d.Field, d.Old, d.New))
	}
	return strings.Join(parts, "; ")
}
