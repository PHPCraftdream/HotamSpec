package invariants

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkSettledRequiresScenario is the ONE-WAY discipline gate
// (PLAN-scenario-generated-spec.md §2 D4, task W2.1): in a domain whose
// manifest.json declares discipline:"full" (loader.ResolveDiscipline ==
// loader.DisciplineFull, surfaced on the loaded graph as g.Discipline), every
// SETTLED requirement MUST carry a real embodiment via ONE of two mutually
// exclusive paths:
//
//  1. the ENGINE path -- enforced_by non-empty (an executable check_*/Test*
//     mechanism already exists for this requirement -- see
//     check_enforced_requires_enforcer_or_authored_link,
//     internal/invariants/authored_links.go, which this check deliberately
//     does NOT duplicate: that check polices the SETTLED+ENFORCED subset
//     against the enforced_by/authored disjunction; THIS check polices the
//     wider SETTLED subset of a discipline:full domain against an ADDITIONAL
//     requirement layered on top of the authored half -- a real SCENARIO, not
//     merely a resolvable link);
//  2. the AUTHORED path -- implemented_by non-empty AND verified_by
//     non-empty AND at least one verified_by entry resolves to a real test
//     whose body calls hotamspec.NewScenario(...) (gate.ResolveSpecTest's
//     HasScenario, W1.3/W1.4 -- the same AST-only, no-`go test`-required
//     signal internal/generator/coverage.go's computeScenarioRatchet already
//     uses for "any ONE verified_by entry narrating is enough").
//
// A SETTLED requirement satisfying NEITHER path fires a violation naming
// exactly what is missing (no enforced_by AND (no implemented_by OR no
// verified_by OR no verified_by entry narrates a scenario)) -- an
// actionable diagnostic, not a generic "not compliant" message.
//
// INHERENTLY_PROSE EXEMPTION (PLAN-scenario-generated-spec.md §2 D4 + §5
// risks: "категория «inherently prose» сжимается при миграции
// per-requirement; любое остаточное исключение видно в COVERAGE, не
// прячется"): a SETTLED requirement whose Enforceability field equals
// ontology.EnforceabilityINHERENTLY_PROSE is EXEMPT from both paths above --
// the same early-`continue` shape as the engine-path exemption, an
// independent third branch. This is the residual category of claims that are
// architectural/temporal-order in nature and cannot be mechanically checked
// by ANY snapshot gate (the same class as R-domain-founded-in-wave-order,
// which was deliberately left honestly PROSE rather than forced onto a fake
// mechanism). The exemption is intentionally NARROW: it triggers ONLY on the
// Enforceability field, never on Enforcement, never on the absence of links
// alone -- so a steward must EXPLICITLY tag a requirement
// enforceability:"INHERENTLY_PROSE" in graph.json to claim it, and that tag
// is itself visible (it surfaces the requirement in COVERAGE.md's
// "Permanent discipline" table, and -- for a discipline:full domain whose
// requirement lacks a carrier -- in the "Discipline-exempt" section
// internal/generator/coverage.go renders, so the exemption is never folded
// silently into "0 violations").
//
// SCOPE EXCLUSION (self-hosting engine requirements, per task instructions):
// for a discipline:full domain, a requirement carrying a real enforced_by
// (the engine path) is ALREADY structurally proven by
// check_enforced_by_resolvable/check_enforced_names_invariant elsewhere --
// this check does NOT ALSO demand a scenario on top of an enforced_by
// requirement. This is what lets domains/hotam-spec-self (task instructions:
// NOT flipped to discipline:full in THIS wave, but designed to tolerate it in
// a future wave per PLAN-scenario-generated-spec.md §2 D4's own "Scope for
// hotam-spec-self" paragraph) keep its ~246 SETTLED requirements' existing
// enforced_by-carried engine mechanisms exactly as they are, with the
// scenario canvas only becoming mandatory for requirements that take the
// AUTHORED path instead.
//
// HONEST NO-OP (identical shape to every other optional-field check in this
// package): a domain whose g.Discipline is not loader.DisciplineFull --
// which includes every domain today (this wave deliberately does NOT flip
// discipline:full on any real domain; see the doc comment on
// loader.DisciplineFull for the one-way semantics this check enforces once a
// domain does opt in) -- contributes ZERO violations from this check,
// regardless of how many SETTLED requirements it has or how bare their
// implemented_by/verified_by/enforced_by fields are. This is the exact
// mechanism that closes PLAN-scenario-generated-spec.md §0's "gpsm-sm with 0
// models and 33 SETTLED requirements passes `hotam all-violations` clean"
// finding -- WITHOUT breaking that domain's existing soft-discipline
// behavior until its own steward deliberately opts it in.
func checkSettledRequiresScenario(g *ontology.Graph) []Violation {
	if g.Discipline != loader.DisciplineFull {
		// Soft discipline (the default, and every domain in this wave) --
		// honest no-op. See doc comment above and loader.DisciplineFull.
		return nil
	}
	specRoot := gate.SpecRootForGraph(g)
	var out []Violation
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			continue
		}
		if len(r.EnforcedBy) > 0 {
			// Engine path: already a real, structurally-checked executable
			// mechanism (check_enforced_by_resolvable et al. police its
			// resolvability elsewhere) -- exempt from the scenario demand,
			// per the self-hosting scope exclusion documented above.
			continue
		}
		if r.Enforceability == ontology.EnforceabilityINHERENTLY_PROSE {
			// Residual category: a claim no snapshot gate could ever
			// mechanically check (architectural / temporal-order in nature),
			// honestly tagged by the steward via the Enforceability field --
			// exempt from BOTH paths, per the INHERENTLY_PROSE EXEMPTION
			// documented above. Independent branch from the engine-path
			// exemption: a carrier, if present, was already handled by the
			// branch above; reaching here means the requirement lacks a
			// carrier, so this is the active exemption. The exemption is
			// VISIBLE: internal/generator/coverage.go's "Discipline-exempt"
			// section names exactly this set for a discipline:full domain, so
			// it never disappears into a silent "0 violations".
			continue
		}
		if len(r.ImplementedBy) == 0 || len(r.VerifiedBy) == 0 {
			out = append(out, Violation{
				Check: "check_settled_requires_scenario",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"discipline:full requires a real carrier for every SETTLED requirement: %s has neither "+
						"enforced_by (engine path) nor both implemented_by and verified_by (authored path) -- "+
						"implemented_by=%d entries, verified_by=%d entries, enforced_by=%d entries",
					r.ID, len(r.ImplementedBy), len(r.VerifiedBy), len(r.EnforcedBy)),
			})
			continue
		}
		if !anyVerifiedByEntryHasScenario(specRoot, g.SelfHosting, r.VerifiedBy) {
			out = append(out, Violation{
				Check: "check_settled_requires_scenario",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"discipline:full requires a scenario on the authored path: %s has implemented_by and verified_by "+
						"but none of its verified_by entries (%s) resolve to a test whose body calls "+
						"hotamspec.NewScenario(...) -- write the verified_by test against the vendored hotamspec "+
						"recorder (spec/hotamspec/hotamspec.go), or name a real enforced_by if this is actually "+
						"an engine-mechanism requirement",
					r.ID, strings.Join(r.VerifiedBy, ", ")),
			})
		}
	}
	return out
}

// anyVerifiedByEntryHasScenario mirrors
// internal/generator/coverage.go's computeScenarioRatchet convention (doc
// comment: "any ONE verified_by entry narrating is enough to count the
// requirement as with scenario") -- AST-only via gate.ResolveSpecTest's
// HasScenario, no `go test` invocation. An entry that fails to parse/resolve
// is silently skipped here (that is checkVerifiedByTestResolvable's
// violation to report, not this check's -- this check only asks "of the
// entries that DO resolve, does at least one narrate a scenario").
func anyVerifiedByEntryHasScenario(specRoot string, selfHosting bool, verifiedBy []string) bool {
	for _, e := range parseSpecEntries(verifiedBy) {
		if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
			continue
		}
		if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, selfHosting); !ok {
			continue
		}
		result, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
		if err != nil || !result.Found {
			continue
		}
		if result.HasScenario {
			return true
		}
	}
	return false
}

var _ = All.MustRegister("check_settled_requires_scenario", Invariant{
	Name:  "check_settled_requires_scenario",
	Canon: methodology.Requirement,
	Claim: "in a discipline:full domain, every SETTLED requirement carries a real carrier -- enforced_by (engine path), " +
		"or implemented_by + verified_by + at least one scenario-narrated verified_by test (authored path) -- UNLESS the " +
		"requirement is honestly tagged enforceability=INHERENTLY_PROSE (the residual category no snapshot gate could " +
		"mechanically check); a domain without discipline:full is an honest no-op.",
	Rule: "IF g.Discipline == loader.DisciplineFull (the domain's manifest.json declares \"discipline\": \"full\"), THEN " +
		"for EVERY Requirement with status == SETTLED, AT LEAST ONE of the following MUST hold: (1) enforced_by is " +
		"non-empty (the engine mechanism -- already structurally checked by check_enforced_by_resolvable/" +
		"check_enforced_names_invariant elsewhere, so this check does not re-demand a scenario on top of it), OR (2) " +
		"implemented_by is non-empty AND verified_by is non-empty AND at least one verified_by entry resolves " +
		"(gate.ResolveSpecTest) to a real Test* function whose body calls hotamspec.NewScenario(...) anywhere " +
		"(gate.SpecTestResult.HasScenario) -- mirroring internal/generator/coverage.go's computeScenarioRatchet " +
		"convention that any ONE narrating verified_by entry is sufficient -- OR (3) the requirement's Enforceability " +
		"field equals ontology.EnforceabilityINHERENTLY_PROSE (the residual category of architectural/temporal-order " +
		"claims no snapshot gate could mechanically check, the same class as R-domain-founded-in-wave-order which was " +
		"deliberately left honestly PROSE rather than forced onto a fake mechanism; PLAN-scenario-generated-spec.md §2 " +
		"D4 + §5 risks: «категория «inherently prose» сжимается при миграции per-requirement; любое остаточное " +
		"исключение видно в COVERAGE, не прячется»). A requirement satisfying NONE of the three fires a violation " +
		"naming exactly which half is missing. The INHERENTLY_PROSE exemption (branch 3) is intentionally NARROW: it " +
		"triggers ONLY on the Enforceability field, never on Enforcement, never on the absence of links alone -- so a " +
		"steward must EXPLICITLY tag a requirement enforceability:\"INHERENTLY_PROSE\" in graph.json to claim it, and " +
		"that tag is itself visible: internal/generator/coverage.go's \"Discipline-exempt\" section names exactly the " +
		"discipline:full subset of these requirements that lack a carrier (the actively-exempted set), so the exemption " +
		"is never folded silently into \"0 violations\". IF g.Discipline is NOT loader.DisciplineFull (empty, or any " +
		"other value -- the default for every domain today), this check is a pure HONEST NO-OP: zero violations " +
		"regardless of how bare a domain's implemented_by/verified_by/enforced_by fields are.",
	Why: "PLAN-scenario-generated-spec.md §0/§2 D4 (steward-directed remediation of the @fh audit finding): every prior " +
		"authored-link check in authored_links.go is NO-OP for a requirement with empty implemented_by/verified_by " +
		"(that file's own header comment, lines 32-36) -- so a domain with ZERO models and dozens of SETTLED " +
		"requirements (the audit's concrete example: gpsm-sm, 0 models, 33 SETTLED) passes `hotam all-violations` " +
		"completely clean, because the methodology RECOMMENDS authored links but never MECHANICALLY OBLIGATES them. " +
		"This check is the opt-in gate that closes that gap for any domain whose steward has declared discipline:full " +
		"in its own manifest.json -- a domain that has NOT opted in keeps today's exact soft behavior (this wave " +
		"deliberately does not flip discipline:full on any real domain; that is future-wave work per the plan's own " +
		"wave ordering), so existing domains (prat, gpsm-sm, hotam-spec-self) see zero new violations from this check " +
		"landing. The scenario requirement on the authored path (not just implemented_by+verified_by resolvability, " +
		"which check_enforced_requires_enforcer_or_authored_link already demands for the narrower SETTLED+ENFORCED " +
		"subset) is what makes discipline:full stronger than the existing disjunctive ENFORCED gate: a discipline:full " +
		"domain's SETTLED requirements -- ENFORCED or not -- must show real, scenario-narrated proof, not merely a " +
		"resolvable link that could still be a thin, non-narrating unit test. RELATION TO " +
		"check_enforced_requires_enforcer_or_authored_link (authored_links.go): that check is the DISJUNCTIVE ENFORCED " +
		"gate for the SETTLED+ENFORCED subset of EVERY domain (enforced_by OR implemented_by+verified_by, no scenario " +
		"demand); THIS check is a SEPARATE, OPT-IN, STRICTER gate for the wider SETTLED subset of ONLY a discipline:full " +
		"domain (same engine-path exemption, but the authored path additionally demands a scenario). The two checks are " +
		"deliberately NOT merged: a plain SETTLED+ENFORCED requirement in a non-discipline:full domain must still pass " +
		"the older, looser gate; only a discipline:full domain's requirements face the stricter scenario demand this " +
		"check adds on top. INHERENTLY_PROSE EXEMPTION RATIONALE: without it, a discipline:full domain with even ONE " +
		"honestly INHERENTLY_PROSE SETTLED requirement (a claim architectural/temporal-order in nature that no snapshot " +
		"gate could ever mechanically check) could never reach a clean 0 violations -- the gate would either force the " +
		"steward to bolt the requirement onto a FAKE mechanism (exactly the anti-pattern R-domain-founded-in-wave-order " +
		"was deliberately left honestly PROSE in task W3.4/#263 to avoid) or leave the domain perpetually red. The " +
		"exemption instead lets the steward tag such a requirement enforceability:\"INHERENTLY_PROSE\" in graph.json -- " +
		"an EXPLICIT, VISIBLE act (the tag surfaces in COVERAGE.md's permanent-discipline table, and for a " +
		"discipline:full domain that lacks a carrier for it, in coverage.go's \"Discipline-exempt\" section too) -- and " +
		"the gate respects it. The §2 D4 / §5 design intent is explicit that this residual category is meant to " +
		"SHRINK during per-requirement migration (each requirement genuinely convertible to a real object+method+scenario " +
		"is converted, and loses the tag), with any genuinely-unconvertible remainder VISIBLE in COVERAGE rather than " +
		"silently hidden behind a 0-violations count.",
	Check: checkSettledRequiresScenario,
})
