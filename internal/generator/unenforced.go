package generator

import (
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func BuildUnenforced(g *ontology.Graph) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	var settled, draft, openReqs, rejected []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
		if r.Status == ontology.StatusDRAFT {
			draft = append(draft, r)
		}
		if r.IsOpen() {
			openReqs = append(openReqs, r)
		}
		if r.Status == ontology.StatusREJECTED {
			rejected = append(rejected, r)
		}
	}

	var settledEnforced, settledUnenforced []ontology.Requirement
	for _, r := range settled {
		if r.Enforcement == ontology.EnforcementENFORCED {
			settledEnforced = append(settledEnforced, r)
		} else {
			settledUnenforced = append(settledUnenforced, r)
		}
	}
	// The closeable-debt band splits into two disjoint subsets whose union
	// equals IsCloseableDebt: closeable-now (real, actionable — a test could
	// be written today) and feature-blocked (the requirement describes a
	// feature that does not exist yet, so no honest test is possible until the
	// blocking feature is built). INHERENTLY_PROSE requirements remain a
	// separate permanent-discipline band.
	var closeableNow, featureBlocked, inherentProse []ontology.Requirement
	for _, r := range settledUnenforced {
		if !r.IsCloseableDebt() {
			inherentProse = append(inherentProse, r)
		} else if r.IsFeatureBlockedDebt() {
			featureBlocked = append(featureBlocked, r)
		} else {
			closeableNow = append(closeableNow, r)
		}
	}

	lines := []string{Banner, ReaderHeaderLine("UNENFORCED", g), ""}
	lines = append(lines, "# UNENFORCED.md — Burn-down meter (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated mirror of the enforcement gradient. Every requirement carries\n"+
			"`enforcement: PROSE | STRUCTURAL | ENFORCED` (R-enforcement-gradient) and an\n"+
			"`enforceability: ENFORCEABLE | INHERENTLY_PROSE` kind (R-enforceability-kind-declared).\n"+
			"This report lists every SETTLED requirement whose enforcement is NOT yet ENFORCED,\n"+
			"split into real closeable debt vs permanent discipline.")
	lines = append(lines, "")
	lines = append(lines,
		"The ratio line below IS the burn-down meter: a healthy direction is SETTLED-ENFORCED\n"+
			"growing while closeable debt (ENFORCEABLE, PROSE/STRUCTURAL of SETTLED) shrinks.\n"+
			"INHERENTLY_PROSE requirements are NOT counted as debt — they are honestly-labeled\n"+
			"judgment calls no check_* could ever verify.")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	lines = append(lines,
		"**Burn-down: SETTLED-ENFORCED "+strconv.Itoa(len(settledEnforced))+" / SETTLED "+strconv.Itoa(len(settled))+
			"; closeable-now "+strconv.Itoa(len(closeableNow))+"; feature-blocked "+strconv.Itoa(len(featureBlocked))+
			"; inherent discipline "+strconv.Itoa(len(inherentProse))+
			"; DRAFT "+strconv.Itoa(len(draft))+"; OPEN "+strconv.Itoa(len(openReqs))+"; REJECTED "+strconv.Itoa(len(rejected))+".**")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Closeable debt — closeable now (real, actionable)")
	lines = append(lines, "")
	if len(closeableNow) == 0 {
		lines = append(lines, "_None — every ENFORCEABLE SETTLED requirement is either ENFORCED or feature-blocked._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | claim |")
		lines = append(lines, "|---|---|---|---|")
		for _, r := range closeableNow {
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Closeable debt — feature-blocked (honest roadmap, not neglected)")
	lines = append(lines, "")
	lines = append(lines,
		"These ENFORCEABLE requirements stay PROSE because the feature they describe does not exist yet — a real enforcement test is impossible until the blocking feature is built (the build itself is frozen by R-speculative-aspects-frozen). The per-item `blocked_on` column names the specific Planned tool or absent package. See docs/reviews/2026-07-13-c1-roadmap-debt-triage.md for the full cluster analysis.")
	lines = append(lines, "")
	if len(featureBlocked) == 0 {
		lines = append(lines, "_None — no closeable-debt requirement is feature-blocked._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | blocked_on | claim |")
		lines = append(lines, "|---|---|---|---|---|")
		for _, r := range featureBlocked {
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+Cell(r.BlockedOn)+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines,
		"## Inherent discipline (INHERENTLY_PROSE — not debt, permanent by design)")
	lines = append(lines, "")
	if len(inherentProse) == 0 {
		lines = append(lines, "_None yet tagged._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | claim |")
		lines = append(lines, "|---|---|---|---|")
		for _, r := range inherentProse {
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## SETTLED and ENFORCED (the substrate's automatic reflexes)")
	lines = append(lines, "")
	if len(settledEnforced) == 0 {
		lines = append(lines, "_None yet._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforced_by | claim |")
		lines = append(lines, "|---|---|---|")
		for _, r := range settledEnforced {
			by := "—"
			switch {
			case len(r.EnforcedBy) > 0:
				by = strings.Join(r.EnforcedBy, ", ")
			case len(r.ImplementedBy) > 0 || len(r.VerifiedBy) > 0:
				// Authored path (PLAN-authored-spec-discipline.md §5/§12): no
				// enforced_by, but the disjunctive ENFORCED gate
				// (check_enforced_requires_enforcer_or_authored_link) accepts
				// implemented_by + verified_by instead -- show that carrier
				// rather than an honest-looking "—" that would misreport this
				// row as having no enforcer at all.
				by = strings.Join(r.ImplementedBy, ", ") + " / " + strings.Join(r.VerifiedBy, ", ")
			}
			lines = append(lines, "| `"+r.ID+"` | "+Cell(by)+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## DRAFT (not yet promoted)")
	lines = append(lines, "")
	if len(draft) == 0 {
		lines = append(lines, "_No DRAFT requirements._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | owner |")
		lines = append(lines, "|---|---|")
		for _, r := range draft {
			lines = append(lines, "| `"+r.ID+"` | `"+r.Owner+"` |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
