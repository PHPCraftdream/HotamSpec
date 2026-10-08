package gate

import (
	"encoding/base64"
	"fmt"
	"html"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func renderNormativeRequirement(g *ontology.Graph, row SpecRow, language string) ([]string, error) {
	return renderNormativeRequirementShared(g, row, language, nil)
}

func renderNormativeRequirementShared(g *ontology.Graph, row SpecRow, language string, seen map[string]string) ([]string, error) {
	claim, err := specClaim(g, row.req, language)
	if err != nil {
		return nil, err
	}
	if row.normativeTexts != nil {
		text, ok := row.normativeTexts[language]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("requirement %s has no normative text for language %q", row.req.ID, language)
		}
		claim = text
	}
	heading, body, _ := strings.Cut(claim, "\n")
	anchor := "rule-" + row.req.ID
	lines := []string{"<a id=\"" + html.EscapeString(anchor) + "\"></a>", "", "## " + heading, ""}
	for _, paragraph := range normativeParagraphs(strings.TrimSpace(body)) {
		if previous, ok := seen[paragraph]; ok {
			reference, err := specText(language, "[Shared normative text](#%s)", previous)
			if err != nil {
				return nil, err
			}
			lines = append(lines, reference, "")
			continue
		}
		lines = append(lines, paragraph, "")
		if seen != nil {
			seen[paragraph] = anchor
		}
	}
	methods := make([]string, 0, len(row.req.ImplementedBy))
	for _, reference := range row.req.ImplementedBy {
		_, symbol, ok := ParseFileColonSymbol(reference)
		if ok {
			methods = append(methods, "`"+symbol+"`")
		}
	}
	model, err := specText(language, "**Domain operation:** %s", strings.Join(methods, ", "))
	if err != nil {
		return nil, err
	}
	lines = append(lines, model, "")
	type outcomeCount struct{ pass, fail int }
	cases := make(map[string]outcomeCount)
	var examples []specObservation
	seenExamples := make(map[string]bool)
	var issues []string
	for _, outcome := range row.outcomes {
		if outcome.sourceError != nil {
			return nil, outcome.sourceError
		}
		if outcome.problem != "" {
			issues = append(issues, outcome.problem)
		}
		for _, artifacts := range [][]specArtifact{outcome.artifacts, outcome.failedArtifacts} {
			for _, artifact := range artifacts {
				identity := artifact.Test
				if artifact.Case != nil {
					identity = artifact.Case.ID
				}
				counts := outcomeCount{}
				if artifact.Verdict == "pass" {
					counts.pass = 1
				} else {
					counts.fail = 1
				}
				if previous, exists := cases[identity]; exists {
					if previous.fail > 0 || counts.fail == 0 {
						continue
					}
				}
				cases[identity] = counts
				// Corpus fixtures are a compliance inventory, not new normative prose.
				if artifact.Case != nil && len(artifact.Case.Fixtures) > 0 {
					continue
				}
				for _, observation := range artifact.Observations {
					if observation.RawInput == nil || observation.RawExpected == nil {
						continue
					}
					key := observation.Name + "\x00" + ReadableObservedValue(observation.RawInput) + "\x00" + ReadableObservedValue(observation.RawExpected)
					if seenExamples[key] && observation.Passed {
						continue
					}
					seenExamples[key] = true
					examples = append(examples, observation)
				}
			}
		}
	}
	passed, failed := 0, 0
	for _, counts := range cases {
		passed += counts.pass
		failed += counts.fail
	}
	verification, err := specText(language, "**Executed checks:** %d passed; %d failed. These examples do not establish exhaustive input coverage.", passed, failed)
	if err != nil {
		return nil, err
	}
	lines = append(lines, verification, "")
	for _, issue := range issues {
		label, err := specText(language, "_Evidence issue: %s._", specCell(issue))
		if err != nil {
			return nil, err
		}
		lines = append(lines, label, "")
	}
	if len(examples) > 0 {
		heading, err := specText(language, "### Verified domain examples")
		if err != nil {
			return nil, err
		}
		lines = append(lines, heading, "")
		for _, example := range examples {
			input, err := specText(language, "**Input**")
			if err != nil {
				return nil, err
			}
			expected, err := specText(language, "**Expected result**")
			if err != nil {
				return nil, err
			}
			lines = append(lines, input, "")
			lines = append(lines, domainValueBlock(example.RawInput)...)
			lines = append(lines, "", expected, "")
			lines = append(lines, domainValueBlock(example.RawExpected)...)
			if !example.Passed {
				actual, err := specText(language, "**Observed discrepancy**")
				if err != nil {
					return nil, err
				}
				lines = append(lines, "", actual, "")
				lines = append(lines, domainValueBlock(example.RawActual)...)
			}
			lines = append(lines, "")
		}
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return nil, err
	}
	target, err := layout.EvidenceRequirementPath(language, row.req.ID)
	if err != nil {
		return nil, err
	}
	base, err := layout.SpecIndexPath(language)
	if err != nil {
		return nil, err
	}
	if g.SelfExecutingAtoms {
		base, err = layout.SpecShardPath(language, SpecPackage(row.req)+".md")
		if err != nil {
			return nil, err
		}
	}
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(base)), filepath.FromSlash(target))
	if err != nil {
		return nil, err
	}
	link, err := specText(language, "[Technical verification evidence](%s)", filepath.ToSlash(relative))
	if err != nil {
		return nil, err
	}
	return append(lines, link, ""), nil
}

func domainValueBlock(value *ontology.ObservedValue) []string {
	text, source := readableSourceBytes(value)
	language := "ktav"
	if !source {
		text, language = ReadableObservedValue(value), "json5"
		if value != nil && value.Kind == "bytes" {
			language = "text"
			if bytes, err := base64.StdEncoding.DecodeString(value.Bytes); err == nil {
				text = fmt.Sprintf("hex: % X", bytes)
			} else {
				text = "base64: " + value.Bytes
			}
		}
	}
	longest, run := 0, 0
	for _, char := range text {
		if char == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	lines := []string{fence + language}
	lines = append(lines, strings.Split(text, "\n")...)
	lines = append(lines, fence)
	if source {
		if notes := sourceCodePointNotes(text); notes != "" {
			lines = append(lines, "", "`UTF-8: "+notes+"`")
		}
	}
	return lines
}

func normativeSectionKey(claim string) string {
	if strings.HasPrefix(claim, "§") {
		if at := strings.IndexAny(claim, ": "); at >= 0 {
			return claim[:at]
		}
	}
	return claim
}

func sortNormativeRequirements(requirements []ontology.Requirement) {
	sort.SliceStable(requirements, func(left, right int) bool {
		return naturalSectionLess(normativeSectionKey(requirements[left].Claim), normativeSectionKey(requirements[right].Claim))
	})
}

func naturalSectionLess(left, right string) bool {
	left = strings.TrimPrefix(left, "§")
	right = strings.TrimPrefix(right, "§")
	for left != "" && right != "" {
		leftPart, leftRest, _ := strings.Cut(left, ".")
		rightPart, rightRest, _ := strings.Cut(right, ".")
		leftNumber, leftErr := strconv.Atoi(leftPart)
		rightNumber, rightErr := strconv.Atoi(rightPart)
		if leftErr != nil || rightErr != nil {
			return left < right
		}
		if leftNumber != rightNumber {
			return leftNumber < rightNumber
		}
		left, right = leftRest, rightRest
	}
	return left == "" && right != ""
}

func normativeParagraphs(text string) []string {
	var paragraphs []string
	var lines []string
	var fence markdownFence
	for _, line := range strings.Split(text, "\n") {
		if !fence.consume(line) && strings.TrimSpace(line) == "" {
			if len(lines) > 0 {
				paragraphs = append(paragraphs, strings.Join(lines, "\n"))
				lines = nil
			}
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		paragraphs = append(paragraphs, strings.Join(lines, "\n"))
	}
	return paragraphs
}
