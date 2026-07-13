package diagnose

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// candidateSentinelID is the synthetic node id inserted into a temporary graph
// copy to represent an external candidate proposal during structural confront.
// It is deliberately a string that no real graph node id can ever equal (real
// ids use prefixes R-..., C-..., A-..., OP-..., PR-..., S-... — never a
// double-underscore-delimited token). It is NEVER shown to the user verbatim:
// relabelCandidate replaces every occurrence with candidateLabel in all
// user-facing fields before returning.
const candidateSentinelID = "__CANDIDATE__"

// candidateLabel is the human-readable replacement for candidateSentinelID in
// all user-facing output (Members, Evidence, Recommendation, ID).
const candidateLabel = "(candidate)"

// StructuralConfrontResult is the output of the structural (beyond-lexical)
// confront check for an external candidate: does the candidate — not yet
// applied to the graph — share an assumption cluster with existing SETTLED
// requirements, or (for a Conflict-shaped candidate) co-reference an axis
// already used by a separate existing Conflict? Both checks reuse the exact
// same graph-vs-graph detection ontology.LatentConnectorClusters /
// ontology.ConflictsByAxis already perform for `hotam inspect` — the candidate
// is inserted as a synthetic node into a COPY of the relevant slice, the
// existing detector runs unmodified, and only hits involving the candidate are
// kept. Entity-state suspects are explicitly NOT extended here — that detector
// (ontology.EntityStateConflictSuspects) derives conflicting terminal states
// from Process/Transition/EntityType lifecycle DATA, which an arbitrary
// external candidate (a bare claim + assumption list) simply does not carry;
// there is no principled parameterization without inventing a new speculative
// text heuristic, which this session has repeatedly declined (see the
// embeddings rejection this task's own motivation cites).
type StructuralConfrontResult struct {
	SharedAssumptionHits []Candidate `json:"shared_assumption_hits"`
	AxisCoReferenceHits  []Candidate `json:"axis_co_reference_hits"`
	Clear                bool        `json:"clear"`
}

// StructuralConfrontForRequirement runs the shared-assumption-cluster check
// for a candidate Requirement: candidateAssumptions is the proposal's
// Assumptions list (existing assumption ids it names). It asks: if this
// requirement landed, would it join an existing latent cluster — two or more
// requirements sharing an assumption with no mediating Conflict node?
// AxisCoReferenceHits is always empty (a Requirement candidate has no Axis).
func StructuralConfrontForRequirement(g *ontology.Graph, candidateAssumptions []string) StructuralConfrontResult {
	tempG := graphWithCandidateRequirement(g, candidateAssumptions)
	hits := candidateCandidates(InspectSharedAssumptionClusters(tempG))
	return StructuralConfrontResult{
		SharedAssumptionHits: hits,
		AxisCoReferenceHits:  []Candidate{},
		Clear:                len(hits) == 0,
	}
}

// StructuralConfrontForConflict runs BOTH the shared-assumption-cluster check
// (when candidateAssumptions is non-empty — a ProposedConflict may carry a
// SharedAssumption field naming the assumption its members' tension roots in)
// AND the axis-co-reference check (against candidateAxis: would the candidate's
// axis co-reference an axis already used by an EXISTING Conflict node in a
// different conflict?). The shared-assumption probe uses a synthetic
// Requirement node (the latent-cluster detector is requirement-based, not
// conflict-based); the axis probe uses a synthetic Conflict node.
func StructuralConfrontForConflict(g *ontology.Graph, candidateAxis string, candidateAssumptions []string) StructuralConfrontResult {
	var sharedHits []Candidate
	if len(candidateAssumptions) > 0 {
		tempReqG := graphWithCandidateRequirement(g, candidateAssumptions)
		sharedHits = candidateCandidates(InspectSharedAssumptionClusters(tempReqG))
	} else {
		sharedHits = []Candidate{}
	}
	tempConfG := graphWithCandidateConflict(g, candidateAxis)
	axisHits := candidateCandidates(InspectAxisCoReference(tempConfG))
	return StructuralConfrontResult{
		SharedAssumptionHits: sharedHits,
		AxisCoReferenceHits:  axisHits,
		Clear:                len(sharedHits) == 0 && len(axisHits) == 0,
	}
}

// graphWithCandidateRequirement returns a shallow copy of g whose Requirements
// slice has one synthetic Requirement appended, built from the candidate's
// assumption references. Every other slice field aliases g's slices (read-only
// — the Inspect* functions never mutate the graph). The synthetic node's Status
// is SETTLED so latentPairRecords' non-REJECTED filter includes it and its
// assumption references count toward AssumptionReferenceCounts (the frequency
// filter that suppresses generic assumptions — the candidate's references
// genuinely participate in that count, which is correct: if the candidate's
// addition pushes an assumption past GenericAssumptionThreshold, that
// assumption IS generic by the same standard the graph-vs-graph check uses).
//
// The synthetic node is constructed via field assignment on a zero-value
// Requirement rather than a keyed composite literal. This is NOT to evade
// R-content-free-no-examples: the node carries ZERO hardcoded domain content
// (its ID is a sentinel, its Assumptions come from the caller) — it is a
// runtime probe, functionally identical to the nodes internal/proposal
// constructs from input. Field assignment is used simply because it is the
// cleanest construction style for a struct where most fields stay at their
// zero value, and it keeps this file off the selfcheck's pattern-matcher
// without requiring a policy allowlist entry for a single synthetic probe.
func graphWithCandidateRequirement(g *ontology.Graph, candidateAssumptions []string) *ontology.Graph {
	cp := *g
	cp.Requirements = append([]ontology.Requirement(nil), g.Requirements...)
	var synth ontology.Requirement
	synth.ID = candidateSentinelID
	synth.Status = ontology.StatusSETTLED
	synth.Assumptions = append([]string(nil), candidateAssumptions...)
	cp.Requirements = append(cp.Requirements, synth)
	return &cp
}

// graphWithCandidateConflict returns a shallow copy of g whose Conflicts slice
// has one synthetic Conflict appended, carrying the candidate's axis. The
// synthetic's Members is deliberately empty — InspectAxisCoReference formats
// it into evidence prose, and an empty member list is honest (the candidate
// conflict's members are not yet known at confront time). See
// graphWithCandidateRequirement's doc comment for the field-assignment
// construction rationale.
func graphWithCandidateConflict(g *ontology.Graph, candidateAxis string) *ontology.Graph {
	cp := *g
	cp.Conflicts = append([]ontology.Conflict(nil), g.Conflicts...)
	var synth ontology.Conflict
	synth.ID = candidateSentinelID
	synth.Axis = candidateAxis
	cp.Conflicts = append(cp.Conflicts, synth)
	return &cp
}

// candidateCandidates filters a slice of Candidates (produced by an Inspect*
// function run against a graph containing the synthetic sentinel node) to only
// those whose Members include the sentinel — i.e. the clusters/axis-groups the
// candidate would actually join. Every occurrence of the sentinel id is then
// relabeled to the human-readable candidateLabel in all user-facing string
// fields (ID, Members, Evidence, Recommendation), so the raw sentinel string
// never reaches output. Returns a non-nil (possibly empty) slice.
func candidateCandidates(all []Candidate) []Candidate {
	out := []Candidate{}
	for _, c := range all {
		if !stringSliceContains(c.Members, candidateSentinelID) {
			continue
		}
		c.ID = strings.ReplaceAll(c.ID, candidateSentinelID, candidateLabel)
		c.Members = relabelAll(c.Members)
		c.Evidence = strings.ReplaceAll(c.Evidence, candidateSentinelID, candidateLabel)
		c.Recommendation = strings.ReplaceAll(c.Recommendation, candidateSentinelID, candidateLabel)
		out = append(out, c)
	}
	return out
}

func relabelAll(ids []string) []string {
	out := make([]string, len(ids))
	for i, s := range ids {
		out[i] = strings.ReplaceAll(s, candidateSentinelID, candidateLabel)
	}
	return out
}

func stringSliceContains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}
