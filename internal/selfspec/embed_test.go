package selfspec

import (
	"strings"
	"testing"
)

// TestSourceFiles_ContainsExpectedFileCount is a smoke test that the
// go:embed directive actually picked up every requirements_*.go file plus
// selfspec.go — not zero files (a typo'd pattern) and not some accidental
// subset.
func TestSourceFiles_ContainsExpectedFileCount(t *testing.T) {
	entries, err := SourceFiles.ReadDir(".")
	if err != nil {
		t.Fatalf("SourceFiles.ReadDir(\".\"): %v", err)
	}

	var requirementsFiles, selfspecFile int
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "requirements_") && strings.HasSuffix(e.Name(), ".go"):
			requirementsFiles++
		case e.Name() == "selfspec.go":
			selfspecFile++
		}
	}

	// The real on-disk count as of task #345 (RAC-A) is 27 requirements_*.go
	// files; assert a generous lower bound rather than pinning the exact
	// count here (an unrelated future topic split should not break this
	// smoke test — TestMergeIntoGraph_AllRequirementsRegistered already pins
	// the requirement-count invariant that actually matters).
	if requirementsFiles < 20 {
		t.Errorf("SourceFiles: found %d requirements_*.go files, want at least 20", requirementsFiles)
	}
	if selfspecFile != 1 {
		t.Errorf("SourceFiles: found %d selfspec.go files, want exactly 1", selfspecFile)
	}
}

func TestSourceFiles_CanReadOneFileContent(t *testing.T) {
	data, err := SourceFiles.ReadFile("selfspec.go")
	if err != nil {
		t.Fatalf("SourceFiles.ReadFile(\"selfspec.go\"): %v", err)
	}
	if !strings.Contains(string(data), "package selfspec") {
		t.Errorf("selfspec.go embedded content missing expected package declaration; got %d bytes", len(data))
	}
}

// TestSourceFileFor_KnownIDFound proves the lookup finds a real,
// known-existing ID (from the operator topic file) in the correct file.
func TestSourceFileFor_KnownIDFound(t *testing.T) {
	const knownID = "R-operator-acting-facet"
	if _, ok := Requirements.Get(knownID); !ok {
		t.Fatalf("test fixture assumption broken: %q is no longer registered", knownID)
	}

	file, ok := SourceFileFor(knownID)
	if !ok {
		t.Fatalf("SourceFileFor(%q): want found, got not-found", knownID)
	}
	if file != "requirements_operator.go" {
		t.Errorf("SourceFileFor(%q) = %q, want %q", knownID, file, "requirements_operator.go")
	}
}

func TestSourceFileFor_UnknownIDNotFound(t *testing.T) {
	file, ok := SourceFileFor("R-this-id-does-not-exist-anywhere-xyz123")
	if ok {
		t.Errorf("SourceFileFor(unknown id): want not-found, got file %q", file)
	}
	if file != "" {
		t.Errorf("SourceFileFor(unknown id): want empty file name, got %q", file)
	}
}
