package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// jsonFenceRe extracts the contents of every ```json ... ``` fenced code
// block in a Markdown file. docs/PROPOSAL-REFERENCE.md documents exactly one
// escaped inline example (backtick-wrapped "```json" prose, not a real
// fence start) which this pattern does not match because it requires the
// fence marker to sit alone on its own line.
var jsonFenceRe = regexp.MustCompile("(?s)```json\r?\n(.*?)\r?\n```")

// TestProposalReferenceExamples_AllParse guards docs/PROPOSAL-REFERENCE.md
// against silent drift from the actual decoder (internal/proposal/types.go
// json tags + cmd/hotam/apply_proposal.go's parseProposal/unmarshalProposal
// strict decode). Every ```json fenced example in that file is expected to
// be a real, individually-postable proposal object that parseProposal
// accepts without error -- if a future edit to types.go renames or removes
// a field, or a future doc edit introduces a typo/stale field name, this
// test fails instead of the drift going unnoticed until a consumer's copy
// of an example proposal fails to apply.
func TestProposalReferenceExamples_AllParse(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	docPath := filepath.Join(repoRoot, "docs", "PROPOSAL-REFERENCE.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	matches := jsonFenceRe.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		t.Fatalf("no ```json fenced blocks found in %s -- extraction regex or doc structure changed", docPath)
	}

	for i, m := range matches {
		block := m[1]
		t.Run(rangeLabel(i, block), func(t *testing.T) {
			p, err := parseProposal(block)
			if err != nil {
				t.Fatalf("parseProposal failed for fenced block #%d:\n%s\n\nerror: %v", i+1, block, err)
			}
			if p.Kind() == "" {
				t.Errorf("fenced block #%d parsed but Kind() is empty: %+v", i+1, p)
			}
		})
	}
}

// rangeLabel builds a short, stable subtest name from the block's declared
// "kind" field (falling back to its index) so a failure's -run path names
// which proposal kind's example broke without needing to open the doc.
var kindFieldRe = regexp.MustCompile(`"kind"\s*:\s*"([^"]+)"`)

func rangeLabel(i int, block []byte) string {
	if m := kindFieldRe.FindSubmatch(block); m != nil {
		return string(m[1]) + "_" + strconv.Itoa(i)
	}
	return "block_" + strconv.Itoa(i)
}
