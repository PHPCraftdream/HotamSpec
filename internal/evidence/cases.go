package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func cloneObservedValue(value *ontology.ObservedValue) *ontology.ObservedValue {
	return ontology.CloneObservedValue(value)
}

func cloneClaimTexts(texts ontology.LocalizedText) ontology.LocalizedText {
	if len(texts) == 0 {
		return nil
	}
	copy := make(ontology.LocalizedText, len(texts))
	for language, text := range texts {
		copy[language] = text
	}
	return copy
}

func cloneCaseDefinitions(cases []ontology.CaseDefinition) []ontology.CaseDefinition {
	return ontology.CloneCaseDefinitions(cases)
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

func normalizeComponents(components []ontology.Component) []ontology.Component {
	copy := append([]ontology.Component(nil), components...)
	sort.Slice(copy, func(i, j int) bool {
		if copy[i].ID != copy[j].ID {
			return copy[i].ID < copy[j].ID
		}
		if copy[i].Role != copy[j].Role {
			return copy[i].Role < copy[j].Role
		}
		if copy[i].Version != copy[j].Version {
			return copy[i].Version < copy[j].Version
		}
		if copy[i].SHA256 != copy[j].SHA256 {
			return copy[i].SHA256 < copy[j].SHA256
		}
		return !copy[i].Measured && copy[j].Measured
	})
	return copy
}

func evidenceFingerprint(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func profileByID(g *ontology.Graph, id string) *ontology.Profile {
	if g == nil || g.Conformance == nil || id == "" {
		return nil
	}
	for i := range g.Conformance.Profiles {
		if g.Conformance.Profiles[i].ID == id {
			return cloneProfile(g.Conformance.Profiles[i])
		}
	}
	return nil
}

func compositionByTarget(g *ontology.Graph, target string) []ontology.Component {
	if g == nil || g.Conformance == nil || target == "" {
		return nil
	}
	for _, composition := range g.Conformance.Compositions {
		if composition.ID == target {
			return normalizeComponents(composition.Components)
		}
	}
	return nil
}

func graphCaseDefinitions(g *ontology.Graph, id string) []ontology.CaseDefinition {
	if g == nil || id == "" {
		return nil
	}
	var definitions []ontology.CaseDefinition
	for _, requirement := range g.Requirements {
		for _, definition := range requirement.Cases {
			if definition.ID == id {
				definitions = append(definitions, definition)
			}
		}
	}
	sort.Slice(definitions, func(i, j int) bool {
		return evidenceFingerprint(definitions[i]) < evidenceFingerprint(definitions[j])
	})
	return cloneCaseDefinitions(definitions)
}

func artifactContext(g *ontology.Graph, artifact decodedArtifact, recorded *decodedContext) Context {
	context := Context{}
	if recorded != nil {
		context.Implementation = recorded.Implementation
		context.ImplementationVersion = recorded.ImplementationVersion
		context.SpecVersion = recorded.SpecVersion
		context.Profile = recorded.Profile
		context.Operation = recorded.Operation
		context.Target = recorded.Target
		context.Producer = recorded.Producer
		context.MeasuredComponents = normalizeComponents(recorded.Components)
	}
	caseID := ""
	if artifact.Case != nil {
		caseID = artifact.Case.ID
	}
	selection := artifact.Case
	definitions := graphCaseDefinitions(g, caseID)
	if len(definitions) > 0 {
		selection = &definitions[0]
		if len(definitions) > 1 {
			context.DeclaredCaseFingerprint = evidenceFingerprint(definitions)
		} else {
			context.DeclaredCaseFingerprint = evidenceFingerprint(definitions[0])
		}
	}
	if artifact.Case != nil {
		context.RecordedCaseFingerprint = evidenceFingerprint(struct {
			Case     ontology.CaseDefinition `json:"case"`
			Input    *ontology.ObservedValue `json:"input,omitempty"`
			Expected *ontology.ObservedValue `json:"expected,omitempty"`
			Test     string                  `json:"test"`
		}{*artifact.Case, artifact.CaseInput, artifact.CaseExpected, artifact.Test})
		context.CaseFingerprint = evidenceFingerprint(struct {
			Recorded string `json:"recorded"`
			Declared string `json:"declared,omitempty"`
		}{context.RecordedCaseFingerprint, context.DeclaredCaseFingerprint})
	}
	selectedProfile := context.Profile
	if selection != nil {
		if selectedProfile == "" {
			selectedProfile = selection.Profile
		}
		context.Composition = compositionByTarget(g, selection.Target)
	}
	context.ProfileDetails = profileByID(g, selectedProfile)
	if selection != nil {
		context.ProfileFingerprint = evidenceFingerprint(struct {
			Profile           *ontology.Profile    `json:"profile,omitempty"`
			SelectedProfile   string               `json:"selected_profile"`
			ObservedProfile   string               `json:"observed_profile,omitempty"`
			SelectedOperation string               `json:"selected_operation,omitempty"`
			ObservedOperation string               `json:"observed_operation,omitempty"`
			SelectedTarget    string               `json:"selected_target,omitempty"`
			ObservedTarget    string               `json:"observed_target,omitempty"`
			Composition       []ontology.Component `json:"composition,omitempty"`
		}{
			context.ProfileDetails, selection.Profile, context.Profile,
			selection.Operation, context.Operation, selection.Target, context.Target, context.Composition,
		})
		context.ProducerFingerprint = evidenceFingerprint(struct {
			Recorded   string               `json:"recorded,omitempty"`
			Declared   string               `json:"declared,omitempty"`
			Measured   string               `json:"measured,omitempty"`
			Components []ontology.Component `json:"components,omitempty"`
		}{artifact.Case.Producer, selection.Producer, context.Producer, context.MeasuredComponents})
	} else {
		if context.ProfileDetails != nil || context.Operation != "" || context.Target != "" ||
			context.Producer != "" || len(context.MeasuredComponents) > 0 {
			context.ProfileFingerprint = evidenceFingerprint(struct {
				Profile   *ontology.Profile `json:"profile,omitempty"`
				Operation string            `json:"operation,omitempty"`
				Target    string            `json:"target,omitempty"`
			}{context.ProfileDetails, context.Operation, context.Target})
			context.ProducerFingerprint = evidenceFingerprint(struct {
				Implementation string               `json:"implementation,omitempty"`
				Version        string               `json:"version,omitempty"`
				Producer       string               `json:"producer,omitempty"`
				Components     []ontology.Component `json:"components,omitempty"`
			}{context.Implementation, context.ImplementationVersion, context.Producer, context.MeasuredComponents})
		}
	}
	return context
}

func caseExecutions(artifact decodedArtifact, observed ArtifactEvidence, primaryAtomID string, caseAtomIDs []string) []conformance.Execution {
	if artifact.Case == nil || artifact.Case.ID == "" || primaryAtomID == "" {
		return nil
	}
	copy := ontology.CloneCaseDefinitions([]ontology.CaseDefinition{*artifact.Case})
	caseMetadata := copy[0]
	caseMetadata.Test = observed.Test
	caseMetadata.AtomIDs = append([]string(nil), caseAtomIDs...)
	caseMetadata.Input = cloneObservedValue(artifact.CaseInput)
	caseMetadata.Expected = cloneObservedValue(artifact.CaseExpected)
	var comparisons []conformance.Comparison
	for _, observation := range observed.Observations {
		comparisons = append(comparisons, conformance.Comparison{
			Name: observation.Name, Passed: observation.Passed,
			InputText: observation.Input, ActualText: observation.Actual, ExpectedText: observation.Expected,
			Input:    cloneObservedValue(observation.RawInput),
			Actual:   cloneObservedValue(observation.RawActual),
			Expected: cloneObservedValue(observation.RawExpected),
		})
	}
	return []conformance.Execution{{
		CaseID: artifact.Case.ID, AtomID: primaryAtomID, Test: observed.Test,
		Verdict: observed.Verdict, Profile: artifact.Case.Profile,
		ObservedProfile: observed.Context.Profile,
		Target:          observed.Context.Target, Operation: observed.Context.Operation,
		Producer:   observed.Context.Producer,
		Conditions: append([]ontology.ConditionEvidence(nil), artifact.Case.Conditions...),
		Selection:  cloneSelection(artifact.Case.Selection), CaseMetadata: &caseMetadata,
		Comparisons: comparisons,
		Components:  normalizeComponents(observed.Context.MeasuredComponents),
	}}
}

func cloneSelection(selection *ontology.SelectionEvidence) *ontology.SelectionEvidence {
	if selection == nil {
		return nil
	}
	copy := *selection
	copy.Matched = slices.Clone(selection.Matched)
	return &copy
}

func caseTestReference(g *ontology.Graph, caseID, actual, packageDir string, testFiles map[string]map[string]string) string {
	if g != nil {
		for _, requirement := range g.Requirements {
			for _, definition := range requirement.Cases {
				if definition.ID == caseID && definition.Test != "" {
					return qualifiedTestReference(definition.Test, actual)
				}
			}
		}
	}
	return atomTestReference(packageDir, actual, testFiles)
}

func requirementTestReferences(requirement ontology.Requirement) []string {
	if requirement.Status == ontology.StatusREJECTED {
		return nil
	}
	seen := make(map[string]bool)
	var references []string
	add := func(raw string) {
		reference := strings.TrimSpace(raw)
		if reference == "" || seen[reference] {
			return
		}
		seen[reference] = true
		references = append(references, reference)
	}
	for _, reference := range requirement.VerifiedBy {
		add(reference)
	}
	for _, definition := range requirement.Cases {
		add(definition.Test)
	}
	return references
}
func reportOnlyAtomClaims(g *ontology.Graph, index *gate.AtomSourceIndex, raw []byte, subjects []string) (ontology.LocalizedText, string) {
	if index != nil {
		artifact, err := gate.DecodeAtomArtifact(raw)
		if err == nil {
			if claims, err := index.DeriveClaims(artifact); err == nil && len(claims) > 0 {
				primary, _ := index.DeriveClaim(artifact)
				if g == nil || len(g.Languages) == 0 {
					return nil, primary
				}
				return cloneClaimTexts(claims), primary
			}
		}
	}
	claims := make(ontology.LocalizedText)
	if index != nil {
		for _, subject := range subjects {
			resolved, err := index.Resolve(subject)
			if err != nil {
				continue
			}
			for language, phrase := range resolved.Phrases {
				if claims[language] == "" {
					claims[language] = phrase
				} else {
					claims[language] += "; " + phrase
				}
			}
			if len(resolved.Phrases) == 0 && resolved.Phrase != "" {
				if claims[""] == "" {
					claims[""] = resolved.Phrase
				} else {
					claims[""] += "; " + resolved.Phrase
				}
			}
		}
	}
	primary := ""
	if g != nil && g.DefaultLanguage != "" {
		primary = claims[g.DefaultLanguage]
	}
	if primary == "" && len(claims) == 1 {
		for _, phrase := range claims {
			primary = phrase
		}
	}
	if primary == "" {
		primary = claims[""]
	}
	if primary == "" {
		primary = reportOnlyAtomClaim(subjects, index)
	}
	if g == nil || len(g.Languages) == 0 {
		claims = nil
	}
	return claims, primary
}

type caseArtifactRecord struct {
	artifact decodedArtifact
	observed ArtifactEvidence
	atomID   string
}

func collectCaseExecutions(g *ontology.Graph, runs map[string]gate.RecordingResult, testFiles map[string]map[string]string) []conformance.Execution {
	var records []caseArtifactRecord
	byCase := make(map[string][]int)
	for _, packageDir := range sortedRunDirs(runs) {
		run := runs[packageDir]
		for _, recorded := range run.Artifacts {
			artifact, err := decodeArtifact(recorded.RawJSON)
			if err != nil || artifact.Mode != "rule" || artifact.Case == nil || artifact.Case.ID == "" {
				continue
			}
			observed, _, _ := makeArtifactEvidence(g, artifact, run.TestVerdicts)
			observed.Test = caseTestReference(g, artifact.Case.ID, artifact.Test, packageDir, testFiles)
			atomID := ""
			if atomIDs := reportAtomIDs(g, artifact); len(atomIDs) == 1 {
				atomID = atomIDs[0]
			}
			index := len(records)
			records = append(records, caseArtifactRecord{artifact: artifact, observed: observed, atomID: atomID})
			byCase[artifact.Case.ID] = append(byCase[artifact.Case.ID], index)
		}
	}

	var executions []conformance.Execution
	seen := make(map[string]bool)
	for _, caseID := range sortedStringKeys(byCase) {
		recordIndexes := byCase[caseID]
		caseAtomIDs := graphCaseAtomIDs(g, caseID)
		for _, index := range recordIndexes {
			record := records[index]
			caseAtomIDs = append(caseAtomIDs, record.artifact.Case.AtomIDs...)
			if record.atomID != "" {
				caseAtomIDs = append(caseAtomIDs, record.atomID)
			}
		}
		caseAtomIDs = normalizeAtomIDs(caseAtomIDs)
		for _, index := range recordIndexes {
			record := records[index]
			metadataAtomIDs := caseAtomIDs
			if len(record.artifact.Case.AtomIDs) > 0 {
				metadataAtomIDs = normalizeAtomIDs(record.artifact.Case.AtomIDs)
			}
			for _, execution := range caseExecutions(record.artifact, record.observed, record.atomID, metadataAtomIDs) {
				key := evidenceFingerprint(execution)
				if seen[key] {
					continue
				}
				seen[key] = true
				executions = append(executions, execution)
			}
		}
	}
	return executions
}

func graphCaseAtomIDs(g *ontology.Graph, caseID string) []string {
	if g == nil || caseID == "" {
		return nil
	}
	var atomIDs []string
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, definition := range requirement.Cases {
			if definition.ID == caseID {
				atomIDs = append(atomIDs, requirement.ID)
				atomIDs = append(atomIDs, definition.AtomIDs...)
			}
		}
	}
	return normalizeAtomIDs(atomIDs)
}

func normalizeAtomIDs(atomIDs []string) []string {
	seen := make(map[string]bool, len(atomIDs))
	var normalized []string
	for _, atomID := range atomIDs {
		if atomID == "" || seen[atomID] {
			continue
		}
		seen[atomID] = true
		normalized = append(normalized, atomID)
	}
	sort.Strings(normalized)
	return normalized
}

func sortedStringKeys(values map[string][]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func activeCaseAtomIDs(g *ontology.Graph, atomIDs []string) []string {
	if g == nil {
		seen := make(map[string]bool, len(atomIDs))
		var active []string
		for _, atomID := range atomIDs {
			if atomID != "" && !seen[atomID] {
				seen[atomID] = true
				active = append(active, atomID)
			}
		}
		sort.Strings(active)
		return active
	}
	seen := make(map[string]bool, len(atomIDs))
	var active []string
	for _, atomID := range atomIDs {
		if atomID == "" || seen[atomID] {
			continue
		}
		seen[atomID] = true
		requirement := requirementByID(g, atomID)
		if requirement == nil || requirement.Status != ontology.StatusREJECTED {
			active = append(active, atomID)
		}
	}
	sort.Strings(active)
	return active
}

func retiredAtomIDs(g *ontology.Graph, artifact decodedArtifact) []string {
	if g == nil {
		return nil
	}
	candidates := artifact.ReqID
	if artifact.Case != nil && len(artifact.Case.AtomIDs) > 0 {
		candidates = ""
	}
	seen := make(map[string]bool)
	var retired []string
	add := func(atomID string) {
		requirement := requirementByID(g, atomID)
		if atomID != "" && requirement != nil && requirement.Status == ontology.StatusREJECTED && !seen[atomID] {
			seen[atomID] = true
			retired = append(retired, atomID)
		}
	}
	if candidates != "" {
		add(candidates)
	}
	if artifact.Case != nil {
		for _, atomID := range artifact.Case.AtomIDs {
			add(atomID)
		}
	}
	for i := range g.Requirements {
		requirement := &g.Requirements[i]
		if requirement.Status != ontology.StatusREJECTED {
			continue
		}
		for _, method := range requirement.ImplementedBy {
			if artifact.ReqID != "" && methodMatches(artifact.ReqID, method) {
				add(requirement.ID)
				break
			}
			for _, step := range artifact.Steps {
				if methodMatches(step.Subject, method) {
					add(requirement.ID)
					break
				}
			}
		}
	}
	sort.Strings(retired)
	return retired
}

func reportAtomIDs(g *ontology.Graph, artifact decodedArtifact) []string {
	if artifact.Case == nil {
		if artifact.ReqID == "" {
			return nil
		}
		return activeCaseAtomIDs(g, []string{artifact.ReqID})
	}
	atomID := primaryCaseAtomID(g, artifact)
	if atomID == "" {
		return nil
	}
	return []string{atomID}
}

func primaryCaseAtomID(g *ontology.Graph, artifact decodedArtifact) string {
	if artifact.Case == nil {
		return ""
	}
	explicitIDs := normalizeAtomIDs(artifact.Case.AtomIDs)
	activeExplicitIDs := activeCaseAtomIDs(g, explicitIDs)
	projectedAtomIDs := graphCaseAtomIDs(g, artifact.Case.ID)
	projected := len(projectedAtomIDs) > 0
	directID := ""
	if g != nil && artifact.ReqID != "" {
		if requirement := requirementByID(g, artifact.ReqID); requirement != nil {
			if requirement.Status == ontology.StatusREJECTED {
				return ""
			}
			directID = requirement.ID
		}
	}
	if projected && directID != "" && !containsAtomID(projectedAtomIDs, directID) {
		return ""
	}
	methodIDs, retiredMethodMatch := methodLinkedAtomIDs(g, artifact, projectedAtomIDs)
	if directID != "" {
		if len(methodIDs) > 0 && !containsAtomID(methodIDs, directID) {
			return ""
		}
		if len(explicitIDs) > 0 && !containsAtomID(activeExplicitIDs, directID) {
			return ""
		}
		return directID
	}
	if len(methodIDs) > 0 {
		if len(explicitIDs) > 0 {
			var matched []string
			for _, atomID := range methodIDs {
				if containsAtomID(activeExplicitIDs, atomID) {
					matched = append(matched, atomID)
				}
			}
			if len(matched) != 1 {
				return ""
			}
			return matched[0]
		}
		if len(methodIDs) == 1 {
			return methodIDs[0]
		}
		return ""
	}
	if len(activeExplicitIDs) == 1 {
		return activeExplicitIDs[0]
	}
	if retiredMethodMatch || (len(explicitIDs) > 0 && len(activeExplicitIDs) == 0) {
		return ""
	}
	if artifact.ReqID != "" {
		return artifact.ReqID
	}
	return ""
}

func methodLinkedAtomIDs(g *ontology.Graph, artifact decodedArtifact, projectedAtomIDs []string) ([]string, bool) {
	if g == nil {
		return nil, false
	}
	var atomIDs []string
	retiredMatch := false
	for _, requirement := range g.Requirements {
		if len(projectedAtomIDs) > 0 && !containsAtomID(projectedAtomIDs, requirement.ID) {
			continue
		}
		matched := false
		for _, method := range requirement.ImplementedBy {
			if artifact.ReqID != "" && methodMatches(artifact.ReqID, method) {
				matched = true
				break
			}
			if len(artifact.Steps) > 0 && methodMatches(artifact.Steps[0].Subject, method) {
				matched = true
			}
			if matched {
				break
			}
		}
		if !matched {
			continue
		}
		if requirement.Status == ontology.StatusREJECTED {
			retiredMatch = true
			continue
		}
		atomIDs = append(atomIDs, requirement.ID)
	}
	return normalizeAtomIDs(atomIDs), retiredMatch
}

func containsAtomID(atomIDs []string, want string) bool {
	for _, atomID := range atomIDs {
		if atomID == want {
			return true
		}
	}
	return false
}
func caseForTest(cases []ontology.CaseDefinition, test string) *ontology.CaseDefinition {
	var found *ontology.CaseDefinition
	for i := range cases {
		if cases[i].Test != test && !referenceMatchesTest(cases[i].Test, test) {
			continue
		}
		if found != nil && found.ID != cases[i].ID {
			return nil
		}
		found = &cases[i]
	}
	return found
}
func cloneApplicability(applicability *ontology.Applicability) *ontology.Applicability {
	if applicability == nil {
		return nil
	}
	copy := *applicability
	copy.Operations = append([]string(nil), applicability.Operations...)
	copy.Profiles = append([]string(nil), applicability.Profiles...)
	copy.Features = append([]string(nil), applicability.Features...)
	return &copy
}
