package loader

import (
	"os"
	"path/filepath"
	"testing"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func TestLoadGraph_DomainHotamSpecSelf(t *testing.T) {
	t.Parallel()
	g, err := LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}

	cases := []struct {
		name string
		got  int
		want int
	}{
		{"axes", len(g.Axes), 9},
		{"stakeholders", len(g.Stakeholders), 4},
		{"assumptions", len(g.Assumptions), 16},
		// 284 + 2: task #223 landed R-authored-spec-links-mechanically-checked
		// and R-enforced-requires-enforcer-or-authored-link, anchoring the six
		// new authored-spec mechanical checks (internal/invariants/authored_links.go)
		// to the framework's own self-hosting bijection discipline.
		// 286 + 4: task #235 landed R-spec-link-embodied-vs-proven,
		// R-authored-spec-layer-progression, R-structural-floor-vs-mirror-audit,
		// and R-authored-spec-projections-are-derived -- the authored-spec
		// discipline's own objects+fields modeled as first-class graph nodes
		// (PLAN-authored-spec-discipline.md §4/§5/§6/§7).
		// 290 + 2: task W3.1b landed R-scenario-spec-obligations-mechanically-
		// enforced and R-vendored-recorder-matches-engine-canon, anchoring the
		// five W1.1/W2.1-W2.4 scenario-generated-spec gates that
		// check_bijection_r_to_enforcer had flagged as orphan enforcers on
		// hotam-spec-self (the engine holding itself to its own obligation).
		// 292 + 1: R-speak-domain-register-by-default -- the speech-register
		// rule (answer in the active domain's language by default; engine
		// internals reserved for explicit-ask or the mediation-loop
		// TRANSLATE/PRESENT/LAND steps), STRUCTURAL/INHERENTLY_PROSE like
		// R-ai-presents-not-decides, rendered into every Role block by
		// RenderOperatorRoleBlock.
		// 293 + 1: R-orientation-faq-answerable -- the orientation
		// showcase requirement (an AI agent orients in this project fast
		// is mechanically checkable, not a hope): ENFORCED via the new
		// check_orientation_faq_answered invariant, which proves each
		// declared manifest.json orientation_faq question is answerable
		// from the generated crystal in <=1 hop (keywords inline OR a
		// one-hop link resolving to a real file). Hand-landed (same
		// apply-proposal / check_spec_md_current structural-defect
		// workaround as R-speak-domain-register-by-default above).
		// 294 + 1: R-gate-signoff-single-carrier -- anchors the three new
		// GateSignoff invariants (check_gate_signoff_monotonic/
		// _deferred_reason_present/_deferred_conflict_resolves,
		// internal/invariants/gate_signoff_checks.go) for this repo's own
		// self-hosting check_bijection_r_to_enforcer discipline; this
		// domain declares no gate_stage_order and carries no gate_signoffs
		// itself, so the checks are honest no-ops here (task E1, same
		// hand-land workaround as the two entries above).
		{"requirements", len(g.Requirements), 295},
		{"conflicts", len(g.Conflicts), 8},
		{"operators", len(g.Operators), 1},
		{"processes", len(g.Processes), 1},
		{"goals", len(g.Goals), 1},
		{"entity_types", len(g.EntityTypes), 0},
		{"entities", len(g.Entities), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}

	if !g.SelfHosting {
		t.Errorf("SelfHosting: want true (manifest.json present), got false")
	}
}

func TestLoadGraph_DomainHotamSpecSelf_GenerateLock(t *testing.T) {
	t.Parallel()
	lockPath := LockPath(domainGraphPath)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Logf("generating %s …", lockPath)
		if err := WriteLock(domainGraphPath, "initial domain lock from test"); err != nil {
			t.Fatalf("WriteLock(%s): %v", domainGraphPath, err)
		}
	}

	ok, err := VerifyLock(domainGraphPath)
	if err != nil {
		t.Fatalf("VerifyLock(%s): %v", domainGraphPath, err)
	}
	if !ok {
		abs, _ := filepath.Abs(domainGraphPath)
		t.Errorf("VerifyLock(%s): lock does not match graph; re-run to regenerate", abs)
	}
}
