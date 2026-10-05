// Package evidence collects fresh, structured runtime evidence from the authored
// methods and tests named by a domain graph. It never writes to the graph and
// never turns a failed or unavailable execution into proof.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
	"path/filepath"
	"sort"
	"strings"
)

// Report is a deterministic snapshot of current observations. It is derived
// afresh by Collect and is never a verdict cache.
type Report struct {
	SchemaVersion int                     `json:"schema_version"`
	Requirements  []RequirementResult     `json:"requirements"`
	Findings      []Finding               `json:"findings"`
	Sources       []source.Check          `json:"sources"`
	Conformance   conformance.Report      `json:"conformance"`
	SpecRows      map[string]gate.SpecRow `json:"-"`
	Limitations   []string                `json:"limitations,omitempty"`
}

type RequirementResult struct {
	ID             string                    `json:"id"`
	Claim          string                    `json:"claim"`
	ClaimTexts     ontology.LocalizedText    `json:"claim_texts,omitempty"`
	AtomKind       string                    `json:"atom_kind,omitempty"`
	Cases          []ontology.CaseDefinition `json:"cases,omitempty"`
	Strength       string                    `json:"strength,omitempty"`
	Applicability  *ontology.Applicability   `json:"applicability,omitempty"`
	ClauseLinks    []ontology.ClauseLink     `json:"clause_links,omitempty"`
	Precedence     []ontology.PrecedenceLink `json:"precedence,omitempty"`
	CoverageStatus string                    `json:"coverage_status"`
	Rationale      string                    `json:"rationale,omitempty"`
	Profile        string                    `json:"profile,omitempty"`
	SourceLinks    []ontology.SourceLink     `json:"source_links,omitempty"`
	Methods        []string                  `json:"methods,omitempty"`
	Tests          []TestEvidence            `json:"tests,omitempty"`
}

type TestEvidence struct {
	Reference string             `json:"reference"`
	Verdict   string             `json:"verdict"`
	Problem   string             `json:"problem,omitempty"`
	Artifacts []ArtifactEvidence `json:"artifacts,omitempty"`
}

type ArtifactEvidence struct {
	Test         string                  `json:"test"`
	CaseID       string                  `json:"case_id,omitempty"`
	Subject      string                  `json:"subject,omitempty"`
	Operation    string                  `json:"operation,omitempty"`
	Target       string                  `json:"target,omitempty"`
	Producer     string                  `json:"producer,omitempty"`
	Input        *ontology.ObservedValue `json:"input,omitempty"`
	Expected     *ontology.ObservedValue `json:"expected,omitempty"`
	Verdict      string                  `json:"verdict"`
	Observations []Observation           `json:"observations,omitempty"`
	Context      Context                 `json:"context,omitempty"`
}

type Observation struct {
	Name        string                  `json:"name"`
	Input       string                  `json:"input,omitempty"`
	Actual      string                  `json:"actual"`
	Expected    string                  `json:"expected"`
	Passed      bool                    `json:"passed"`
	RawInput    *ontology.ObservedValue `json:"raw_input,omitempty"`
	RawActual   *ontology.ObservedValue `json:"raw_actual,omitempty"`
	RawExpected *ontology.ObservedValue `json:"raw_expected,omitempty"`
}

type Context struct {
	Implementation          string               `json:"implementation,omitempty"`
	ImplementationVersion   string               `json:"implementation_version,omitempty"`
	SpecVersion             string               `json:"spec_version,omitempty"`
	Profile                 string               `json:"profile,omitempty"`
	ProfileDetails          *ontology.Profile    `json:"profile_details,omitempty"`
	Operation               string               `json:"operation,omitempty"`
	Target                  string               `json:"target,omitempty"`
	Producer                string               `json:"producer,omitempty"`
	Composition             []ontology.Component `json:"composition,omitempty"`
	MeasuredComponents      []ontology.Component `json:"measured_components,omitempty"`
	ProfileFingerprint      string               `json:"profile_fingerprint,omitempty"`
	ProducerFingerprint     string               `json:"producer_fingerprint,omitempty"`
	CaseFingerprint         string               `json:"case_fingerprint,omitempty"`
	RecordedCaseFingerprint string               `json:"recorded_case_fingerprint,omitempty"`
	DeclaredCaseFingerprint string               `json:"declared_case_fingerprint,omitempty"`
}

type Finding struct {
	ID            string                 `json:"id"`
	RequirementID string                 `json:"requirement_id,omitempty"`
	ClauseID      string                 `json:"clause_id,omitempty"`
	CaseID        string                 `json:"case_id,omitempty"`
	Claim         string                 `json:"claim,omitempty"`
	ClaimTexts    ontology.LocalizedText `json:"claim_texts,omitempty"`
	Test          string                 `json:"test,omitempty"`
	Subject       string                 `json:"subject,omitempty"`
	Kind          string                 `json:"kind"`
	ReviewStatus  string                 `json:"review_status"`
	Disposition   string                 `json:"disposition,omitempty"`
	Rationale     string                 `json:"rationale,omitempty"`
	Profile       string                 `json:"profile,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	Target        string                 `json:"target,omitempty"`
	Producer      string                 `json:"producer,omitempty"`
	SourceLinks   []ontology.SourceLink  `json:"source_links,omitempty"`
	Observations  []Observation          `json:"observations,omitempty"`
	Context       Context                `json:"context,omitempty"`
}

// Finding disposition values classify a conformance finding as blocking or
// non-blocking.
const (
	FindingDispositionQualification = conformance.DispositionQualification
	FindingDispositionViolation     = conformance.DispositionViolation
)

const (
	FindingDiscrepancy          = "discrepancy"
	FindingTestFailure          = "test_failure"
	FindingExecutionUnavailable = "execution_unavailable"
	FindingSourceDrift          = "source_drift"
	FindingConformance          = "conformance_audit"
	ReviewUnreviewed            = "unreviewed"

	VerdictPass       = "pass"
	VerdictFail       = "fail"
	VerdictSkip       = "skip"
	CoverageRetired   = "retired"
	VerdictUnverified = "unverified"
)

// Collect creates one graph-aware package snapshot, then uses it for the
// evidence report and SPEC projection. It never runs a package per locale,
// case, or comparison.
func Collect(g *ontology.Graph) (Report, error) {
	if g == nil {
		return Report{}, errors.New("evidence: nil graph")
	}
	snapshot, err := gate.CollectAtomExecutionSnapshot(g)
	if err != nil {
		return Report{}, err
	}
	return CollectFromSnapshot(g, snapshot)
}

// CollectFromSnapshot derives evidence and SPEC rows from one shared package
// recording result; it does not execute Go.
func CollectFromSnapshot(g *ontology.Graph, snapshot *gate.AtomExecutionSnapshot) (Report, error) {
	if g == nil {
		return Report{}, errors.New("evidence: nil graph")
	}
	if snapshot == nil {
		return Report{}, errors.New("evidence: nil atom execution snapshot")
	}
	report := Report{
		SchemaVersion: 2,
		Requirements:  make([]RequirementResult, 0, len(g.Requirements)),
		Findings:      make([]Finding, 0),
		Sources:       make([]source.Check, 0),
		Conformance:   conformance.Report{},
	}
	authoredCoverageValid := make(map[string]bool, len(g.Requirements))
	coverageErrors := make(map[string]string)
	for i := range g.Requirements {
		r := &g.Requirements[i]
		result := RequirementResult{
			ID: r.ID, Claim: r.Claim, ClaimTexts: cloneClaimTexts(r.ClaimTexts),
			AtomKind: r.AtomKind, Cases: cloneCaseDefinitions(r.Cases),
			Strength: r.Strength, Applicability: cloneApplicability(r.Applicability),
			ClauseLinks:    append([]ontology.ClauseLink(nil), r.ClauseLinks...),
			Precedence:     append([]ontology.PrecedenceLink(nil), r.Precedence...),
			CoverageStatus: ontology.CoverageUnverified,
			Methods:        append([]string(nil), r.ImplementedBy...),
			SourceLinks:    append([]ontology.SourceLink(nil), r.SourceLinks...),
		}
		if r.Coverage != nil {
			result.Rationale, result.Profile = r.Coverage.Rationale, r.Coverage.Profile
		}
		if r.Status == ontology.StatusREJECTED {
			result.CoverageStatus = CoverageRetired
			report.Requirements = append(report.Requirements, result)
			continue
		}
		if err := source.ValidateCoverage(*r); err != nil {
			coverageErrors[r.ID] = err.Error()
		} else if r.Coverage != nil {
			authoredCoverageValid[r.ID] = true
			if isAuthoredQualification(r.Coverage.Status) {
				result.CoverageStatus = r.Coverage.Status
			}
		}
		report.Requirements = append(report.Requirements, result)
	}
	sort.Slice(report.Requirements, func(i, j int) bool { return report.Requirements[i].ID < report.Requirements[j].ID })

	report.Sources = append(report.Sources[:0], source.Verify(g)...)
	sortSourceChecks(report.Sources)
	allSources := append([]source.Check(nil), report.Sources...)
	var findings []Finding
	var limitations []string
	for requirementID, message := range coverageErrors {
		r := requirementByID(g, requirementID)
		finding := baseFinding(r, requirementID, "", "coverage declaration", FindingSourceDrift, message)
		finding.ID = findingID(finding, matchingSources(allSources, sourceIDs(r)))
		findings = append(findings, finding)
		limitations = append(limitations, message)
	}
	for _, check := range allSources {
		if check.Status == source.StatusVerified {
			continue
		}
		linked := linkedRequirements(g, check.SourceID)
		if len(linked) == 0 {
			finding := Finding{
				Subject: check.SourceID, Kind: FindingSourceDrift, ReviewStatus: ReviewUnreviewed,
				Rationale: check.Message, Context: Context{SpecVersion: check.Version},
			}
			finding.ID = findingID(finding, []source.Check{check})
			findings = append(findings, finding)
		} else {
			for _, r := range linked {
				finding := baseFinding(r, r.ID, "", check.SourceID, FindingSourceDrift, check.Message)
				finding.Context.SpecVersion = check.Version
				finding.ID = findingID(finding, matchingSources(allSources, sourceIDs(r)))
				findings = append(findings, finding)
			}
		}
		limitations = append(limitations, check.SourceID+": "+check.Message)
	}

	report.SpecRows = gate.CollectSpecRowsFromSnapshot(g, snapshot)
	packageRuns := snapshot.PackageRuns
	atomIndex := snapshot.SourceIndex
	atomTestFiles := snapshot.TestFiles
	for _, problem := range []struct {
		kind string
		err  error
	}{
		{kind: "atom method descriptions unavailable", err: snapshot.SourceErr},
		{kind: "atom test-name resolution unavailable", err: snapshot.DiscoveryErr},
	} {
		if problem.err == nil {
			continue
		}
		message := problem.kind + ": " + problem.err.Error()
		limitations = append(limitations, message)
		finding := Finding{Test: "spec/model", Subject: "atom discovery", Kind: FindingExecutionUnavailable, ReviewStatus: ReviewUnreviewed, Rationale: message}
		finding.ID = findingID(finding, nil)
		findings = append(findings, finding)
	}
	caseRows := collectCaseExecutions(g, packageRuns, atomTestFiles)

	for i := range report.Requirements {
		result := &report.Requirements[i]
		r := requirementByID(g, result.ID)
		if r == nil {
			continue
		}
		for _, raw := range requirementTestReferences(*r) {
			file, testName, ok := gate.ParseFileColonSymbol(strings.TrimSpace(raw))
			if !ok {
				message := "malformed verified_by test reference"
				testEvidence := TestEvidence{Reference: raw, Verdict: VerdictUnverified, Problem: message}
				result.Tests = append(result.Tests, testEvidence)
				findings = append(findings, executionFinding(*result, raw, message, allSources))
				limitations = append(limitations, r.ID+": "+message+": "+raw)
				continue
			}
			run, found := packageRuns[filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))]
			if !found {
				message := "test package was not executed"
				testEvidence := TestEvidence{Reference: raw, Verdict: VerdictUnverified, Problem: message}
				result.Tests = append(result.Tests, testEvidence)
				findings = append(findings, executionFinding(*result, raw, message, allSources))
				limitations = append(limitations, r.ID+": "+message+": "+raw)
				continue
			}
			testVerdict, executed := terminalVerdict(run.TestVerdicts, testName)
			testEvidence := TestEvidence{Reference: raw, Verdict: VerdictUnverified}
			if executed {
				testEvidence.Verdict = testVerdict
			} else {
				testEvidence.Problem = runProblem(run)
				if testEvidence.Problem == "" {
					testEvidence.Problem = "test produced no terminal Go test action"
				}
				findings = append(findings, executionFinding(*result, raw, testEvidence.Problem, allSources))
				limitations = append(limitations, r.ID+": "+testEvidence.Problem+": "+raw)
			}
			if executed && testVerdict == VerdictSkip {
				testEvidence.Problem = "test was skipped; it provides no executed proof"
				findings = append(findings, executionFinding(*result, raw, testEvidence.Problem, allSources))
				limitations = append(limitations, r.ID+": "+testEvidence.Problem+": "+raw)
			}
			hasLinkedAtom := false
			for _, recorded := range run.Artifacts {
				artifact, err := decodeArtifact(recorded.RawJSON)
				if err != nil || !matchesTest(artifact.Test, testName) {
					continue
				}
				if g.SelfExecutingAtoms && (artifact.Mode == "fact" || artifact.Mode == "holds" || artifact.Mode == "rule") {
					if artifactMatchesRequirement(g, artifact, r) {
						hasLinkedAtom = true
					}
					continue
				}
				if !artifactMatchesRequirement(g, artifact, r) {
					continue
				}
				artifactEvidence, _, problem := makeArtifactEvidence(g, artifact, run.TestVerdicts)
				artifactEvidence.Test = qualifiedTestReference(raw, artifact.Test)
				if problem != "" {
					limitations = append(limitations, r.ID+": "+problem+": "+raw)
				}
				testEvidence.Artifacts = append(testEvidence.Artifacts, artifactEvidence)
				findings = append(findings, observationFindings(*result, artifactEvidence, allSources)...)
				if artifactEvidence.Verdict == VerdictFail && testVerdict != VerdictFail {
					findings = append(findings, testFailureFinding(*result, artifactEvidence.Test, []ArtifactEvidence{artifactEvidence}, allSources))
				}
			}
			if executed && testVerdict == VerdictFail && !(g.SelfExecutingAtoms && hasLinkedAtom) {
				findings = append(findings, testFailureFinding(*result, raw, testEvidence.Artifacts, allSources))
			}
			if executed && testVerdict == VerdictPass && len(testEvidence.Artifacts) == 0 && !hasLinkedAtom {
				testEvidence.Problem = "test passed without recorder observations"
				limitations = append(limitations, r.ID+": "+testEvidence.Problem+": "+raw)
			}
			result.Tests = append(result.Tests, testEvidence)
		}
	}

	if g.SelfExecutingAtoms {
		atomFindings, atomLimitations := collectAtoms(g, atomIndex, atomTestFiles, packageRuns, &report, allSources)
		findings = append(findings, atomFindings...)
		limitations = append(limitations, atomLimitations...)
	}
	report.Conformance = conformance.AuditFromSources(g, caseRows, allSources)
	attachDeclaredCaseContexts(g, report, findings, allSources)
	findings = append(findings, conformanceFindings(g, report, findings, allSources)...)

	for i := range report.Requirements {
		result := &report.Requirements[i]
		r := requirementByID(g, result.ID)
		if r == nil {
			continue
		}
		if r.Status == ontology.StatusREJECTED {
			result.CoverageStatus = CoverageRetired
			continue
		}
		result.CoverageStatus = deriveCoverage(*result, *r, authoredCoverageValid[r.ID], coverageErrors[r.ID] != "", allSources)
		if r.AtomKind == "rule" {
			for _, issue := range report.Conformance.Issues {
				if issue.AtomID == r.ID && issue.Disposition == conformance.DispositionViolation &&
					(issue.Code == conformance.IssueCaseDiscrepancy || issue.Code == conformance.IssuePrecedenceDiscrepancy || issue.Code == conformance.IssueProfileUnsupported) {
					result.CoverageStatus = ontology.CoverageDiscrepancy
					break
				}
			}
		}
	}
	sortReportRequirements(&report)
	report.Findings = uniqueFindings(findings)
	report.Limitations = uniqueStrings(limitations)
	return report, nil
}

func sortReportRequirements(report *Report) {
	sort.Slice(report.Requirements, func(i, j int) bool {
		return report.Requirements[i].ID < report.Requirements[j].ID
	})
	for i := range report.Requirements {
		requirement := &report.Requirements[i]
		sort.Slice(requirement.Tests, func(a, b int) bool {
			left, right := requirement.Tests[a], requirement.Tests[b]
			if left.Reference != right.Reference {
				return left.Reference < right.Reference
			}
			if left.Verdict != right.Verdict {
				return left.Verdict < right.Verdict
			}
			return left.Problem < right.Problem
		})
		for testIndex := range requirement.Tests {
			artifacts := requirement.Tests[testIndex].Artifacts
			sort.SliceStable(artifacts, func(a, b int) bool {
				return artifactEvidenceLess(artifacts[a], artifacts[b])
			})
		}
		sort.Slice(requirement.Cases, func(a, b int) bool {
			left, right := requirement.Cases[a], requirement.Cases[b]
			if left.ID != right.ID {
				return left.ID < right.ID
			}
			if left.Test != right.Test {
				return left.Test < right.Test
			}
			return evidenceFingerprint(left) < evidenceFingerprint(right)
		})
	}
}

func artifactEvidenceLess(a, b ArtifactEvidence) bool {
	if a.CaseID != b.CaseID {
		return a.CaseID < b.CaseID
	}
	if a.Test != b.Test {
		return a.Test < b.Test
	}
	if a.Subject != b.Subject {
		return a.Subject < b.Subject
	}
	if a.Verdict != b.Verdict {
		return a.Verdict < b.Verdict
	}
	for i := 0; i < len(a.Observations) && i < len(b.Observations); i++ {
		left, right := a.Observations[i], b.Observations[i]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Input != right.Input {
			return left.Input < right.Input
		}
		if left.Actual != right.Actual {
			return left.Actual < right.Actual
		}
		if left.Expected != right.Expected {
			return left.Expected < right.Expected
		}
		if left.Passed != right.Passed {
			return !left.Passed
		}
		leftRaw := evidenceFingerprint(struct {
			Input    *ontology.ObservedValue
			Actual   *ontology.ObservedValue
			Expected *ontology.ObservedValue
		}{left.RawInput, left.RawActual, left.RawExpected})
		rightRaw := evidenceFingerprint(struct {
			Input    *ontology.ObservedValue
			Actual   *ontology.ObservedValue
			Expected *ontology.ObservedValue
		}{right.RawInput, right.RawActual, right.RawExpected})
		if leftRaw != rightRaw {
			return leftRaw < rightRaw
		}
	}
	if len(a.Observations) != len(b.Observations) {
		return len(a.Observations) < len(b.Observations)
	}
	if a.Context.Implementation != b.Context.Implementation {
		return a.Context.Implementation < b.Context.Implementation
	}
	if a.Context.ImplementationVersion != b.Context.ImplementationVersion {
		return a.Context.ImplementationVersion < b.Context.ImplementationVersion
	}
	if a.Context.SpecVersion != b.Context.SpecVersion {
		return a.Context.SpecVersion < b.Context.SpecVersion
	}
	if a.Context.Profile != b.Context.Profile {
		return a.Context.Profile < b.Context.Profile
	}
	if a.Context.ProfileFingerprint != b.Context.ProfileFingerprint {
		return a.Context.ProfileFingerprint < b.Context.ProfileFingerprint
	}
	if a.Context.CaseFingerprint != b.Context.CaseFingerprint {
		return a.Context.CaseFingerprint < b.Context.CaseFingerprint
	}
	if a.Context.ProducerFingerprint != b.Context.ProducerFingerprint {
		return a.Context.ProducerFingerprint < b.Context.ProducerFingerprint
	}
	if a.Context.Operation != b.Context.Operation {
		return a.Context.Operation < b.Context.Operation
	}
	if a.Context.Target != b.Context.Target {
		return a.Context.Target < b.Context.Target
	}
	if a.Context.Producer != b.Context.Producer {
		return a.Context.Producer < b.Context.Producer
	}
	return evidenceFingerprint(a.Context.MeasuredComponents) < evidenceFingerprint(b.Context.MeasuredComponents)
}

type decodedArtifact struct {
	ReqID        string                   `json:"req_id"`
	Test         string                   `json:"test"`
	Title        string                   `json:"title"`
	Steps        []decodedStep            `json:"steps"`
	Verdict      string                   `json:"verdict"`
	Mode         string                   `json:"mode"`
	Case         *ontology.CaseDefinition `json:"case,omitempty"`
	CaseInput    *ontology.ObservedValue  `json:"case_input,omitempty"`
	CaseExpected *ontology.ObservedValue  `json:"case_expected,omitempty"`
}

type decodedStep struct {
	Subject      string               `json:"subject"`
	Context      *decodedContext      `json:"context,omitempty"`
	Observations []decodedObservation `json:"observations,omitempty"`
}

type decodedContext struct {
	Implementation        string               `json:"implementation,omitempty"`
	ImplementationVersion string               `json:"implementation_version,omitempty"`
	SpecVersion           string               `json:"spec_version,omitempty"`
	Profile               string               `json:"profile,omitempty"`
	Target                string               `json:"target,omitempty"`
	Operation             string               `json:"operation,omitempty"`
	Producer              string               `json:"producer,omitempty"`
	Components            []ontology.Component `json:"components,omitempty"`
}

type decodedObservation struct {
	Name        string                  `json:"name"`
	Input       string                  `json:"input,omitempty"`
	Actual      string                  `json:"actual"`
	Expected    string                  `json:"expected"`
	Passed      bool                    `json:"passed"`
	RawInput    *ontology.ObservedValue `json:"raw_input,omitempty"`
	RawActual   *ontology.ObservedValue `json:"raw_actual,omitempty"`
	RawExpected *ontology.ObservedValue `json:"raw_expected,omitempty"`
}

func decodeArtifact(raw []byte) (decodedArtifact, error) {
	var artifact decodedArtifact
	err := json.Unmarshal(raw, &artifact)
	return artifact, err
}

func atomTestReference(packageDir, test string, files map[string]map[string]string) string {
	topLevel := test
	if slash := strings.IndexByte(topLevel, '/'); slash >= 0 {
		topLevel = topLevel[:slash]
	}
	if file := files[packageDir][topLevel]; file != "" {
		return file + ":" + test
	}
	return test
}

func terminalVerdict(verdicts []gate.TestVerdict, test string) (string, bool) {
	for _, verdict := range verdicts {
		if verdict.Test == test {
			return verdict.Verdict, true
		}
	}
	return "", false
}

func runProblem(run gate.RecordingResult) string {
	switch {
	case run.Skipped:
		return run.InfraWarning
	case run.CompileFailed:
		return "test package did not compile: " + strings.TrimSpace(run.Output)
	case run.Err != nil:
		return "test execution unavailable: " + run.Err.Error()
	default:
		return ""
	}
}

func qualifiedTestReference(reference, actual string) string {
	file, test, ok := gate.ParseFileColonSymbol(reference)
	if ok && matchesTest(actual, test) {
		return file + ":" + actual
	}
	return reference
}
func matchesTest(actual, requested string) bool {
	return actual == requested || strings.HasPrefix(actual, requested+"/")
}

func artifactMatchesRequirement(g *ontology.Graph, artifact decodedArtifact, r *ontology.Requirement) bool {
	if artifact.Case != nil {
		return containsAtomID(reportAtomIDs(g, artifact), r.ID)
	}
	if artifact.ReqID == r.ID {
		return true
	}
	if artifact.Mode != "fact" && artifact.Mode != "holds" && artifact.Mode != "rule" {
		return false
	}
	for _, step := range artifact.Steps {
		for _, method := range r.ImplementedBy {
			if methodMatches(step.Subject, method) {
				return true
			}
		}
	}
	return false
}

func methodMatches(subject, entry string) bool {
	_, symbol, ok := gate.ParseFileColonSymbol(entry)
	if !ok || symbol == "" {
		return false
	}
	return subject == symbol || strings.HasSuffix(subject, "."+symbol)
}

func makeArtifactEvidence(g *ontology.Graph, artifact decodedArtifact, verdicts []gate.TestVerdict) (ArtifactEvidence, string, string) {
	result := ArtifactEvidence{Test: artifact.Test, Verdict: VerdictUnverified}
	terminal, hasTerminal := terminalVerdict(verdicts, artifact.Test)
	if hasTerminal && (terminal == VerdictPass || terminal == VerdictFail || terminal == VerdictSkip) {
		result.Verdict = terminal
	}
	if artifact.Verdict == VerdictFail && result.Verdict != VerdictSkip {
		result.Verdict = VerdictFail
	}
	if artifact.Case != nil {
		result.CaseID = artifact.Case.ID
		result.Operation = artifact.Case.Operation
		result.Target = artifact.Case.Target
		result.Producer = artifact.Case.Producer
		result.Input = cloneObservedValue(artifact.CaseInput)
		result.Expected = cloneObservedValue(artifact.CaseExpected)
	}
	var recordedContext *decodedContext
	for _, step := range artifact.Steps {
		if result.Subject == "" && step.Subject != "" {
			result.Subject = step.Subject
		}
		if recordedContext == nil && step.Context != nil {
			recordedContext = step.Context
		}
		for _, observation := range step.Observations {
			result.Observations = append(result.Observations, Observation{
				Name: observation.Name, Input: observation.Input, Actual: observation.Actual,
				Expected: observation.Expected, Passed: observation.Passed,
				RawInput:    cloneObservedValue(observation.RawInput),
				RawActual:   cloneObservedValue(observation.RawActual),
				RawExpected: cloneObservedValue(observation.RawExpected),
			})
		}
	}
	result.Context = artifactContext(g, artifact, recordedContext)
	problem := ""
	if !hasTerminal {
		problem = "artifact has no terminal per-test Go verdict"
	}
	return result, terminal, problem
}

func observationFindings(requirement RequirementResult, artifact ArtifactEvidence, checks []source.Check) []Finding {
	var out []Finding
	for _, observation := range artifact.Observations {
		if observation.Passed {
			continue
		}
		subject := artifact.Subject
		if subject == "" && len(requirement.Methods) > 0 {
			_, symbol, ok := gate.ParseFileColonSymbol(requirement.Methods[0])
			if ok {
				subject = symbol
			}
		}
		profile := requirement.Profile
		operation, target, producer := artifact.Context.Operation, artifact.Context.Target, artifact.Context.Producer
		if definition := caseForTest(requirement.Cases, artifact.Test); definition != nil {
			profile, operation = definition.Profile, definition.Operation
			target, producer = definition.Target, definition.Producer
		}
		finding := Finding{
			RequirementID: requirement.ID, CaseID: artifact.CaseID,
			Claim: requirement.Claim, ClaimTexts: cloneClaimTexts(requirement.ClaimTexts),
			Test: artifact.Test, Subject: subject, Kind: FindingDiscrepancy,
			ReviewStatus: ReviewUnreviewed,
			Rationale:    "recorded comparison did not match its expected value",
			Profile:      profile, Operation: operation,
			Target: target, Producer: producer,
			SourceLinks:  append([]ontology.SourceLink(nil), requirement.SourceLinks...),
			Observations: []Observation{observation}, Context: artifact.Context,
		}
		finding.ID = findingID(finding, matchingSources(checks, sourceIDsFromLinks(requirement.SourceLinks)))
		out = append(out, finding)
	}
	return out
}

func collectAtoms(g *ontology.Graph, index *gate.AtomSourceIndex, testFiles map[string]map[string]string, runs map[string]gate.RecordingResult, report *Report, checks []source.Check) ([]Finding, []string) {
	var findings []Finding
	var limitations []string
	syntheticIDs := make(map[string]bool)
	syntheticSubjects := make(map[string]map[string]bool)
	for _, dir := range sortedRunDirs(runs) {
		run := runs[dir]
		seenAtomTests := make(map[string]bool)
		for _, recorded := range run.Artifacts {
			artifact, err := decodeArtifact(recorded.RawJSON)
			if err != nil {
				message := "recorder artifact could not be decoded: " + err.Error()
				finding := Finding{Test: dir, Subject: "artifact", Kind: FindingExecutionUnavailable, ReviewStatus: ReviewUnreviewed, Rationale: message}
				finding.ID = findingID(finding, nil)
				findings = append(findings, finding)
				limitations = append(limitations, message)
				continue
			}
			if artifact.Mode != "fact" && artifact.Mode != "holds" && artifact.Mode != "rule" {
				continue
			}
			seenAtomTests[artifact.Test] = true
			retiredIDs := retiredAtomIDs(g, artifact)
			linked := atomLinkedRequirements(g, report, artifact)
			if artifact.Case == nil {
				if exact := requirementByID(g, artifact.ReqID); exact != nil && exact.Status != ontology.StatusREJECTED {
					if result := findReportRequirement(report, exact.ID); result != nil {
						linked = appendUniqueRequirement(linked, result)
					}
				}
			}
			artifactEvidence, _, problem := makeArtifactEvidence(g, artifact, run.TestVerdicts)
			artifactEvidence.Test = atomTestReference(dir, artifact.Test, testFiles)
			if problem != "" {
				limitations = append(limitations, artifactEvidence.Test+": "+problem)
			}
			for _, atomID := range retiredIDs {
				if result := findReportRequirement(report, atomID); result != nil {
					appendAtomTest(result, artifactEvidence)
				}
			}
			if len(linked) == 0 && len(retiredIDs) > 0 {
				continue
			}
			if len(linked) == 0 {
				reportIDs := reportAtomIDs(g, artifact)
				if len(reportIDs) == 0 {
					limitations = append(limitations, "report-only recorder artifact has no stable atom ID")
					continue
				}
				methods := atomSubjects(artifact)
				claimTexts, claim := reportOnlyAtomClaims(g, index, recorded.RawJSON, methods)
				for _, atomID := range reportIDs {
					subjects := syntheticSubjects[atomID]
					if subjects == nil {
						subjects = make(map[string]bool)
						syntheticSubjects[atomID] = subjects
					}
					for _, subject := range methods {
						subjects[subject] = true
					}
					result := findReportRequirement(report, atomID)
					if result == nil {
						result = appendRequirementResult(report, RequirementResult{
							ID: atomID, CoverageStatus: ontology.CoverageUnverified,
						})
						limitations = append(limitations, "atom "+atomID+" is report-only; it does not create a graph requirement or normative SPEC claim")
					}
					result.Claim = claim
					result.ClaimTexts = cloneClaimTexts(claimTexts)
					result.Methods = sortedSet(subjects)
					if artifact.Mode == "rule" {
						result.AtomKind = "rule"
					}
					if artifact.Case != nil {
						caseDefinition := *artifact.Case
						caseDefinition.Test = artifactEvidence.Test
						caseDefinition.Input = cloneObservedValue(artifact.CaseInput)
						caseDefinition.Expected = cloneObservedValue(artifact.CaseExpected)
						result.Cases = append(result.Cases, caseDefinition)
					}
					syntheticIDs[atomID] = true
					appendAtomTest(result, artifactEvidence)
					findings = append(findings, observationFindings(*result, artifactEvidence, checks)...)
					if artifactEvidence.Verdict == VerdictFail {
						findings = append(findings, testFailureFinding(*result, artifactEvidence.Test, []ArtifactEvidence{artifactEvidence}, checks))
					}
					if artifactEvidence.Verdict == VerdictUnverified || artifactEvidence.Verdict == VerdictSkip {
						message := "atom test was not executed to a terminal passing verdict"
						findings = append(findings, executionFinding(*result, artifactEvidence.Test, message, checks))
					}
				}
				continue
			}
			for _, result := range linked {
				if syntheticIDs[result.ID] && artifact.Case != nil {
					caseDefinition := *artifact.Case
					caseDefinition.Test = artifactEvidence.Test
					caseDefinition.Input = cloneObservedValue(artifact.CaseInput)
					caseDefinition.Expected = cloneObservedValue(artifact.CaseExpected)
					result.Cases = append(result.Cases, caseDefinition)
				}
				appendAtomTest(result, artifactEvidence)
				findings = append(findings, observationFindings(*result, artifactEvidence, checks)...)
				if artifactEvidence.Verdict == VerdictFail {
					findings = append(findings, testFailureFinding(*result, artifactEvidence.Test, []ArtifactEvidence{artifactEvidence}, checks))
				}
				if artifactEvidence.Verdict == VerdictUnverified || artifactEvidence.Verdict == VerdictSkip {
					message := "atom test was not executed to a terminal passing verdict"
					findings = append(findings, executionFinding(*result, artifactEvidence.Test, message, checks))
				}
			}
		}
		for _, verdict := range run.TestVerdicts {
			if seenAtomTests[verdict.Test] {
				continue
			}
			testReference := atomTestReference(dir, verdict.Test, testFiles)
			switch verdict.Verdict {
			case VerdictFail:
				finding := Finding{Test: testReference, Subject: "test", Kind: FindingTestFailure, ReviewStatus: ReviewUnreviewed, Rationale: "the Go test reported a failing terminal verdict"}
				finding.ID = findingID(finding, nil)
				findings = append(findings, finding)
			case VerdictSkip:
				finding := Finding{Test: testReference, Subject: "test execution", Kind: FindingExecutionUnavailable, ReviewStatus: ReviewUnreviewed, Rationale: "the Go test was skipped and provides no executed proof"}
				finding.ID = findingID(finding, nil)
				findings = append(findings, finding)
			}
		}
		if message := runProblem(run); message != "" {
			finding := Finding{Test: dir, Subject: "package", Kind: FindingExecutionUnavailable, ReviewStatus: ReviewUnreviewed, Rationale: message}
			finding.ID = findingID(finding, nil)
			findings = append(findings, finding)
			limitations = append(limitations, dir+": "+message)
		}
	}
	for id := range syntheticIDs {
		result := findReportRequirement(report, id)
		if result != nil {
			var r ontology.Requirement
			r.ID = result.ID
			r.ImplementedBy = result.Methods
			r.AtomKind = result.AtomKind
			result.CoverageStatus = deriveCoverage(*result, r, false, false, checks)
		}
	}
	sort.Slice(report.Requirements, func(i, j int) bool { return report.Requirements[i].ID < report.Requirements[j].ID })
	return findings, limitations
}

func reportOnlyAtomClaim(subjects []string, index *gate.AtomSourceIndex) string {
	phrases := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		phrase := ""
		if index != nil {
			if resolved, err := index.Resolve(subject); err == nil {
				phrase = strings.TrimSpace(resolved.Phrase)
			}
		}
		if phrase == "" {
			phrase = "method " + subject
		}
		phrases = append(phrases, phrase)
	}
	sort.Strings(phrases)
	return "Report-only atom method description: " + strings.Join(phrases, "; ")
}

func sortedSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func atomSubjects(artifact decodedArtifact) []string {
	seen := make(map[string]bool)
	var subjects []string
	for _, step := range artifact.Steps {
		if step.Subject != "" && !seen[step.Subject] {
			seen[step.Subject] = true
			subjects = append(subjects, step.Subject)
		}
	}
	sort.Strings(subjects)
	return subjects
}

func atomLinkedRequirements(g *ontology.Graph, report *Report, artifact decodedArtifact) []*RequirementResult {
	var out []*RequirementResult
	if artifact.Case != nil {
		for _, atomID := range reportAtomIDs(g, artifact) {
			if result := findReportRequirement(report, atomID); result != nil {
				out = appendUniqueRequirement(out, result)
			}
		}
		return out
	}
	for i := range g.Requirements {
		r := &g.Requirements[i]
		if r.Status == ontology.StatusREJECTED {
			continue
		}
		for _, step := range artifact.Steps {
			matched := false
			for _, method := range r.ImplementedBy {
				if methodMatches(step.Subject, method) {
					matched = true
					break
				}
			}
			if matched {
				if result := findReportRequirement(report, r.ID); result != nil {
					out = appendUniqueRequirement(out, result)
				}
				break
			}
		}
	}
	return out
}

func findReportRequirement(report *Report, id string) *RequirementResult {
	for i := range report.Requirements {
		if report.Requirements[i].ID == id {
			return &report.Requirements[i]
		}
	}
	return nil
}

func appendUniqueRequirement(items []*RequirementResult, item *RequirementResult) []*RequirementResult {
	for _, existing := range items {
		if existing.ID == item.ID {
			return items
		}
	}
	return append(items, item)
}

func appendRequirementResult(report *Report, result RequirementResult) *RequirementResult {
	if existing := findReportRequirement(report, result.ID); existing != nil {
		return existing
	}
	report.Requirements = append(report.Requirements, result)
	return &report.Requirements[len(report.Requirements)-1]
}

func appendAtomTest(result *RequirementResult, artifact ArtifactEvidence) {
	for i := range result.Tests {
		if referenceMatchesTest(result.Tests[i].Reference, artifact.Test) {
			result.Tests[i].Artifacts = append(result.Tests[i].Artifacts, artifact)
			if artifact.Verdict == VerdictFail {
				result.Tests[i].Verdict = VerdictFail
			} else if result.Tests[i].Verdict == VerdictUnverified && artifact.Verdict == VerdictPass {
				result.Tests[i].Verdict = VerdictPass
			}
			return
		}
	}
	result.Tests = append(result.Tests, TestEvidence{
		Reference: artifact.Test, Verdict: artifact.Verdict,
		Problem: atomProblem(artifact.Verdict), Artifacts: []ArtifactEvidence{artifact},
	})
}

func referenceMatchesTest(reference, test string) bool {
	if reference == test || strings.HasPrefix(test, reference+"/") {
		return true
	}
	_, symbol, ok := gate.ParseFileColonSymbol(reference)
	return ok && (test == symbol || strings.HasPrefix(test, symbol+"/"))
}

func atomProblem(verdict string) string {
	if verdict == VerdictUnverified {
		return "no terminal per-test Go verdict"
	}
	if verdict == VerdictSkip {
		return "atom test was skipped"
	}
	return ""
}

func sortedRunDirs(runs map[string]gate.RecordingResult) []string {
	dirs := make([]string, 0, len(runs))
	for dir := range runs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func deriveCoverage(result RequirementResult, r ontology.Requirement, authoredValid, authoredInvalid bool, checks []source.Check) string {
	if authoredInvalid || hasFailedTest(result.Tests) || hasFailedObservation(result.Tests) || hasLinkedSourceFailure(r, checks) {
		return ontology.CoverageDiscrepancy
	}
	if authoredValid && r.Coverage != nil && isAuthoredQualification(r.Coverage.Status) {
		return r.Coverage.Status
	}
	if r.AtomKind == "rule" {
		return ontology.CoverageUnverified
	}
	if hasPassingObservation(result.Tests) && allTestsPass(result.Tests) && linkedSourcesVerified(r, checks) {
		return ontology.CoverageVerified
	}
	return ontology.CoverageUnverified
}

func isAuthoredQualification(status string) bool {
	return status == ontology.CoverageUnsupported || status == ontology.CoverageUnreachable || status == ontology.CoverageUnverified
}

func hasFailedTest(tests []TestEvidence) bool {
	for _, test := range tests {
		if test.Verdict == VerdictFail {
			return true
		}
		for _, artifact := range test.Artifacts {
			if artifact.Verdict == VerdictFail {
				return true
			}
		}
	}
	return false
}

func hasFailedObservation(tests []TestEvidence) bool {
	for _, test := range tests {
		for _, artifact := range test.Artifacts {
			for _, observation := range artifact.Observations {
				if !observation.Passed {
					return true
				}
			}
		}
	}
	return false
}

func hasPassingObservation(tests []TestEvidence) bool {
	for _, test := range tests {
		for _, artifact := range test.Artifacts {
			for _, observation := range artifact.Observations {
				if observation.Passed {
					return true
				}
			}
		}
	}
	return false
}

func allTestsPass(tests []TestEvidence) bool {
	if len(tests) == 0 {
		return false
	}
	for _, test := range tests {
		if test.Verdict != VerdictPass {
			return false
		}
	}
	return true
}

func hasLinkedSourceFailure(r ontology.Requirement, checks []source.Check) bool {
	for _, link := range r.SourceLinks {
		for _, check := range checks {
			if check.SourceID == link.SourceID && check.Status != source.StatusVerified {
				return true
			}
		}
	}
	return false
}

func linkedSourcesVerified(r ontology.Requirement, checks []source.Check) bool {
	for _, link := range r.SourceLinks {
		found := false
		for _, check := range checks {
			if check.SourceID != link.SourceID {
				continue
			}
			found = true
			if check.Status != source.StatusVerified {
				return false
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func executionFinding(result RequirementResult, test, message string, checks []source.Check) Finding {
	finding := Finding{
		RequirementID: result.ID, Claim: result.Claim,
		ClaimTexts: cloneClaimTexts(result.ClaimTexts), Test: test,
		Subject: "test execution", Kind: FindingExecutionUnavailable,
		ReviewStatus: ReviewUnreviewed, Rationale: message,
		Profile: result.Profile, SourceLinks: append([]ontology.SourceLink(nil), result.SourceLinks...),
	}
	if definition := caseForTest(result.Cases, test); definition != nil {
		finding.CaseID = definition.ID
		finding.Operation = definition.Operation
		finding.Target = definition.Target
		finding.Producer = definition.Producer
	}
	finding.ID = findingID(finding, matchingSources(checks, sourceIDsFromLinks(result.SourceLinks)))
	return finding
}

func testFailureFinding(result RequirementResult, test string, artifacts []ArtifactEvidence, checks []source.Check) Finding {
	finding := Finding{
		RequirementID: result.ID, Claim: result.Claim,
		ClaimTexts: cloneClaimTexts(result.ClaimTexts), Test: test, Subject: "test",
		Kind: FindingTestFailure, ReviewStatus: ReviewUnreviewed,
		Rationale: "the Go test reported a failing terminal verdict",
		Profile:   result.Profile, SourceLinks: append([]ontology.SourceLink(nil), result.SourceLinks...),
	}
	ordered := append([]ArtifactEvidence(nil), artifacts...)
	sort.SliceStable(ordered, func(i, j int) bool { return artifactEvidenceLess(ordered[i], ordered[j]) })
	var caseID, operation, target, producer string
	caseSet, operationSet, targetSet, producerSet := false, false, false, false
	caseConflict, operationConflict, targetConflict, producerConflict := false, false, false, false
	for _, artifact := range ordered {
		if finding.Subject == "test" && artifact.Subject != "" {
			finding.Subject = artifact.Subject
		}
		finding.Observations = append(finding.Observations, artifact.Observations...)
		if contextEmpty(finding.Context) && !contextEmpty(artifact.Context) {
			finding.Context = artifact.Context
		}
		updateIdentity(&caseID, &caseSet, &caseConflict, artifact.CaseID)
		updateIdentity(&operation, &operationSet, &operationConflict, artifact.Operation)
		updateIdentity(&target, &targetSet, &targetConflict, artifact.Target)
		updateIdentity(&producer, &producerSet, &producerConflict, artifact.Producer)
	}
	if caseSet {
		finding.CaseID = caseID
	}
	if operationSet {
		finding.Operation = operation
	}
	if targetSet {
		finding.Target = target
	}
	if producerSet {
		finding.Producer = producer
	}
	if finding.Subject == "test" && len(result.Methods) > 0 {
		if _, symbol, ok := gate.ParseFileColonSymbol(result.Methods[0]); ok {
			finding.Subject = symbol
		}
	}
	finding.ID = findingID(finding, matchingSources(checks, sourceIDsFromLinks(result.SourceLinks)))
	return finding
}

func contextEmpty(ctx Context) bool {
	return ctx.Implementation == "" && ctx.ImplementationVersion == "" && ctx.SpecVersion == "" &&
		ctx.Profile == "" && ctx.ProfileDetails == nil && ctx.Operation == "" && ctx.Target == "" &&
		ctx.Producer == "" && len(ctx.Composition) == 0 && len(ctx.MeasuredComponents) == 0 &&
		ctx.ProfileFingerprint == "" && ctx.ProducerFingerprint == "" && ctx.CaseFingerprint == "" &&
		ctx.RecordedCaseFingerprint == "" && ctx.DeclaredCaseFingerprint == ""
}

func baseFinding(r *ontology.Requirement, requirementID, test, subject, kind, rationale string) Finding {
	finding := Finding{RequirementID: requirementID, Test: test, Subject: subject, Kind: kind, ReviewStatus: ReviewUnreviewed, Rationale: rationale}
	if r != nil {
		finding.Claim = r.Claim
		finding.ClaimTexts = cloneClaimTexts(r.ClaimTexts)
		finding.SourceLinks = append([]ontology.SourceLink(nil), r.SourceLinks...)
		if r.Coverage != nil {
			finding.Profile = r.Coverage.Profile
		}
	}
	return finding
}

func updateIdentity(current *string, hasValue, conflict *bool, candidate string) {
	if *conflict || candidate == "" {
		return
	}
	if !*hasValue {
		*current = candidate
		*hasValue = true
		return
	}
	if *current != candidate {
		*current = ""
		*hasValue = false
		*conflict = true
	}
}

type legacySourceCheck struct {
	SourceID       string `json:"source_id"`
	Path           string `json:"path"`
	Version        string `json:"version"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256,omitempty"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

func legacySourceChecks(checks []source.Check) []legacySourceCheck {
	ordered := append([]source.Check(nil), checks...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		if a.ExpectedSHA256 != b.ExpectedSHA256 {
			return a.ExpectedSHA256 < b.ExpectedSHA256
		}
		return a.ActualSHA256 < b.ActualSHA256
	})
	out := make([]legacySourceCheck, len(ordered))
	for i, check := range ordered {
		out[i] = legacySourceCheck{
			SourceID: check.SourceID, Path: check.Path, Version: check.Version,
			ExpectedSHA256: check.ExpectedSHA256, ActualSHA256: check.ActualSHA256,
			Status: check.Status, Message: check.Message,
		}
	}
	return out
}

type legacyObservation struct {
	Name     string `json:"name"`
	Input    string `json:"input,omitempty"`
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
}

type legacyContext struct {
	Implementation        string `json:"implementation,omitempty"`
	ImplementationVersion string `json:"implementation_version,omitempty"`
	SpecVersion           string `json:"spec_version,omitempty"`
	Profile               string `json:"profile,omitempty"`
}

func legacyObservations(observations []Observation) []legacyObservation {
	if observations == nil {
		return nil
	}
	out := make([]legacyObservation, len(observations))
	for i, observation := range observations {
		out[i] = legacyObservation{
			Name: observation.Name, Input: observation.Input, Actual: observation.Actual,
			Expected: observation.Expected, Passed: observation.Passed,
		}
	}
	return out
}

func legacyFindingContext(context Context) legacyContext {
	return legacyContext{
		Implementation: context.Implementation, ImplementationVersion: context.ImplementationVersion,
		SpecVersion: context.SpecVersion, Profile: context.Profile,
	}
}

func findingID(finding Finding, checks []source.Check) string {
	var identity any
	if finding.CaseID == "" && finding.ClauseID == "" && len(finding.ClaimTexts) == 0 && finding.Disposition == "" &&
		finding.Operation == "" && finding.Target == "" && finding.Producer == "" {
		identity = struct {
			RequirementID string                `json:"requirement_id"`
			Claim         string                `json:"claim"`
			Test          string                `json:"test"`
			Subject       string                `json:"subject"`
			Kind          string                `json:"kind"`
			Rationale     string                `json:"rationale"`
			Profile       string                `json:"profile"`
			SourceLinks   []ontology.SourceLink `json:"source_links"`
			Observations  []legacyObservation   `json:"observations"`
			Context       legacyContext         `json:"context"`
			Sources       []legacySourceCheck   `json:"sources"`
		}{
			RequirementID: finding.RequirementID, Claim: finding.Claim, Test: finding.Test,
			Subject: finding.Subject, Kind: finding.Kind, Rationale: finding.Rationale,
			Profile: finding.Profile, SourceLinks: finding.SourceLinks,
			Observations: legacyObservations(finding.Observations), Context: legacyFindingContext(finding.Context), Sources: legacySourceChecks(checks),
		}
	} else {
		claim := finding.Claim
		if len(finding.ClaimTexts) > 0 {
			claim = ""
		}
		identity = struct {
			RequirementID string                 `json:"requirement_id"`
			ClauseID      string                 `json:"clause_id"`
			CaseID        string                 `json:"case_id"`
			Claim         string                 `json:"claim"`
			ClaimTexts    ontology.LocalizedText `json:"claim_texts,omitempty"`
			Test          string                 `json:"test"`
			Subject       string                 `json:"subject"`
			Kind          string                 `json:"kind"`
			Disposition   string                 `json:"disposition"`
			Rationale     string                 `json:"rationale"`
			Profile       string                 `json:"profile"`
			Operation     string                 `json:"operation"`
			Target        string                 `json:"target"`
			Producer      string                 `json:"producer"`
			SourceLinks   []ontology.SourceLink  `json:"source_links"`
			Observations  []Observation          `json:"observations"`
			Context       Context                `json:"context"`
			Sources       []source.Check         `json:"sources"`
		}{
			RequirementID: finding.RequirementID, CaseID: finding.CaseID,
			ClauseID: finding.ClauseID,
			Claim:    claim, ClaimTexts: finding.ClaimTexts, Test: finding.Test,
			Subject: finding.Subject, Kind: finding.Kind, Disposition: finding.Disposition,
			Rationale: finding.Rationale, Profile: finding.Profile, Operation: finding.Operation,
			Target: finding.Target, Producer: finding.Producer, SourceLinks: finding.SourceLinks,
			Observations: finding.Observations, Context: finding.Context, Sources: checks,
		}
	}
	data, _ := json.Marshal(identity)
	sum := sha256.Sum256(data)
	return "F-" + hex.EncodeToString(sum[:])
}

func sourceIDs(r *ontology.Requirement) []string {
	if r == nil {
		return nil
	}
	return sourceIDsFromLinks(r.SourceLinks)
}

func sourceIDsFromLinks(links []ontology.SourceLink) []string {
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.SourceID)
	}
	sort.Strings(ids)
	return ids
}

func matchingSources(checks []source.Check, ids []string) []source.Check {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var out []source.Check
	for _, check := range checks {
		if wanted[check.SourceID] {
			out = append(out, check)
		}
	}
	sortSourceChecks(out)
	return out
}

func linkedRequirements(g *ontology.Graph, sourceID string) []*ontology.Requirement {
	var out []*ontology.Requirement
	for i := range g.Requirements {
		for _, link := range g.Requirements[i].SourceLinks {
			if link.SourceID == sourceID {
				out = append(out, &g.Requirements[i])
				break
			}
		}
	}
	return out
}

func requirementByID(g *ontology.Graph, id string) *ontology.Requirement {
	for i := range g.Requirements {
		if g.Requirements[i].ID == id {
			return &g.Requirements[i]
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if value == "" || (len(out) > 0 && out[len(out)-1] == value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func uniqueFindings(findings []Finding) []Finding {
	byID := make(map[string]Finding, len(findings))
	for _, finding := range findings {
		byID[finding.ID] = finding
	}
	out := make([]Finding, 0, len(byID))
	for _, finding := range byID {
		out = append(out, finding)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortSourceChecks(checks []source.Check) {
	sort.Slice(checks, func(i, j int) bool {
		a, b := checks[i], checks[j]
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.Anchor != b.Anchor {
			return a.Anchor < b.Anchor
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		if a.ExpectedSHA256 != b.ExpectedSHA256 {
			return a.ExpectedSHA256 < b.ExpectedSHA256
		}
		return a.ActualSHA256 < b.ActualSHA256
	})
}
