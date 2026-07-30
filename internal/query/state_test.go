package query

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestShowRequirement_PopulatesState drives ShowRequirement through the REAL
// selfspec.RequirementState computation (not a hand-faked string) across two
// distinct proof-lifecycle states: NO_CARRIER (zero verified_by entries -- the
// function returns immediately without spawning any subprocess) and UNVERIFIED
// (a verified_by entry pointing at a nonexistent test under a temp dir with no
// go.mod -- gate.RunVerifiedByTestRecording returns Err, the entry is skipped,
// no verdict is produced).
func TestShowRequirement_PopulatesState(t *testing.T) {
	t.Parallel()

	// NO_CARRIER: requirement with zero verified_by entries.
	g1 := &ontology.Graph{
		DomainDir: t.TempDir(),
		Requirements: []ontology.Requirement{
			{ID: "R-state-no-carrier", Claim: "a claim", Status: ontology.StatusSETTLED},
		},
	}
	card1, err := ShowRequirement(g1, "R-state-no-carrier")
	if err != nil {
		t.Fatalf("ShowRequirement NO_CARRIER: %v", err)
	}
	if card1.State != "NO_CARRIER" {
		t.Fatalf("State = %q, want NO_CARRIER", card1.State)
	}

	// UNVERIFIED: a verified_by entry that cannot resolve (no go.mod, no test
	// file anywhere under the temp-dir spec root).
	g2 := &ontology.Graph{
		DomainDir: t.TempDir(),
		Requirements: []ontology.Requirement{
			{
				ID:         "R-state-unverified",
				Claim:      "a claim",
				Status:     ontology.StatusSETTLED,
				VerifiedBy: []string{"model/does_not_exist_test.go:TestNoSuchTest"},
			},
		},
	}
	card2, err := ShowRequirement(g2, "R-state-unverified")
	if err != nil {
		t.Fatalf("ShowRequirement UNVERIFIED: %v", err)
	}
	if card2.State != "UNVERIFIED" {
		t.Fatalf("State = %q, want UNVERIFIED", card2.State)
	}

	// Sanity: the two states are genuinely different.
	if card1.State == card2.State {
		t.Fatalf("expected two distinct states, both were %q", card1.State)
	}
}

// TestFormatRequirementCard_IncludesProofStateLine proves the human-readable
// card renders the State field as a labeled "proof state:" line positioned
// alongside enforcement, not buried at the bottom.
func TestFormatRequirementCard_IncludesProofStateLine(t *testing.T) {
	t.Parallel()
	card := RequirementCard{
		ID:          "R-test",
		Status:      "SETTLED",
		Claim:       "test claim",
		Enforcement: "ENFORCED",
		State:       "PROVEN",
	}
	text := FormatRequirementCard(card)
	if !strings.Contains(text, "proof state: PROVEN") {
		t.Errorf("expected 'proof state: PROVEN' in output, got:\n%s", text)
	}
	// The proof state line must appear after the enforcement line, not at the
	// bottom.
	enforcementIdx := strings.Index(text, "enforcement:")
	proofStateIdx := strings.Index(text, "proof state:")
	if enforcementIdx < 0 || proofStateIdx < 0 {
		t.Fatalf("missing enforcement or proof state line:\n%s", text)
	}
	if proofStateIdx < enforcementIdx {
		t.Errorf("proof state line must come after enforcement line:\n%s", text)
	}
}

// TestRequirementCard_JSONIncludesState proves the new State field flows into
// the --json output shape automatically via encoding/json.
func TestRequirementCard_JSONIncludesState(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		DomainDir: t.TempDir(),
		Requirements: []ontology.Requirement{
			{ID: "R-json-state", Claim: "a claim", Status: ontology.StatusSETTLED},
		},
	}
	card, err := ShowRequirement(g, "R-json-state")
	if err != nil {
		t.Fatalf("ShowRequirement: %v", err)
	}
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"state":"NO_CARRIER"`) {
		t.Errorf("expected `\"state\":\"NO_CARRIER\"` in JSON, got:\n%s", raw)
	}
}

// TestBrief_ConflictRendersNoStateLine proves Brief on a Conflict anchor does
// not render any proof-state content -- State is a Requirement-only concept.
func TestBrief_ConflictRendersNoStateLine(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "C-ab", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	text := FormatBrief(b)
	if strings.Contains(text, "proof state") {
		t.Errorf("Conflict brief must not contain proof state content, got:\n%s", text)
	}
	// JSON must not carry a top-level "state" key for a Conflict brief.
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), `"state"`) {
		t.Errorf("Conflict brief JSON must not contain a state key, got:\n%s", string(data))
	}
}

// TestBrief_AssumptionRendersNoStateLine proves Brief on an Assumption anchor
// does not render any proof-state content.
func TestBrief_AssumptionRendersNoStateLine(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "A-shared", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	text := FormatBrief(b)
	if strings.Contains(text, "proof state") {
		t.Errorf("Assumption brief must not contain proof state content, got:\n%s", text)
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), `"state"`) {
		t.Errorf("Assumption brief JSON must not contain a state key, got:\n%s", string(data))
	}
}

// TestBrief_RequirementIncludesStateLine proves Brief on a Requirement anchor
// DOES render the proof-state line (State flows through the composed
// RequirementCard, which Context/ShowRequirement already populates).
func TestBrief_RequirementIncludesStateLine(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "R-alpha", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Requirement == nil {
		t.Fatal("Requirement card is nil")
	}
	// fixtureGraph requirements have empty VerifiedBy -> NO_CARRIER.
	if b.Requirement.State != "NO_CARRIER" {
		t.Errorf("Requirement.State = %q, want NO_CARRIER", b.Requirement.State)
	}
	text := FormatBrief(b)
	if !strings.Contains(text, "proof state: NO_CARRIER") {
		t.Errorf("expected 'proof state: NO_CARRIER' in brief output, got:\n%s", text)
	}
}
