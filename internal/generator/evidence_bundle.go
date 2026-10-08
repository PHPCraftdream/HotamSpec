package generator

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
)

func reportLayout(g *ontology.Graph) (docbundle.Layout, error) {
	if g == nil {
		return docbundle.NewLayout(nil, "")
	}
	return docbundle.NewLayout(g.Languages, g.DefaultLanguage)
}

func compactEvidenceReport(report evidence.Report) evidence.Report {
	compact := report
	compact.Sources = make([]source.Check, 0)
	seen := make(map[string]bool)
	for _, check := range report.Sources {
		if check.Status != "verified" {
			compact.Sources = append(compact.Sources, check)
			continue
		}
		key := check.SourceID + "\x00" + check.Path + "\x00" + check.Version + "\x00" + check.ExpectedSHA256 + "\x00" + check.ActualSHA256
		if seen[key] {
			continue
		}
		seen[key] = true
		check.Anchor, check.Message = "", ""
		compact.Sources = append(compact.Sources, check)
	}
	return compact
}

// BuildEvidenceBundleLocalized renders an overview, requirement indexes and
// individual case pages from the same immutable execution snapshot.
func BuildEvidenceBundleLocalized(g *ontology.Graph, report evidence.Report, language string) (map[string]string, error) {
	var identity *EvidenceCaseIdentity
	if len(report.Conformance.Cases) != 0 {
		var err error
		identity, err = NewEvidenceCaseIdentity(g, report)
		if err != nil {
			return nil, err
		}
	}
	return BuildEvidenceBundleLocalizedWithIdentity(g, report, language, identity)
}

// BuildEvidenceBundleLocalizedWithIdentity shares provenance across locales.
func BuildEvidenceBundleLocalizedWithIdentity(g *ontology.Graph, report evidence.Report, language string, identity *EvidenceCaseIdentity) (map[string]string, error) {
	language = selectedReportLanguage(g, language)
	layout, err := reportLayout(g)
	if err != nil {
		return nil, err
	}
	documents := make(map[string]string, 1+len(report.Requirements)+len(report.Conformance.Cases))
	overview, err := BuildEvidenceLocalized(g, report, language)
	if err != nil {
		return nil, err
	}
	overviewPath, err := layout.DocumentPath("docs/gen/EVIDENCE.md", language)
	if err != nil {
		return nil, err
	}
	documents[overviewPath] = overview
	sourceChecks := compactEvidenceReport(report).Sources
	casesByAtom := make(map[string][]int)
	for index := range report.Conformance.Cases {
		item := &report.Conformance.Cases[index]
		casesByAtom[item.AtomID] = append(casesByAtom[item.AtomID], index)
		target, err := layout.EvidenceCasePath(language, item.AtomID, item.ID)
		if err != nil {
			return nil, err
		}
		packet := evidence.Report{SchemaVersion: report.SchemaVersion}
		packet.Conformance.Cases = report.Conformance.Cases[index : index+1]
		packet.Conformance.CasesDeclared = 1
		if len(item.Executions) > 0 {
			packet.Conformance.CasesObserved = 1
		}
		contents, err := renderEvidencePage(g, packet, language, target, false)
		if err != nil {
			return nil, err
		}
		contents, err = identity.Stamp(report, *item, language, target, contents)
		if err != nil {
			return nil, err
		}
		if _, duplicate := documents[target]; duplicate {
			return nil, fmt.Errorf("duplicate evidence case identity %s/%s", item.AtomID, item.ID)
		}
		documents[target] = contents
	}
	for _, requirement := range report.Requirements {
		target, err := layout.EvidenceRequirementPath(language, requirement.ID)
		if err != nil {
			return nil, err
		}
		packet := evidence.Report{SchemaVersion: report.SchemaVersion, Sources: sourceChecks}
		for _, clause := range report.Conformance.Clauses {
			for _, atomID := range clause.AtomIDs {
				if atomID == requirement.ID {
					packet.Conformance.Clauses = append(packet.Conformance.Clauses, clause)
					break
				}
			}
		}
		result := requirement
		if len(casesByAtom[requirement.ID]) > 0 {
			result.Cases = nil
			result.Tests = nil
		}
		packet.Requirements = []evidence.RequirementResult{result}
		for _, finding := range report.Findings {
			if finding.RequirementID == requirement.ID {
				packet.Findings = append(packet.Findings, finding)
			}
		}
		contents, err := renderEvidencePage(g, packet, language, target, len(casesByAtom[requirement.ID]) > 0)
		if err != nil {
			return nil, err
		}
		if indexes := casesByAtom[requirement.ID]; len(indexes) > 0 {
			caseIndex, err := renderEvidenceCaseIndex(layout, report.Conformance.Cases, indexes, language, target)
			if err != nil {
				return nil, err
			}
			contents += caseIndex
		}
		documents[target] = contents
	}
	return documents, nil
}

func renderEvidencePage(g *ontology.Graph, report evidence.Report, language, target string, indexedCases bool) (string, error) {
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(target)), filepath.FromSlash("docs/gen"))
	if err != nil {
		return "", err
	}
	var renderErr error
	contents, err := renderLocalizedReport(language, func(text reportText) string {
		result, err := renderEvidence(g, report, language, text, true, filepath.ToSlash(relative), indexedCases)
		renderErr = err
		return result
	})
	if err != nil {
		return "", err
	}
	return contents, renderErr
}

func renderEvidenceCaseIndex(layout docbundle.Layout, cases []conformance.CaseAssessment, indexes []int, language, target string) (string, error) {
	var pathErr error
	contents, renderErr := renderLocalizedReport(language, func(text reportText) string {
		lines := []string{text("## Declared cases and observations"), "", text("| Case | Atom | Clause scope | Test | Profile | Operation | Target | Producer | Sides | Fixtures | Status | Qualification | Input (typed) | Expected (typed) |"), "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|"}
		for _, index := range indexes {
			item := &cases[index]
			casePath, err := layout.EvidenceCasePath(language, item.AtomID, item.ID)
			if err != nil {
				pathErr = err
				return ""
			}
			relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(target)), filepath.FromSlash(casePath))
			if err != nil {
				pathErr = err
				return ""
			}
			lines = append(lines, "| ["+Cell(item.ID)+"]("+filepath.ToSlash(relative)+") | "+Cell(item.AtomID)+" | "+Cell(strings.Join(item.ClauseIDs, ", "))+" | "+Cell(item.Test)+" | "+Cell(item.Profile)+" | "+Cell(item.Operation)+" | "+Cell(item.Target)+" | "+Cell(item.Producer)+" | "+Cell(strings.Join(item.Sides, ", "))+" | "+itoa(len(item.Fixtures))+" | "+Cell(reportStatus(text, item.Status))+" | "+Cell(reportStatus(text, item.Qualification))+" | — | — |")
		}
		return strings.Join(lines, "\n") + "\n"
	})
	if pathErr != nil {
		return "", pathErr
	}
	return contents, renderErr
}

func reportLinkFrom(markdown, prefix string) string {
	if prefix == "" {
		return markdown
	}
	at := strings.LastIndex(markdown, "](")
	if at < 0 || !strings.HasSuffix(markdown, ")") {
		return markdown
	}
	return markdown[:at+2] + path.Join(prefix, markdown[at+2:len(markdown)-1]) + ")"
}
