// coverage.go implements GEN-CODE-CONTRACT.md §3.1's mandatory coverage-
// completeness audit: "atom found" for a requirement is not the same claim
// as "claim fully covered". For every SETTLED requirement, this file
// extracts a deterministic set of candidate terms from the claim text (never
// LLM-guessed — contract §0/§3's own closing paragraph) and resolves each
// one against the WHOLE domain graph (every EntityType's fields/states, not
// only the ones that already produced an atom for THIS requirement, and
// every other SETTLED requirement's id) using the SAME termMatch/
// wholeWordMatch infrastructure requirements.go's atom classification
// already relies on — no second, parallel matching system.
//
// The result (coverageReport) is threaded into audit.go's per-requirement
// section as an additional "Coverage: N/M candidate terms resolved" line,
// with every unresolved candidate listed and flagged as either a plausible
// graph-concept gap (matches some OTHER EntityType's field/state the claim's
// candidate looks like, by termMatch, but that EntityType is not structurally
// tied to this requirement) or a term with no graph correlate at all (out of
// model).
package gocode

import (
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// capitalizedTokenPattern matches a run of Latin letters/digits that starts
// with an uppercase letter and contains at least one more uppercase letter OR
// is short enough to plausibly be an abbreviation (e.g. "SA", "DoR",
// "COSMIC", "US", "AC", "UC"). It deliberately does not require the WHOLE
// token to be uppercase (DoR mixes case) — the shared shape across contract
// §3.1's own examples is "starts uppercase, reads as an acronym/proper noun
// in claim prose", not "all-caps". Ordinary capitalized English sentence
// words (a claim's leading "The", "Feature", etc.) are excluded downstream by
// the meta-token/stopword filters, not by this pattern itself (over-broad
// capture here is fine — resolution against the graph is what decides
// whether a candidate is meaningful, per this file's own "resolves or
// doesn't" design, contract §0's mirror principle: refuse to silently
// pre-judge which candidates matter).
// Length is constrained to 2+ characters: a lone capitalized letter (e.g.
// the "P" in "Gate P-G3", the "B"/"C" in "Ось A/B/C", the "N"/"M" in "N:M")
// is never itself an abbreviation in the sense contract §3.1's own examples
// give (SA, DoR, COSMIC, US, AC, UC are all 2+ characters) — a single letter
// carries no meaning to resolve against the graph and would only ever
// spuriously "resolve" via accidental substring containment against
// unrelated requirement ids, which is exactly the dishonest-resolution
// failure mode this file exists to avoid (contract §0 mirror principle).
var capitalizedTokenPattern = regexp.MustCompile(`\b[A-Z][A-Za-z]+\b`)

// quotedTermPattern matches a double-quoted or backtick-delimited span in
// claim text (contract §3.1: "квотированные/бэктик-выделенные термины, если
// такой синтаксис встречается в claim"). Not currently exercised by any real
// prat claim (verified by the real-domain coverage test), but the mechanism
// must exist per the task brief.
var quotedTermPattern = regexp.MustCompile("\"([^\"]+)\"|`([^`]+)`")

// coverageStopWords are common English/claim-prose capitalized words that
// capitalizedTokenPattern's shape would otherwise catch (sentence-initial
// capitalization, common nouns/pronouns that begin an English clause embedded
// in an otherwise-Cyrillic claim) but that are not graph terms and would
// never meaningfully resolve — contract §3.1 asks for "candidate terms",
// not literally every capitalized word including grammatical filler. This is
// NOT the same list as the reserved meta-tokens (metaTokenPattern) — those
// are excluded separately and explicitly per the task brief; this is a
// narrower, purely-grammatical stopword set for common sentence-shape words
// that would otherwise flood every report with unresolvable noise.
var coverageStopWords = map[string]struct{}{
	"The": {}, "A": {}, "An": {}, "Is": {}, "Are": {}, "Be": {}, "To": {},
	"Of": {}, "In": {}, "On": {}, "At": {}, "For": {}, "And": {}, "Or": {},
	"With": {}, "Before": {}, "After": {}, "Sub": {},
}

// candidateTermKind classifies HOW a coverage candidate term was extracted
// from claim text (contract §3.1's three extraction rules), for audit
// rendering only — resolution logic does not depend on it.
type candidateTermKind int

const (
	candidateKindCapitalized candidateTermKind = iota
	candidateKindGraphName
	candidateKindQuoted
)

// candidateResolutionKind classifies WHERE (if anywhere) a coverage
// candidate resolves in the domain graph.
type candidateResolutionKind int

const (
	// candidateUnresolved means the candidate matches no field, state,
	// entity slug, or other requirement id anywhere in the domain.
	candidateUnresolved candidateResolutionKind = iota
	// candidateResolvedAtom means the candidate resolves to a graph element
	// that IS already one of this requirement's own atoms (field/state/
	// entity/gate-correlate) — fully covered, not a gap.
	candidateResolvedAtom
	// candidateResolvedElsewhere means the candidate resolves to a real
	// graph element (a field/state of an EntityType, or another
	// requirement's id) that this requirement's classification did NOT
	// capture as one of its own atoms — contract §3.1's "partial coverage
	// gap": looks like a graph concept, but not mirrored into this
	// requirement's atoms.
	candidateResolvedElsewhere
)

// candidateTerm is one extracted-and-resolved coverage candidate.
type candidateTerm struct {
	text       string
	kind       candidateTermKind
	resolution candidateResolutionKind
	// resolvedEntity/resolvedField/resolvedState/resolvedRequirementID
	// describe WHERE candidateResolvedElsewhere (or candidateResolvedAtom)
	// found its match, for the audit line's "-> where" detail. At most one
	// of resolvedField/resolvedState/resolvedRequirementID is set alongside
	// resolvedEntity (field and state are mutually exclusive; a
	// requirement-id correlate sets only resolvedRequirementID).
	resolvedEntity        *entityModel
	resolvedField         *fieldModel
	resolvedState         *stateModel
	resolvedRequirementID string
}

// coverageReport is one SETTLED requirement's full §3.1 coverage-
// completeness result: every extracted candidate term, in deterministic
// order, each tagged with its resolution.
type coverageReport struct {
	candidates []candidateTerm
}

// resolvedCount reports how many of the report's candidates resolved to
// SOME graph element (atom of this requirement OR elsewhere) — the "N" in
// "Coverage: N/M candidate terms resolved".
func (c coverageReport) resolvedCount() int {
	n := 0
	for _, cand := range c.candidates {
		if cand.resolution != candidateUnresolved {
			n++
		}
	}
	return n
}

// gaps returns every candidate that either did not resolve at all, or
// resolved to a DIFFERENT graph element than one of this requirement's own
// atoms (contract §3.1's "partial coverage gap") — the list rendered under
// "Coverage: N/M ..." in requirements_audit.md.
func (c coverageReport) gaps() []candidateTerm {
	var out []candidateTerm
	for _, cand := range c.candidates {
		if cand.resolution != candidateResolvedAtom {
			out = append(out, cand)
		}
	}
	return out
}

// atomEntityFieldSet/atomEntityStateSet build lookup sets of the
// requirementModel's OWN already-classified atoms (by entity struct name +
// field/state identifier), so BuildCoverageReport can tell "resolves to an
// atom THIS requirement already has" (candidateResolvedAtom) apart from
// "resolves to a DIFFERENT graph element" (candidateResolvedElsewhere,
// contract §3.1's actual gap signal) without re-deriving the classification
// a second time.
func (m *requirementModel) atomEntityFieldKeys() map[string]struct{} {
	out := make(map[string]struct{})
	for _, fa := range m.fields {
		out[fa.entity.structName+"."+fa.field.fieldName] = struct{}{}
	}
	return out
}

func (m *requirementModel) atomEntityStateKeys() map[string]struct{} {
	out := make(map[string]struct{})
	if m.statePair != nil {
		for _, s := range m.statePair.states {
			out[m.statePair.entity.structName+"."+s.constant] = struct{}{}
		}
	}
	for _, c := range m.gate.correlates {
		if c.kind == gateAnchorCorrelateState {
			out[c.stateEntity.structName+"."+c.state.constant] = struct{}{}
		}
	}
	return out
}

func (m *requirementModel) atomEntitySlugKeys() map[string]struct{} {
	out := make(map[string]struct{})
	for _, em := range m.interEntity {
		out[em.structName] = struct{}{}
	}
	if m.kind == atomKindField {
		for _, fa := range m.fields {
			out[fa.entity.structName] = struct{}{}
		}
	}
	return out
}

func (m *requirementModel) atomRequirementIDKeys() map[string]struct{} {
	out := make(map[string]struct{})
	for _, c := range m.gate.correlates {
		if c.kind == gateAnchorCorrelateRequirement {
			out[c.requirementID] = struct{}{}
		}
	}
	return out
}

// extractCapitalizedCandidates implements contract §3.1's first extraction
// rule: capitalized Latin tokens/abbreviations in claim text, excluding the
// reserved meta-language tokens (MUST/ALWAYS/NEVER/ONLY/ANY/MUST NOT — these
// are not graph terms, they drive gate/order classification instead,
// requirements.go's metaTokenPattern) and a narrow grammatical stopword list
// (coverageStopWords) for common English sentence-shape words that would
// otherwise flood the report with unresolvable noise. Deduplicated and
// sorted for determinism (contract §5).
func extractCapitalizedCandidates(claim string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, tok := range capitalizedTokenPattern.FindAllString(claim, -1) {
		if metaTokenPattern.MatchString(tok) {
			continue
		}
		if _, stop := coverageStopWords[tok]; stop {
			continue
		}
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

// extractQuotedCandidates implements contract §3.1's third extraction rule:
// quoted/backtick-delimited spans in claim text. Deduplicated and sorted for
// determinism.
func extractQuotedCandidates(claim string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, m := range quotedTermPattern.FindAllStringSubmatch(claim, -1) {
		text := m[1]
		if text == "" {
			text = m[2]
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if _, dup := seen[text]; dup {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
	}
	sort.Strings(out)
	return out
}

// graphNameCandidate is one EntityType field/state name (raw graph spelling
// PLUS its translated Go identifier) that termMatch/wholeWordMatch found
// present in claim text, from ANY EntityType in the domain — contract §3.1's
// second extraction rule: "переведённые имена полей/состояний ЛЮБОГО
// EntityType домена (не только тех, что уже попали в атомы ЭТОГО
// требования)".
//
// ambiguous marks a single-word-translated field candidate whose translated
// word is shared by 2+ EntityTypes in the domain (the exact shape task #208/
// resolveScopedFieldMatches guards against for atom classification) AND for
// which entityHasClaimBindingSignal found no independent binding signal
// tying the claim to THIS specific candidate's entity. Per the task brief's
// explicit instruction ("если термин single-word и неоднозначен между
// несколькими EntityType без сигнала привязки, это тоже honest gap, не
// наврать про resolution"), such a candidate must be reported unresolved,
// never silently resolved to one arbitrary entity out of the ambiguous set.
type graphNameCandidate struct {
	displayText string
	entity      *entityModel
	field       *fieldModel
	state       *stateModel
	ambiguous   bool
}

// extractGraphNameCandidates walks EVERY EntityType's fields and states
// (not just the ones already structurally tied to req by BuildRequirementModel's
// classification) and keeps every field/state whose raw graph name OR
// translated Go identifier termMatch-hits claim text — contract §3.1's
// explicit requirement that this scan cover the WHOLE domain, so a claim
// mentioning a field of an EntityType this requirement's own atoms never
// touched is still surfaced as a candidate (and, if not one of this
// requirement's own atoms, correctly flagged as a partial coverage gap
// below). Single-word-translated field hits shared by 2+ EntityTypes are
// tagged ambiguous (see graphNameCandidate's doc comment) rather than
// resolved to a false-specific entity, reusing the SAME word-ownership scan
// resolveScopedFieldMatches already performs for atom classification (no
// second, independently-maintained ambiguity detector).
func extractGraphNameCandidates(claim string, entityModels []*entityModel) []graphNameCandidate {
	wordOwners := make(map[string]map[string]struct{}) // translatedWord -> entity struct names sharing it
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

	var out []graphNameCandidate
	for _, em := range entityModels {
		for i := range em.fields {
			f := em.fields[i]
			rawHit := termMatch(claim, f.src.Name)
			translatedHit := termMatch(claim, f.fieldName)
			if !rawHit && !translatedHit {
				continue
			}
			gc := graphNameCandidate{
				displayText: em.structName + "." + f.fieldName,
				entity:      em,
				field:       &em.fields[i],
			}
			if !rawHit && translatedHit {
				if w, ok := singleTranslatedWord(f.fieldName); ok && len(wordOwners[w]) >= 2 {
					if !entityHasClaimBindingSignal(claim, em, w) {
						gc.ambiguous = true
					}
				}
			}
			out = append(out, gc)
		}
		for i := range em.states {
			s := em.states[i]
			if wholeWordMatch(claim, s.src.Name) {
				out = append(out, graphNameCandidate{
					displayText: em.structName + "." + s.constant,
					entity:      em,
					state:       &em.states[i],
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].displayText < out[j].displayText })
	return out
}

// BuildCoverageReport implements GEN-CODE-CONTRACT.md §3.1 for one already-
// classified requirementModel: extract every candidate term from its claim
// (capitalized tokens/abbreviations, graph field/state names from the WHOLE
// domain, quoted/backtick spans), resolve each candidate against the graph
// and against m's OWN already-computed atoms, and return the full report.
// entityModels and otherSettled are the same already-built inputs
// BuildRequirementModel used for m — this function never re-derives an
// identifier or re-implements matching, it only re-applies termMatch/
// wholeWordMatch (already used by requirements.go) more broadly across the
// WHOLE domain graph instead of stopping at the first structural hit.
func BuildCoverageReport(m *requirementModel, entityModels []*entityModel, otherSettled []ontology.Requirement) coverageReport {
	claim := m.src.Claim
	atomFields := m.atomEntityFieldKeys()
	atomStates := m.atomEntityStateKeys()
	atomSlugs := m.atomEntitySlugKeys()
	atomReqIDs := m.atomRequirementIDKeys()

	var candidates []candidateTerm

	// Rule 1: capitalized Latin tokens/abbreviations.
	for _, tok := range extractCapitalizedCandidates(claim) {
		candidates = append(candidates, resolveCapitalizedCandidate(tok, entityModels, otherSettled, atomSlugs, atomReqIDs))
	}

	// Rule 2: translated field/state names of ANY EntityType in the domain.
	// Ambiguous candidates (same shared single-word translated term, 2+
	// EntityTypes, no binding signal - see graphNameCandidate's doc comment)
	// are collapsed into ONE reported row keyed by the shared word itself,
	// not one row per candidate entity: reporting "Forecast (plan-package)"
	// AND "Forecast (design-package)" as two separate unresolved rows would
	// imply two independent candidate terms when the claim only contains one
	// ambiguous word.
	seenAmbiguousWords := make(map[string]struct{})
	for _, gc := range extractGraphNameCandidates(claim, entityModels) {
		if gc.ambiguous {
			word, _ := singleTranslatedWord(gc.field.fieldName)
			if _, dup := seenAmbiguousWords[word]; dup {
				continue
			}
			seenAmbiguousWords[word] = struct{}{}
			ct := resolveGraphNameCandidate(gc, atomFields, atomStates)
			ct.text = word
			candidates = append(candidates, ct)
			continue
		}
		candidates = append(candidates, resolveGraphNameCandidate(gc, atomFields, atomStates))
	}

	// Rule 3: quoted/backtick-delimited terms.
	for _, q := range extractQuotedCandidates(claim) {
		candidates = append(candidates, resolveQuotedCandidate(q, entityModels, otherSettled, atomSlugs, atomReqIDs))
	}

	return coverageReport{candidates: candidates}
}

// resolveCapitalizedCandidate resolves one Rule-1 candidate (a capitalized
// Latin token) against: (a) any EntityType's own slug/struct name (a claim
// naming an EntityType directly), (b) any other SETTLED requirement's id,
// (c) any EntityType's field/state translated identifier (a capitalized
// token IS frequently exactly a single-word translated field/state name,
// e.g. "Forecast") — this last check reuses the same graph-name candidate
// resolution so a token like "COSMIC" that happens to also be a field name
// is not reported twice with two different verdicts.
func resolveCapitalizedCandidate(tok string, entityModels []*entityModel, otherSettled []ontology.Requirement, atomSlugs, atomReqIDs map[string]struct{}) candidateTerm {
	ct := candidateTerm{text: tok, kind: candidateKindCapitalized}

	for _, em := range entityModels {
		if strings.EqualFold(em.structName, tok) || wholeWordMatch(tok, em.src.Slug) || strings.EqualFold(em.src.Slug, tok) {
			ct.resolvedEntity = em
			if _, ok := atomSlugs[em.structName]; ok {
				ct.resolution = candidateResolvedAtom
			} else {
				ct.resolution = candidateResolvedElsewhere
			}
			return ct
		}
	}

	for _, em := range entityModels {
		for i := range em.fields {
			f := em.fields[i]
			if strings.EqualFold(f.fieldName, tok) || strings.EqualFold(f.src.Name, tok) {
				ct.resolvedEntity = em
				ct.resolvedField = &em.fields[i]
				if _, ok := atomSlugs[em.structName]; ok {
					ct.resolution = candidateResolvedAtom
				} else {
					ct.resolution = candidateResolvedElsewhere
				}
				return ct
			}
		}
		for i := range em.states {
			s := em.states[i]
			if strings.EqualFold(s.constant, tok) || strings.EqualFold(s.src.Name, tok) {
				ct.resolvedEntity = em
				ct.resolvedState = &em.states[i]
				ct.resolution = candidateResolvedElsewhere
				return ct
			}
		}
	}

	for _, other := range otherSettled {
		if requirementIDWordMatch(other.ID, tok) {
			ct.resolvedRequirementID = other.ID
			if _, ok := atomReqIDs[other.ID]; ok {
				ct.resolution = candidateResolvedAtom
			} else {
				ct.resolution = candidateResolvedElsewhere
			}
			return ct
		}
	}

	ct.resolution = candidateUnresolved
	return ct
}

// requirementIDWordMatch reports whether tok (a coverage candidate — a
// capitalized token or a quoted span) matches one of reqID's OWN
// hyphen-separated words as a whole word, case-insensitively — e.g. "BRD"
// matches "R-brd-integrity-zero-blockers" (word "brd"), "FR" matches
// "R-fr-to-feature-nm-pm-decides" (word "fr"). This is deliberately NOT
// resolveGateAnchorCorrelate's own normalizeAnchorForCorrelation-based
// substring-containment rule (requirements.go): that rule exists for
// id-SHAPED anchors (e.g. "P-G1-R", contract §3 row 3) which legitimately
// appear as fragments inside a longer requirement id after hyphens are
// stripped ("...-pg1r-..."). A short plain-word candidate like "BRD"/"FR"/
// "PM" is a different shape (contract §3.1 rule 1's capitalized-token/
// abbreviation candidates, not row 3's id anchors) — matching it by
// substring containment against a normalized (hyphen-stripped) id would
// spuriously hit almost any sufficiently long id that happens to contain the
// same 2-3 letter run inside an unrelated word, which is exactly the
// dishonest-resolution failure mode this file exists to avoid. Whole-word
// matching against the id's own declared word segments is the same
// discipline wholeWordMatch already applies elsewhere in this package.
func requirementIDWordMatch(reqID, tok string) bool {
	lowerTok := strings.ToLower(tok)
	for _, word := range strings.Split(reqID, "-") {
		if strings.EqualFold(word, lowerTok) {
			return true
		}
	}
	return false
}

// resolveGraphNameCandidate resolves one Rule-2 candidate (already found via
// termMatch/wholeWordMatch against SOME EntityType's field/state, from the
// whole-domain scan in extractGraphNameCandidates) against m's own atom set:
// candidateResolvedAtom if THIS requirement's classification already used
// exactly this entity+field/state, candidateResolvedElsewhere otherwise
// (contract §3.1's partial coverage gap — matched a real graph concept, but
// not mirrored into this requirement's own atoms).
func resolveGraphNameCandidate(gc graphNameCandidate, atomFields, atomStates map[string]struct{}) candidateTerm {
	ct := candidateTerm{
		text:           gc.displayText,
		kind:           candidateKindGraphName,
		resolvedEntity: gc.entity,
		resolvedField:  gc.field,
		resolvedState:  gc.state,
	}
	if gc.field != nil {
		key := gc.entity.structName + "." + gc.field.fieldName
		if _, ok := atomFields[key]; ok {
			ct.resolution = candidateResolvedAtom
			return ct
		}
	}
	if gc.state != nil {
		key := gc.entity.structName + "." + gc.state.constant
		if _, ok := atomStates[key]; ok {
			ct.resolution = candidateResolvedAtom
			return ct
		}
	}
	if gc.ambiguous {
		// Task #208's referencer-scoping finding, extended to coverage: a
		// single-word translated field name shared by 2+ EntityTypes, with
		// no independent binding signal tying the claim to THIS specific
		// candidate's entity, is not an honest "resolves elsewhere" claim —
		// reporting it as candidateResolvedElsewhere would assert a specific
		// entity attribution the claim text does not actually support.
		// Treated as unresolved (contract §3.1's own instruction: "это тоже
		// honest gap, не наврать про resolution").
		ct.resolution = candidateUnresolved
		return ct
	}
	ct.resolution = candidateResolvedElsewhere
	return ct
}

// resolveQuotedCandidate resolves one Rule-3 candidate (a quoted/backtick
// span) with the same graph-wide search resolveCapitalizedCandidate uses —
// quoted text carries no extraction-time assumption about its shape (unlike
// Rule 1's capitalized-token pattern), so it is checked against entity
// slugs/struct names, field/state names, and other requirement ids uniformly.
func resolveQuotedCandidate(text string, entityModels []*entityModel, otherSettled []ontology.Requirement, atomSlugs, atomReqIDs map[string]struct{}) candidateTerm {
	ct := resolveCapitalizedCandidate(text, entityModels, otherSettled, atomSlugs, atomReqIDs)
	ct.text = text
	ct.kind = candidateKindQuoted
	return ct
}
