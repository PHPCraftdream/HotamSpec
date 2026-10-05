package source

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestVerifyOldGraphIsNoOp(t *testing.T) {
	if got := Verify(&ontology.Graph{}); len(got) != 0 {
		t.Fatalf("Verify on an unlinked graph = %#v, want no checks", got)
	}
	if err := ValidateCoverage(ontology.Requirement{ID: "R-old-mode"}); err != nil {
		t.Fatalf("ValidateCoverage without an authored declaration: %v", err)
	}
}

func TestVerifySourcesReportsMissingDriftAndCurrentHash(t *testing.T) {
	dir := t.TempDir()
	current := []byte("current source bytes\n")
	if err := os.WriteFile(filepath.Join(dir, "current.md"), current, 0o600); err != nil {
		t.Fatal(err)
	}
	staleExpected := sha256Hex([]byte("earlier source bytes\n"))
	g := &ontology.Graph{
		DomainDir: dir,
		SpecificationSources: []ontology.SpecificationSource{
			{ID: "source-current", Path: "current.md", Version: "3", SHA256: sha256Hex(current)},
			{ID: "source-drift", Path: "current.md", Version: "2", SHA256: staleExpected},
			{ID: "source-missing", Path: "absent.md", Version: "1", SHA256: sha256Hex([]byte("missing"))},
		},
	}
	checks := VerifySources(g)
	if len(checks) != 3 {
		t.Fatalf("VerifySources returned %d checks, want one per declared source: %#v", len(checks), checks)
	}
	byID := make(map[string]Check, len(checks))
	for _, check := range checks {
		byID[check.SourceID] = check
	}
	if got := byID["source-current"]; got.Status != StatusVerified || got.ActualSHA256 != sha256Hex(current) || got.Version != "3" {
		t.Errorf("current source check did not retain declared version/current content hash: %#v", got)
	}
	if got := byID["source-drift"]; got.Status != StatusDrift || got.ActualSHA256 != sha256Hex(current) || got.Version != "2" {
		t.Errorf("drift source check = %#v", got)
	}
	if got := byID["source-missing"]; got.Status != StatusMissing {
		t.Errorf("missing source check = %#v", got)
	}
}

func TestSelfHostingSourcePathsResolveFromRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(root, "internal")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte("# Self-hosted source\n")
	if err := os.WriteFile(filepath.Join(sourceDir, "contract.md"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	domainDir := filepath.Join(root, "domains", "hotam-spec-self")
	g := &ontology.Graph{
		SelfHosting: true,
		DomainDir:   domainDir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "self-source", Path: "internal/contract.md", Version: "v1", SHA256: sha256Hex(content),
		}},
	}
	checks := VerifySources(g)
	if len(checks) != 1 || checks[0].Status != StatusVerified {
		t.Fatalf("self-hosted source path should resolve against repository root: %#v", checks)
	}
}

func TestVerifyLinksRequireKnownSourceAndRealAnchors(t *testing.T) {
	dir := t.TempDir()
	doc := "# Live Heading\n\ntext\n```md\n# Hidden Heading\n```\n## Second Section\n"
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	g := &ontology.Graph{
		DomainDir: dir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "spec", Path: "spec.md", Version: "v1", SHA256: sha256Hex([]byte(doc)),
		}},
		Requirements: []ontology.Requirement{{
			ID: "R-linked",
			SourceLinks: []ontology.SourceLink{
				{SourceID: "absent", Anchor: "L1"},
				{SourceID: "spec", Anchor: "#live-heading"},
				{SourceID: "spec", Anchor: "#second-section"},
				{SourceID: "spec", Anchor: "L1"},
				{SourceID: "spec", Anchor: "#hidden-heading"},
				{SourceID: "spec", Anchor: "L3-L4"},
				{SourceID: "spec", Anchor: "L0-L2"},
			},
		}},
	}
	checks := VerifyLinks(g)
	if len(checks) != 7 {
		t.Fatalf("VerifyLinks returned %d checks, want all seven authored links: %#v", len(checks), checks)
	}
	var valid, invalid int
	for _, check := range checks {
		if check.Status == StatusVerified {
			valid++
		} else if check.Status == StatusInvalid {
			invalid++
		}
	}
	if valid != 4 || invalid != 3 {
		t.Errorf("link statuses: verified=%d invalid=%d, want 4 and 3: %#v", valid, invalid, checks)
	}
	if !strings.Contains(checks[0].Message, "unknown specification source") {
		t.Errorf("unknown source did not get a precise diagnostic: %#v", checks[0])
	}
	// The hidden heading exists only inside a fenced code block and is not a
	// Markdown heading for source-link purposes.
	foundHiddenError := false
	for _, check := range checks {
		if strings.Contains(check.Message, "hidden-heading") && check.Status == StatusInvalid {
			foundHiddenError = true
		}
	}
	if !foundHiddenError {
		t.Errorf("fenced heading was accepted as an anchor: %#v", checks)
	}
}

func TestVerifyRejectsInvalidSourceDeclarations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# Ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	checks := VerifySources(&ontology.Graph{
		DomainDir: dir,
		SpecificationSources: []ontology.SpecificationSource{{
			ID: "spec", Path: "spec.md", SHA256: "bad-hash",
		}},
	})
	if len(checks) != 1 || checks[0].Status != StatusInvalid || checks[0].Version != "" {
		t.Fatalf("invalid source descriptor must be explicit and retain its fields: %#v", checks)
	}
	if checks[0].ActualSHA256 != sha256Hex([]byte("# Ready\n")) {
		t.Errorf("invalid descriptor omitted actual hash of readable current bytes: %#v", checks[0])
	}
	if !strings.Contains(checks[0].Message, "version is empty") || !strings.Contains(checks[0].Message, "sha256") {
		t.Errorf("invalid source diagnostic omitted qualification failures: %#v", checks[0])
	}
}

func TestValidateCoverageQualificationContract(t *testing.T) {
	link := []ontology.SourceLink{{SourceID: "spec", Anchor: "#scope"}}
	cases := []struct {
		name string
		r    ontology.Requirement
		want string
	}{
		{name: "no declaration", r: ontology.Requirement{ID: "R-none"}},
		{name: "unsupported requires source", r: ontology.Requirement{ID: "R-unsupported", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnsupported, Rationale: "outside the selected profile"}}, want: "source_link"},
		{name: "unsupported qualifies", r: ontology.Requirement{ID: "R-unsupported-ok", SourceLinks: link, Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnsupported, Rationale: "outside selected support"}}},
		{name: "unreachable requires rationale", r: ontology.Requirement{ID: "R-unreachable", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnreachable, Profile: "production"}}, want: "rationale"},
		{name: "unreachable requires profile", r: ontology.Requirement{ID: "R-unreachable-profile", SourceLinks: link, Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnreachable, Rationale: "not exposed", Profile: " "}}, want: "profile"},
		{name: "unreachable requires source", r: ontology.Requirement{ID: "R-unreachable-source", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnreachable, Rationale: "not exposed", Profile: "production"}}, want: "source_link"},
		{name: "unreachable qualifies", r: ontology.Requirement{ID: "R-unreachable-ok", SourceLinks: link, Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnreachable, Rationale: "not exposed", Profile: "production"}}},
		{name: "unverified rationale", r: ontology.Requirement{ID: "R-unverified", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnverified, Rationale: "evidence is not available yet"}}},
		{name: "empty rationale", r: ontology.Requirement{ID: "R-empty", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageUnverified, Rationale: " \t"}}, want: "rationale"},
		{name: "verified is computed", r: ontology.Requirement{ID: "R-verified", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageVerified, Rationale: "claim appears true"}}, want: "computed"},
		{name: "computed status takes precedence over missing rationale", r: ontology.Requirement{ID: "R-computed-empty", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageVerified}}, want: "computed"},
		{name: "discrepancy is computed", r: ontology.Requirement{ID: "R-discrepancy", Coverage: &ontology.CoverageDeclaration{Status: ontology.CoverageDiscrepancy, Rationale: "test failed"}}, want: "computed"},
		{name: "unknown status", r: ontology.Requirement{ID: "R-unknown", Coverage: &ontology.CoverageDeclaration{Status: "complete", Rationale: "claim appears true"}}, want: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCoverage(tc.r)
			if tc.want == "" && err != nil {
				t.Fatalf("ValidateCoverage: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("ValidateCoverage error = %v, want diagnostic containing %q", err, tc.want)
			}
		})
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func TestVerifyChecksClauseSourceVersionHashAndAnchor(t *testing.T) {
	root := t.TempDir()
	contents := []byte("# Atomic clause\nMUST preserve the selected result.\n")
	path := filepath.Join(root, "spec.md")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	g := &ontology.Graph{}
	g.DomainDir = root
	g.SpecificationSources = []ontology.SpecificationSource{{
		ID: "language-spec", Path: "spec.md", Version: "v2", SHA256: sha256Hex(contents),
	}}
	g.Conformance = &ontology.ConformanceConfig{Clauses: []ontology.SourceClause{{
		ID: "clause-result", SourceLinks: []ontology.SourceLink{{SourceID: "language-spec", Anchor: "L1"}},
	}}}
	checks := Verify(g)
	var sourceVerified, linkVerified bool
	for _, check := range checks {
		if check.SourceID != "language-spec" || check.Version != "v2" || check.ExpectedSHA256 != sha256Hex(contents) || check.ActualSHA256 != sha256Hex(contents) {
			continue
		}
		if check.Status != StatusVerified {
			continue
		}
		if check.Anchor == "" {
			sourceVerified = true
		}
		if check.Anchor == "L1" {
			linkVerified = true
		}
	}
	if !sourceVerified || !linkVerified {
		t.Fatalf("clause source did not retain verified version/hash/anchor checks: %+v", checks)
	}

	changed := []byte("# Atomic clause\nchanged bytes\n")
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	checks = Verify(g)
	for _, check := range checks {
		if check.SourceID == "language-spec" && check.Version == "v2" && check.Status == StatusDrift {
			return
		}
	}
	t.Fatalf("source edit did not invalidate the inventory pin: %+v", checks)
}
