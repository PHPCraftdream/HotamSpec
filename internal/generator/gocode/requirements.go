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

// gateAtom is one "meta-token + typed anchor" hit (contract §3 row 3): the
// literal anchor tokens found (already ASCII by construction — the pattern
// only matches Latin-letter/digit/hyphen shapes), used to name the
// generated sub-test without embedding the (possibly Cyrillic) claim text.
type gateAtom struct {
	anchors []string
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
func BuildRequirementModel(req ontology.Requirement, entityModels []*entityModel) *requirementModel {
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
	// EntityType.slug) both present in the claim.
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
		m.kind = atomKindGate
		m.gate = gateAtom{anchors: anchors}
		return m
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
		m := BuildRequirementModel(r, entityModels)
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
