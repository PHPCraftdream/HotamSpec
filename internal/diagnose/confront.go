package diagnose

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Classification and Confidence contain stable machine-readable labels for
// advisory lexical/link evidence and explicit formal carrier matches.
const (
	ClassificationLexicalSuspicion = "lexical_suspicion"
	ClassificationLinkedSuspicion  = "linked_suspicion"
	ClassificationFormalConflict   = "formal_conflict"

	ConfidenceAdvisory     = "advisory"
	ConfidenceCorroborated = "corroborated"
	ConfidenceExplicit     = "explicit"
)

// ConfrontHit is a lexical/metadata suspicion or a match to an explicit
// unresolved Conflict carrier. Classification and Reasons explain the
// machine's evidence; only formal_conflict is a hard blocker.
type ConfrontHit struct {
	ID             string   `json:"id"`
	Claim          string   `json:"claim"`
	Score          int      `json:"score"`
	Shared         []string `json:"shared"`
	OppositeMarker string   `json:"opposite_marker,omitempty"`
	ReplacedBy     []string `json:"replaced_by,omitempty"`
	Classification string   `json:"classification"`
	Confidence     string   `json:"confidence"`
	Reasons        []string `json:"reasons"`
	ConflictIDs    []string `json:"conflict_ids,omitempty"`
}

// HasOppositeMarker indicates recognized opposite marker text. It does not
// establish that the claims share a scope or contradict one another.
func (h ConfrontHit) HasOppositeMarker() bool {
	return h.OppositeMarker != ""
}

// ConfrontResult carries lexical suspicions in Settled/Rejected and explicit
// matching unresolved carriers separately in FormalConflicts.
type ConfrontResult struct {
	Candidate       string        `json:"candidate"`
	Settled         []ConfrontHit `json:"settled"`
	Rejected        []ConfrontHit `json:"rejected"`
	FormalConflicts []ConfrontHit `json:"formal_conflicts"`
	Clear           bool          `json:"clear"`
}

// Confront checks candidateText for lexical overlap with the SETTLED and
// REJECTED requirements of g. Opposite markers lower the lexical overlap
// threshold but are evidence of textual polarity only: every lexical result is
// advisory, never proof of semantic contradiction and never a blocker.
func Confront(g *ontology.Graph, candidateText string) ConfrontResult {
	var candidate ontology.Requirement
	candidate.Claim = candidateText
	return confront(g, candidate)
}

// ConfrontRequirement checks a candidate Requirement using its claim and
// authored metadata. Exact implementation/source links and explicit relations
// can strengthen a lexical suspicion or surface a linked suspicion without
// lexical overlap. Only an explicit unresolved Conflict carrier naming the
// candidate ID and another member ID is classified as formal_conflict.
func ConfrontRequirement(g *ontology.Graph, candidate ontology.Requirement) ConfrontResult {
	return confront(g, candidate)
}

func confront(g *ontology.Graph, candidate ontology.Requirement) ConfrontResult {
	candidateText := candidate.Claim
	common := corpusCommonTokens(g)
	candTokens := claimTokens(candidateText, common)
	candMarks := markerHits(candidateText)

	replaces := ontology.ReplacesMap(g)

	var settled, rejected []ConfrontHit
	for _, r := range g.Requirements {
		if candidate.ID != "" && r.ID == candidate.ID {
			continue
		}
		switch r.Status {
		case ontology.StatusSETTLED, ontology.StatusREJECTED:
		default:
			continue
		}
		hit := confrontHit(candTokens, candMarks, candidate, r, common)
		if hit == nil {
			continue
		}
		if r.Status == ontology.StatusREJECTED {
			if succ, ok := replaces[r.ID]; ok && len(succ) > 0 {
				cp := make([]string, len(succ))
				copy(cp, succ)
				hit.ReplacedBy = cp
			}
			rejected = append(rejected, *hit)
		} else {
			settled = append(settled, *hit)
		}
	}

	formal := formalConflictHits(g, candidate)
	sortConfrontHits(settled)
	sortConfrontHits(rejected)
	sortConfrontHits(formal)

	if settled == nil {
		settled = []ConfrontHit{}
	}
	if rejected == nil {
		rejected = []ConfrontHit{}
	}
	if formal == nil {
		formal = []ConfrontHit{}
	}

	return ConfrontResult{
		Candidate:       candidateText,
		Settled:         settled,
		Rejected:        rejected,
		FormalConflicts: formal,
		Clear:           len(settled) == 0 && len(rejected) == 0 && len(formal) == 0,
	}
}

// confrontHit returns a populated *ConfrontHit for candidate vs r when there
// is enough lexical overlap or an exact authored metadata link.
func confrontHit(candTokens map[string]struct{}, candMarks map[string]string, candidate, r ontology.Requirement, common map[string]struct{}) *ConfrontHit {
	reqTokens := claimTokens(r.Claim, common)
	var shared []string
	for t := range candTokens {
		if _, ok := reqTokens[t]; ok {
			shared = append(shared, t)
		}
	}
	sort.Strings(shared)

	opposite := oppositeMarkerBetween(candMarks, markerHits(r.Claim))
	linked, linkReasons := sharedMetadataLinks(candidate, r)

	threshold := MinLexicalOverlapTokens
	if opposite != "" {
		threshold = MinLexicalOverlapTokensWithMarker
	}
	if len(shared) < threshold && !linked {
		return nil
	}

	score := len(shared)
	reasons := make([]string, 0, 2+len(linkReasons))
	if len(shared) > 0 {
		reasons = append(reasons, "shared lexical tokens: ["+strings.Join(shared, ", ")+"]")
	}
	if opposite != "" {
		score += 3
		reasons = append(reasons, "opposite marker text: "+opposite+" (lexical evidence only)")
	}
	reasons = append(reasons, linkReasons...)

	classification := ClassificationLexicalSuspicion
	confidence := ConfidenceAdvisory
	if linked {
		classification = ClassificationLinkedSuspicion
		confidence = ConfidenceCorroborated
	}
	return &ConfrontHit{
		ID:             r.ID,
		Claim:          r.Claim,
		Score:          score,
		Shared:         shared,
		OppositeMarker: opposite,
		Classification: classification,
		Confidence:     confidence,
		Reasons:        reasons,
	}
}

func sharedMetadataLinks(candidate, existing ontology.Requirement) (bool, []string) {
	var reasons []string
	for _, candidateLink := range candidate.ImplementedBy {
		if candidateLink == "" {
			continue
		}
		for _, existingLink := range existing.ImplementedBy {
			if candidateLink == existingLink {
				reasons = append(reasons, "same implemented_by link: "+candidateLink)
			}
		}
	}
	for _, candidateLink := range candidate.VerifiedBy {
		if candidateLink == "" {
			continue
		}
		for _, existingLink := range existing.VerifiedBy {
			if candidateLink == existingLink {
				reasons = append(reasons, "same verified_by link: "+candidateLink)
			}
		}
	}
	for _, candidateLink := range candidate.SourceLinks {
		if candidateLink.SourceID == "" || candidateLink.Anchor == "" {
			continue
		}
		for _, existingLink := range existing.SourceLinks {
			if candidateLink == existingLink {
				reasons = append(reasons, "same source anchor: "+candidateLink.SourceID+"#"+candidateLink.Anchor)
			}
		}
	}
	for _, candidateRelation := range candidate.Relations {
		if candidateRelation.Kind == "" || candidateRelation.Target == "" {
			continue
		}
		for _, existingRelation := range existing.Relations {
			if candidateRelation == existingRelation {
				reasons = append(reasons, "same explicit relation: "+candidateRelation.Kind+" -> "+candidateRelation.Target)
			}
		}
		if existing.ID != "" && candidateRelation.Target == existing.ID {
			reasons = append(reasons, "explicit relation to matched requirement: "+candidateRelation.Kind+" -> "+existing.ID)
		}
	}
	for _, existingRelation := range existing.Relations {
		if candidate.ID != "" && existingRelation.Kind != "" && existingRelation.Target == candidate.ID {
			reasons = append(reasons, "matched requirement explicitly relates to candidate: "+existingRelation.Kind+" -> "+candidate.ID)
		}
	}
	sort.Strings(reasons)
	return len(reasons) > 0, reasons
}

func formalConflictHits(g *ontology.Graph, candidate ontology.Requirement) []ConfrontHit {
	if candidate.ID == "" {
		return nil
	}
	requirements := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		requirements[r.ID] = r
	}
	byMember := make(map[string]*ConfrontHit)
	for _, conflict := range g.Conflicts {
		if !conflict.IsUnresolved() || conflict.ID == "" || !containsID(conflict.Members, candidate.ID) {
			continue
		}
		for _, memberID := range conflict.Members {
			if memberID == "" || memberID == candidate.ID {
				continue
			}
			hit, ok := byMember[memberID]
			if !ok {
				hit = &ConfrontHit{
					ID:             memberID,
					Classification: ClassificationFormalConflict,
					Confidence:     ConfidenceExplicit,
					Reasons:        []string{},
				}
				if r, exists := requirements[memberID]; exists {
					hit.Claim = r.Claim
				}
				byMember[memberID] = hit
			}
			hit.ConflictIDs = append(hit.ConflictIDs, conflict.ID)
			hit.Reasons = append(hit.Reasons, "unresolved "+conflict.Lifecycle+" Conflict "+conflict.ID+" names both "+candidate.ID+" and "+memberID)
		}
	}
	out := make([]ConfrontHit, 0, len(byMember))
	for _, hit := range byMember {
		sort.Strings(hit.ConflictIDs)
		sort.Strings(hit.Reasons)
		out = append(out, *hit)
	}
	return out
}

func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// oppositeMarkerBetween returns the human-readable "a vs b" label for the first
// oppositeMarkerPair whose two sides are split across the two mark maps (one
// side in a, the other in b), or "" when no such split exists. It is the same
// comparison InspectLexicalClaimOverlap inlines over two settled requirements.
func oppositeMarkerBetween(a, b map[string]string) string {
	keys := make([]string, 0, len(a))
	for key := range a {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		sideA := a[key]
		sideB, ok := b[key]
		if !ok || sideB == sideA {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		if len(parts) == 2 {
			return parts[0] + " vs " + parts[1]
		}
	}
	return ""
}

func sortConfrontHits(hits []ConfrontHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
}
