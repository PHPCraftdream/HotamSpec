// requirements.go resolves GEN-CODE-CONTRACT.md §3's atom-classification
// heuristic: for every SETTLED ontology.Requirement, find which parts of its
// claim text are mirrored by already-generated Go structure (EntityType
// fields, lifecycle state pairs, meta-token+typed-anchor gate/order
// assertions, or cross-entity invariants), and which parts have no
// structural carrier at all (honest gap, §3's closing row).
//
// The heuristic is deterministic regex/substring matching over already-
// resolved entityModel data (never LLM-guessing, never re-deriving an
// identifier — contract §0/§3's closing paragraph) and is computed exactly
// once per requirement here, then threaded through both
// requirements_test_gen.go (the .go assertions) and audit.go's Requirements
// section (the requirements_audit.md anchors) — the same single-source
// discipline BuildEntityModel already gives entity/state/transition
// identifiers.
package gocode

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// metaTokenPattern matches GEN-CODE-CONTRACT.md §3's literal meta-language
// reserved tokens (MUST NOT checked before MUST so it is not shadowed).
var metaTokenPattern = regexp.MustCompile(`\bMUST NOT\b|\bMUST\b|\bALWAYS\b|\bNEVER\b|\bONLY\b|\bANY\b`)

// idAnchorPattern matches a generic "typed identifier-looking" token in
// claim text: a capitalized word followed by one or more hyphen-joined
// segments (e.g. "P-G3", "P-G1-R", "E-G1", "R-gate-pg3-brd-approved").
// This is deliberately domain-agnostic (no hardcoded "P-G" or similar) —
// it recognizes the SHAPE contract §3 calls out ("R-gate-*", artifact ids),
// not any one domain's specific gate-naming convention.
var idAnchorPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]*(-[A-Za-z0-9]+)+\b`)

// gateAnchorTokenPattern matches the narrower "gate/stage token" SHAPE
// inside a claim: a single capital letter, a hyphen, a capital letter
// immediately followed by digits, then optional further hyphen-joined
// segments — e.g. "P-G3", "P-G4", "P-G1-R", "P-G3-CQA", "E-G1". This is a
// strict subset of idAnchorPattern (row 3's generic id-shaped anchor): it
// deliberately does NOT match multi-letter-word ids like
// "R-gate-pg3-brd-approved" or phrase-shaped tokens like "Feature-Lead",
// only the short stage/gate-marker shape. Used by resolveScopedFieldMatches'
// gate-token guard (task #218): a FIELD whose name happens to spell such a
// token (prat: risk-registry.p_g3 — termMatch splits it into [p g3], which
// word-sequence-matches the literal gate mention "P-G3" in ANY claim that
// talks about that gate) must not become a row-1 field atom for a
// requirement whose only tie to the field is that shared gate token — the
// claim is referencing the GATE, and the gate/order path (row 3) is the
// honest carrier for that reference.
var gateAnchorTokenPattern = regexp.MustCompile(`\b[A-Z]-[A-Z][0-9]+(-[A-Za-z0-9]+)*\b`)

// atomKind is the GEN-CODE-CONTRACT.md §3 atom classification for one
// requirement. Exactly one kind is assigned per requirement (first matching
// row in the contract's table wins), mirroring the table's explicit
// top-to-bottom priority order.
type atomKind int

const (
	atomKindNone atomKind = iota
	atomKindField
	atomKindStatePair
	atomKindGate
	atomKindInterEntity
)

// fieldAtom is one "claim references EntityType.field" hit (contract §3 row
// 1): the field's already-resolved fieldModel (via the owning entityModel),
// so the rendered assertion reuses BuildEntityModel's identifiers rather
// than re-deriving them.
//
// pipelineGate is set (task #209, contract §0's mirror principle applied
// across field atoms and pipeline gates) when field is a kind:reference
// field for which pipeline.go's BuildPipelineGateModels already built a
// gate function (precise-state, contract §2.1, or the general
// RequiresTerminal fallback) — the SAME structural edge, found once by
// BuildPipelineGateModels and reused here rather than re-resolved. When set,
// renderFieldAtomBody (requirements_test_gen.go) renders an ADDITIONAL
// sub-test, beyond the plain "field is not empty" Validate() check, that
// builds the referenced entity in (and out of) the gate's required state and
// asserts the gate function accepts/rejects accordingly — closing the gap
// where a field-atom requirement whose claim names a concrete referenced
// state (e.g. "forecast_v2") was previously mirrored only at the
// presence-check level, never at the state-precision level the pipeline
// gate already enforces.
type fieldAtom struct {
	entity       *entityModel
	field        fieldModel
	pipelineGate *pipelineGateModel
}

// statePairAtom is one "claim mentions a pair of lifecycle.states of one
// EntityType" hit (contract §3 row 2): the owning entityModel plus the
// (2+) already-resolved stateModels the claim literally names.
type statePairAtom struct {
	entity *entityModel
	states []stateModel
}

// gateAnchorCorrelateKind classifies WHERE an anchor token (contract §3 row
// 3) was independently found elsewhere in the domain graph, so the rendered
// assertion in requirements_test.go can compare against that SAME real
// correlate at runtime, instead of re-asserting the literal the claim-text
// regex itself produced (the bug this type exists to fix: a self-authored
// literal is not a graph check).
type gateAnchorCorrelateKind int

const (
	// gateAnchorCorrelateNone means this anchor token was not found anywhere
	// else in the domain graph (neither a lifecycle.state.name, nor an
	// EntityType.why, nor another SETTLED requirement's id). It carries no
	// verifiable runtime correlate.
	gateAnchorCorrelateNone gateAnchorCorrelateKind = iota
	// gateAnchorCorrelateState means the anchor is a substring of some
	// EntityType's lifecycle.state.name (contract §3 row 3, sub-clause a).
	// The match is ASCII-safe: stateModel.value is a 1:1 ToKebabCase
	// translation of that same raw state name (identifiers.go), so an
	// anchor-substring hit against the raw name transfers directly to a hit
	// against the already-ASCII Go constant's string value — comparable at
	// runtime with no Cyrillic involved.
	gateAnchorCorrelateState
	// gateAnchorCorrelateWhy means the anchor is a substring of some
	// EntityType's why text (contract §3 row 3, sub-clause b). This
	// correlate is real but textual-only: why is legitimately Cyrillic
	// (contract §1.1), so it cannot be mirrored as a runtime .go assertion
	// — it is documented in requirements_audit.md, never embedded in a
	// string literal in the .go layer.
	gateAnchorCorrelateWhy
	// gateAnchorCorrelateRequirement means the anchor is a substring of
	// another SETTLED requirement's id (contract §3 row 3, sub-clause c).
	// Requirement ids are ASCII by the framework's own id convention, so
	// this is directly comparable at runtime.
	gateAnchorCorrelateRequirement
)

// gateAnchorCorrelate is one anchor token's resolved correlate: the anchor
// text itself, which kind of graph element it was found in, and enough
// already-resolved identifier data to render a concrete runtime assertion
// (never a re-derived or re-translated literal).
type gateAnchorCorrelate struct {
	anchor string
	kind   gateAnchorCorrelateKind
	// state is set when kind == gateAnchorCorrelateState: the entity whose
	// lifecycle.state.name matched, and the matched stateModel itself
	// (state.value is the ASCII runtime-comparable string).
	stateEntity *entityModel
	state       stateModel
	// requirementID is set when kind == gateAnchorCorrelateRequirement: the
	// other SETTLED requirement's id (already ASCII) the anchor matched.
	requirementID string
}

// gateAtom is one "meta-token + typed anchor" hit (contract §3 row 3): the
// literal anchor tokens found (already ASCII by construction — the pattern
// only matches Latin-letter/digit/hyphen shapes) plus, for every anchor,
// where (if anywhere) it independently correlates elsewhere in the domain
// graph (see gateAnchorCorrelate) — the rendered assertion mirrors that real
// correlate, never the anchor list alone (the vacuous-test bug this type
// replaces).
type gateAtom struct {
	anchors    []string
	correlates []gateAnchorCorrelate
}

// hasStructuralCorrelate reports whether at least one of this gate atom's
// anchors resolved to a runtime-comparable correlate (state or requirement
// id — NOT why-only, since why is Cyrillic and cannot be mirrored into a
// .go runtime assertion per contract §1.1). A gate/order requirement with no
// structural correlate at all is honest-gap, not a vacuous self-check
// (contract §3's closing row, §0 mirror principle).
func (g gateAtom) hasStructuralCorrelate() bool {
	for _, c := range g.correlates {
		if c.kind == gateAnchorCorrelateState || c.kind == gateAnchorCorrelateRequirement {
			return true
		}
	}
	return false
}

// requirementModel is the resolved, Go-identifier-shaped view of one
// ontology.Requirement, built once so the .go test function/sub-tests and
// the audit.md section never re-derive the function name or re-classify the
// claim independently (contract §0 "one source of truth").
type requirementModel struct {
	src         ontology.Requirement
	funcName    string // Test_<R-id-with-hyphens-as-underscores>
	anchorSlug  string // requirements_audit.md heading anchor (kebab, lowercased id)
	kind        atomKind
	fields      []fieldAtom
	statePair   *statePairAtom
	gate        gateAtom
	interEntity []*entityModel
}

// requirementFuncNameBody converts an already-ASCII requirement id (e.g.
// "R-gate-pg3-brd-approved") into a valid Go identifier body by replacing
// every '-' with '_'. Per the task brief, requirement ids are already ASCII
// by the framework's own id convention (not a graph-name needing
// glossary/abbreviation translation via ToPascalCase) — this is a plain
// mechanical substitution, not a second translation path competing with
// resolveParts.
func requirementFuncNameBody(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}

// BuildRequirementModel resolves one SETTLED requirement's identifiers and
// classifies its claim into the GEN-CODE-CONTRACT.md §3 atom taxonomy
// against the domain's already-built entity models. entityModels must be
// the same slice (or an equivalent rebuild) BuildEntityModel already
// produced for entities.go/lifecycle.go, so field/state identifiers here are
// never independently re-derived.
//
// otherSettled is every OTHER SETTLED requirement in the domain (req itself
// excluded by the caller or skipped internally) — needed by row 3's gate/
// order classification (contract §3), which must verify each anchor token
// against a real graph correlate (lifecycle.state.name, EntityType.why, or
// another requirement's id), not just record the literal it found in req's
// own claim text. Pass nil (or an empty slice) when no cross-requirement
// corpus is available (e.g. isolated unit fixtures) — the requirement-id
// correlate is then simply never found, same as any other missing correlate.
//
// gates is the domain's already-built pipeline gate models (pipeline.go's
// BuildPipelineGateModels), threaded through so a row-1 field-atom match on
// a kind:reference field can find and reuse its own already-built pipeline
// gate (task #209) rather than re-deriving the referencer/ref_target
// resolution a second time. Pass nil when no pipeline gates are available
// (e.g. isolated unit fixtures, or a caller that only needs stage-4 output)
// — every reference-field match then simply carries no pipelineGate, exactly
// the pre-task-#209 behavior.
func BuildRequirementModel(req ontology.Requirement, entityModels []*entityModel, otherSettled []ontology.Requirement, gates []*pipelineGateModel) *requirementModel {
	m := &requirementModel{
		src:        req,
		funcName:   "Test_" + requirementFuncNameBody(req.ID),
		anchorSlug: strings.ToLower(req.ID),
	}

	// Row 1 (contract §3): claim references an EntityType.field by its
	// original graph name OR its translated Go identifier, checked via
	// termMatch — a Unicode-aware whole-word/whole-phrase match (works for
	// both Cyrillic field names and ASCII ones like "dor", "cosmic" — the
	// same field names entities.go's struct already carries). termMatch also
	// covers a claim phrase spelled with spaces against a graph name spelled
	// with underscores/hyphens (e.g. claim "Feature Lead" against graph field
	// name "feature_lead") and against the field's PascalCase translation
	// (e.g. "FeatureLead") — see termMatch's doc comment for the exact
	// matching rule (contract §3.1, the "feature_lead" gap found on prat).
	// Sorted (entity slug, field name) for determinism: the source domain's
	// entityModels are already sorted by slug (GenerateLifecycleFromGraph),
	// and each entity's own field order is graph-declaration order — the
	// walk below preserves both, so no extra sort is needed.
	//
	// A match found ONLY via the translated identifier (f.fieldName), never
	// via the raw graph name (f.src.Name), AND whose translated identifier is
	// a SINGLE word (e.g. "Forecast") is a riskier, more coincidental hit
	// than a raw-name match or a multi-word translated phrase (e.g.
	// "FeatureLead" - contract §3.1's actual gap, deliberately left alone):
	// a short single translated word can legitimately belong to more than one
	// EntityType in the domain (prat: both brd-package and sdr-package have a
	// "прогноз" field that translates to "Forecast") and then matches a claim
	// substring (e.g. "forecast_v3") regardless of which of those EntityTypes
	// the claim is actually about. resolveScopedFieldMatches applies
	// referencer-scoping to exactly that ambiguous case (contract §2.1's
	// resolvePreciseGateState idea, reused here) — every other match
	// (raw-name hit, or multi-word translated phrase) is kept unconditionally.
	m.fields = resolveScopedFieldMatches(req.Claim, entityModels, gates)
	if len(m.fields) > 0 {
		m.kind = atomKindField
		return m
	}

	// Row 2 (contract §3): claim mentions a PAIR (2+) of a single
	// EntityType's lifecycle.states, by original graph state name. A lone
	// state-name hit is not a "pair" per the contract's own wording, so it
	// does not qualify this requirement for the state/transition atom (it
	// may still qualify via a different row, or fall through honestly).
	for _, em := range entityModels {
		var matchedStates []stateModel
		for _, s := range em.states {
			if wholeWordMatch(req.Claim, s.src.Name) {
				matchedStates = append(matchedStates, s)
			}
		}
		if len(matchedStates) >= 2 {
			m.statePair = &statePairAtom{entity: em, states: matchedStates}
			break
		}
	}
	if m.statePair != nil {
		m.kind = atomKindStatePair
		return m
	}

	// Row 3 (contract §3): a literal meta-token (MUST/ALWAYS/NEVER/ONLY/ANY/
	// MUST NOT) AND a typed anchor (an id-shaped token, or a literal
	// EntityType.slug) both present in the claim. An id-shaped anchor alone
	// is not enough to call this "gate/order" honestly (GEN-CODE-CONTRACT.md
	// §0 mirror principle) — the generator independently re-finds where
	// (if anywhere) each anchor correlates elsewhere in the domain graph
	// (resolveGateAnchorCorrelate below) BEFORE deciding the classification,
	// so the eventual rendered assertion checks a real graph fact, never a
	// self-authored literal the regex itself produced.
	hasMeta := metaTokenPattern.MatchString(req.Claim)
	var anchors []string
	for _, tok := range idAnchorPattern.FindAllString(req.Claim, -1) {
		anchors = append(anchors, tok)
	}
	for _, em := range entityModels {
		if wholeWordMatch(req.Claim, em.src.Slug) {
			anchors = append(anchors, em.src.Slug)
		}
	}
	anchors = dedupeSorted(anchors)
	if hasMeta && len(anchors) > 0 {
		var correlates []gateAnchorCorrelate
		for _, a := range anchors {
			correlates = append(correlates, resolveGateAnchorCorrelate(a, req.ID, entityModels, otherSettled))
		}
		gate := gateAtom{anchors: anchors, correlates: correlates}
		if gate.hasStructuralCorrelate() {
			// At least one anchor independently resolves to a runtime-
			// comparable graph correlate (a lifecycle.state.name or another
			// requirement's id) — classification stands, and the rendered
			// assertion (requirements_test_gen.go) mirrors THAT correlate,
			// not the anchor list alone.
			m.kind = atomKindGate
			m.gate = gate
			return m
		}
		// Every anchor either found nothing anywhere else in the graph, or
		// found only a why-text (Cyrillic, non-runtime-comparable) hit.
		// Imitating a structural check here would be exactly the vacuous
		// self-check this classification exists to avoid — fall through to
		// row 5's honest gap instead (contract §3 closing row). The why-only
		// correlate (if any) is still worth recording for the audit file, so
		// it is kept on m.gate even though m.kind will not be atomKindGate.
		m.gate = gate
	}

	// Row 4 (contract §3): claim literally names 2+ distinct EntityType
	// slugs that both resolve in this domain's graph — an inter-entity
	// invariant. (Anchors collected above already include any literal slug
	// hits regardless of the meta-token gate above, so reuse that scan
	// restricted to slug-only hits.) Kept as *entityModel (not raw slug
	// strings) so the renderer emits the already-resolved struct/constructor
	// identifiers, never re-deriving or re-embedding the graph-name slug
	// (which may contain hyphens - not a valid bare Go identifier) itself.
	var slugHits []*entityModel
	for _, em := range entityModels {
		if wholeWordMatch(req.Claim, em.src.Slug) {
			slugHits = append(slugHits, em)
		}
	}
	slugHits = dedupeEntityModels(slugHits)
	if len(slugHits) >= 2 {
		m.kind = atomKindInterEntity
		m.interEntity = slugHits
		return m
	}

	// Row 5 (contract §3): no structural carrier found — honest gap.
	m.kind = atomKindNone
	return m
}

// fieldMatchCandidate is one entity/field pair whose claim match was found
// during resolveScopedFieldMatches' first pass, plus enough detail to decide
// whether it needs referencer-scoping (rawHit) before it is accepted.
type fieldMatchCandidate struct {
	entity *entityModel
	field  fieldModel
	// rawHit is true when termMatch(claim, field.src.Name) itself matched
	// (the raw, un-translated graph name) — a raw-name hit is never
	// ambiguous the way a translated-only hit can be (the raw graph name is
	// this EntityType's own authored spelling, not a derived English word
	// that another EntityType's own different raw name can coincidentally
	// translate to as well), so it is always kept unconditionally.
	rawHit bool
	// translatedWord is the single lowercased word f.fieldName reduces to
	// when it is exactly one word (both by the space/underscore/hyphen split
	// and by the camelCase split — see termMatch), or "" when fieldName is
	// multi-word (e.g. "FeatureLead") or the match was a rawHit. Only
	// single-word translated identifiers are short/coincidental enough to
	// need cross-entity ambiguity detection (contract task brief: multi-word
	// phrases like "Feature Lead" are deliberately left untouched).
	translatedWord string
}

// resolveScopedFieldMatches implements contract §3 row 1's field-atom match
// PLUS a referencer-scoping guard for the one case found to over-match on
// the real prat domain: a claim matched an EntityType.field ONLY via that
// field's translated single-word Go identifier (never via the field's raw
// graph name), and that SAME translated word is also the translation of a
// DIFFERENT EntityType's OWN field elsewhere in the domain (prat:
// brd-package.прогноз and sdr-package.прогноз both translate to "Forecast").
// In that situation a claim substring like "forecast_v3" matches both
// EntityTypes' Forecast field identically, even though the claim (contract
// §2.1's precise-state idea: P-G4/SDR names forecast_v3, P-G3/BRD names
// forecast_v2) is really only about ONE of them.
//
// Resolution mirrors resolvePreciseGateState's (pipeline.go) referencer-
// binding idea, applied to field atoms instead of pipeline gates: for each
// ambiguous EntityType candidate, an extra signal independently ties the
// claim to THAT SPECIFIC EntityType, not just to the coincidentally-shared
// translated word:
//
//  1. entity-slug signal: the claim also whole-word-matches this EntityType's
//     own graph slug (e.g. a claim naming "sdr-package" or "brd-package"
//     directly) - the same slug-hit rule row 4's inter-entity atom already
//     uses.
//  2. sibling-field signal: the claim also termMatch-hits a DIFFERENT
//     required field of this SAME EntityType (raw name or translated name) -
//     evidence the claim is describing this EntityType's own artifact as a
//     whole, not just borrowing one coincidentally-shared word.
//  3. why-token signal: this EntityType's own why text contains a token that
//     the claim ALSO contains, immediately adjacent to the ambiguous
//     translated word's lowercase form in the claim (e.g. why says
//     "forecast_v3" and the claim also says "forecast_v3", but a sibling
//     EntityType's why says "forecast_v2" instead) - the same
//     "referencer.src.Why quotes the concrete token" idea
//     resolvePreciseGateState (pipeline.go, contract §2.1) already applies
//     to pipeline gates, reused here for field atoms.
//
// If exactly one ambiguous candidate resolves via any of these signals, only
// that candidate's field atom is kept. If zero or more than one resolve
// (genuine ambiguity, or no candidate has any binding signal at all), NONE
// of the ambiguous candidates for that translated word are kept - an honest
// gap is preferred over a false-positive cross-entity atom (contract §0
// "явно и громко отказать", never silently guess). Non-ambiguous words
// (raw-name hits, and translated words unique to one EntityType in the
// domain) are entirely unaffected.
//
// gates (task #209) is the domain's already-built pipeline gate models,
// passed through unchanged to newFieldAtom so every produced fieldAtom
// carries its own already-built pipeline gate (if any) without re-deriving
// the referencer/ref_target resolution BuildPipelineGateModels already
// performed.
func resolveScopedFieldMatches(claim string, entityModels []*entityModel, gates []*pipelineGateModel) []fieldAtom {
	// Gate-token guard (task #218, the second real over-match found on prat
	// after #208's translated-single-word one): a field NAMED like a gate/
	// stage token (risk-registry.p_g3 — kind:reference "which brd-package
	// this registry's P-G3 review row points at") raw-name-matches the
	// literal gate mention "P-G3" in EVERY claim that talks about gate P-G3
	// at all (termMatch splits "p_g3" -> [p g3] and the claim's hyphen-joined
	// "P-G3" is the same word sequence). Row 1's priority over row 3 then
	// hijacks such requirements into a field atom on a foreign EntityType
	// (prat: R-brd-integrity-zero-blockers, a claim about brd-package's
	// integrity audit, was enforced as "RiskRegistry.PG3 is not empty").
	//
	// Detection (approach (a) of the task brief): blank every gate-token-
	// SHAPED anchor (gateAnchorTokenPattern — the narrow "P-G3"/"E-G1" shape,
	// not the generic idAnchorPattern) out of the claim; a candidate whose
	// raw AND translated matches both disappear on the blanked claim matched
	// ONLY inside gate mentions. Resolution (approach (b), mirroring #208's
	// referencer-scoping): such a candidate is kept only when an independent
	// binding signal ties the claim to this candidate's OWN EntityType
	// (gateTokenOnlyFieldBindingSignal below); with no signal it is dropped,
	// so the requirement falls through to row 3's gate-correlate path — the
	// honest carrier for a claim that references the GATE, not the field.
	// Both halves are combined because neither alone is safe: dropping every
	// gate-shaped match (pure (a)) would strip risk-registry.p_g3/p_g4 from
	// R-risk-review-cadence, whose claim ("Реестр рисков MUST быть
	// пересмотрен ... на P-G3 ... и на P-G4") IS about those exact registry
	// review-row fields (it also names the registry's own lifecycle state
	// "пересмотрен" — the binding signal that keeps them); keeping them by
	// referencer-scoping alone (pure (b)) would leave non-gate-shaped raw
	// hits subject to a guard they never needed.
	blankedClaim := claim
	if gateAnchorTokenPattern.MatchString(claim) {
		blankedClaim = gateAnchorTokenPattern.ReplaceAllString(claim, " ")
	}

	var candidates []fieldMatchCandidate
	for _, em := range entityModels {
		for _, f := range em.fields {
			rawHit := termMatch(claim, f.src.Name)
			translatedHit := termMatch(claim, f.fieldName)
			if !rawHit && !translatedHit {
				continue
			}
			if blankedClaim != claim && !termMatch(blankedClaim, f.src.Name) && !termMatch(blankedClaim, f.fieldName) {
				// Every hit for this field lies INSIDE a gate-token-shaped
				// anchor of the claim (see guard comment above) — keep it only
				// with an independent binding signal to this EntityType.
				if !gateTokenOnlyFieldBindingSignal(claim, blankedClaim, em, f) {
					continue
				}
			}
			cand := fieldMatchCandidate{entity: em, field: f, rawHit: rawHit}
			if !rawHit && translatedHit {
				if w, ok := singleTranslatedWord(f.fieldName); ok {
					cand.translatedWord = w
				}
			}
			candidates = append(candidates, cand)
		}
	}

	// Group every single-word-translated-only candidate by its translated
	// word, scanning ALL entityModels' fields (not just candidates) so a
	// word's ambiguity is judged against the WHOLE domain — the same
	// "search the whole graph, not just this claim's own hits" discipline
	// resolveGateAnchorCorrelate already applies.
	wordOwners := make(map[string]map[string]struct{}) // translatedWord -> set of entity struct names that HAVE such a field anywhere in the domain
	for _, em := range entityModels {
		for _, f := range em.fields {
			if w, ok := singleTranslatedWord(f.fieldName); ok {
				if wordOwners[w] == nil {
					wordOwners[w] = make(map[string]struct{})
				}
				wordOwners[w][em.structName] = struct{}{}
			}
		}
	}

	var out []fieldAtom
	for _, cand := range candidates {
		if cand.translatedWord == "" {
			// rawHit, or a multi-word (or non-word-shaped) translated
			// identifier — never subject to this scoping guard.
			out = append(out, newFieldAtom(cand.entity, cand.field, gates))
			continue
		}
		if len(wordOwners[cand.translatedWord]) < 2 {
			// This translated word is not shared by any other EntityType in
			// the domain — unambiguous, kept as before.
			out = append(out, newFieldAtom(cand.entity, cand.field, gates))
			continue
		}
		if entityHasClaimBindingSignal(claim, cand.entity, cand.translatedWord) {
			out = append(out, newFieldAtom(cand.entity, cand.field, gates))
		}
		// No binding signal for this ambiguous candidate: dropped silently
		// here: if NO candidate for this word resolves, the word simply
		// contributes zero field atoms (honest gap), exactly the intended
		// outcome — never partially kept without a real signal.
	}
	return out
}

// newFieldAtom builds one fieldAtom for entity/field, looking up (task #209)
// whether pipeline.go's BuildPipelineGateModels already built a pipeline
// gate for this exact referencer+field pair — set only when field is a
// kind:reference field whose ref_target resolves to another EntityType of
// this domain (findPipelineGate, pipeline.go). Non-reference fields (and
// reference fields with no resolvable ref_target) simply carry a nil
// pipelineGate, unchanged from pre-task-#209 behavior.
func newFieldAtom(entity *entityModel, field fieldModel, gates []*pipelineGateModel) fieldAtom {
	return fieldAtom{
		entity:       entity,
		field:        field,
		pipelineGate: findPipelineGate(gates, entity, field.fieldName),
	}
}

// singleTranslatedWord reports whether term (a translated Go identifier,
// e.g. fieldModel.fieldName) reduces to exactly one word under BOTH
// termMatch's separator split and its camelCase split (or is already a bare
// single token with no camelCase split at all) — the same "single word,
// short and more coincidence-prone" shape termMatch's own doc comment
// distinguishes from a multi-word phrase like "FeatureLead". Returns the
// lowercased word and true when so; ("", false) for multi-word identifiers.
func singleTranslatedWord(term string) (string, bool) {
	words := splitTermWords(term)
	if len(words) != 1 {
		return "", false
	}
	camelWords := splitCamelWords(term)
	if len(camelWords) >= 2 && !sameWords(camelWords, words) {
		// e.g. "FeatureLead" splits by separator into one token
		// ("featurelead") but by camelCase into two ("feature","lead") —
		// that is contract §3.1's actual multi-word gap, not this
		// single-word ambiguity guard's target.
		return "", false
	}
	return words[0], true
}

// entityHasClaimBindingSignal reports whether claim independently ties
// itself to entity specifically, beyond the coincidentally-shared
// translatedWord match alone — see resolveScopedFieldMatches' doc comment
// for the three signals tried (entity slug, sibling required field, why-
// token adjacency).
func entityHasClaimBindingSignal(claim string, entity *entityModel, translatedWord string) bool {
	// Signal 1: claim names this EntityType's own graph slug directly.
	if wholeWordMatch(claim, entity.src.Slug) {
		return true
	}

	// Signal 2: claim also matches a DIFFERENT field of this SAME EntityType
	// (raw name or translated name) - evidence the claim is about this
	// EntityType's own artifact, not just the one shared word.
	for _, f := range entity.fields {
		if strings.EqualFold(f.fieldName, translatedWord) {
			continue // the ambiguous field itself, not a sibling
		}
		if termMatch(claim, f.src.Name) || termMatch(claim, f.fieldName) {
			return true
		}
	}

	// Signal 3: this EntityType's own why text contains a token that the
	// claim ALSO contains, immediately adjacent (joined by a separator, no
	// intervening word) to translatedWord's own occurrence in the claim -
	// e.g. why quotes "forecast_v3" and the claim also says "forecast_v3",
	// but a sibling EntityType's why quotes "forecast_v2" instead. This
	// mirrors resolvePreciseGateState's (pipeline.go, contract §2.1) "token
	// survives in referencer.src.Why" discipline, applied to field atoms.
	if entity.src.Why == "" {
		return false
	}
	claimTokens := adjacentTokens(claim, translatedWord)
	whyLower := strings.ToLower(entity.src.Why)
	for _, tok := range claimTokens {
		if strings.Contains(whyLower, strings.ToLower(tok)) {
			return true
		}
	}
	return false
}

// gateTokenOnlyFieldBindingSignal reports whether claim independently ties
// itself to entity — the owner of a field whose ONLY claim match sits inside
// gate-token-shaped anchors (see resolveScopedFieldMatches' gate-token
// guard, task #218) — beyond that shared gate token itself. Three signals,
// the same referencer-binding discipline entityHasClaimBindingSignal (#208)
// and resolvePreciseGateState (pipeline.go, contract §2.1) already apply:
//
//  1. entity-slug signal: the claim whole-word-names this EntityType's own
//     graph slug (checked on the ORIGINAL claim — slugs are lowercase kebab
//     and never gate-token-shaped, so blanking cannot have eaten them).
//  2. sibling-field signal: the claim ALSO matches a different field of this
//     SAME EntityType — checked on the BLANKED claim, so a sibling that
//     itself only matches via another gate token (prat: p_g4 next to p_g3)
//     is not circular "evidence" that the claim is about this EntityType.
//  3. lifecycle-state signal: the claim whole-word-names one of this
//     EntityType's own lifecycle.state names (original claim — a state name
//     is this EntityType's own authored vocabulary, the same "claim speaks
//     this entity's language" evidence a sibling field gives; prat:
//     R-risk-review-cadence's "пересмотрен" is risk-registry's own state,
//     binding its p_g3/p_g4 review-row fields legitimately).
//
// No signal -> false -> the candidate is dropped and the requirement falls
// through to row 3's gate-correlate path (an honest gate atom or an honest
// gap — contract §0 "явно и громко отказать", never a cross-entity guess).
func gateTokenOnlyFieldBindingSignal(claim, blankedClaim string, entity *entityModel, field fieldModel) bool {
	// Signal 1: claim names this EntityType's own graph slug directly.
	if wholeWordMatch(claim, entity.src.Slug) {
		return true
	}

	// Signal 2: a sibling field of this SAME EntityType matches the claim
	// OUTSIDE gate-token anchors (blanked claim).
	for _, sib := range entity.fields {
		if sib.fieldName == field.fieldName {
			continue // the gate-shaped field itself, not a sibling
		}
		if termMatch(blankedClaim, sib.src.Name) || termMatch(blankedClaim, sib.fieldName) {
			return true
		}
	}

	// Signal 3: claim names one of this EntityType's own lifecycle states.
	for _, s := range entity.states {
		if wholeWordMatch(claim, s.src.Name) {
			return true
		}
	}
	return false
}

// adjacentTokensPattern captures translatedWord immediately joined (no
// space) by '_'/'-' plus a following alphanumeric run, e.g. "forecast_v3"
// or "forecast-v2" — the same shape resolvePreciseGateState's own
// "<slug>_<state>"/"<slug>-<state>" token construction produces, found here
// in the other direction (extracted FROM the claim, not built and searched
// for).
func adjacentTokens(claim, translatedWord string) []string {
	pattern := `(?i)\b` + regexp.QuoteMeta(translatedWord) + `[_\-][\p{L}\p{N}]+\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re.FindAllString(claim, -1)
}

// resolveGateAnchorCorrelate searches the WHOLE domain (not just the
// EntityTypes/requirement this one claim happens to mention) for a real
// correlate of one gate/order anchor token, per GEN-CODE-CONTRACT.md §3 row
// 3's sub-clauses:
//
//	(a) any EntityType's lifecycle.state.name, anywhere in the domain
//	(b) any EntityType's why text, anywhere in the domain
//	(c) any OTHER SETTLED requirement's id
//
// Matching is case-insensitive substring containment (not whole-word): gate/
// order anchors are short structured tokens (e.g. "P-G3", "P-G1-R") that
// legitimately appear as a PREFIX of a longer state name ("P-G1-R-pass") or
// requirement id ("R-gate-pg1r-risk-registry-mandatory", where "P-G1-R"
// matches after hyphens are stripped) — a whole-word match would miss both
// of those real, independently-verified hits found while diagnosing this
// bug on the real prat domain. Sub-clause (a) is checked first because a
// state-name correlate is preferred (it is the more specific, more
// "structural" of the two runtime-comparable kinds); (b) is checked next
// only to record a textual (non-runtime-comparable) correlate for the audit
// file; (c) last.
func resolveGateAnchorCorrelate(anchor, reqID string, entityModels []*entityModel, otherSettled []ontology.Requirement) gateAnchorCorrelate {
	normAnchor := normalizeAnchorForCorrelation(anchor)

	// (a) lifecycle.state.name of ANY EntityType in the domain.
	for _, em := range entityModels {
		for _, s := range em.states {
			if strings.Contains(normalizeAnchorForCorrelation(s.src.Name), normAnchor) {
				return gateAnchorCorrelate{
					anchor:      anchor,
					kind:        gateAnchorCorrelateState,
					stateEntity: em,
					state:       s,
				}
			}
		}
	}

	// (b) EntityType.why of ANY EntityType in the domain (textual-only —
	// why is legitimately Cyrillic, contract §1.1, so this correlate is
	// real but cannot itself justify a runtime .go assertion).
	for _, em := range entityModels {
		if em.src.Why != "" && strings.Contains(em.src.Why, anchor) {
			return gateAnchorCorrelate{anchor: anchor, kind: gateAnchorCorrelateWhy}
		}
	}

	// (c) id of any OTHER SETTLED requirement in the domain.
	for _, other := range otherSettled {
		if other.ID == reqID {
			continue
		}
		if strings.Contains(normalizeAnchorForCorrelation(other.ID), normAnchor) {
			return gateAnchorCorrelate{
				anchor:        anchor,
				kind:          gateAnchorCorrelateRequirement,
				requirementID: other.ID,
			}
		}
	}

	return gateAnchorCorrelate{anchor: anchor, kind: gateAnchorCorrelateNone}
}

// normalizeAnchorForCorrelation lowercases and strips hyphens, so an anchor
// like "P-G1-R" matches both a hyphen-preserving state name ("P-G1-R-pass")
// and a fully hyphenated-then-lowercased requirement id fragment
// ("...-pg1r-..." inside "R-gate-pg1r-risk-registry-mandatory") — the same
// token spelled with or without its internal hyphens in different graph
// authoring conventions is still the same anchor.
func normalizeAnchorForCorrelation(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "-", ""))
}

// DuplicateRequirementFuncNameError is returned when two SETTLED
// requirements resolve to the same Go test function name (only possible if
// two requirement ids differ only by a character requirementFuncNameBody
// discards, which the framework's id convention does not currently permit,
// but this generator refuses loudly rather than silently emit two
// colliding func declarations — contract §0's "refuse loudly, never guess").
type DuplicateRequirementFuncNameError struct {
	FuncName string
	First    string
	Second   string
}

func (e *DuplicateRequirementFuncNameError) Error() string {
	return fmt.Sprintf("gocode: requirements %q and %q both resolve to Go test function %q", e.First, e.Second, e.FuncName)
}

// BuildRequirementModels resolves every SETTLED requirement in reqs against
// entityModels (see BuildRequirementModel), sorted by requirement id for
// determinism (contract §5), and refuses if two requirements collide on the
// same generated Go function name.
//
// gates (task #209) is the domain's already-built pipeline gate models
// (pipeline.go's BuildPipelineGateModels) — threaded into every
// BuildRequirementModel call so field atoms on kind:reference fields can
// find and reuse their own already-built pipeline gate. Pass nil when no
// pipeline gates are available (isolated unit fixtures, or a caller that
// only needs stage-4 output on its own).
func BuildRequirementModels(reqs []ontology.Requirement, entityModels []*entityModel, gates []*pipelineGateModel) ([]*requirementModel, error) {
	var settled []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
	}
	sort.Slice(settled, func(i, j int) bool { return settled[i].ID < settled[j].ID })

	byFuncName := make(map[string]string, len(settled))
	models := make([]*requirementModel, 0, len(settled))
	for _, r := range settled {
		// otherSettled is the full SETTLED corpus (resolveGateAnchorCorrelate
		// skips r.ID itself when matching, so passing the whole slice — not
		// a per-requirement filtered copy — avoids an O(n^2) slice-build
		// here while still correctly excluding self-matches inside the
		// correlate resolver).
		m := BuildRequirementModel(r, entityModels, settled, gates)
		if prior, dup := byFuncName[m.funcName]; dup {
			return nil, &DuplicateRequirementFuncNameError{FuncName: m.funcName, First: prior, Second: r.ID}
		}
		byFuncName[m.funcName] = r.ID
		models = append(models, m)
	}
	return models, nil
}

// wholeWordMatch reports whether term appears in claim as a whole word:
// bounded by string edges or a non-letter/non-digit rune on both sides.
// Unicode-aware (via \p{L}/\p{N}) so it works identically for Cyrillic
// field/state names and ASCII ones (e.g. "dor", "cosmic") — the same
// original graph-name spelling BuildEntityModel resolved into Go
// identifiers, never a re-translated form.
func wholeWordMatch(claim, term string) bool {
	if term == "" {
		return false
	}
	pattern := `(^|[^\p{L}\p{N}])` + regexp.QuoteMeta(term) + `($|[^\p{L}\p{N}])`
	re, err := regexp.Compile(`(?i)` + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(claim)
}

// termWordSplitPattern splits a graph name or claim phrase into "words" on
// the three separator kinds contract §3.1's field-atom gap crosses: space,
// underscore, hyphen. A run of one or more of these separators is a single
// boundary (so "feature__lead" and "feature - lead" both split the same as
// "feature_lead"). Used only by termMatch below — wholeWordMatch above is
// deliberately left untouched (rows 2-4 of BuildRequirementModel keep the
// narrower single-token match; broadening THEIR matching was not the bug
// found on prat and risks new false positives on short state/anchor tokens).
var termWordSplitPattern = regexp.MustCompile(`[ _\-]+`)

// splitTermWords lowercases and splits s into its non-empty word parts on
// space/underscore/hyphen, for termMatch's word-sequence comparison.
func splitTermWords(s string) []string {
	fields := termWordSplitPattern.Split(strings.ToLower(strings.TrimSpace(s)), -1)
	out := fields[:0]
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// termMatch reports whether term (a graph-name spelling — raw underscore/
// hyphen-joined field/state name, or a translated PascalCase identifier)
// appears in claim as the SAME sequence of words, in the same order,
// regardless of which separator (space, underscore, hyphen) either side
// uses, and regardless of case. This generalizes wholeWordMatch's
// single-token whole-word match to multi-word phrases:
//
//   - single-word term: behaves exactly like wholeWordMatch (same
//     word-boundary semantics — no broadening of the short-token case
//     wholeWordMatch already covers, e.g. "us"/"ac"/"dor").
//   - multi-word term (2+ words, contract §3.1's actual gap): every word
//     must appear in claim, consecutively, in the declared order, each
//     itself bounded by a non-letter/non-digit rune — so a claim phrase
//     spelled "Feature Lead" (space-joined) matches a graph field name
//     spelled "feature_lead" (underscore-joined), and a PascalCase
//     translation "FeatureLead" is split into ["feature","lead"] by
//     splitCamelWords below before the same word-sequence comparison runs.
//
// This is deliberately NOT "all words present anywhere in the claim in any
// order" — that would match unrelated two-word coincidences (contract §3.1
// warns explicitly about false-positive risk); requiring the SAME order,
// consecutively, keeps the match tied to an actual phrase, not a bag of
// words.
func termMatch(claim, term string) bool {
	if term == "" {
		return false
	}
	words := splitTermWords(term)
	if len(words) == 0 {
		return false
	}

	// term's PascalCase/camelCase word-split (e.g. "FeatureLead" ->
	// ["feature","lead"]), tried IN ADDITION to the space/underscore/hyphen
	// split above, in case the caller passed a translated Go identifier
	// (fieldModel.fieldName) rather than the raw graph name — a single
	// "word" by the separator split can still carry multiple words once its
	// internal casing is considered.
	camelWords := splitCamelWords(term)

	if len(words) == 1 && (len(camelWords) < 2 || sameWords(camelWords, words)) {
		// No separator-based split AND no distinct camelCase split: the
		// pre-existing single-token whole-word rule, unchanged (the case
		// wholeWordMatch already handles correctly, e.g. short ASCII tokens
		// like "dor"/"us"/"ac" — no broadening here).
		return wholeWordMatch(claim, term)
	}

	if wordSequenceMatch(claim, words) {
		return true
	}
	if len(camelWords) >= 2 && !sameWords(camelWords, words) {
		return wordSequenceMatch(claim, camelWords)
	}
	return false
}

// wordSequenceMatch reports whether claim contains words (already
// lowercased) as a consecutive, in-order sequence, joined by any run of
// space/underscore/hyphen, each end bounded by a non-letter/non-digit rune
// (or a string edge) — the core multi-word rule termMatch applies to both
// the separator-based split and the camelCase-based split of term.
func wordSequenceMatch(claim string, words []string) bool {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = regexp.QuoteMeta(w)
	}
	pattern := `(?i)(^|[^\p{L}\p{N}])` + strings.Join(quoted, `[ _\-]+`) + `($|[^\p{L}\p{N}])`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(claim)
}

// splitCamelWords splits a PascalCase/camelCase Go identifier (e.g.
// "FeatureLead") into lowercased words (["feature","lead"]) by breaking
// before each uppercase rune that follows a lowercase/digit rune. Runs of
// uppercase letters (e.g. an acronym like "SA" or "DoR" already embedded in
// an identifier) are kept together as one word rather than split
// letter-by-letter.
func splitCamelWords(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
			if len(cur) > 0 {
				words = append(words, strings.ToLower(string(cur)))
			}
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		words = append(words, strings.ToLower(string(cur)))
	}
	return words
}

// sameWords reports whether a and b contain the same words in the same
// order (used by termMatch to skip a redundant second regex pass when the
// PascalCase split produced the identical word sequence as the raw-name
// split).
func sameWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// dedupeEntityModels removes duplicate *entityModel entries (by struct
// name) and sorts the result by struct name, for the same determinism
// reason dedupeSorted exists for plain string anchors.
func dedupeEntityModels(in []*entityModel) []*entityModel {
	seen := make(map[string]struct{}, len(in))
	out := make([]*entityModel, 0, len(in))
	for _, em := range in {
		if _, ok := seen[em.structName]; ok {
			continue
		}
		seen[em.structName] = struct{}{}
		out = append(out, em)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].structName < out[j].structName })
	return out
}

func dedupeSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
