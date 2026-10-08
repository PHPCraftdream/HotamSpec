package conformance

import (
	"slices"
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func profileQualification(requirement *ontology.Requirement, profile string) string {
	if requirement == nil || requirement.Coverage == nil {
		return ""
	}
	if requirement.Coverage.Profile != "" && requirement.Coverage.Profile != profile {
		return ""
	}
	return requirement.Coverage.Status
}

func requirementStrength(requirement *ontology.Requirement, clauses map[string]ontology.SourceClause) string {
	if requirement == nil {
		return ""
	}
	strength := requirement.Strength
	for _, link := range requirement.ClauseLinks {
		candidate := clauses[link.ClauseID].Strength
		if strengthRank(candidate) > strengthRank(strength) {
			strength = candidate
		}
	}
	return strength
}

func strengthRank(strength string) int {
	switch strength {
	case "MUST":
		return 3
	case "SHOULD":
		return 2
	case "MAY":
		return 1
	default:
		return 0
	}
}

func compositionComponents(target string, declarations []ontology.Composition) []ontology.Component {
	for _, declaration := range declarations {
		if declaration.ID == target {
			return orderedComponents(declaration.Components)
		}
	}
	return nil
}

func orderedComponents(components []ontology.Component) []ontology.Component {
	out := append([]ontology.Component(nil), components...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		if out[i].Version != out[j].Version {
			return out[i].Version < out[j].Version
		}
		if out[i].SHA256 != out[j].SHA256 {
			return out[i].SHA256 < out[j].SHA256
		}
		return !out[i].Measured && out[j].Measured
	})
	return out
}

func componentIdentity(components []ontology.Component) []struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
} {
	ordered := orderedComponents(components)
	out := make([]struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Version string `json:"version,omitempty"`
		SHA256  string `json:"sha256,omitempty"`
	}, len(ordered))
	for i, component := range ordered {
		out[i].ID = component.ID
		out[i].Role = component.Role
		out[i].Version = component.Version
		out[i].SHA256 = component.SHA256
	}
	return out
}

func componentsMatch(declared, observed []ontology.Component) bool {
	declared = orderedComponents(declared)
	observed = orderedComponents(observed)
	if len(declared) != len(observed) {
		return false
	}
	for i := range declared {
		if declared[i].ID != observed[i].ID || declared[i].Role != observed[i].Role {
			return false
		}
		if declared[i].Version != "" && declared[i].Version != observed[i].Version {
			return false
		}
		if declared[i].SHA256 != "" && declared[i].SHA256 != observed[i].SHA256 {
			return false
		}
	}
	return true
}

func cloneProfile(profile ontology.Profile) *ontology.Profile {
	copy := profile
	copy.Operations = append([]string(nil), profile.Operations...)
	copy.Features = append([]string(nil), profile.Features...)
	if profile.PreservesOrder != nil {
		value := *profile.PreservesOrder
		copy.PreservesOrder = &value
	}
	if profile.Capabilities != nil {
		copy.Capabilities = make(map[string]string, len(profile.Capabilities))
		for key, value := range profile.Capabilities {
			copy.Capabilities[key] = value
		}
	}
	return &copy
}

func cloneCaseDefinition(definition ontology.CaseDefinition) *ontology.CaseDefinition {
	copy := ontology.CloneCaseDefinitions([]ontology.CaseDefinition{definition})
	return &copy[0]
}
func issueDisposition(code string) string {
	switch code {
	case IssueProfileNotApplicable, IssueProfileUnsupported, IssueProfileUnreachable,
		IssueProfileUnverified, IssueCaseMissingExecution, IssueCaseUnverified,
		IssueCaseUnknownExecution, IssuePrecedenceUnwitnessed, IssueCompositionUnmeasured,
		IssueProducerIdentityDiffers:
		return DispositionQualification
	default:
		return DispositionViolation
	}
}
func comparisonSnapshotKey(execution Execution) string {
	return fingerprint(struct {
		CaseID          string                       `json:"case_id"`
		Test            string                       `json:"test"`
		Verdict         string                       `json:"verdict"`
		Profile         string                       `json:"profile"`
		ObservedProfile string                       `json:"observed_profile"`
		Target          string                       `json:"target"`
		Operation       string                       `json:"operation"`
		Producer        string                       `json:"producer"`
		Conditions      []ontology.ConditionEvidence `json:"conditions,omitempty"`
		Selection       *ontology.SelectionEvidence  `json:"selection,omitempty"`
		CaseMetadata    *ontology.CaseDefinition     `json:"case_metadata,omitempty"`
		Comparisons     []Comparison                 `json:"comparisons,omitempty"`
		Components      []ontology.Component         `json:"components,omitempty"`
	}{
		CaseID: execution.CaseID, Test: execution.Test, Verdict: execution.Verdict,
		Profile: execution.Profile, ObservedProfile: execution.ObservedProfile,
		Target: execution.Target, Operation: execution.Operation, Producer: execution.Producer,
		Conditions: execution.Conditions, Selection: execution.Selection,
		CaseMetadata: execution.CaseMetadata, Comparisons: execution.Comparisons,
		Components: orderedComponents(execution.Components),
	})
}

func cloneExecution(execution Execution) Execution {
	execution.Conditions = append([]ontology.ConditionEvidence(nil), execution.Conditions...)
	if execution.CaseMetadata != nil {
		execution.CaseMetadata = cloneCaseDefinition(*execution.CaseMetadata)
	}
	execution.Components = orderedComponents(execution.Components)
	if execution.Selection != nil {
		selection := *execution.Selection
		selection.Matched = slices.Clone(execution.Selection.Matched)
		execution.Selection = &selection
	}
	execution.Comparisons = append([]Comparison(nil), execution.Comparisons...)
	for i := range execution.Comparisons {
		execution.Comparisons[i].Input = cloneObserved(execution.Comparisons[i].Input)
		execution.Comparisons[i].Actual = cloneObserved(execution.Comparisons[i].Actual)
		execution.Comparisons[i].Expected = cloneObserved(execution.Comparisons[i].Expected)
	}
	return execution
}
func reportOnlyCaseAssessment(execution Execution, profiles map[string]ontology.Profile, compositions []ontology.Composition) CaseAssessment {
	assessment := CaseAssessment{
		ID: execution.CaseID, AtomID: execution.AtomID, Test: execution.Test,
		Status: "report_only", Qualification: "not_declared",
		Executions: []Execution{cloneExecution(execution)},
	}
	if execution.Verdict == "fail" || hasFailedComparison(execution.Comparisons) {
		assessment.Status = "report_only_discrepancy"
	}
	if metadata := execution.CaseMetadata; metadata != nil {
		assessment.Test = metadata.Test
		assessment.Profile = metadata.Profile
		assessment.Operation = metadata.Operation
		assessment.Target = metadata.Target
		assessment.Producer = metadata.Producer
		assessment.Input = cloneObserved(metadata.Input)
		assessment.Expected = cloneObserved(metadata.Expected)
		assessment.Fixtures = append([]ontology.FixtureRef(nil), metadata.Fixtures...)
		assessment.Sides = sortedStrings(metadata.Sides)
		assessment.ClauseIDs = slices.Clone(metadata.ClauseIDs)
		profile, hasProfile := profiles[metadata.Profile]
		if hasProfile {
			assessment.ProfileDetails = cloneProfile(profile)
		}
		assessment.ProfileFingerprint = fingerprint(struct {
			ProfileID   string               `json:"profile_id"`
			Profile     ontology.Profile     `json:"profile"`
			Operation   string               `json:"operation"`
			Target      string               `json:"target"`
			Composition []ontology.Component `json:"composition,omitempty"`
		}{metadata.Profile, profile, metadata.Operation, metadata.Target, compositionComponents(metadata.Target, compositions)})
		assessment.CaseFingerprint = reportOnlyCaseFingerprint(assessment.Executions)
		assessment.ProducerFingerprint = producerFingerprint(metadata.Producer, metadata.Target, assessment.Executions)
	}
	if execution.Producer != "" {
		assessment.ObservedProducers = []string{execution.Producer}
	}
	if execution.ObservedProfile != "" {
		assessment.ObservedProfiles = []string{execution.ObservedProfile}
	}
	if execution.Operation != "" {
		assessment.ObservedOperations = []string{execution.Operation}
	}
	if execution.Target != "" {
		assessment.ObservedTargets = []string{execution.Target}
	}
	return assessment
}
func executionOperation(execution Execution) string {
	if execution.Operation != "" {
		return execution.Operation
	}
	if execution.CaseMetadata != nil {
		return execution.CaseMetadata.Operation
	}
	return ""
}
func caseReviewFingerprint(declared ontology.CaseDefinition, executions []Execution) string {
	var recorded []string
	for _, execution := range executions {
		if execution.CaseMetadata != nil {
			recorded = append(recorded, fingerprint(*execution.CaseMetadata))
		}
	}
	sort.Strings(recorded)
	return fingerprint(struct {
		Declared ontology.CaseDefinition `json:"declared"`
		Recorded []string                `json:"recorded,omitempty"`
	}{declared, recorded})
}

func reportOnlyCaseFingerprint(executions []Execution) string {
	var recorded []string
	for _, execution := range executions {
		if execution.CaseMetadata != nil {
			recorded = append(recorded, fingerprint(*execution.CaseMetadata))
		}
	}
	sort.Strings(recorded)
	return fingerprint(recorded)
}

func producerFingerprint(declared, target string, executions []Execution) string {
	type observation struct {
		Profile    string               `json:"profile,omitempty"`
		Target     string               `json:"target,omitempty"`
		Operation  string               `json:"operation,omitempty"`
		Producer   string               `json:"producer,omitempty"`
		Components []ontology.Component `json:"components,omitempty"`
	}
	observed := make([]observation, 0, len(executions))
	for _, execution := range executions {
		observed = append(observed, observation{
			Profile: execution.ObservedProfile, Target: execution.Target,
			Operation: execution.Operation, Producer: execution.Producer,
			Components: orderedComponents(execution.Components),
		})
	}
	sort.Slice(observed, func(i, j int) bool {
		return fingerprint(observed[i]) < fingerprint(observed[j])
	})
	return fingerprint(struct {
		Declared string        `json:"declared,omitempty"`
		Target   string        `json:"target,omitempty"`
		Observed []observation `json:"observed,omitempty"`
	}{declared, target, observed})
}
func appendUniqueSorted(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}
