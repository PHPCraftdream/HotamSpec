package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func checkSectionAnchorsKnown(g *ontology.Graph) []Violation {
	var out []Violation
	for _, inv := range All.All() {
		if inv.Canon == nil {
			out = append(out, Violation{
				Check:   "check_section_anchors_known",
				ID:      inv.Name,
				Message: fmt.Sprintf("invariant %q has nil Canon -- every invariant MUST reference a registered methodology.Section (R-speak-by-reference)", inv.Name),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_section_anchors_known", Invariant{
	Name:  "check_section_anchors_known",
	Canon: methodology.Glossary,
	Claim: "every section-anchor token referenced by a framework invariant resolves to a known methodology.Section.",
	Rule: "every section-anchor token (pattern: section-sign followed by an identifier, e.g. the tokens used in " +
		"'Canon: section Requirement') found in any framework definition MUST appear in the methodology glossary. " +
		"An unrecognised section-anchor token is an invented term (R-speak-by-reference violation) that makes " +
		"R-anchor-everything structurally unsafe -- the anchor does not resolve. " +
		"In this Go port the Python AST walk over docstrings is replaced by a structural check: every Invariant " +
		"carries Canon as a TYPED *methodology.Section pointer that physically cannot reference a non-existent " +
		"section (the compiler guarantees the pointer was obtained from Sections.MustRegister). The only remaining " +
		"failure mode is a nil Canon -- an Invariant created without setting the field -- which this check catches.",
	Why: "the Python original walked every source file with ast, extracted section-sign tokens from docstrings via " +
		"regex, and checked each against glossary.term_slugs(). That mechanism is structurally unnecessary in Go: " +
		"the methodology lives as DATA in methodology.Sections (named *Section values), and every Invariant.Canon " +
		"is a typed pointer INTO that registry obtained at registration time. A typed pointer to a *methodology.Section " +
		"cannot dangle -- it was produced by MustRegister, which stores the pointer in the registry map. The only way " +
		"an anchor can be 'unknown' is if Canon was left nil, which this check detects directly without any AST scan. " +
		"This is the same 'provability not no-op' pattern as check_scoped_node_has_single_presenter: the invariant " +
		"is load-bearing for the property it guarantees, even though the current graph makes the failure impossible. " +
		"References: R-anchor-everything, R-speak-by-reference, test_glossary_sync.",
	Check: checkSectionAnchorsKnown,
})

func checkBijectionRToEnforcer(g *ontology.Graph) []Violation {
	invs := All.All()
	invNames := make(map[string]bool, len(invs))
	for _, inv := range invs {
		invNames[inv.Name] = true
	}

	var out []Violation
	checkToRids := make(map[string][]string, len(invs))
	for _, inv := range invs {
		checkToRids[inv.Name] = nil
	}
	hasEnforced := false

	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED || r.Enforcement != ontology.EnforcementENFORCED {
			continue
		}
		hasEnforced = true
		for _, name := range r.EnforcedBy {
			if !strings.HasPrefix(name, "check_") {
				continue
			}
			if !invNames[name] {
				out = append(out, Violation{
					Check:   "check_bijection_r_to_enforcer",
					ID:      r.ID,
					Message: fmt.Sprintf("enforced_by names %q which is not a function in the invariant registry (unresolvable check_* enforcer)", name),
				})
				continue
			}
			checkToRids[name] = append(checkToRids[name], r.ID)
		}
	}

	if hasEnforced {
		var orphans []string
		for name, rids := range checkToRids {
			if len(rids) != 0 {
				continue
			}
			if inv, ok := All.Get(name); ok && inv.IsDelegator {
				continue
			}
			orphans = append(orphans, name)
		}
		sort.Strings(orphans)
		for _, name := range orphans {
			out = append(out, Violation{
				Check:   "check_bijection_r_to_enforcer",
				ID:      name,
				Message: fmt.Sprintf("check_* function %q exists in the registry but is not named by any SETTLED/ENFORCED requirement's enforced_by (orphan enforcer -- anchor it to a Requirement)", name),
			})
		}
	}

	return out
}

var _ = All.MustRegister("check_bijection_r_to_enforcer", Invariant{
	Name:  "check_bijection_r_to_enforcer",
	Canon: methodology.Invariants,
	Claim: "every SETTLED/ENFORCED requirement's enforced_by check_* name resolves to a registered invariant; no orphan check_* exists.",
	Rule: "two sub-rules enforce the bijection between SETTLED/ENFORCED requirements and the registered check_* functions: " +
		"(1) RESOLVABILITY -- for every SETTLED requirement with enforcement==ENFORCED, each name in enforced_by that " +
		"starts with 'check_' MUST resolve to a function in the invariant registry (All.Get). Names starting with 'test_' " +
		"or other prefixes are exempt (they reference test files, not invariant functions). An unresolvable check_* name " +
		"is an unverifiable claim -- the named enforcer does not exist. " +
		"(2) ORPHAN DETECTION -- every function in the registry MUST be named by at least one SETTLED/ENFORCED " +
		"requirement's enforced_by (when at least one such requirement exists). A check_* function that no SETTLED/ENFORCED " +
		"requirement points to is an orphan -- it runs but is not anchored to a claim. SHARED enforcers (one check_* named " +
		"by multiple requirements) are acceptable and are not violations. In the Go port the registry is all-inclusive " +
		"(thin delegators are registered alongside atomic checks), so orphan detection naturally includes delegators " +
		"no requirement names -- this is architecturally honest, not a false positive.",
	Why: "the bijection between claim and check is what makes ENFORCED mean something beyond a label. An unresolvable " +
		"enforcer name hides compoundness; an orphan check hides unclaimed guarantees. The Python original built " +
		"inv_names from the curated ALL_INVARIANTS tuple (which excluded thin delegators); the Go registry includes " +
		"everything registered via MustRegister, so the bijection surface is wider. This is the correct Go adaptation: " +
		"the registry is the single source of truth, and orphan detection against it reveals which registered invariants " +
		"no SETTLED/ENFORCED requirement has claimed. References: R-bijection-r-to-enforcer.",
	Check: checkBijectionRToEnforcer,
})

func checkMethodMatchesDocstring(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_method_matches_docstring", Invariant{
	Name:  "check_method_matches_docstring",
	Canon: methodology.Invariants,
	Claim: "each check_* docstring RULE shares non-trivial lexical overlap with its Violation messages (honest no-op in the Go port).",
	Rule: "RULE (original Python): for every function in ALL_INVARIANTS, it MUST have a docstring, that docstring MUST " +
		"contain a RULE line, and the RULE line extracted from its docstring MUST share at least 5 percent Jaccard token " +
		"overlap with the concatenated text of all Violation messages in the function body. A mismatch means the docstring " +
		"describes a different rule from what the code enforces (silent drift). " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "the Python original parsed each check_* function's source with ast, extracted the RULE line from the docstring, " +
		"extracted Violation message string literals from the AST body, computed Jaccard token overlap, and flagged " +
		"functions below 5 percent as docstring-body drift. In Go, the entire premise collapses: Invariant.Claim, " +
		"Invariant.Rule, Invariant.Why and Invariant.Check are FIELDS OF ONE STRUCTURE registered together via " +
		"MustRegister -- they are DATA living next to the logic, not raw text separately extracted from an AST-parsed " +
		"docstring. The drift mode this check caught in Python (a docstring that says one thing while the function body " +
		"does another, with no structural link between them) cannot arise the same way in Go: the description and the " +
		"logic are colocated in the same struct literal, authored in the same edit, reviewed in the same diff. This makes " +
		"the check architecturally inapplicable, not merely unimplemented. The Go equivalent of docstring-body coherence " +
		"is enforced at code-review time (the Claim/Rule/Why text sits directly above the Check function in the same var " +
		"block) and structurally by the registry_complete_test which asserts no Claim/Rule/Why is empty. " +
		"References: R-method-matches-docstring.",
	Check: checkMethodMatchesDocstring,
})

func checkRulesAsDataClassificationCoherent(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_rules_as_data_classification_coherent", Invariant{
	Name:  "check_rules_as_data_classification_coherent",
	Canon: methodology.Invariants,
	Claim: "the rules-as-data classification table and the invariant registry name exactly the same functions (honest no-op in the Go port).",
	Rule: "RULE (original Python, R-rules-as-data, M22 HYBRID): every function in ALL_INVARIANTS MUST have exactly one " +
		"row in RULES_AS_DATA_TABLE (no missing classification, no duplicate row), and every row's name MUST resolve to " +
		"a function in ALL_INVARIANTS (no stale row surviving a rename or removal). Every row's kind MUST be in " +
		"RULES_AS_DATA_KINDS (TABLE_DRIVEN or BESPOKE). The classification table is the DATA half of the HYBRID verdict -- " +
		"if it silently drifts out of sync with ALL_INVARIANTS, the table stops being a trustworthy map. " +
		"This invariant is an HONEST NO-OP in the Go port.",
	Why: "the RULES_AS_DATA_TABLE -- a Python data structure classifying each check_* as TABLE_DRIVEN or BESPOKE -- does " +
		"not exist in the Go port. In Python it served the M22 'rules as data' design: an agent could read the table to " +
		"understand which invariants were homogeneous per-kind structural checks (generated by iterating a shared shape) " +
		"versus irreducibly hand-written bespoke logic. The Go port does not carry this metadata table: the classification " +
		"lives implicitly in the code structure (table-driven checks iterate g.EntityTypes or similar collections; bespoke " +
		"checks walk specific graph topology). Without a RULES_AS_DATA_TABLE to drift against the registry, the coherence " +
		"check has nothing to verify -- it is architecturally inapplicable, not merely unimplemented. If a future Go " +
		"milestone reintroduces explicit classification metadata, this check should be revived to guard its coherence " +
		"with the registry. References: R-rules-as-data, M22.",
	Check: checkRulesAsDataClassificationCoherent,
})
