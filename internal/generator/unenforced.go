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
	var closeableDebt, inherentProse []ontology.Requirement
	for _, r := range settledUnenforced {
		if r.IsCloseableDebt() {
			closeableDebt = append(closeableDebt, r)
		} else {
			inherentProse = append(inherentProse, r)
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
			"; closeable debt "+strconv.Itoa(len(closeableDebt))+"; inherent discipline "+strconv.Itoa(len(inherentProse))+
			"; DRAFT "+strconv.Itoa(len(draft))+"; OPEN "+strconv.Itoa(len(openReqs))+"; REJECTED "+strconv.Itoa(len(rejected))+".**")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Closeable debt (ENFORCEABLE, no enforcer yet)")
	lines = append(lines, "")
	if len(closeableDebt) == 0 {
		lines = append(lines, "_None — all ENFORCEABLE SETTLED requirements are ENFORCED._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | enforcement | owner | claim |")
		lines = append(lines, "|---|---|---|---|")
		for _, r := range closeableDebt {
			lines = append(lines, "| `"+r.ID+"` | "+r.Enforcement+" | `"+r.Owner+"` | "+Cell(r.Claim)+" |")
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
			if len(r.EnforcedBy) > 0 {
				by = strings.Join(r.EnforcedBy, ", ")
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
