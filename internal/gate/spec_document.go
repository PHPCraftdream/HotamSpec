package gate

import (
	"errors"
	"fmt"
	"html"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Document state is private collection data, not graph-authored execution proof.
// The reserved map key also lets an entirely informative document have no rows.
const specDocumentRowKey = "\x00document"

type specDocumentSnapshot struct {
	texts    map[string]ontology.LocalizedText
	examples map[string][]specObservation
	err      error
}

type specDocumentEvidence struct {
	requirementID string
	caseID        string
}

type specDocumentProjection struct {
	sections       []ontology.DocumentSection
	depths         map[string]int
	anchors        map[string]bool
	cases          map[string]ontology.CaseDefinition
	evidence       map[string][]specDocumentEvidence
	qualifications map[string][]specDocumentEvidence
	comparisons    map[string][]specObservation
	data           *specDocumentSnapshot
}

var specDocumentID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var specDocumentFragment = regexp.MustCompile(`\]\(#([^\s)]+)\)`)
var specSectionReference = regexp.MustCompile(`§\s*([0-9]+(?:\.[0-9]+)*)`)
var specDocumentMarkdownLink = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

func hasSpecDocument(g *ontology.Graph) bool {
	return g != nil && g.Conformance != nil && len(g.Conformance.DocumentSections) != 0
}

func collectSpecDocumentSnapshot(g *ontology.Graph, snapshot *AtomExecutionSnapshot, rows map[string]SpecRow) *specDocumentSnapshot {
	data := &specDocumentSnapshot{texts: make(map[string]ontology.LocalizedText), examples: make(map[string][]specObservation)}
	if snapshot.SourceErr != nil {
		data.err = snapshot.SourceErr
		return data
	}
	if snapshot.DiscoveryErr != nil {
		data.err = snapshot.DiscoveryErr
		return data
	}
	if snapshot.SourceIndex == nil {
		data.err = fmt.Errorf("normative document requires the invocation source snapshot")
		return data
	}
	if err := ValidateNormativeDocument(g, snapshot.SourceIndex); err != nil {
		data.err = err
		return data
	}
	selected := make(map[string][]string)
	for _, section := range g.Conformance.DocumentSections {
		for _, block := range section.Blocks {
			if _, found := data.texts[block.TextRef]; !found {
				texts, err := snapshot.SourceIndex.ResolveNormativeText(block.TextRef)
				if err != nil {
					data.err = fmt.Errorf("normative block %s: %w", block.ID, err)
					return data
				}
				data.texts[block.TextRef] = texts
			}
			for _, example := range block.Examples {
				selected[example.CaseID] = example.ComparisonNames
			}
		}
	}
	data.err = collectSpecDocumentExamples(g, rows, selected, data.examples)
	return data
}

func collectSpecDocumentExamples(g *ontology.Graph, rows map[string]SpecRow, selected map[string][]string, examples map[string][]specObservation) error {
	type origin struct{ requirement, entry, test, packageDir string }
	origins := make(map[string]origin)
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		row := rows[requirement.ID]
		if row.sourceError != nil {
			return row.sourceError
		}
		for _, outcome := range row.outcomes {
			if outcome.sourceError != nil {
				return outcome.sourceError
			}
			for _, artifacts := range [][]specArtifact{outcome.artifacts, outcome.failedArtifacts} {
				for _, artifact := range artifacts {
					if artifact.Case == nil {
						continue
					}
					id := artifact.Case.ID
					names, chosen := selected[id]
					if !chosen {
						continue
					}
					if artifact.Verdict != "pass" || !outcome.passed || outcome.problem != "" {
						return fmt.Errorf("selected example %s has no passing execution in the shared snapshot", id)
					}
					observations, err := resolveSpecDocumentComparisons(id, names, artifact.Observations)
					if err != nil {
						return err
					}
					file, _, _ := ParseFileColonSymbol(outcome.entry)
					packageDir := snapshotPackageDir(file)
					if previousOrigin, found := origins[id]; found {
						sameProjection := previousOrigin.test == artifact.Test && previousOrigin.packageDir == packageDir &&
							(previousOrigin.requirement != requirement.ID || previousOrigin.entry != outcome.entry)
						if !sameProjection || !reflect.DeepEqual(examples[id], observations) {
							return fmt.Errorf("selected example %s has ambiguous recorded executions", id)
						}
						continue
					}
					examples[id] = observations
					origins[id] = origin{requirement.ID, outcome.entry, artifact.Test, packageDir}
				}
			}
		}
	}
	for id := range selected {
		if _, observed := origins[id]; !observed {
			return fmt.Errorf("selected example %s was not observed in the shared execution snapshot", id)
		}
	}
	return nil
}

// Names are an authored selector, not a guess based on payload fields, order,
// output kind or display suitability. Every name addresses exactly one record.
func resolveSpecDocumentComparisons(caseID string, names []string, observations []specObservation) ([]specObservation, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("selected example %s has no authored comparison names", caseID)
	}
	indexes := make(map[string]int, len(names))
	for i, name := range names {
		if name == "" || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("selected example %s has an empty or malformed comparison name", caseID)
		}
		if _, duplicate := indexes[name]; duplicate {
			return nil, fmt.Errorf("selected example %s repeats comparison %q", caseID, name)
		}
		indexes[name] = i
	}
	selected := make([]specObservation, len(names))
	found := make([]bool, len(names))
	for _, observation := range observations {
		i, chosen := indexes[observation.Name]
		if !chosen {
			continue
		}
		if found[i] {
			return nil, fmt.Errorf("selected example %s comparison %q is ambiguous", caseID, observation.Name)
		}
		if err := validateSpecDocumentObservation(observation); err != nil {
			return nil, fmt.Errorf("selected example %s comparison %q: %w", caseID, observation.Name, err)
		}
		if observation.RawInput == nil || observation.RawExpected == nil || observation.RawActual == nil ||
			!observation.Passed || !ontology.EqualObservedValues(observation.RawActual, observation.RawExpected) {
			return nil, fmt.Errorf("selected example %s comparison %q has no complete passing input/result comparison", caseID, observation.Name)
		}
		selected[i], found[i] = observation, true
	}
	for i, present := range found {
		if !present {
			return nil, fmt.Errorf("selected example %s comparison %q was not observed in this snapshot", caseID, names[i])
		}
	}
	return selected, nil
}

func validateSpecDocumentObservation(observation specObservation) error {
	for _, field := range [3]struct {
		name  string
		value *ontology.ObservedValue
	}{
		{"input", observation.RawInput},
		{"expected", observation.RawExpected},
		{"actual", observation.RawActual},
	} {
		if field.value != nil {
			if err := field.value.Validate(); err != nil {
				return fmt.Errorf("invalid %s value: %w", field.name, err)
			}
		}
	}
	return nil
}

// prepareSpecDocument validates the whole authored document before choosing a
// locale or focused reading view. No missing block can disappear in a shard.
func prepareSpecDocument(g *ontology.Graph, rows map[string]SpecRow) (*specDocumentProjection, error) {
	data := rows[specDocumentRowKey].document
	if data == nil {
		return nil, fmt.Errorf("normative document has no collected source/execution snapshot")
	}
	if data.err != nil {
		return nil, data.err
	}
	projection := &specDocumentProjection{
		depths: make(map[string]int), anchors: make(map[string]bool),
		cases: make(map[string]ontology.CaseDefinition), evidence: make(map[string][]specDocumentEvidence),
		qualifications: make(map[string][]specDocumentEvidence), comparisons: make(map[string][]specObservation), data: data,
	}
	children := make(map[string][]ontology.DocumentSection)
	sections := make(map[string]ontology.DocumentSection)
	clauses := make(map[string]bool)
	for _, clause := range g.Conformance.Clauses {
		clauses[clause.ID] = false
	}
	refs := make(map[string]string)
	selected := make(map[string]bool)
	addAnchor := func(id string) error {
		if !specDocumentID.MatchString(id) || projection.anchors[id] {
			return fmt.Errorf("normative document has invalid or duplicate anchor %q", id)
		}
		projection.anchors[id] = true
		return nil
	}
	for _, section := range g.Conformance.DocumentSections {
		if err := addAnchor(section.ID); err != nil {
			return nil, err
		}
		sections[section.ID] = section
		children[section.ParentID] = append(children[section.ParentID], section)
		for _, block := range section.Blocks {
			if err := addAnchor(block.ID); err != nil {
				return nil, err
			}
			if previous, found := refs[block.TextRef]; found {
				return nil, fmt.Errorf("normative text %q is authored by both %s and %s; shared text must have one block", block.TextRef, previous, block.ID)
			}
			refs[block.TextRef] = block.ID
			if strings.TrimSpace(block.TextRef) == "" || data.texts[block.TextRef] == nil {
				return nil, fmt.Errorf("normative block %s has unresolved text reference %q", block.ID, block.TextRef)
			}
			switch block.Role {
			case "normative":
				if len(block.ClauseIDs) == 0 {
					return nil, fmt.Errorf("normative block %s has no explicit clauses", block.ID)
				}
			case "informative", "recommendation", "profile_qualification":
			default:
				return nil, fmt.Errorf("normative block %s has unknown role %q", block.ID, block.Role)
			}
			seenClauses := make(map[string]bool)
			for _, id := range block.ClauseIDs {
				if _, found := clauses[id]; !found || seenClauses[id] {
					return nil, fmt.Errorf("normative block %s has unknown or duplicate clause %q", block.ID, id)
				}
				seenClauses[id], clauses[id] = true, true
			}
			for _, example := range block.Examples {
				if selected[example.CaseID] {
					return nil, fmt.Errorf("selected example %s is authored more than once", example.CaseID)
				}
				if len(example.ComparisonNames) == 0 {
					return nil, fmt.Errorf("selected example %s has no authored comparisons", example.CaseID)
				}
				selected[example.CaseID] = true
			}
		}
	}
	for id, represented := range clauses {
		if !represented {
			return nil, fmt.Errorf("source clause %s is omitted from the normative document", id)
		}
	}
	for _, section := range sections {
		if section.ParentID != "" {
			if _, found := sections[section.ParentID]; !found {
				return nil, fmt.Errorf("document section %s has unknown parent %s", section.ID, section.ParentID)
			}
		}
	}
	for parent, siblings := range children {
		sort.SliceStable(siblings, func(i, j int) bool { return siblings[i].Order < siblings[j].Order })
		for i := 1; i < len(siblings); i++ {
			if siblings[i-1].Order == siblings[i].Order {
				return nil, fmt.Errorf("document sections under %q have ambiguous order %d", parent, siblings[i].Order)
			}
		}
		children[parent] = siblings
	}
	var visit func(string, int) error
	visit = func(parent string, depth int) error {
		if len(children[parent]) == 0 {
			return nil
		}
		if depth > 5 {
			return fmt.Errorf("normative document section hierarchy exceeds Markdown heading depth")
		}
		for _, section := range children[parent] {
			projection.sections = append(projection.sections, section)
			projection.depths[section.ID] = depth
			if err := visit(section.ID, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit("", 1); err != nil {
		return nil, err
	}
	if len(projection.sections) != len(sections) {
		return nil, fmt.Errorf("normative document section hierarchy contains a cycle")
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		if row := rows[requirement.ID]; row.sourceError != nil {
			return nil, row.sourceError
		}
		for _, definition := range requirement.Cases {
			item, found := projection.cases[definition.ID]
			if found && !reflect.DeepEqual(item, definition) {
				return nil, fmt.Errorf("case %s has ambiguous declarations", definition.ID)
			}
			projection.cases[definition.ID] = definition
			for _, clauseID := range definition.ClauseIDs {
				projection.evidence[clauseID] = append(projection.evidence[clauseID], specDocumentEvidence{requirementID: requirement.ID, caseID: definition.ID})
			}
		}
		for _, link := range requirement.ClauseLinks {
			projection.qualifications[link.ClauseID] = append(projection.qualifications[link.ClauseID], specDocumentEvidence{requirementID: requirement.ID})
		}
	}
	for _, section := range projection.sections {
		for _, block := range section.Blocks {
			for _, example := range block.Examples {
				id := example.CaseID
				item, found := projection.cases[id]
				if !found || !specIDsIntersect(item.ClauseIDs, block.ClauseIDs) {
					return nil, fmt.Errorf("selected example %s has no explicit witness binding to block %s", id, block.ID)
				}
				comparisons, err := resolveSpecDocumentComparisons(id, example.ComparisonNames, data.examples[id])
				if err != nil {
					return nil, err
				}
				projection.comparisons[id] = comparisons
			}
		}
	}
	return projection, nil
}

func specIDsIntersect(left, right []string) bool {
	for _, a := range left {
		for _, b := range right {
			if a == b {
				return true
			}
		}
	}
	return false
}

func specDocumentLocalized(texts ontology.LocalizedText, language, identity string) (string, error) {
	text, found := texts[language]
	if !found || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("normative document %s has no text for language %q", identity, language)
	}
	return text, nil
}

func specDocumentRelative(base, target string) (string, error) {
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(base)), filepath.FromSlash(target))
	return filepath.ToSlash(relative), err
}

// Fragments always address the complete canonical document. In focused views,
// even references to another package's blocks remain resolvable.
func (p *specDocumentProjection) linkedText(text, canonicalLink string, sectionNumbers map[string]string) (string, error) {
	var fence markdownFence
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if fence.consume(line) || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if !strings.Contains(line, "](#") && !strings.Contains(line, "§") {
			continue
		}
		linked, err := specDocumentOutsideCode(line, func(prose string) (string, error) {
			for _, match := range specDocumentFragment.FindAllStringSubmatch(prose, -1) {
				if !p.anchors[match[1]] {
					return "", fmt.Errorf("normative text references unknown document anchor %q", match[1])
				}
			}
			prose = specDocumentFragment.ReplaceAllString(prose, "]("+canonicalLink+"#$1)")
			return specDocumentLinkSections(prose, canonicalLink, sectionNumbers), nil
		})
		if err != nil {
			return "", err
		}
		lines[i] = linked
	}
	return strings.Join(lines, "\n"), nil
}

func specDocumentOutsideCode(line string, transform func(string) (string, error)) (string, error) {
	var result strings.Builder
	for len(line) > 0 {
		at := strings.IndexByte(line, '`')
		if at == -1 {
			at = len(line)
		}
		prose, err := transform(line[:at])
		if err != nil {
			return "", err
		}
		result.WriteString(prose)
		line = line[at:]
		if line == "" {
			break
		}
		n := 1
		for n < len(line) && line[n] == '`' {
			n++
		}
		end := -1
		for offset := n; offset < len(line); {
			start := strings.IndexByte(line[offset:], '`')
			if start == -1 {
				break
			}
			start += offset
			stop := start + 1
			for stop < len(line) && line[stop] == '`' {
				stop++
			}
			if stop-start == n {
				end = stop
				break
			}
			offset = stop
		}
		if end == -1 {
			// An unmatched opener is literal punctuation, not a code span;
			// later delimiters and prose references still need interpretation.
			result.WriteString(line[:n])
			line = line[n:]
			continue
		}
		result.WriteString(line[:end])
		line = line[end:]
	}
	return result.String(), nil
}

func specDocumentLinkSections(line, canonicalLink string, sectionNumbers map[string]string) string {
	if !strings.Contains(line, "§") {
		return line
	}
	linkPlain := func(prose string) string {
		return specSectionReference.ReplaceAllStringFunc(prose, func(reference string) string {
			parts := specSectionReference.FindStringSubmatch(reference)
			if target := sectionNumbers[parts[1]]; target != "" {
				return "[" + reference + "](" + canonicalLink + "#" + target + ")"
			}
			return reference
		})
	}
	var result strings.Builder
	last := 0
	for _, span := range specDocumentMarkdownLink.FindAllStringIndex(line, -1) {
		result.WriteString(linkPlain(line[last:span[0]]))
		result.WriteString(line[span[0]:span[1]])
		last = span[1]
	}
	result.WriteString(linkPlain(line[last:]))
	return result.String()
}

func renderSpecDocument(g *ontology.Graph, projection *specDocumentProjection, language, basePath, canonicalPath string, focus map[string]bool) (string, error) {
	banner, err := specText(language, specBanner)
	if err != nil {
		return "", err
	}
	title, err := specText(language, "# Domain specification")
	if err != nil {
		return "", err
	}
	intro, err := specText(language, "This document follows the authored language structure. Illustrative examples are explicitly selected and verified in the same execution snapshot; they are not exhaustive proof. Clause witnesses, implementation qualifications and exact observations are available through the evidence links.")
	if err != nil {
		return "", err
	}
	canonicalLink := ""
	if basePath != canonicalPath {
		canonicalLink, err = specDocumentRelative(basePath, canonicalPath)
		if err != nil {
			return "", err
		}
	}
	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		return "", err
	}
	reader, err := specReaderHeaderLine(g, language)
	if err != nil {
		return "", err
	}
	lines := []string{banner}
	if reader != "" {
		lines = append(lines, reader)
	}
	lines = append(lines, "", title, "", intro, "")
	if canonicalLink != "" {
		link, err := specText(language, "[Complete canonical specification](%s)", canonicalLink)
		if err != nil {
			return "", err
		}
		lines = append(lines, link, "")
	}
	visible := make(map[string]bool)
	sectionNumbers := make(map[string]string)
	for _, section := range projection.sections {
		name, err := specDocumentLocalized(section.TitleTexts, language, section.ID)
		if err != nil {
			return "", err
		}
		if match := specSectionReference.FindStringSubmatch(name); match != nil {
			if previous := sectionNumbers[match[1]]; previous != "" {
				return "", fmt.Errorf("document sections %s and %s have the same semantic number %s", previous, section.ID, match[1])
			}
			sectionNumbers[match[1]] = section.ID
		}
		for _, block := range section.Blocks {
			if focus == nil || focus[block.ID] {
				visible[section.ID] = true
			}
		}
		if focus == nil {
			visible[section.ID] = true
		}
	}
	for i := len(projection.sections) - 1; i >= 0; i-- {
		section := projection.sections[i]
		if visible[section.ID] && section.ParentID != "" {
			visible[section.ParentID] = true
		}
	}
	contents, err := specText(language, "## Contents")
	if err != nil {
		return "", err
	}
	lines = append(lines, contents, "")
	for _, section := range projection.sections {
		if !visible[section.ID] {
			continue
		}
		name, _ := specDocumentLocalized(section.TitleTexts, language, section.ID)
		lines = append(lines, strings.Repeat("  ", projection.depths[section.ID]-1)+"- ["+name+"]("+canonicalLink+"#"+section.ID+")")
	}
	lines = append(lines, "")
	for _, section := range projection.sections {
		if !visible[section.ID] {
			continue
		}
		name, _ := specDocumentLocalized(section.TitleTexts, language, section.ID)
		lines = append(lines, `<a id="`+html.EscapeString(section.ID)+`"></a>`, "", strings.Repeat("#", projection.depths[section.ID]+1)+" "+name, "")
		for _, block := range section.Blocks {
			if focus != nil && !focus[block.ID] {
				continue
			}
			text, err := specDocumentLocalized(projection.data.texts[block.TextRef], language, block.ID)
			if err != nil {
				return "", err
			}
			text, err = projection.linkedText(text, canonicalLink, sectionNumbers)
			if err != nil {
				return "", err
			}
			lines = append(lines, `<a id="`+html.EscapeString(block.ID)+`"></a>`, "")
			if block.Role != "normative" {
				var template string
				switch block.Role {
				case "informative":
					template = "**Informative**"
				case "recommendation":
					template = "**Recommendation**"
				case "profile_qualification":
					template = "**Implementation qualification**"
				}
				label, err := localization.Lookup(language, template)
				if err != nil {
					return "", err
				}
				lines = append(lines, label, "")
			}
			lines = append(lines, text, "")
			if block.QualificationTexts != nil {
				qualification, err := specDocumentLocalized(block.QualificationTexts, language, block.ID+" qualification")
				if err != nil {
					return "", err
				}
				qualification, err = projection.linkedText(qualification, canonicalLink, sectionNumbers)
				if err != nil {
					return "", err
				}
				lines = append(lines, qualification, "")
			}
			for _, example := range block.Examples {
				id := example.CaseID
				exampleLink, err := projection.exampleEvidenceLink(layout, language, basePath, id)
				if err != nil {
					return "", err
				}
				for _, observation := range projection.comparisons[id] {
					input, err := specText(language, "**Input**")
					if err != nil {
						return "", err
					}
					expected, err := specText(language, "**Expected result**")
					if err != nil {
						return "", err
					}
					inputBlock, err := specDocumentValueBlock(observation.RawInput, language, exampleLink)
					if err != nil {
						return "", fmt.Errorf("selected example %s input: %w", id, err)
					}
					expectedBlock, err := specDocumentValueBlock(observation.RawExpected, language, exampleLink)
					if err != nil {
						return "", fmt.Errorf("selected example %s expected value: %w", id, err)
					}
					lines = append(lines, input, "")
					lines = append(lines, inputBlock...)
					lines = append(lines, "", expected, "")
					lines = append(lines, expectedBlock...)
					lines = append(lines, "")
				}
				link, err := specText(language, "[Example evidence](%s)", exampleLink)
				if err != nil {
					return "", err
				}
				lines = append(lines, link, "")
			}
			links, err := projection.evidenceLinks(layout, language, basePath, block)
			if err != nil {
				return "", err
			}
			lines = append(lines, links...)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n", nil
}

func (p *specDocumentProjection) evidenceLinks(layout docbundle.Layout, language, basePath string, block ontology.NormativeBlock) ([]string, error) {
	if len(block.ClauseIDs) == 0 {
		return nil, nil
	}
	label, err := specText(language, "Clause evidence")
	if err != nil {
		return nil, err
	}
	var clauses []string
	for _, clauseID := range block.ClauseIDs {
		targets := p.evidence[clauseID]
		if len(targets) == 0 {
			// A clause without scoped execution links to its qualifications;
			// it is never relabelled as a passing witness.
			targets = p.qualifications[clauseID]
		}
		var paths []string
		seen := make(map[string]bool)
		for _, item := range targets {
			var target string
			var err error
			if item.caseID == "" {
				target, err = layout.EvidenceRequirementPath(language, item.requirementID)
			} else {
				target, err = layout.EvidenceCasePath(language, item.requirementID, item.caseID)
			}
			if err != nil {
				return nil, err
			}
			if !seen[target] {
				paths = append(paths, target)
				seen[target] = true
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("document clause %s has no evidence or qualification carrier", clauseID)
		}
		var links []string
		for i, target := range paths {
			relative, err := specDocumentRelative(basePath, target)
			if err != nil {
				return nil, err
			}
			links = append(links, fmt.Sprintf("[%d](%s)", i+1, relative))
		}
		clauses = append(clauses, html.EscapeString(clauseID)+": "+strings.Join(links, ", "))
	}
	return []string{"<details><summary>" + label + "</summary>", "", strings.Join(clauses, "; "), "", "</details>", ""}, nil
}

func (p *specDocumentProjection) exampleEvidenceLink(layout docbundle.Layout, language, basePath, caseID string) (string, error) {
	for _, clauseID := range p.cases[caseID].ClauseIDs {
		for _, item := range p.evidence[clauseID] {
			if item.caseID != caseID {
				continue
			}
			target, err := layout.EvidenceCasePath(language, item.requirementID, caseID)
			if err != nil {
				return "", err
			}
			return specDocumentRelative(basePath, target)
		}
	}
	return "", fmt.Errorf("selected example %s has no exact case evidence page", caseID)
}

func specDocumentValueBlock(value *ontology.ObservedValue, locale, evidenceLink string) ([]string, error) {
	text, source := readableSourceBytes(value)
	language := "ktav"
	escapedSource := false
	if source {
		for _, char := range text {
			if unicode.IsControl(char) && char != '\n' {
				escapedSource = true
				break
			}
		}
	}
	if source {
		if err := value.Validate(); err != nil {
			return nil, err
		}
		if escapedSource {
			// The byte sequence was decoded once above. Display its Unicode
			// text as an escaped string, never as normalized Ktav source.
			text, language = json5String(text), "json5"
		}
	} else {
		var err error
		text, err = ReadableObservedValueForDocument(value)
		if errors.Is(err, ErrOpaqueDocumentBytes) {
			explanation, err := specText(locale, "> **Opaque bytes (not UTF-8).** This typed value contains bytes that cannot be represented as a Unicode Ktav literal or semantic JSON5 value. [Exact typed example evidence](%s)", evidenceLink)
			if err != nil {
				return nil, err
			}
			return []string{explanation}, nil
		}
		if err != nil {
			return nil, err
		}
		language = "json5"
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
	var lines []string
	if escapedSource {
		explanation, err := specText(locale, "> Source text is shown with explicit escapes for control characters; the original byte sequence is preserved in [exact example evidence](%s).", evidenceLink)
		if err != nil {
			return nil, err
		}
		lines = append(lines, explanation, "")
	}
	lines = append(lines, fence+language)
	lines = append(lines, strings.Split(text, "\n")...)
	return append(lines, fence), nil
}
