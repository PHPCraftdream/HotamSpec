package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
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
	t.Parallel()
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

// --- Required/Optional field-list drift guard (task #150, review-8 R8-g) ---

// proposalKindsSample holds one zero-value instance of every Proposed* type —
// the same 13 kinds docs/PROPOSAL-REFERENCE.md documents. Each is a
// proposal.Proposal so the loop can call Kind() to map it to its doc section.
var proposalKindsSample = []proposal.Proposal{
	proposal.ProposedStakeholder{},
	proposal.ProposedAxis{},
	proposal.ProposedRequirement{},
	proposal.ProposedConflict{},
	proposal.ProposedConflictTransition{},
	proposal.ProposedRejection{},
	proposal.ProposedAssumption{},
	proposal.ProposedAssumptionTransition{},
	proposal.ProposedAssumptionRewrite{},
	proposal.ProposedConflictMemberUpdate{},
	proposal.ProposedReviewMark{},
	proposal.ProposedOperatorBudget{},
	proposal.ProposedEntityType{},
	proposal.ProposedProcess{},
}

// TestProposalReferenceRequiredOptionalFields_InSync guards the hand-written
// **Required:**/**Optional:** field lists in docs/PROPOSAL-REFERENCE.md against
// silent drift from the actual Proposed* structs in internal/proposal/types.go.
//
// The doc already mechanically round-trips every ```json EXAMPLE through the
// real decoder (TestProposalReferenceExamples_AllParse above). What that check
// CANNOT see is the prose Required/Optional field lists: a field added to a
// struct without updating those lists, or a field removed from a struct but
// left named in the prose, is invisible to the example round-trip. This test
// closes that gap in BOTH directions:
//
//	FORWARD  — every json-tagged field of each Proposed* struct appears in the
//	           union of its section's Required and Optional backtick field
//	           lists. A struct field missing from BOTH lists is the live drift
//	           (e.g. blocked_on was missing before this task landed).
//	REVERSE  — every field-name-position backtick token in those lists
//	           corresponds to a real json-tagged field of the struct. A token
//	           that does not is a typo, or a stale name left after a struct
//	           field was renamed/removed.
//
// "Field-name position" = parenthesis depth 0 in the Required/Optional prose:
// field names sit at the top of the list; enum values, defaults, and inline
// examples sit inside parentheticals (depth >= 1). extractDepthZeroBacktickTokens
// implements this, ignoring parens that appear INSIDE backticks (e.g.
// `OPEN(<question>)`) and treating markdown-link parens as balanced.
//
// Non-vacuity: see TestProposalReferenceFieldDetector_NonVacuous.
func TestProposalReferenceRequiredOptionalFields_InSync(t *testing.T) {
	t.Parallel()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	docPath := filepath.Join(repoRoot, "docs", "PROPOSAL-REFERENCE.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	lines := strings.Split(string(data), "\n")

	for _, p := range proposalKindsSample {
		kind := p.Kind()
		typeName := reflect.TypeOf(p).Name()
		body := findProposalSectionBody(t, lines, kind)
		reqTokens := extractDepthZeroBacktickTokens(extractFieldListBlock(body, "**Required:**"))
		optTokens := extractDepthZeroBacktickTokens(extractFieldListBlock(body, "**Optional:**"))
		docFields := append(append([]string{}, reqTokens...), optTokens...)
		missing, stale := diffFieldSets(jsonFieldNames(p), docFields)

		t.Run(kind, func(t *testing.T) {
			for _, f := range missing {
				t.Errorf("FORWARD drift: %q is a json-tagged field of %s (internal/proposal/types.go) but is missing from BOTH the **Required:** and **Optional:** field lists in the %q section of docs/PROPOSAL-REFERENCE.md — the hand-written field list drifted from the struct. Add %q to the matching Required/Optional line.", f, typeName, kind, f)
			}
			for _, f := range stale {
				t.Errorf("REVERSE drift: %q is listed as a field in the %q section's **Required:**/**Optional:** prose of docs/PROPOSAL-REFERENCE.md but is not a json-tagged field of %s — a typo, or a stale name left after the struct field was renamed/removed.", f, kind, typeName)
			}
		})
	}
}

// TestProposalReferenceFieldDetector_NonVacuous is the non-vacuity control:
// synthetic input proves the detectors genuinely flag drift in BOTH directions
// (so the main test above can never be vacuously green because a detector
// silently matched nothing) — the same "prove the test isn't vacuous"
// discipline this codebase applies to every structural drift-guard.
func TestProposalReferenceFieldDetector_NonVacuous(t *testing.T) {
	t.Parallel()

	// (a) extractDepthZeroBacktickTokens: only depth-0 tokens; tokens inside
	// parentheticals (depth >= 1) are excluded; parens INSIDE a backtick token
	// (e.g. `OPEN(<q>)`) do not leak out and change the external depth.
	got := extractDepthZeroBacktickTokens("`a` (`b` | `c`), `d` — see [x](#y); `e` (`OPEN(<q>)`)")
	want := []string{"a", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractDepthZeroBacktickTokens = %v, want %v (depth-0 field-position tokens only; b/c inside parens excluded; OPEN(<q>) backtick parens must not leak)", got, want)
	}

	// (b) jsonFieldNames: reflect a synthetic struct's explicit json tags,
	// excluding `json:"-"` and tagless fields.
	type synth struct {
		Foo  string `json:"foo"`
		Bar  int    `json:"bar"`
		Skip string `json:"-"`
	}
	gotNames := jsonFieldNames(synth{})
	wantNames := []string{"foo", "bar"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("jsonFieldNames = %v, want %v (explicit json names only; `json:\"-\"` excluded)", gotNames, wantNames)
	}

	// (c) diffFieldSets: FORWARD surfaces struct-fields-not-in-doc; REVERSE
	// surfaces doc-tokens-not-in-struct. If either direction were inert the
	// main test would be vacuous.
	missing, stale := diffFieldSets(
		[]string{"a", "b", "x"},     // struct has a, b, x
		[]string{"a", "b", "stale"}, // doc lists a, b, stale
	)
	if len(missing) != 1 || missing[0] != "x" {
		t.Fatalf("FORWARD (struct - doc) must flag %q as missing from doc, got %v — the main test's FORWARD check would be vacuous", "x", missing)
	}
	if len(stale) != 1 || stale[0] != "stale" {
		t.Fatalf("REVERSE (doc - struct) must flag %q as not a real field, got %v — the main test's REVERSE check would be vacuous", "stale", stale)
	}
}

// jsonFieldNames returns the explicit json-tagged field names of v's concrete
// struct type, in declaration order. Fields tagged `json:"-"` or with no
// explicit name (e.g. `json:",omitempty"`) are excluded — every Proposed*
// field carries an explicit name, so those are not wire fields.
func jsonFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var names []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// findProposalSectionBody returns the body lines of the level-2 ("## ") section
// whose heading matches kind. A heading "## <text>" matches kind when text
// equals kind or starts with kind+" " — so "## Conflict (creation)" matches
// kind "Conflict" while "## ConflictTransition" matches "ConflictTransition"
// exactly (ConflictTransition does NOT start with "Conflict "). It fatals if
// zero or more than one heading matches (doc structure changed).
func findProposalSectionBody(t *testing.T, lines []string, kind string) []string {
	t.Helper()
	var headingIdx []int
	for i, line := range lines {
		h := strings.TrimSpace(line)
		if !strings.HasPrefix(h, "## ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(h, "## "))
		if title == kind || strings.HasPrefix(title, kind+" ") {
			headingIdx = append(headingIdx, i)
		}
	}
	if len(headingIdx) == 0 {
		t.Fatalf("no level-2 ## section heading matches kind %q in docs/PROPOSAL-REFERENCE.md — doc structure changed; update the heading or this test's kind map", kind)
	}
	if len(headingIdx) > 1 {
		t.Fatalf("kind %q matches %d level-2 headings in docs/PROPOSAL-REFERENCE.md — the heading-matching rule is now ambiguous and needs tightening", kind, len(headingIdx))
	}
	start := headingIdx[0] + 1
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return lines[start:end]
}

// extractFieldListBlock returns the contiguous non-blank run of lines starting
// at the first line whose trimmed text begins with marker (e.g.
// "**Required:**" or "**Optional:**"), stopping at a blank line, the other
// bold list marker, or a code fence. Returns "" if no such line exists.
func extractFieldListBlock(body []string, marker string) string {
	var out []string
	started := false
	for _, line := range body {
		trim := strings.TrimSpace(line)
		if !started {
			if strings.HasPrefix(trim, marker) {
				started = true
				out = append(out, line)
			}
			continue
		}
		if trim == "" ||
			strings.HasPrefix(trim, "**Required:**") ||
			strings.HasPrefix(trim, "**Optional:**") ||
			strings.HasPrefix(trim, "```") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractDepthZeroBacktickTokens scans text and returns every backtick-wrapped
// token whose OPENING backtick sits at parenthesis depth 0 (outside any "(...)"
// group). Field names in the Required/Optional prose always sit at depth 0;
// enum values, defaults, and inline examples sit inside parentheticals
// (depth >= 1). Parentheses that appear INSIDE a backtick token — e.g.
// `OPEN(<question>)`, `{"name","kind"}` — are treated as inert code and do not
// affect the external depth, so they cannot accidentally demote a real field
// name or promote an enum value to depth 0.
func extractDepthZeroBacktickTokens(text string) []string {
	var tokens []string
	depth := 0
	inTick := false
	start := 0
	tokenDepth := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '`':
			if !inTick {
				inTick = true
				start = i + 1
				tokenDepth = depth
			} else {
				inTick = false
				if tokenDepth == 0 {
					if tok := text[start:i]; tok != "" {
						tokens = append(tokens, tok)
					}
				}
			}
		case inTick:
			// inside a backtick token: parens are inert code, skip
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return tokens
}

// diffFieldSets returns the set difference of struct fields vs doc field tokens
// in BOTH directions: missingFromDoc = struct fields absent from the doc lists
// (FORWARD drift — a wire field the prose forgot), staleInDoc = doc-listed
// tokens absent from the struct (REVERSE drift — a typo or removed field).
func diffFieldSets(structFields, docFields []string) (missingFromDoc, staleInDoc []string) {
	docSet := map[string]bool{}
	for _, f := range docFields {
		docSet[f] = true
	}
	structSet := map[string]bool{}
	for _, f := range structFields {
		structSet[f] = true
	}
	for _, f := range structFields {
		if !docSet[f] {
			missingFromDoc = append(missingFromDoc, f)
		}
	}
	for _, f := range docFields {
		if !structSet[f] {
			staleInDoc = append(staleInDoc, f)
		}
	}
	return missingFromDoc, staleInDoc
}
