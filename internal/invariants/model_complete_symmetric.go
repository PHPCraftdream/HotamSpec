// model_complete_symmetric.go holds check_public_surface_linked_or_marked
// (task #396/W1.4): the SYMMETRIC INVERSE of check_model_complete
// (model_complete.go). check_model_complete asks "for every method ALREADY
// cited as implemented_by by a SETTLED requirement, is it scenario-complete?"
// -- a public method/interface-method/constructor no requirement has ever
// cited is completely invisible to that check, so a domain's model can
// silently accumulate uncovered public surface with no gate noticing. This
// check closes that gap: every EXPORTED authored symbol in the domain's
// scanned model inventory -- a receiver method (ModelObject.Methods), an
// interface's own declared method (ModelObject.InterfaceMethods, Name != ""
// entries only -- an Embedded entry is not this object's own declared
// method), or a top-level function/constructor (ModelFile.Funcs) -- must be
// EITHER (a) cited by a SETTLED requirement's implemented_by AND
// scenario-complete (the IDENTICAL bar check_model_complete already
// applies, via the SAME collectCitedSymbols pass), OR (b) EXPLICITLY marked
// infrastructure/ignored with a stated reason (infrastructureOrIgnoredReason,
// a doc-comment reserved-word convention -- see that function's own doc
// comment for why this is NOT a new AST extraction category). No silent
// exemptions: not even for mocks, ports, or constructors -- if a symbol
// legitimately does not need a citation, its doc comment must say so
// explicitly.
//
// OPT-IN TRIGGER (Decision 1, task #396 brief): this check activates SOLELY
// on g.PublicSurfaceAuthorityLinked (loader.PublicSurfaceAuthorityLinked,
// manifest.json's "public_surface_authority": "linked") -- a BRAND NEW,
// wholly INDEPENDENT trigger, never g.Discipline == loader.DisciplineFull.
// check_model_complete and its three siblings (check_settled_requires_
// scenario, check_scenario_executes_impl, check_spec_md_current) plus
// check_discipline_ratchet are ALL already bundled under discipline:"full",
// and TWO REAL CONSUMER DOMAINS (PRAT-hotam/domains/prat, domains/gpsm-sm)
// have ALREADY flipped that flag, consenting only to the obligation set live
// at the time they did -- never to this NEW one. Tacking this check onto
// discipline:"full" would reproduce EXACTLY the live regression task #369
// caused and task #388 fixed (see R-opt-in-trigger-owns-its-own-obligations,
// internal/selfspec/requirements_authoredspec.go): both domains would jump
// from 0 to many violations overnight, since neither has ever cited an
// interface method or constructor (those categories did not exist in the
// scan before task #393) nor added any infrastructure/ignored marker
// anywhere. This check does NOT require g.Discipline == loader.DisciplineFull
// as a co-requirement either (unlike loader.ClaimAuthorityScenario, which
// requires discipline:"full" IN ADDITION TO claim_authority:"scenario") --
// public_surface_authority:"linked" alone is sufficient and necessary.
//
// HONEST NO-OP for every domain today: this wave deliberately does not flip
// public_surface_authority:"linked" on any real domain (domains/hotam-spec-
// self, domains/hotam-dev, or anything under PRAT-hotam) -- the check is a
// proven, honest no-op everywhere it is not explicitly opted into.
package invariants

import (
	"fmt"
	"go/token"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// infrastructureMarker and ignoredMarker are the two reserved-word literals
// infrastructureOrIgnoredReason recognizes -- ordinary prose text embedded
// WITHIN a symbol's doc comment, never a directive-line convention (Decision
// 2, task #396 brief: gate.docText joins a *ast.CommentGroup via
// (*ast.CommentGroup).Text(), which AUTOMATICALLY STRIPS Go-directive-shaped
// lines like "//go:generate" from its output -- a bare "//tool:directive"
// marker would be silently deleted before ModelMethod.Doc/ModelFunc.Doc/
// ModelInterfaceMethod.Doc ever saw it). This mirrors the codebase's own
// established ALL-CAPS-reserved-word-embedded-in-prose convention (CLAUDE.md's
// TRANSLATE step: ALWAYS/NEVER/MUST/etc. as literal reserved tokens inside
// ordinary authored text, not a separate machine-only syntax).
const (
	infrastructureMarker = "INFRASTRUCTURE:"
	ignoredMarker        = "IGNORED:"
)

// infrastructureOrIgnoredReason scans doc (a symbol's already-joined doc
// comment text, e.g. ModelMethod.Doc) for the literal substring
// "INFRASTRUCTURE:" or "IGNORED:" and, when found, returns the trimmed text
// that follows it as reason. marked is true ONLY when a marker is found AND
// the trimmed trailing text is non-empty -- a bare "INFRASTRUCTURE:" with
// nothing after it (or only whitespace) is NOT a real explanation and does
// NOT count as marked (Decision 2: "a reason is required, an empty
// 'INFRASTRUCTURE:' with nothing after it is not a real explanation").
//
// When BOTH markers appear in the same doc string, the one appearing FIRST
// (lowest byte offset) wins -- an arbitrary but deterministic tie-break for
// an authoring edge case that should not occur in practice (a symbol is
// either infrastructure or explicitly ignored, not both).
//
// Marker text embedded mid-sentence is still detected (this is a substring
// search, not a line-anchored match) -- e.g. "Recognize probes the OCR
// service. INFRASTRUCTURE: thin adapter wrapper, no independent behavior to
// prove." marks with reason "thin adapter wrapper, no independent behavior
// to prove."
func infrastructureOrIgnoredReason(doc string) (reason string, marked bool) {
	type candidate struct {
		marker string
		idx    int
	}
	var found []candidate
	if idx := strings.Index(doc, infrastructureMarker); idx >= 0 {
		found = append(found, candidate{infrastructureMarker, idx})
	}
	if idx := strings.Index(doc, ignoredMarker); idx >= 0 {
		found = append(found, candidate{ignoredMarker, idx})
	}
	if len(found) == 0 {
		return "", false
	}
	sort.Slice(found, func(i, j int) bool { return found[i].idx < found[j].idx })
	chosen := found[0]
	trailing := strings.TrimSpace(doc[chosen.idx+len(chosen.marker):])
	if trailing == "" {
		// Empty reason after the marker -- Decision 2 is explicit: this is
		// NOT marked. Do not fall back to a second marker later in the
		// string either (an author who wrote a bare "INFRASTRUCTURE:" made a
		// mistake worth surfacing as a violation, not silently rescued by a
		// coincidental second marker).
		return "", false
	}
	return trailing, true
}

// checkPublicSurfaceLinkedOrMarked is the symmetric-inverse gate itself. See
// this file's own package-level doc comment for the full design (scope,
// opt-in trigger, compliance rule).
func checkPublicSurfaceLinkedOrMarked(g *ontology.Graph) []Violation {
	if !g.PublicSurfaceAuthorityLinked {
		// Not opted in -- honest no-op. See package doc comment: this is a
		// BRAND NEW, wholly independent trigger, never co-gated with
		// g.Discipline == loader.DisciplineFull.
		return nil
	}

	files, err := gate.ScanAuthoredModels(g)
	if err != nil {
		// A scan failure is not this check's violation to diagnose -- mirrors
		// check_model_complete's own scanErr branch.
		return nil
	}
	if len(files) == 0 {
		// No authored models at all -- nothing to be uncovered. Honest no-op.
		return nil
	}

	cited := collectCitedSymbols(g, files)

	// citedComplete looks up whether a given (kind, objName, symbolName)
	// triple is cited AND scenario-complete -- the identical bar
	// check_model_complete applies via the SAME shared aggregation.
	citedComplete := func(kind gate.SymbolKind, objName, symbolName string) (citedAtAll, complete bool) {
		agg, ok := cited[citedSymbolKey(kind, objName, symbolName)]
		if !ok {
			return false, false
		}
		return true, agg.anyScenario
	}

	var out []Violation
	for _, f := range files {
		for _, obj := range f.Objects {
			for _, m := range obj.Methods {
				if !token.IsExported(m.Name) {
					continue
				}
				if v, ok := evaluatePublicSurfaceSymbol(f.RelPath, obj.Name, m.Name, "method", m.Doc,
					gate.SymbolKindMethod, citedComplete); ok {
					out = append(out, v)
				}
			}
			for _, im := range obj.InterfaceMethods {
				if im.Name == "" {
					// Embedded interface entry -- not this object's own
					// declared method (task #396 brief: "skip Embedded
					// entries, they're not this object's own declared
					// methods").
					continue
				}
				if !token.IsExported(im.Name) {
					continue
				}
				if v, ok := evaluatePublicSurfaceSymbol(f.RelPath, obj.Name, im.Name, "interface method", im.Doc,
					gate.SymbolKindInterfaceMethod, citedComplete); ok {
					out = append(out, v)
				}
			}
		}
		for _, fn := range f.Funcs {
			if !token.IsExported(fn.Name) {
				continue
			}
			if v, ok := evaluatePublicSurfaceSymbol(f.RelPath, "", fn.Name, "function", fn.Doc,
				gate.SymbolKindFunc, citedComplete); ok {
				out = append(out, v)
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// evaluatePublicSurfaceSymbol applies checkPublicSurfaceLinkedOrMarked's
// compliance rule to ONE exported symbol and returns the Violation to report
// (ok=true) or ok=false when the symbol is compliant. objName is "" for a
// top-level function (kindLabel "function"); category is a human-readable
// label ("method" / "interface method" / "function") used only in the
// violation message.
func evaluatePublicSurfaceSymbol(
	relPath, objName, symbolName, category, doc string,
	kind gate.SymbolKind,
	citedComplete func(kind gate.SymbolKind, objName, symbolName string) (citedAtAll, complete bool),
) (Violation, bool) {
	citedAtAll, complete := citedComplete(kind, objName, symbolName)
	if citedAtAll && complete {
		// Compliant: cited by a SETTLED requirement AND scenario-complete.
		return Violation{}, false
	}
	if reason, marked := infrastructureOrIgnoredReason(doc); marked {
		// Compliant: explicitly marked infrastructure/ignored with a stated
		// reason -- REGARDLESS of citation state (Decision 3: "compliant iff
		// (cited ... ) OR (marked)"). The reason is not surfaced in a
		// violation (there is none), but the marker text itself is the
		// audit trail a human/LLM mirror-reviewer can grep for.
		_ = reason
		return Violation{}, false
	}

	id := symbolID(objName, symbolName)
	var detail string
	switch {
	case citedAtAll && !complete:
		detail = fmt.Sprintf(
			"cited by a SETTLED requirement's implemented_by but not scenario-complete (no citing requirement has a " +
				"verified_by entry resolving to a test calling hotamspec.NewScenario) -- add a scenario-narrated " +
				"verified_by test, or mark the doc comment INFRASTRUCTURE:/IGNORED: with a stated reason")
	default:
		detail = fmt.Sprintf(
			"not cited by any SETTLED requirement's implemented_by, and its doc comment carries no INFRASTRUCTURE:/" +
				"IGNORED: marker -- either cite it from a SETTLED requirement with a scenario-narrated verified_by " +
				"test, or add a doc comment stating why it needs none (e.g. \"INFRASTRUCTURE: thin adapter wrapper, " +
				"no independent behavior to prove.\")")
	}

	return Violation{
		Check: "check_public_surface_linked_or_marked",
		ID:    id,
		Message: fmt.Sprintf(
			"exported %s %s (%s) is neither linked (cited + scenario-complete) nor explicitly marked "+
				"infrastructure/ignored (task #396/W1.4, the symmetric inverse of check_model_complete): %s",
			category, id, relPath, detail),
	}, true
}

// symbolID renders a stable, human-readable identifier for a Violation's ID
// field: "Type.Method" for a receiver/interface method, or the bare function
// name for a top-level func (objName == "").
func symbolID(objName, symbolName string) string {
	if objName == "" {
		return symbolName
	}
	return objName + "." + symbolName
}

var _ = All.MustRegister("check_public_surface_linked_or_marked", Invariant{
	Name:  "check_public_surface_linked_or_marked",
	Canon: methodology.Domain,
	Claim: "in a domain whose manifest.json declares public_surface_authority:\"linked\", every EXPORTED authored " +
		"symbol in the domain's scanned model inventory (a receiver method, an interface's own declared method, or a " +
		"top-level function/constructor) is EITHER cited by a SETTLED requirement's implemented_by AND " +
		"scenario-complete (the identical bar check_model_complete applies), OR its doc comment carries an explicit " +
		"INFRASTRUCTURE:/IGNORED: marker with a non-empty stated reason; a domain that has not opted into " +
		"public_surface_authority:\"linked\" is an honest no-op, regardless of discipline:\"full\" or model state.",
	Rule: "IF g.PublicSurfaceAuthorityLinked is false (the domain's manifest.json does not declare " +
		"\"public_surface_authority\": \"linked\" -- a BRAND NEW, wholly independent trigger, NOT co-gated with " +
		"g.Discipline == loader.DisciplineFull), THEN this check is a pure HONEST NO-OP: zero violations regardless " +
		"of model state or discipline. OTHERWISE, compute the domain's authored model inventory via " +
		"gate.ScanAuthoredModels (the SAME scan check_model_complete uses) and the SAME collectCitedSymbols " +
		"citation-aggregation pass check_model_complete uses (SETTLED requirements' implemented_by entries resolved " +
		"via gate.MatchCitedSymbol against receiver methods, interface methods with Name != \"\", and top-level funcs). " +
		"For EVERY exported symbol in the inventory (ModelObject.Methods, ModelObject.InterfaceMethods with Name != " +
		"\"\" -- Embedded entries skipped, and ModelFile.Funcs), the symbol is COMPLIANT iff (it is cited by at least " +
		"one SETTLED requirement AND that citation's aggregated anyVerifiedByEntryHasScenario is true) OR (its own " +
		"Doc contains the literal substring \"INFRASTRUCTURE:\" or \"IGNORED:\" followed by non-empty trimmed " +
		"trailing text -- infrastructureOrIgnoredReason; an empty reason after the marker does NOT count as marked). " +
		"A non-compliant symbol fires ONE violation naming the owning object (or bare function name), the file, and " +
		"whether it is uncited entirely or cited-but-incomplete.",
	Why: "task #396 (W1.4): check_model_complete (model_complete.go) only asks 'for every method ALREADY cited as " +
		"implemented_by, is it scenario-complete?' -- a public method/interface-method/constructor NO requirement has " +
		"ever cited is completely invisible to that check, so a domain's model can silently accumulate uncovered " +
		"public surface with no gate noticing. This check adds the SYMMETRIC inverse: every public authored symbol " +
		"must be EITHER cited + scenario-complete, OR explicitly marked infrastructure/ignored -- no silent " +
		"exemptions, not even for mocks/ports/constructors. This task was blocked on task #393 specifically because " +
		"'symmetry only makes sense once the scan sees the WHOLE public surface, including interface methods and " +
		"constructors' -- before #393, gate.ScanAuthoredModels could not see those categories at all (ModelObject." +
		"InterfaceMethods, ModelFile.Funcs). OPT-IN TRIGGER (R-opt-in-trigger-owns-its-own-obligations, internal/" +
		"selfspec/requirements_authoredspec.go): this is a BRAND NEW obligation, so it gets its OWN, " +
		"separately-declared, separately-ratcheted trigger (public_surface_authority:\"linked\", loader." +
		"PublicSurfaceAuthorityLinked) rather than riding in on the already-spent discipline:\"full\" trigger " +
		"PRAT-hotam/domains/prat and domains/gpsm-sm consented to before this check existed -- tacking this onto " +
		"discipline:\"full\" would reproduce the EXACT live regression task #369 caused and task #388 fixed (both " +
		"domains jumping from 0 to many violations overnight with zero action on their part, since neither has ever " +
		"cited an interface method/constructor nor added any infrastructure/ignored marker). REGROUPING, not " +
		"re-implementation: this check reuses check_model_complete's own collectCitedSymbols aggregation (task #396's " +
		"own refactor of that check's former inline citation-collection loop) and gate.ScanAuthoredModels verbatim -- " +
		"no new coverage run, no second spec/ walk, no second AST pass over implemented_by. The " +
		"cited-but-incomplete overlap with check_model_complete's own violation is deliberate (same 'regrouping, not " +
		"re-implementation' pattern that check's own doc comment already establishes for its relationship to " +
		"check_settled_requires_scenario/check_scenario_executes_impl): this check's genuinely new value is making the " +
		"UNCITED case visible at all, something check_model_complete's own citation-driven iteration structurally " +
		"cannot see. Doc-comment marker convention (INFRASTRUCTURE:/IGNORED:), not a directive-line convention: " +
		"gate.docText joins a *ast.CommentGroup via (*ast.CommentGroup).Text(), which strips Go-directive-shaped " +
		"lines (e.g. //go:generate) from its output -- a bare directive-line marker would be silently deleted before " +
		"Doc ever captured it, so the marker must be ordinary prose text embedded within the doc comment, mirroring " +
		"this codebase's own established ALL-CAPS-reserved-word-in-prose convention (CLAUDE.md's TRANSLATE step).",
	Check: checkPublicSurfaceLinkedOrMarked,
})
