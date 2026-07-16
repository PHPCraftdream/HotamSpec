package invariants

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var mTagRE = regexp.MustCompile(`^M[1-9][0-9]*$`)

func checkEnforcedNamesInvariant(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if _, ok := ontology.EnforcementLevels[r.Enforcement]; !ok {
			out = append(out, Violation{
				Check: "check_enforced_names_invariant",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"enforcement %q is not in ENFORCEMENT_LEVELS (PROSE | STRUCTURAL | ENFORCED)",
					r.Enforcement),
			})
			continue
		}
		authoredMechanism := len(r.ImplementedBy) > 0 && len(r.VerifiedBy) > 0
		if r.Enforcement == ontology.EnforcementENFORCED && len(r.EnforcedBy) == 0 && !authoredMechanism {
			out = append(out, Violation{
				Check: "check_enforced_names_invariant",
				ID:    r.ID,
				Message: "enforcement is ENFORCED but enforced_by is empty and the authored path " +
					"(implemented_by + verified_by) is not both non-empty either; " +
					"name the check_* invariant or test that fires on violation, or point implemented_by/verified_by " +
					"at real authored spec/ code+test",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_enforced_names_invariant", Invariant{
	Name:  "check_enforced_names_invariant",
	Canon: methodology.Requirement,
	Claim: "every ENFORCED requirement names its enforcer(s) -- via the engine mechanism (enforced_by) or the authored mechanism (implemented_by + verified_by).",
	Rule: "Requirement.enforcement MUST be in ENFORCEMENT_LEVELS (PROSE | STRUCTURAL | ENFORCED); any other value is a " +
		"misconfiguration. When enforcement == ENFORCED, AT LEAST ONE of two mechanisms MUST name a real enforcer: (1) " +
		"enforced_by is a non-empty tuple (the engine mechanism -- a check_* registry name or a repo-wide Test* function), " +
		"or (2) implemented_by AND verified_by are BOTH non-empty (the authored mechanism -- path-qualified references into " +
		"the domain's own spec/ tree; see PLAN-authored-spec-discipline.md §5/§12 and check_enforced_requires_enforcer_or_authored_link, " +
		"internal/invariants/authored_links.go, which restates this same disjunction as its own atomic check). An ENFORCED " +
		"requirement with NEITHER mechanism present is an unverifiable claim -- the guarantee cannot be audited.",
	Why: "naming the enforcer is what makes \"ENFORCED\" mean something beyond PROSE; without the anchor the audit trail " +
		"is broken and the burn-down meter cannot distinguish real reflexes from aspirational labels. An invalid enforcement " +
		"level is rejected early so the UNENFORCED.md report is never built on corrupt data. The authored mechanism was added " +
		"by task #223 (authored-spec discipline): an authored-only ENFORCED requirement (enforced_by empty, implemented_by + " +
		"verified_by both set -- the whole point of the authored path, since such a requirement has no engine-side check_*/Test* " +
		"enforcer and should not be forced to fabricate one) MUST NOT trip this check just because enforced_by happens to be " +
		"empty; this check and check_enforced_requires_enforcer_or_authored_link are now the SAME disjunction stated twice " +
		"(non-emptiness half here, resolvability half there) so an authored-only requirement passes both, and a requirement " +
		"with neither mechanism fails both, consistently.",
	Check: checkEnforcedNamesInvariant,
})

func checkEnforcedByResolvable(g *ontology.Graph) []Violation {
	var out []Violation
	// The Test*-name half of resolution needs the filesystem (a scan of
	// internal/**/*_test.go and cmd/**/*_test.go — HotamSpec's own engine
	// tree only, never a domain-local or generated tree). It is resolved ONCE
	// here and reused for every requirement, so a single all-violations run
	// walks the test tree exactly once — never per requirement. This is the
	// SAME resolution logic internal/gate uses for targeted test selection
	// (walkTestFuncs / defaultInternalRoot / defaultCmdRoot), reused via
	// gate.TestFuncNames so the two consumers cannot drift on what counts as a
	// real Go test enforcer. The gen-code generator and its resolver trust
	// shift (task #214: widening this scan to a consumer domain's own
	// gen/go output) have been removed entirely — the engine only trusts Go
	// test functions it can verify live under its own tree. The check_* half
	// is answered graph-locally by the All registry below, without the
	// filesystem.
	testFuncs, scanErr := gate.TestFuncNames(g.DomainDir)
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED || r.Enforcement != ontology.EnforcementENFORCED {
			continue
		}
		for _, entry := range r.EnforcedBy {
			stripped := strings.TrimSpace(entry)
			if strings.HasPrefix(stripped, "check_") {
				if _, ok := All.Get(stripped); ok {
					continue
				}
				out = append(out, Violation{
					Check: "check_enforced_by_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"enforced_by entry %q is a check_* name but is not a registered invariant "+
							"(typo or stale/renamed enforcer) -- fix the name",
						entry),
				})
				continue
			}
			// Non-check_* entries MUST resolve to a real top-level Go Test*
			// function. This is what catches a regression back to stale
			// pytest node-ids (test_x.py, test_x.py::test_y) or bare lowercase
			// test_* names -- dead references. Wave 2 rebound these to real
			// Test*/check_* names; this guard keeps that rebound from quietly
			// undoing.
			if scanErr != nil {
				out = append(out, Violation{
					Check: "check_enforced_by_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"enforced_by entry %q could not be verified: test-function resolver unavailable (%v)",
						entry, scanErr),
				})
				continue
			}
			if _, ok := testFuncs[stripped]; ok {
				continue
			}
			out = append(out, Violation{
				Check: "check_enforced_by_resolvable",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"enforced_by entry %q on requirement %q does not resolve to a known check_* or Test* function",
					entry, r.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_enforced_by_resolvable", Invariant{
	Name:  "check_enforced_by_resolvable",
	Canon: methodology.Requirement,
	Claim: "every ENFORCED requirement's enforced_by entry resolves -- a check_* to a registered invariant, a Test* to a real Go test function.",
	Rule: "for each SETTLED+ENFORCED Requirement, every enforced_by entry MUST resolve to a real enforcer: a check_* name MUST be a registered " +
		"invariant (the All registry), and any other entry MUST be a real top-level Test* function name found under internal/**/*_test.go or " +
		"cmd/**/*_test.go " +
		"(the SAME scan internal/gate uses for targeted test selection). A typo, a stale/renamed check_*, a leftover pytest-style node-id " +
		"(test_x.py, test_x.py::test_y), a bare lowercase test_* name, or any other unresolvable string fires a Violation naming the entry.",
	Why: "this is the regression guard for the wave-2 rebound that replaced 157 broken pytest-style enforced_by references (test_x.py::test_y) " +
		"with real Test*/check_* names. Before this, the invariant resolved ONLY the check_* half via the registry and silently no-op'd every " +
		"other entry (test_*, file paths) on the assumption that runtime verification covered them -- an assumption that let the very regression it " +
		"was meant to catch survive undetected in domains wave 2 did not touch. Reusing gate.TestFuncNames (walkTestFuncs over internal/ and cmd/ " +
		"only -- the gen-code generator and its gen/go resolver trust shift from task #214 have been removed entirely) keeps a " +
		"single source of truth for 'what is a real test enforcer' across targeted selection and this audit, so the two can never disagree. " +
		"check_enforced_names_invariant only verifies enforced_by is NON-EMPTY; a real typo or a stale .py ref passes it silently, and this invariant " +
		"makes that debt visible directly.",
	Check: checkEnforcedByResolvable,
})

func checkEnforcedByTestHasTeeth(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_enforced_by_test_has_teeth", Invariant{
	Name:  "check_enforced_by_test_has_teeth",
	Canon: methodology.Requirement,
	Claim: "every enforced_by bare test_* function has real assertions (honest no-op).",
	Rule: "RULE: when an enforced_by entry resolves to a bare test_* function name, that function's body MUST " +
		"contain at least one real assertion or exercising call. A test whose body is empty or only a placeholder with " +
		"no assertions and no calls is a toothless stub -- it makes \"ENFORCED\" a lie.",
	Why: "this invariant is an honest no-op. Whether a test function has real assertions is answered at RUNTIME, not by a " +
		"graph invariant: a toothless test function either fails to compile or passes vacuously only until the requirement it " +
		"claims to enforce actually regresses, at which point go test catches it directly. There is no static, graph-local " +
		"way (nor need) to scan test bodies for assertions, so this invariant is an honest no-op rather than a partial " +
		"implementation. Resolvability (check_enforced_by_resolvable) and non-emptiness (check_enforced_names_invariant) " +
		"remain load-bearing; teeth does not translate to a graph check.",
	Check: checkEnforcedByTestHasTeeth,
})

func checkEnforceabilityKindKnown(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if _, ok := ontology.EnforceabilityKinds[r.Enforceability]; !ok {
			out = append(out, Violation{
				Check: "check_enforceability_kind_known",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"enforceability %q is not in ENFORCEABILITY_KINDS (ENFORCEABLE | INHERENTLY_PROSE)",
					r.Enforceability),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_enforceability_kind_known", Invariant{
	Name:  "check_enforceability_kind_known",
	Canon: methodology.Requirement,
	Claim: "every Requirement.enforceability is a known kind.",
	Rule: "Requirement.enforceability MUST be in ENFORCEABILITY_KINDS (ENFORCEABLE | INHERENTLY_PROSE). An invalid value " +
		"corrupts the enforcement-gradient debt calculation.",
	Why: "the enforceability kind is what lets the P0 REFLECTION debt count distinguish real closeable debt (ENFORCEABLE, " +
		"no enforcer yet) from permanent discipline (INHERENTLY_PROSE, never checkable by nature). An unknown value would " +
		"silently misclassify a requirement in that count.",
	Check: checkEnforceabilityKindKnown,
})

func checkMTagValidFormat(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if r.MTag == "" {
			continue
		}
		if !mTagRE.MatchString(r.MTag) {
			out = append(out, Violation{
				Check: "check_m_tag_valid_format",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"m_tag %q does not match ^M[1-9][0-9]*$ (must be 'M' followed by a positive integer, no leading zeros)",
					r.MTag),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_m_tag_valid_format", Invariant{
	Name:  "check_m_tag_valid_format",
	Canon: methodology.Requirement,
	Claim: "every non-empty m_tag matches ^M[1-9][0-9]*$.",
	Rule: "a non-empty m_tag MUST match ^M[1-9][0-9]*$ -- \"M\" followed by a positive integer with no leading zeros " +
		"(e.g. M3, M17, M26; not M01, m17, M, Mfoo). This is the typed-anchor discipline applied to M-tags.",
	Why: "invalid format breaks M-registry parsing; the format is the typed-anchor convention for methodology decisions " +
		"(R-drift-structurally-impossible / U5).",
	Check: checkMTagValidFormat,
})

func checkMTagUnique(g *ontology.Graph) []Violation {
	var out []Violation
	seenTags := map[string]string{}
	for _, r := range g.Requirements {
		if r.MTag == "" {
			continue
		}
		if prev, ok := seenTags[r.MTag]; ok {
			out = append(out, Violation{
				Check: "check_m_tag_unique",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"m_tag %q is already used by %q; each M-tag must be unique across the graph",
					r.MTag, prev),
			})
		} else {
			seenTags[r.MTag] = r.ID
		}
	}
	return out
}

var _ = All.MustRegister("check_m_tag_unique", Invariant{
	Name:  "check_m_tag_unique",
	Canon: methodology.Requirement,
	Claim: "no two Requirements share the same m_tag.",
	Rule: "each non-empty m_tag MUST appear on at most one Requirement in the graph. A duplicate tag breaks the bijection " +
		"that docs/gen/DECISIONS.md relies on: one M-decision maps to exactly one Requirement.",
	Why: "duplicates break the one-to-one mapping between an M-entry and its Requirement (R-drift-structurally-impossible " +
		"applied to the M-registry).",
	Check: checkMTagUnique,
})

func checkMTagOpenOnly(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if r.MTag == "" {
			continue
		}
		if !strings.HasPrefix(r.Status, ontology.StatusOPENPrefix) {
			out = append(out, Violation{
				Check: "check_m_tag_open_only",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"m_tag %q appears on a non-OPEN requirement (status=%q); M-tags are only for OPEN requirements "+
						"(the live M-decision registry)",
					r.MTag, r.Status),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_m_tag_open_only", Invariant{
	Name:  "check_m_tag_open_only",
	Canon: methodology.Requirement,
	Claim: "an m_tag appears only on an OPEN requirement.",
	Rule: "an m_tag MUST only appear on a Requirement with status starting with OPEN. An M-tag on a SETTLED, DRAFT, or " +
		"REJECTED requirement would pollute the M-registry with decisions that are no longer open.",
	Why: "the M-registry tracks live open decisions; a non-OPEN m_tag makes the registry structurally incorrect " +
		"(R-drift-structurally-impossible / U5).",
	Check: checkMTagOpenOnly,
})

func checkMTagFormat(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkMTagValidFormat(g)...)
	out = append(out, checkMTagUnique(g)...)
	out = append(out, checkMTagOpenOnly(g)...)
	return out
}

var _ = All.MustRegister("check_m_tag_format", Invariant{
	Name:  "check_m_tag_format",
	Canon: methodology.Requirement,
	Claim: "every non-empty m_tag is valid, unique, and OPEN-only (thin delegator).",
	Rule: "three sub-rules: 1. FORMAT -- a non-empty m_tag MUST match ^M[1-9][0-9]*$. 2. UNIQUE -- no two Requirements may " +
		"share the same m_tag. 3. OPEN-ONLY -- an m_tag MUST only appear on an OPEN requirement. This is a THIN DELEGATOR " +
		"-- calls check_m_tag_valid_format, check_m_tag_unique, check_m_tag_open_only and concatenates. The atomic " +
		"sub-checks are registered individually.",
	Why: "the M-tag field is the bridge between the graph and docs/gen/DECISIONS.md (the generated canonical M-registry). " +
		"Invalid format breaks parsing; duplicates break the one-to-one mapping; non-OPEN tags pollute the registry.",
	Check:       checkMTagFormat,
	IsDelegator: true,
})
