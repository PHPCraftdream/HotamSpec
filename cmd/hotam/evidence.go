package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func cmdEvidence(args []string) error {
	fs := newFlagSet("evidence")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit one machine-readable report document")
	write := fs.Bool("write", false, "write the generated report bundle under docs/gen")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: hotam evidence [--domain <path>] [--json] [--write]")
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	graph, err := loadDomainGraph(domainDir)
	if err != nil {
		return fmt.Errorf("load domain graph: %w", err)
	}

	if _, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage); err != nil {
		return fmt.Errorf("invalid evidence language configuration: %w", err)
	}

	report, collectErr := evidence.Collect(graph)
	machine, encodeErr := encodeEvidenceReport(report)
	if encodeErr != nil {
		return fmt.Errorf("encode evidence report: %w", encodeErr)
	}

	var documents map[string]string
	var defaultDocument string
	if !*asJSON || *write {
		documents, defaultDocument, err = buildEvidenceDocuments(graph, report)
		if err != nil {
			return fmt.Errorf("render evidence bundle: %w", err)
		}
	}
	var writeErr error
	if *write {
		if report.SchemaVersion <= 0 {
			writeErr = fmt.Errorf("collector returned no valid report schema (schema_version=%d)", report.SchemaVersion)
		} else {
			writeErr = writeEvidenceBundle(domainDir, documents, machine)
		}
	}
	if *asJSON {
		if _, err := os.Stdout.Write(machine); err != nil {
			return fmt.Errorf("write evidence report to stdout: %w", err)
		}
	} else {
		if _, err := fmt.Fprint(os.Stdout, defaultDocument); err != nil {
			return fmt.Errorf("write evidence report to stdout: %w", err)
		}
		if *write && writeErr == nil {
			fmt.Fprintf(os.Stderr, "wrote localized EVIDENCE/FINDINGS views and one docs/gen/evidence.json under %s\n", domainDir)
		}
	}

	var commandErr error
	if collectErr != nil {
		commandErr = errors.Join(commandErr, fmt.Errorf("collect evidence: %w", collectErr))
	}
	if writeErr != nil {
		commandErr = errors.Join(commandErr, fmt.Errorf("write evidence bundle: %w", writeErr))
	}
	if reportHasFailures(report) {
		details := "inspect the emitted report or rerun with --write to save docs/gen/EVIDENCE.md and FINDINGS.md"
		if *write {
			details = "inspect docs/gen/EVIDENCE.md and docs/gen/FINDINGS.md"
		}
		commandErr = errors.Join(commandErr, fmt.Errorf("evidence report contains observed discrepancies or source/execution failures; %s", details))
	}
	return commandErr
}

func encodeEvidenceReport(report evidence.Report) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildEvidenceDocuments(graph *ontology.Graph, report evidence.Report) (map[string]string, string, error) {
	layout, err := docbundle.NewLayout(graph.Languages, graph.DefaultLanguage)
	if err != nil {
		return nil, "", err
	}
	defaultLanguage := layout.DefaultLanguage
	if len(graph.Languages) == 1 && defaultLanguage == "" {
		defaultLanguage = graph.Languages[0]
	}
	documents := make(map[string]string, len(layout.LanguagesForViews())*2)
	var defaultDocument string
	for _, language := range layout.LanguagesForViews() {
		view := *graph
		view.RenderLanguage = language
		var evidenceMD, findingsMD string
		if len(graph.Languages) == 0 {
			evidenceMD = generator.BuildEvidence(&view, report)
			findingsMD = generator.BuildFindings(&view, report)
		} else {
			evidenceMD, err = generator.BuildEvidenceLocalized(&view, report, language)
			if err != nil {
				return nil, "", fmt.Errorf("render EVIDENCE for %q: %w", language, err)
			}
			findingsMD, err = generator.BuildFindingsLocalized(&view, report, language)
			if err != nil {
				return nil, "", fmt.Errorf("render FINDINGS for %q: %w", language, err)
			}
		}
		evidencePath, err := layout.DocumentPath("docs/gen/EVIDENCE.md", language)
		if err != nil {
			return nil, "", err
		}
		findingsPath, err := layout.DocumentPath("docs/gen/FINDINGS.md", language)
		if err != nil {
			return nil, "", err
		}
		documents[evidencePath] = evidenceMD
		documents[findingsPath] = findingsMD
		if language == defaultLanguage {
			defaultDocument = evidenceMD
		}
	}
	if defaultDocument == "" {
		return nil, "", fmt.Errorf("no evidence view exists for default language %q", defaultLanguage)
	}
	return documents, defaultDocument, nil
}

func writeEvidenceBundle(domainDir string, documents map[string]string, machine []byte) error {
	genDir := filepath.Join(domainDir, "docs", "gen")
	if err := validateEvidenceDocumentTargets(domainDir, documents); err != nil {
		return err
	}
	paths := make([]string, 0, len(documents)+1)
	for relative := range documents {
		paths = append(paths, filepath.Join(domainDir, filepath.FromSlash(relative)))
	}
	sort.Strings(paths)
	contents := make([][]byte, 0, len(paths)+1)
	for _, path := range paths {
		relative, err := filepath.Rel(domainDir, path)
		if err != nil {
			return err
		}
		contents = append(contents, []byte(documents[filepath.ToSlash(relative)]))
	}
	rawPath := filepath.Join(genDir, "evidence.json")
	paths = append(paths, rawPath)
	contents = append(contents, machine)
	if err := writeFilesParallel(paths, contents); err != nil {
		return err
	}
	stale := docbundle.StalePaths(docbundle.ReportCandidates(genDir), paths, nil)
	for _, path := range stale {
		relative, err := filepath.Rel(domainDir, path)
		if err != nil {
			return err
		}
		owned := generatedReportDocumentPredicate(filepath.ToSlash(relative))
		if owned == nil {
			continue
		}
		exists, generated, err := inspectGeneratedOutput(path, owned)
		if err != nil {
			return err
		}
		if !exists || !generated {
			continue
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove obsolete evidence view %s: %w", path, err)
		}
	}
	return nil
}

func validateEvidenceDocumentTargets(domainDir string, documents map[string]string) error {
	relativePaths := make([]string, 0, len(documents))
	for relative := range documents {
		relativePaths = append(relativePaths, relative)
	}
	sort.Strings(relativePaths)
	for _, relative := range relativePaths {
		owns := generatedReportDocumentPredicate(relative)
		if owns == nil {
			return fmt.Errorf("evidence renderer returned an unowned report path %q", relative)
		}
		path := filepath.Join(domainDir, filepath.FromSlash(relative))
		exists, generated, err := inspectGeneratedOutput(path, owns)
		if err != nil {
			return err
		}
		if exists && !generated {
			return fmt.Errorf("refusing to overwrite authored or unrecognized evidence report %s", path)
		}
	}
	return nil
}

func generatedReportDocumentPredicate(relative string) func(string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if !strings.HasPrefix(clean, "docs/gen/") {
		if strings.Contains(clean, "/") {
			return nil
		}
		clean = filepath.ToSlash(filepath.Join("docs", "gen", clean))
	}
	if !docbundle.OwnedDomainPath(clean) {
		return nil
	}
	name := filepath.Base(clean)
	switch {
	case name == "EVIDENCE.md" || strings.HasPrefix(name, "EVIDENCE.") && strings.HasSuffix(name, ".md"):
		return generator.IsGeneratedEvidence
	case name == "FINDINGS.md" || strings.HasPrefix(name, "FINDINGS.") && strings.HasSuffix(name, ".md"):
		return generator.IsGeneratedFindings
	default:
		return nil
	}
}

func reportHasFailures(report evidence.Report) bool {
	if report.Conformance.HasBlockingIssues() {
		return true
	}
	for _, finding := range report.Findings {
		if finding.Disposition != evidence.FindingDispositionQualification {
			return true
		}
	}
	for _, source := range report.Sources {
		if source.Status != "verified" {
			return true
		}
	}
	for _, req := range report.Requirements {
		if req.CoverageStatus == "discrepancy" {
			return true
		}
		for _, test := range req.Tests {
			if failingVerdict(test.Verdict) {
				return true
			}
			for _, artifact := range test.Artifacts {
				if failingVerdict(artifact.Verdict) || observationsHaveFailures(artifact.Observations) {
					return true
				}
			}
		}
	}
	return false
}

func failingVerdict(verdict string) bool {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "fail", "failed", "failure", "error", "unavailable", "execution_unavailable":
		return true
	default:
		return false
	}
}

func observationsHaveFailures(observations []evidence.Observation) bool {
	for _, observation := range observations {
		if !observation.Passed {
			return true
		}
	}
	return false
}
