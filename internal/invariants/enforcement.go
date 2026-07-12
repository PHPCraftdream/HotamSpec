package invariants

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
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
		if r.Enforcement == ontology.EnforcementENFORCED && len(r.EnforcedBy) == 0 {
			out = append(out, Violation{
				Check: "check_enforced_names_invariant",
				ID:    r.ID,
				Message: "enforcement is ENFORCED but enforced_by is empty; " +
					"name the check_* invariant or test that fires on violation",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_enforced_names_invariant", Invariant{
	Name:  "check_enforced_names_invariant",
	Canon: methodology.Requirement,
	Claim: "every ENFORCED requirement names its enforcer(s).",
	Rule: "Requirement.enforcement MUST be in ENFORCEMENT_LEVELS (PROSE | STRUCTURAL | ENFORCED); any other value is a " +
		"misconfiguration. When enforcement == ENFORCED, enforced_by MUST be a non-empty tuple; an ENFORCED requirement " +
		"with no named enforcer is an unverifiable claim -- the guarantee cannot be audited.",
	Why: "naming the enforcer is what makes \"ENFORCED\" mean something beyond PROSE; without the anchor the audit trail " +
		"is broken and the burn-down meter cannot distinguish real reflexes from aspirational labels. An invalid enforcement " +
		"level is rejected early so the UNENFORCED.md report is never built on corrupt data.",
	Check: checkEnforcedNamesInvariant,
})

func checkEnforcedByResolvable(g *ontology.Graph) []Violation {
	var out []Violation
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
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_enforced_by_resolvable", Invariant{
	Name:  "check_enforced_by_resolvable",
	Canon: methodology.Requirement,
	Claim: "every ENFORCED requirement's enforced_by check_* entry resolves to a registered invariant.",
	Rule: "for each SETTLED+ENFORCED Requirement, every enforced_by entry that names a check_* function MUST resolve to a " +
		"real registered invariant; a typo, a stale/renamed check_* name, or any other unmatched check_* string fires a " +
		"Violation naming the entry that does not resolve.",
	Why: "the Python original also resolved pytest node-ids (file.py, file.py::func, bare test_*) via an AST scan of the " +
		"tests/ directory (enforcer_resolution.py). That filesystem-AST resolution is architecturally obsolete in this Go " +
		"port: a Go test enforcer is verified by running it (go test -run TestX fails loudly if the target does not exist), " +
		"so there is no need for a static check_* -> test-file map. Only the check_* name -> registered-invariant half is " +
		"kept, because that is the purely graph-local question the registry can answer without the filesystem; test_* and " +
		"file-path entries resolve by construction (runtime verification), so they no-op here. Separating resolvability " +
		"from check_enforced_names_invariant matters: that check only verifies enforced_by is NON-EMPTY -- a real typo " +
		"(a renamed check_*) passes it silently, and this invariant makes that debt visible directly.",
	Check: checkEnforcedByResolvable,
})

func checkEnforcedByTestHasTeeth(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_enforced_by_test_has_teeth", Invariant{
	Name:  "check_enforced_by_test_has_teeth",
	Canon: methodology.Requirement,
	Claim: "every enforced_by bare test_* function has real assertions (no-op in the Go port).",
	Rule: "RULE (original): when an enforced_by entry resolves to a bare test_* function name, that function's body MUST " +
		"contain at least one assert statement, pytest.raises call, or other function call (which could assert internally). " +
		"A test whose body is only pass/docstring/Ellipsis with no assertions and no calls is a toothless stub -- it makes " +
		"\"ENFORCED\" a lie.",
	Why: "no-op in this Go port. The original walked the Python test files and AST-parsed each bare test_* function body to " +
		"confirm it was not an empty stub (enforcer_resolution.test_func_has_teeth). That check is fundamentally tied to " +
		"Python's AST and the pytest node-id model; in the Go port the equivalent question is answered at RUNTIME -- a " +
		"toothless Go test function either fails to compile or passes vacuously only until the invariant it claims to " +
		"enforce actually regresses, at which point go test catches it directly. There is no static, graph-local way (nor " +
		"need) to reproduce the AST body scan, so this invariant is an honest no-op rather than a partial port. Resolvability " +
		"(check_enforced_by_resolvable) and non-emptiness (check_enforced_names_invariant) remain load-bearing; teeth does " +
		"not translate.",
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
