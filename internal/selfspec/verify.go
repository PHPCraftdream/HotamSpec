package selfspec

import (
	"fmt"
	"reflect"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// VerifyAppendOnly is a PURE guard, independent of SyncGraph's own merge
// logic: given two *ontology.Graph snapshots — old (before a mutation) and
// new (after) — it checks that every append-only journal old carried is
// still there in new, as an exact PREFIX (never truncated, never edited in
// place), for every Requirement in old. This is strictly stronger than
// either a length-only check (which would accept a truncate-then-pad back to
// the same length) or a superset/set-membership check (which would accept
// the entries being present but reordered or duplicated) — a prefix check
// asserts both that nothing already recorded was removed AND that nothing
// already recorded was altered, element by element, in its original order.
//
// For every Requirement r in old.Requirements:
//
//  1. r.ID must exist in new.Requirements — SyncGraph must never delete a
//     node (this also catches a node silently dropped by some other bug,
//     not just an intentional deletion).
//  2. r.History must be an exact prefix of the matching new Requirement's
//     History: len(new.History) >= len(r.History), and new.History[i] ==
//     r.History[i] for every i < len(r.History).
//  3. r.GateSignoffs — itself an append-only journal by the same nature as
//     History (see GateSignoff's own doc comment) — must be an exact prefix
//     of the matching new Requirement's GateSignoffs, by the identical rule.
//
// VerifyAppendOnly takes two full graph snapshots and knows nothing about
// SyncGraph, MergeIntoGraph, or any other specific mutation — it is a
// general "did this graph transition respect the append-only journal
// invariant" checker, reusable against any before/after pair.
func VerifyAppendOnly(old, new *ontology.Graph) error {
	if old == nil {
		return fmt.Errorf("selfspec: VerifyAppendOnly: nil old graph")
	}
	if new == nil {
		return fmt.Errorf("selfspec: VerifyAppendOnly: nil new graph")
	}

	newByID := make(map[string]ontology.Requirement, len(new.Requirements))
	for _, r := range new.Requirements {
		newByID[r.ID] = r
	}

	for _, oldReq := range old.Requirements {
		newReq, ok := newByID[oldReq.ID]
		if !ok {
			return fmt.Errorf("selfspec: VerifyAppendOnly: requirement %q present in old graph is missing from new graph", oldReq.ID)
		}

		if err := verifyHistoryPrefix(oldReq.ID, oldReq.History, newReq.History); err != nil {
			return err
		}
		if err := verifyGateSignoffsPrefix(oldReq.ID, oldReq.GateSignoffs, newReq.GateSignoffs); err != nil {
			return err
		}
	}

	return nil
}

// verifyHistoryPrefix checks that oldHistory is an exact prefix of
// newHistory, for the requirement named id.
func verifyHistoryPrefix(id string, oldHistory, newHistory []ontology.HistoryEntry) error {
	if len(newHistory) < len(oldHistory) {
		return fmt.Errorf("selfspec: VerifyAppendOnly: requirement %q: History shrank from %d to %d entries", id, len(oldHistory), len(newHistory))
	}
	for i, oldEntry := range oldHistory {
		if !reflect.DeepEqual(oldEntry, newHistory[i]) {
			return fmt.Errorf("selfspec: VerifyAppendOnly: requirement %q: History entry %d changed: old=%+v new=%+v", id, i, oldEntry, newHistory[i])
		}
	}
	return nil
}

// verifyGateSignoffsPrefix checks that oldSignoffs is an exact prefix of
// newSignoffs, for the requirement named id — the same append-only prefix
// rule verifyHistoryPrefix applies to History, applied here to GateSignoffs.
func verifyGateSignoffsPrefix(id string, oldSignoffs, newSignoffs []ontology.GateSignoff) error {
	if len(newSignoffs) < len(oldSignoffs) {
		return fmt.Errorf("selfspec: VerifyAppendOnly: requirement %q: GateSignoffs shrank from %d to %d entries", id, len(oldSignoffs), len(newSignoffs))
	}
	for i, oldEntry := range oldSignoffs {
		if !reflect.DeepEqual(oldEntry, newSignoffs[i]) {
			return fmt.Errorf("selfspec: VerifyAppendOnly: requirement %q: GateSignoffs entry %d changed: old=%+v new=%+v", id, i, oldEntry, newSignoffs[i])
		}
	}
	return nil
}
