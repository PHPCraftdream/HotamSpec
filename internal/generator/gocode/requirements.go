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
type fieldAtom struct {
	entity *entityModel
	field  fieldModel
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
func BuildRequirementModel(req ontology.Requirement, entityModels []*entityModel, otherSettled []ontology.Requirement) *requirementModel {
	m := &requirementModel{
		src:        req,
		funcName:   "Test_" + requirementFuncNameBody(req.ID),
		anchorSlug: strings.ToLower(req.ID),
	}

	// Row 1 (contract §3): claim references an EntityType.field by its
	// original graph name, checked as a Unicode-aware whole-word match
	// (works for both Cyrillic field names and ASCII ones like "dor",
	// "cosmic" — the same field names entities.go's struct already carries).
	// Sorted (entity slug, field name) for determinism: the source domain's
	// entityModels are already sorted by slug (GenerateLifecycleFromGraph),
	// and each entity's own field order is graph-declaration order — the
	// walk below preserves both, so no extra sort is needed.
	for _, em := range entityModels {
		for _, f := range em.fields {
			if wholeWordMatch(req.Claim, f.src.Name) {
				m.fields = append(m.fields, fieldAtom{entity: em, field: f})
			}
		}
	}
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
func BuildRequirementModels(reqs []ontology.Requirement, entityModels []*entityModel) ([]*requirementModel, error) {
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
		m := BuildRequirementModel(r, entityModels, settled)
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
