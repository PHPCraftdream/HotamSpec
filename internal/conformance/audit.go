// Package conformance audits explicitly declared source clauses, cases, and
// implementation profiles. Results are structural obligations relative to the
// author's inventory, never claims of semantic completeness.
package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
)

const (
	IssueClauseMissingDecomposition = "clause_missing_decomposition"
	IssueClauseMissingSide          = "clause_missing_side"
	IssueClauseMissingSource        = "clause_missing_source"
	IssueClauseUnknownLink          = "clause_unknown_link"
	IssueAtomMissingMethod          = "atom_missing_method"
	IssueAtomMissingCases           = "atom_missing_cases"
	IssueCaseMissingExecution       = "case_missing_execution"
	IssueCaseUnverified             = "case_unverified"
	IssueCaseDiscrepancy            = "case_discrepancy"
	IssueCaseTestMismatch           = "case_test_mismatch"
	IssueCaseTargetMismatch         = "case_target_mismatch"
	IssueCaseProfileMismatch        = "case_profile_mismatch"
	IssueCaseOperationMismatch      = "case_operation_mismatch"
	IssueProducerIdentityDiffers    = "producer_identity_differs"
	IssueCaseMetadataConflict       = "case_metadata_conflict"
	IssueCaseUnknownExecution       = "case_unknown_execution"
	IssueCaseUnknownAtom            = "case_unknown_atom"
	IssueSourceUnverified           = "source_unverified"
	IssueProfileNotApplicable       = "profile_not_applicable"
	IssueProfileUnsupported         = "profile_unsupported"
	IssueProfileUnreachable         = "profile_unreachable"
	IssueProfileUnverified          = "profile_unverified"
	IssuePrecedenceCycle            = "precedence_cycle"
	IssuePrecedenceUnwitnessed      = "precedence_unwitnessed"
	IssuePrecedenceDiscrepancy      = "precedence_discrepancy"
	IssuePrecedenceUnknownTarget    = "precedence_unknown_target"
	IssueCompositionUnmeasured      = "composition_unmeasured"
	IssueCompositionFingerprint     = "composition_fingerprint_changed"
)

// Disposition values classify audit issues as blocking violations or
// non-blocking qualifications.
const (
	DispositionQualification = "qualification"
	DispositionViolation     = "violation"
)

// Comparison is one producer-recorded result comparison. Values are typed and
// optional so absence cannot be confused with a zero or empty observation.
type Comparison struct {
	Name         string                  `json:"name"`
	InputText    string                  `json:"input_text,omitempty"`
	ActualText   string                  `json:"actual_text,omitempty"`
	ExpectedText string                  `json:"expected_text,omitempty"`
	Passed       bool                    `json:"passed"`
	Input        *ontology.ObservedValue `json:"input,omitempty"`
	Actual       *ontology.ObservedValue `json:"actual,omitempty"`
	Expected     *ontology.ObservedValue `json:"expected,omitempty"`
}

// Execution is the pure execution-side input to Audit. It contains only
// producer-observed data; Audit never constructs a trace or invokes code.
type Execution struct {
	CaseID          string                       `json:"case_id"`
	AtomID          string                       `json:"atom_id"`
	Test            string                       `json:"test,omitempty"`
	Verdict         string                       `json:"verdict"`
	Profile         string                       `json:"profile,omitempty"`
	ObservedProfile string                       `json:"observed_profile,omitempty"`
	Target          string                       `json:"target,omitempty"`
	Operation       string                       `json:"operation,omitempty"`
	Producer        string                       `json:"producer,omitempty"`
	Conditions      []ontology.ConditionEvidence `json:"conditions,omitempty"`
	Selection       *ontology.SelectionEvidence  `json:"selection,omitempty"`
	CaseMetadata    *ontology.CaseDefinition     `json:"case_metadata,omitempty"`
	Comparisons     []Comparison                 `json:"comparisons,omitempty"`
	Components      []ontology.Component         `json:"components,omitempty"`
}

// Report describes declared inventory and observed execution separately.
type Report struct {
	InventoryDeclared     int                     `json:"inventory_declared"`
	InventoryLinked       int                     `json:"inventory_linked"`
	AtomsLinked           int                     `json:"atoms_linked"`
	AtomsWithMethods      int                     `json:"atoms_with_methods"`
	CasesDeclared         int                     `json:"cases_declared"`
	CasesObserved         int                     `json:"cases_observed"`
	CasesExecuted         int                     `json:"cases_executed"`
	ComparisonsExecuted   int                     `json:"comparisons_executed"`
	Discrepancies         int                     `json:"discrepancies"`
	ProfileQualifications int                     `json:"profile_qualifications"`
	Clauses               []ClauseAssessment      `json:"clauses,omitempty"`
	Cases                 []CaseAssessment        `json:"cases,omitempty"`
	Profiles              []ProfileAssessment     `json:"profiles,omitempty"`
	Precedence            []PrecedenceAssessment  `json:"precedence,omitempty"`
	Compositions          []CompositionAssessment `json:"compositions,omitempty"`
	Issues                []Issue                 `json:"issues,omitempty"`
}

type Issue struct {
	Code        string `json:"code"`
	Disposition string `json:"disposition"`
	ClauseID    string `json:"clause_id,omitempty"`
	AtomID      string `json:"atom_id,omitempty"`
	CaseID      string `json:"case_id,omitempty"`
	Profile     string `json:"profile,omitempty"`
	Target      string `json:"target,omitempty"`
	Producer    string `json:"producer,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Operation   string `json:"operation,omitempty"`
	Message     string `json:"message"`
}

type ClauseAssessment struct {
	ID           string                `json:"id"`
	SourceLinks  []ontology.SourceLink `json:"source_links,omitempty"`
	Strength     string                `json:"strength,omitempty"`
	AtomIDs      []string              `json:"atom_ids,omitempty"`
	MissingSides []string              `json:"missing_sides,omitempty"`
	Status       string                `json:"status"`
}
type CaseAssessment struct {
	ID                  string                  `json:"id"`
	AtomID              string                  `json:"atom_id"`
	Test                string                  `json:"test,omitempty"`
	Profile             string                  `json:"profile,omitempty"`
	ProfileDetails      *ontology.Profile       `json:"profile_details,omitempty"`
	Operation           string                  `json:"operation,omitempty"`
	Target              string                  `json:"target,omitempty"`
	Producer            string                  `json:"producer,omitempty"`
	ObservedProducers   []string                `json:"observed_producers,omitempty"`
	ObservedProfiles    []string                `json:"observed_profiles,omitempty"`
	ObservedOperations  []string                `json:"observed_operations,omitempty"`
	ObservedTargets     []string                `json:"observed_targets,omitempty"`
	Input               *ontology.ObservedValue `json:"input,omitempty"`
	Expected            *ontology.ObservedValue `json:"expected,omitempty"`
	Fixtures            []ontology.FixtureRef   `json:"fixtures,omitempty"`
	Sides               []string                `json:"sides,omitempty"`
	Status              string                  `json:"status"`
	Qualification       string                  `json:"qualification,omitempty"`
	Executions          []Execution             `json:"executions,omitempty"`
	ProfileFingerprint  string                  `json:"profile_fingerprint,omitempty"`
	ProducerFingerprint string                  `json:"producer_fingerprint,omitempty"`
	CaseFingerprint     string                  `json:"case_fingerprint,omitempty"`
}

type ProfileAssessment struct {
	ID         string            `json:"id"`
	AtomID     string            `json:"atom_id,omitempty"`
	CaseID     string            `json:"case_id,omitempty"`
	Definition *ontology.Profile `json:"definition,omitempty"`
	Operation  string            `json:"operation,omitempty"`
	Target     string            `json:"target,omitempty"`
	Producer   string            `json:"producer,omitempty"`
	Applicable bool              `json:"applicable"`
	Status     string            `json:"status"`
}

type PrecedenceAssessment struct {
	AtomID    string `json:"atom_id"`
	Target    string `json:"target"`
	Scope     string `json:"scope"`
	Profile   string `json:"profile,omitempty"`
	Operation string `json:"operation,omitempty"`
	Status    string `json:"status"`
}

type CompositionAssessment struct {
	ID                  string               `json:"id"`
	Components          []ontology.Component `json:"components"`
	ObservedComponents  []ontology.Component `json:"observed_components,omitempty"`
	DeclaredFingerprint string               `json:"declared_fingerprint"`
	MeasuredFingerprint string               `json:"measured_fingerprint,omitempty"`
	Status              string               `json:"status"`
}

// Audit evaluates explicit ontology declarations and real execution records,
// verifying source links once for callers that do not already have checks.
func Audit(g *ontology.Graph, executions []Execution) Report {
	return AuditFromSources(g, executions, source.Verify(g))
}

// AuditFromSources evaluates with the caller's authoritative source checks and
// does not reread source files. Evidence publishers can share one check snapshot
// across the report and audit.
func AuditFromSources(g *ontology.Graph, executions []Execution, sourceChecks []source.Check) Report {
	out := Report{
		Clauses:      make([]ClauseAssessment, 0),
		Cases:        make([]CaseAssessment, 0),
		Profiles:     make([]ProfileAssessment, 0),
		Precedence:   make([]PrecedenceAssessment, 0),
		Compositions: make([]CompositionAssessment, 0),
		Issues:       make([]Issue, 0),
	}
	if g == nil {
		out.add(Issue{Code: "invalid_graph", Message: "conformance audit requires a graph"})
		return out
	}
	var config ontology.ConformanceConfig
	if g.Conformance != nil {
		config = *g.Conformance
	}
	out.InventoryDeclared = len(config.Clauses)

	profiles := make(map[string]ontology.Profile, len(config.Profiles))
	for _, profile := range config.Profiles {
		profiles[profile.ID] = profile
	}
	requirements := make(map[string]*ontology.Requirement, len(g.Requirements))
	for i := range g.Requirements {
		requirements[g.Requirements[i].ID] = &g.Requirements[i]
	}

	clauseByID := make(map[string]ontology.SourceClause, len(config.Clauses))
	clauseAtoms := make(map[string]map[string]bool, len(config.Clauses))
	for _, clause := range config.Clauses {
		if _, exists := clauseByID[clause.ID]; exists {
			out.add(Issue{Code: IssueCaseMetadataConflict, ClauseID: clause.ID, Message: "source clause ID is duplicated"})
			continue
		}
		clauseByID[clause.ID] = clause
		clauseAtoms[clause.ID] = make(map[string]bool)
		if len(clause.SourceLinks) == 0 {
			out.add(Issue{Code: IssueClauseMissingSource, ClauseID: clause.ID, Message: "declared source clause has no source link"})
		}
		for _, link := range clause.SourceLinks {
			if !sourceLinkVerified(sourceChecks, link) {
				out.add(Issue{Code: IssueSourceUnverified, ClauseID: clause.ID, Message: "source clause link is missing, stale, or unresolved: " + link.SourceID + "#" + link.Anchor})
			}
		}
	}

	caseDefinitions := make(map[string]ontology.CaseDefinition)
	caseMetadataConflicts := make(map[string]bool)
	caseAtoms := make(map[string]map[string]bool)
	atomCases := make(map[string]map[string]bool)
	for i := range g.Requirements {
		requirement := &g.Requirements[i]
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, link := range requirement.ClauseLinks {
			clause, ok := clauseByID[link.ClauseID]
			if !ok {
				out.add(Issue{Code: IssueClauseUnknownLink, AtomID: requirement.ID, ClauseID: link.ClauseID, Message: "atom links an unknown source clause"})
				continue
			}
			clauseAtoms[clause.ID][requirement.ID] = true
			if link.Side != "" && !contains(clause.Sides, link.Side) {
				out.add(Issue{Code: IssueClauseMissingSide, AtomID: requirement.ID, ClauseID: clause.ID, Message: "atom links an undeclared clause side: " + link.Side})
			}
		}
		if (len(requirement.ClauseLinks) > 0 || requirement.AtomKind == "rule" || len(requirement.Cases) > 0) && len(requirement.ImplementedBy) == 0 {
			out.add(Issue{Code: IssueAtomMissingMethod, AtomID: requirement.ID, Message: "inventory-linked or rule atom has no implementation method"})
		}
		if requirement.AtomKind == "rule" && len(requirement.Cases) == 0 {
			out.add(Issue{Code: IssueAtomMissingCases, AtomID: requirement.ID, Message: "rule atom has no declared cases"})
		}
		for _, definition := range requirement.Cases {
			prior, exists := caseDefinitions[definition.ID]
			if exists && !sameCase(prior, definition) {
				caseMetadataConflicts[definition.ID] = true
				out.add(Issue{Code: IssueCaseMetadataConflict, AtomID: requirement.ID, CaseID: definition.ID, Message: "the same case ID has conflicting declared metadata"})
			} else if !exists {
				caseDefinitions[definition.ID] = definition
				caseAtoms[definition.ID] = make(map[string]bool)
			}
			if atomCases[requirement.ID] == nil {
				atomCases[requirement.ID] = make(map[string]bool)
			}
			atomCases[requirement.ID][definition.ID] = true
			if !caseAtoms[definition.ID][requirement.ID] {
				caseAtoms[definition.ID][requirement.ID] = true
			}
			for _, atomID := range definition.AtomIDs {
				target, exists := requirements[atomID]
				if !exists {
					caseMetadataConflicts[definition.ID] = true
					out.add(Issue{Code: IssueCaseMetadataConflict, AtomID: atomID, CaseID: definition.ID, Message: "case references an unknown atom"})
					continue
				}
				if target.Status == ontology.StatusREJECTED {
					continue
				}
				caseAtoms[definition.ID][atomID] = true
				if atomCases[atomID] == nil {
					atomCases[atomID] = make(map[string]bool)
				}
				atomCases[atomID][definition.ID] = true
			}
		}
	}

	for _, clause := range config.Clauses {
		linked := clauseAtoms[clause.ID]
		assessment := ClauseAssessment{
			ID: clause.ID, SourceLinks: append([]ontology.SourceLink(nil), clause.SourceLinks...),
			Strength: clause.Strength, AtomIDs: mapKeys(linked), Status: "decomposed",
		}
		if len(linked) == 0 {
			assessment.Status = "missing_decomposition"
			out.add(Issue{Code: IssueClauseMissingDecomposition, ClauseID: clause.ID, Message: "declared source clause has no linked atom"})
		}
		for _, side := range clause.Sides {
			covered := false
			for atomID := range linked {
				requirement := requirements[atomID]
				if requirement == nil || !hasClauseSide(*requirement, clause.ID, side) {
					continue
				}
				for caseID, definition := range caseDefinitions {
					if !caseAtoms[caseID][atomID] || !contains(definition.Sides, side) {
						continue
					}
					covered = true
					break
				}
				if covered {
					break
				}
			}
			if !covered {
				assessment.MissingSides = append(assessment.MissingSides, side)
				out.add(Issue{Code: IssueClauseMissingSide, ClauseID: clause.ID, Message: "declared clause side has no linked atom and case: " + side})
			}
		}
		if len(assessment.MissingSides) > 0 && assessment.Status == "decomposed" {
			assessment.Status = "incomplete_sides"
		}
		out.Clauses = append(out.Clauses, assessment)
		if len(linked) > 0 {
			out.InventoryLinked++
		}
	}
	linkedAtoms := make(map[string]bool)
	for _, atomIDs := range clauseAtoms {
		for atomID := range atomIDs {
			linkedAtoms[atomID] = true
		}
	}
	out.AtomsLinked = len(linkedAtoms)
	for atomID := range linkedAtoms {
		if requirement := requirements[atomID]; requirement != nil && len(requirement.ImplementedBy) > 0 {
			out.AtomsWithMethods++
		}
	}

	out.CasesDeclared = len(caseDefinitions)
	seenExecution := make(map[string]bool)
	seenComparisonSnapshot := make(map[string]bool)
	seenObservedCases := make(map[string]bool)
	seenReportOnlyCases := make(map[string]int)
	seenReportOnlyMetadata := make(map[string]ontology.CaseDefinition)
	for _, execution := range executions {
		if execution.CaseID != "" && !seenObservedCases[execution.CaseID] {
			out.CasesObserved++
			seenObservedCases[execution.CaseID] = true
		}
		comparisonKey := comparisonSnapshotKey(execution)
		if !seenComparisonSnapshot[comparisonKey] {
			out.ComparisonsExecuted += len(execution.Comparisons)
			seenComparisonSnapshot[comparisonKey] = true
		}
		if _, known := caseDefinitions[execution.CaseID]; !known {
			out.add(Issue{Code: IssueCaseUnknownExecution, AtomID: execution.AtomID, CaseID: execution.CaseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "execution has no graph-declared case; observations remain report-only"})
			if execution.CaseMetadata != nil {
				if prior, exists := seenReportOnlyMetadata[execution.CaseID]; exists {
					if !sameCase(prior, *execution.CaseMetadata) {
						out.add(Issue{Code: IssueCaseMetadataConflict, AtomID: execution.AtomID, CaseID: execution.CaseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "report-only executions reuse a case ID with conflicting metadata"})
					}
				} else {
					seenReportOnlyMetadata[execution.CaseID] = *cloneCaseDefinition(*execution.CaseMetadata)
				}
			}
			rowKey := executionKey(execution.CaseID, execution.AtomID)
			if index, exists := seenReportOnlyCases[rowKey]; exists {
				row := &out.Cases[index]
				row.Executions = append(row.Executions, cloneExecution(execution))
				if execution.Verdict == "fail" || hasFailedComparison(execution.Comparisons) {
					row.Status = "report_only_discrepancy"
				}
				row.CaseFingerprint = reportOnlyCaseFingerprint(row.Executions)
				row.ObservedProducers = appendUniqueSorted(row.ObservedProducers, execution.Producer)
				row.ObservedProfiles = appendUniqueSorted(row.ObservedProfiles, execution.ObservedProfile)
				row.ObservedOperations = appendUniqueSorted(row.ObservedOperations, execution.Operation)
				row.ObservedTargets = appendUniqueSorted(row.ObservedTargets, execution.Target)
				declaredProducer, declaredTarget := "", ""
				if row.Executions[0].CaseMetadata != nil {
					declaredProducer = row.Executions[0].CaseMetadata.Producer
					declaredTarget = row.Executions[0].CaseMetadata.Target
				}
				row.ProducerFingerprint = producerFingerprint(declaredProducer, declaredTarget, row.Executions)
			} else {
				out.Cases = append(out.Cases, reportOnlyCaseAssessment(execution, profiles, config.Compositions))
				seenReportOnlyCases[rowKey] = len(out.Cases) - 1
			}
			if execution.Verdict == "fail" || hasFailedComparison(execution.Comparisons) {
				out.add(Issue{Code: IssueCaseDiscrepancy, AtomID: execution.AtomID, CaseID: execution.CaseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "report-only execution or comparison failed"})
			}
			continue
		}
		if !caseAtoms[execution.CaseID][execution.AtomID] {
			out.add(Issue{Code: IssueCaseUnknownAtom, AtomID: execution.AtomID, CaseID: execution.CaseID, Message: "execution atom is not declared for this case"})
			continue
		}
		if !seenExecution[execution.CaseID] {
			out.CasesExecuted++
			seenExecution[execution.CaseID] = true
		}
	}
	selectedProfiles := make(map[string]bool, len(profiles))
	for _, caseID := range sortedKeys(caseDefinitions) {
		definition := caseDefinitions[caseID]
		for _, atomID := range mapKeys(caseAtoms[caseID]) {
			assessment := CaseAssessment{
				ID: definition.ID, AtomID: atomID, Test: definition.Test,
				Profile: definition.Profile, Operation: definition.Operation,
				Target: definition.Target, Producer: definition.Producer,
				Input: cloneObserved(definition.Input), Expected: cloneObserved(definition.Expected),
				Fixtures: append([]ontology.FixtureRef(nil), definition.Fixtures...),
				Sides:    sortedStrings(definition.Sides), Status: "unverified",
			}
			selectionConflict := caseMetadataConflicts[caseID]
			profile, hasProfile := profiles[definition.Profile]
			if definition.Profile != "" && !hasProfile {
				assessment.Qualification = "profile_unverified"
				out.add(Issue{Code: IssueProfileUnverified, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "case selects a profile with no declared capability record"})
			}
			if hasProfile {
				assessment.ProfileDetails = cloneProfile(profile)
			}
			profileSelection := struct {
				ProfileID   string               `json:"profile_id"`
				Profile     ontology.Profile     `json:"profile"`
				Operation   string               `json:"operation"`
				Target      string               `json:"target"`
				Composition []ontology.Component `json:"composition,omitempty"`
			}{
				ProfileID: definition.Profile, Profile: profile, Operation: definition.Operation,
				Target: definition.Target, Composition: compositionComponents(definition.Target, config.Compositions),
			}
			assessment.ProfileFingerprint = fingerprint(profileSelection)
			applicable := true
			if definition.Operation != "" && hasProfile && len(profile.Operations) > 0 && !contains(profile.Operations, definition.Operation) {
				applicable = false
			}
			if requirement := requirements[atomID]; requirement != nil {
				if !applicabilityMatches(requirement.Applicability, definition.Profile, definition.Operation, profile, hasProfile) {
					applicable = false
				}
			}
			if !clauseApplicabilityMatches(g, atomID, definition.Profile, definition.Operation, profile, hasProfile) {
				applicable = false
			}
			if !applicable {
				assessment.Status = "not_applicable"
				out.add(Issue{Code: IssueProfileNotApplicable, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "declared profile and operation do not satisfy the atom or clause applicability"})
			}
			for _, execution := range executions {
				if execution.CaseID != caseID || execution.AtomID != atomID {
					continue
				}
				if definition.Test != "" && execution.Test != definition.Test {
					selectionConflict = true
					out.add(Issue{Code: IssueCaseTestMismatch, AtomID: atomID, CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "observed test/subtest differs from the declared case execution reference"})
				}
				if execution.CaseMetadata != nil && !sameCase(definition, *execution.CaseMetadata) {
					selectionConflict = true
					out.add(Issue{Code: IssueCaseMetadataConflict, AtomID: atomID, CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "recorded input, expectation, or case selection differs from the graph case descriptor"})
				}
				assessment.Executions = append(assessment.Executions, cloneExecution(execution))
				if execution.Producer != "" && !contains(assessment.ObservedProducers, execution.Producer) {
					assessment.ObservedProducers = append(assessment.ObservedProducers, execution.Producer)
				}
				if execution.ObservedProfile != "" && !contains(assessment.ObservedProfiles, execution.ObservedProfile) {
					assessment.ObservedProfiles = append(assessment.ObservedProfiles, execution.ObservedProfile)
				}
				if execution.Operation != "" && !contains(assessment.ObservedOperations, execution.Operation) {
					assessment.ObservedOperations = append(assessment.ObservedOperations, execution.Operation)
				}
				if execution.Target != "" && !contains(assessment.ObservedTargets, execution.Target) {
					assessment.ObservedTargets = append(assessment.ObservedTargets, execution.Target)
				}
				if (definition.Profile != execution.Profile && (definition.Profile != "" || execution.Profile != "")) || (execution.ObservedProfile != "" && execution.ObservedProfile != definition.Profile) {
					selectionConflict = true
					out.add(Issue{Code: IssueCaseProfileMismatch, AtomID: atomID, CaseID: caseID, Profile: execution.ObservedProfile, Target: execution.Target, Producer: execution.Producer, Message: "observed profile differs from the case profile selection"})
				}
				if definition.Operation != "" && execution.Operation != "" && execution.Operation != definition.Operation {
					selectionConflict = true
					out.add(Issue{Code: IssueCaseOperationMismatch, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Target: execution.Target, Producer: execution.Producer, Message: "observed operation differs from the case operation selection"})
				}
				if definition.Target != "" && execution.Target != "" && execution.Target != definition.Target {
					selectionConflict = true
					out.add(Issue{Code: IssueCaseTargetMismatch, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Target: execution.Target, Producer: execution.Producer, Message: "observed target differs from the case target selection"})
				}
				if definition.Producer != execution.Producer && (definition.Producer != "" || execution.Producer != "") {
					out.add(Issue{Code: IssueProducerIdentityDiffers, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Target: execution.Target, Producer: execution.Producer, Message: "observed producer identity differs from the declared producer; no component is assigned blame"})
				}
				switch {
				case execution.Verdict == "fail" || hasFailedComparison(execution.Comparisons):
					assessment.Status = "discrepancy"
					out.add(Issue{Code: IssueCaseDiscrepancy, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Target: execution.Target, Producer: execution.Producer, Message: "observed execution or comparison failed"})
				case execution.Verdict == "pass" && len(execution.Comparisons) > 0:
					if assessment.Status != "discrepancy" && applicable && !selectionConflict {
						assessment.Status = "verified_case"
					}
				default:
					if assessment.Status != "discrepancy" {
						assessment.Status = "unverified"
					}
					out.add(Issue{Code: IssueCaseUnverified, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Target: execution.Target, Producer: execution.Producer, Message: "execution has no passing recorded comparison or a non-passing terminal verdict"})
				}
				selectedProfiles[definition.Profile] = true
				out.assessProfile(atomID, definition, execution, profile, applicable, hasProfile)
			}
			if selectionConflict && assessment.Status != "discrepancy" {
				assessment.Status = "unverified"
			}
			qualification := profileQualification(requirements[atomID], definition.Profile)
			if !applicable {
				assessment.Qualification = "not_applicable"
			} else if len(assessment.Executions) == 0 {
				switch {
				case qualification == ontology.CoverageUnreachable:
					assessment.Status = "unreachable"
					assessment.Qualification = "unreachable"
					out.add(Issue{Code: IssueProfileUnreachable, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "case is qualified unreachable for this profile; no execution is claimed"})
				case qualification == ontology.CoverageUnsupported:
					assessment.Qualification = "unsupported"
					strength := requirementStrength(requirements[atomID], clauseByID)
					if strength == "MUST" {
						assessment.Status = "discrepancy"
						out.add(Issue{Code: IssueProfileUnsupported, Disposition: DispositionViolation, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "a MUST obligation is explicitly unsupported"})
					} else {
						assessment.Status = "unsupported"
						out.add(Issue{Code: IssueProfileUnsupported, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "case has an authored unsupported qualification; no execution is claimed"})
					}
				case requirementStrength(requirements[atomID], clauseByID) == "MAY":
					assessment.Status = "optional"
					assessment.Qualification = "optional"
				default:
					assessment.Status = "unverified"
					out.add(Issue{Code: IssueCaseMissingExecution, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "declared applicable case has no observed execution"})
				}
			} else {
				switch qualification {
				case ontology.CoverageUnsupported:
					assessment.Qualification = "unsupported"
					out.add(Issue{Code: IssueProfileUnsupported, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "unsupported qualification is retained beside the observed result"})
				case ontology.CoverageUnreachable:
					assessment.Qualification = "unreachable"
					out.add(Issue{Code: IssueProfileUnreachable, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "unreachable qualification is retained beside the observed result"})
				case ontology.CoverageUnverified:
					assessment.Qualification = "unverified"
					out.add(Issue{Code: IssueProfileUnverified, AtomID: atomID, CaseID: caseID, Profile: definition.Profile, Message: "authored unverified qualification is retained"})
				}
			}
			assessment.ObservedProducers = sortedStrings(assessment.ObservedProducers)
			assessment.ObservedProfiles = sortedStrings(assessment.ObservedProfiles)
			assessment.ObservedOperations = sortedStrings(assessment.ObservedOperations)
			assessment.ObservedTargets = sortedStrings(assessment.ObservedTargets)
			assessment.CaseFingerprint = caseReviewFingerprint(definition, assessment.Executions)
			assessment.ProducerFingerprint = producerFingerprint(definition.Producer, definition.Target, assessment.Executions)
			out.Cases = append(out.Cases, assessment)
		}
	}
	for _, profile := range config.Profiles {
		if !selectedProfiles[profile.ID] {
			out.Profiles = append(out.Profiles, ProfileAssessment{ID: profile.ID, Definition: cloneProfile(profile), Applicable: true, Status: "declared_unselected"})
		}
	}
	for _, clause := range config.Clauses {
		for _, requirementID := range mapKeys(clauseAtoms[clause.ID]) {
			if len(atomCases[requirementID]) == 0 {
				out.add(Issue{Code: IssueAtomMissingCases, AtomID: requirementID, ClauseID: clause.ID, Message: "inventory-linked atom has no declared case"})
			}
		}
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, link := range requirement.Precedence {
			target, exists := requirements[link.Target]
			if !exists || target.Status == ontology.StatusREJECTED {
				out.add(Issue{Code: IssuePrecedenceUnknownTarget, AtomID: requirement.ID, Target: link.Target, Scope: link.Scope, Message: "precedence target does not resolve to an active requirement"})
			}
		}
	}
	out.auditPrecedence(g, profiles, executions)
	out.auditCompositions(config.Compositions, caseDefinitions, executions)
	out.sort()
	return out
}

func (r *Report) add(issue Issue) {
	if issue.Disposition == "" {
		issue.Disposition = issueDisposition(issue.Code)
	}
	r.Issues = append(r.Issues, issue)
}
func (r Report) HasBlockingIssues() bool {
	for _, issue := range r.Issues {
		if issue.Disposition == DispositionViolation {
			return true
		}
	}
	return false
}

func (r *Report) assessProfile(atomID string, definition ontology.CaseDefinition, execution Execution, profile ontology.Profile, applicable, hasProfile bool) {
	status := "applicable"
	if !applicable {
		status = "not_applicable"
	}
	if !hasProfile && definition.Profile != "" {
		status = "unverified"
	}
	assessment := ProfileAssessment{
		ID: definition.Profile, AtomID: atomID, CaseID: definition.ID,
		Operation: definition.Operation, Target: definition.Target,
		Producer: execution.Producer, Applicable: applicable, Status: status,
	}
	if hasProfile {
		assessment.Definition = cloneProfile(profile)
	}
	r.Profiles = append(r.Profiles, assessment)
}

func (r *Report) auditPrecedence(g *ontology.Graph, profiles map[string]ontology.Profile, executions []Execution) {
	type edge struct {
		from string
		link ontology.PrecedenceLink
	}
	byScope := make(map[string][]edge)
	profileIDs := map[string]bool{"": true}
	operationIDs := map[string]bool{}
	requirementByID := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, requirement := range g.Requirements {
		requirementByID[requirement.ID] = requirement
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, link := range requirement.Precedence {
			target, exists := requirementByID[link.Target]
			if !exists || target.Status == ontology.StatusREJECTED {
				continue
			}
			byScope[link.Scope] = append(byScope[link.Scope], edge{from: requirement.ID, link: link})
			if link.Applicability != nil {
				for _, operation := range link.Applicability.Operations {
					operationIDs[operation] = true
				}
				for _, profileID := range link.Applicability.Profiles {
					profileIDs[profileID] = true
				}
			}
		}
		for _, definition := range requirement.Cases {
			if definition.Profile != "" {
				profileIDs[definition.Profile] = true
			}
			if definition.Operation != "" {
				operationIDs[definition.Operation] = true
			}
		}
	}
	for profileID := range profiles {
		profileIDs[profileID] = true
		for _, operation := range profiles[profileID].Operations {
			operationIDs[operation] = true
		}
	}
	for _, execution := range executions {
		if execution.Profile != "" {
			profileIDs[execution.Profile] = true
		}
		if operation := executionOperation(execution); operation != "" {
			operationIDs[operation] = true
		}
	}
	operations := mapKeys(operationIDs)
	if len(operations) == 0 {
		operations = []string{""}
	}
	for _, scope := range sortedKeys(byScope) {
		for _, profileID := range mapKeys(profileIDs) {
			profile, hasProfile := profiles[profileID]
			for _, operation := range operations {
				adjacent := make(map[string][]string)
				var active []edge
				for _, item := range byScope[scope] {
					if !applicabilityMatches(item.link.Applicability, profileID, operation, profile, hasProfile) {
						continue
					}
					adjacent[item.from] = append(adjacent[item.from], item.link.Target)
					active = append(active, item)
				}
				if cycle := precedenceCycle(adjacent); len(cycle) > 0 {
					r.add(Issue{Code: IssuePrecedenceCycle, Scope: scope, Profile: profileID, Operation: operation, Target: strings.Join(cycle, " -> "), Message: "strict precedence declarations contain a cycle in this scope and selected profile/operation"})
				}
				for _, item := range active {
					status := "unwitnessed"
					witnessFound := false
					for _, execution := range executions {
						if execution.Profile != profileID || executionOperation(execution) != operation || execution.Selection == nil {
							continue
						}
						if !contains(execution.Selection.Matched, item.from) || !contains(execution.Selection.Matched, item.link.Target) {
							continue
						}
						witnessFound = true
						if execution.Selection.Selected != item.from {
							status = "discrepancy"
							r.add(Issue{Code: IssuePrecedenceDiscrepancy, AtomID: item.from, CaseID: execution.CaseID, Profile: profileID, Operation: operation, Target: execution.Selection.Selected, Producer: execution.Producer, Scope: scope, Message: "observed competing selection did not select the declared higher-precedence atom"})
						} else if status != "discrepancy" {
							status = "witnessed"
						}
					}
					if !witnessFound {
						r.add(Issue{Code: IssuePrecedenceUnwitnessed, AtomID: item.from, Target: item.link.Target, Scope: scope, Profile: profileID, Operation: operation, Message: "no observed case selected with both precedence conditions matched"})
					}
					r.Precedence = append(r.Precedence, PrecedenceAssessment{AtomID: item.from, Target: item.link.Target, Scope: scope, Status: status, Profile: profileID, Operation: operation})
				}
			}
		}
	}
}

func (r *Report) auditCompositions(declarations []ontology.Composition, cases map[string]ontology.CaseDefinition, executions []Execution) {
	for _, declaration := range declarations {
		declaredComponents := orderedComponents(declaration.Components)
		declaredHash := fingerprint(componentIdentity(declaredComponents))
		assessment := CompositionAssessment{
			ID: declaration.ID, Components: declaredComponents,
			DeclaredFingerprint: declaredHash, Status: "declared",
		}
		selected := false
		for caseID, definition := range cases {
			if definition.Target != declaration.ID {
				continue
			}
			for _, execution := range executions {
				if execution.CaseID != caseID {
					continue
				}
				selected = true
				if execution.Target != declaration.ID {
					assessment.Status = "unmeasured"
					r.add(Issue{Code: IssueCompositionUnmeasured, CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "declared composed target was selected but no matching measured target identity was recorded"})
					continue
				}
				if len(execution.Components) == 0 {
					assessment.Status = "unmeasured"
					r.add(Issue{Code: IssueCompositionUnmeasured, AtomID: strings.Join(definition.AtomIDs, ","), CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "selected composition has no caller-recorded component evidence"})
					continue
				}
				measuredComponents := orderedComponents(execution.Components)
				assessment.ObservedComponents = measuredComponents
				assessment.MeasuredFingerprint = fingerprint(componentIdentity(measuredComponents))
				allMeasured := true
				for _, component := range measuredComponents {
					if !component.Measured {
						allMeasured = false
					}
				}
				switch {
				case !allMeasured:
					assessment.Status = "unmeasured"
					r.add(Issue{Code: IssueCompositionUnmeasured, CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "one or more caller-recorded composition components are marked as declared, not measured"})
				case !componentsMatch(declaredComponents, measuredComponents):
					assessment.Status = "changed"
					r.add(Issue{Code: IssueCompositionFingerprint, CaseID: caseID, Profile: execution.Profile, Target: execution.Target, Producer: execution.Producer, Message: "measured component identity differs from its composition declaration; no component is assigned blame"})
				default:
					assessment.Status = "measured_match"
				}
			}
		}
		if !selected {
			assessment.Status = "unselected"
		}
		r.Compositions = append(r.Compositions, assessment)
	}
}
func clauseApplicabilityMatches(g *ontology.Graph, atomID, profileID, operation string, profile ontology.Profile, hasProfile bool) bool {
	if g.Conformance == nil {
		return true
	}
	for _, requirement := range g.Requirements {
		if requirement.ID != atomID || requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, link := range requirement.ClauseLinks {
			for _, clause := range g.Conformance.Clauses {
				if clause.ID == link.ClauseID && !applicabilityMatches(clause.Applicability, profileID, operation, profile, hasProfile) {
					return false
				}
			}
		}
	}
	return true
}

func applicabilityMatches(app *ontology.Applicability, profileID, operation string, profile ontology.Profile, hasProfile bool) bool {
	if app == nil {
		return true
	}
	if len(app.Profiles) > 0 && !contains(app.Profiles, profileID) {
		return false
	}
	if len(app.Operations) > 0 {
		if operation != "" {
			if !contains(app.Operations, operation) {
				return false
			}
		} else if !hasProfile || !intersects(app.Operations, profile.Operations) {
			return false
		}
	}
	if operation != "" && hasProfile && len(profile.Operations) > 0 && !contains(profile.Operations, operation) {
		return false
	}
	if len(app.Features) > 0 {
		if !hasProfile {
			return false
		}
		features := make(map[string]bool, len(profile.Features))
		for _, feature := range profile.Features {
			features[feature] = true
		}
		for _, feature := range app.Features {
			if !features[feature] {
				return false
			}
		}
	}
	return true
}

func hasClauseSide(requirement ontology.Requirement, clauseID, side string) bool {
	for _, link := range requirement.ClauseLinks {
		if link.ClauseID == clauseID && link.Side == side {
			return true
		}
	}
	return false
}

func sourceLinkVerified(checks []source.Check, link ontology.SourceLink) bool {
	sourcePinVerified, anchorVerified := false, false
	for _, check := range checks {
		if check.SourceID != link.SourceID {
			continue
		}
		if check.Anchor == "" {
			sourcePinVerified = true
			if check.Status != source.StatusVerified {
				return false
			}
		}
		if check.Anchor == link.Anchor {
			anchorVerified = true
			if check.Status != source.StatusVerified {
				return false
			}
		}
	}
	return sourcePinVerified && anchorVerified
}

func sameCase(a, b ontology.CaseDefinition) bool {
	if !ontology.EqualObservedValues(a.Input, b.Input) || !ontology.EqualObservedValues(a.Expected, b.Expected) {
		return false
	}
	a.Input, b.Input = nil, nil
	a.Expected, b.Expected = nil, nil
	return ontology.EqualCaseDefinitions([]ontology.CaseDefinition{a}, []ontology.CaseDefinition{b})
}

func cloneObserved(value *ontology.ObservedValue) *ontology.ObservedValue {
	return ontology.CloneObservedValue(value)
}

func hasFailedComparison(comparisons []Comparison) bool {
	for _, comparison := range comparisons {
		if !comparison.Passed {
			return true
		}
	}
	return false
}

func precedenceCycle(adjacent map[string][]string) []string {
	state := make(map[string]uint8)
	var stack []string
	var cycle []string
	var visit func(string) bool
	visit = func(node string) bool {
		state[node] = 1
		stack = append(stack, node)
		for _, next := range adjacent[node] {
			if state[next] == 0 {
				if visit(next) {
					return true
				}
			} else if state[next] == 1 {
				start := 0
				for i := range stack {
					if stack[i] == next {
						start = i
						break
					}
				}
				cycle = append(append([]string(nil), stack[start:]...), next)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = 2
		return false
	}
	for _, node := range sortedKeys(adjacent) {
		if state[node] == 0 && visit(node) {
			return cycle
		}
	}
	return nil
}

func fingerprint(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func executionKey(caseID, atomID string) string { return caseID + "\x00" + atomID }

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func intersects(left, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func sortedKeys[V any](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func mapKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (r *Report) sort() {
	sort.Slice(r.Clauses, func(i, j int) bool { return r.Clauses[i].ID < r.Clauses[j].ID })
	for i := range r.Cases {
		sort.Slice(r.Cases[i].Executions, func(a, b int) bool {
			return comparisonSnapshotKey(r.Cases[i].Executions[a]) < comparisonSnapshotKey(r.Cases[i].Executions[b])
		})
	}
	sort.Slice(r.Cases, func(i, j int) bool {
		if r.Cases[i].ID != r.Cases[j].ID {
			return r.Cases[i].ID < r.Cases[j].ID
		}
		return r.Cases[i].AtomID < r.Cases[j].AtomID
	})
	sort.Slice(r.Profiles, func(i, j int) bool {
		if r.Profiles[i].ID != r.Profiles[j].ID {
			return r.Profiles[i].ID < r.Profiles[j].ID
		}
		if r.Profiles[i].CaseID != r.Profiles[j].CaseID {
			return r.Profiles[i].CaseID < r.Profiles[j].CaseID
		}
		if r.Profiles[i].AtomID != r.Profiles[j].AtomID {
			return r.Profiles[i].AtomID < r.Profiles[j].AtomID
		}
		return r.Profiles[i].Operation < r.Profiles[j].Operation
	})
	sort.Slice(r.Precedence, func(i, j int) bool {
		a, b := r.Precedence[i], r.Precedence[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Profile != b.Profile {
			return a.Profile < b.Profile
		}
		if a.Operation != b.Operation {
			return a.Operation < b.Operation
		}
		if a.AtomID != b.AtomID {
			return a.AtomID < b.AtomID
		}
		return a.Target < b.Target
	})
	sort.Slice(r.Compositions, func(i, j int) bool { return r.Compositions[i].ID < r.Compositions[j].ID })
	sort.Slice(r.Issues, func(i, j int) bool {
		a, b := r.Issues[i], r.Issues[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.ClauseID != b.ClauseID {
			return a.ClauseID < b.ClauseID
		}
		if a.AtomID != b.AtomID {
			return a.AtomID < b.AtomID
		}
		if a.CaseID != b.CaseID {
			return a.CaseID < b.CaseID
		}
		if a.Profile != b.Profile {
			return a.Profile < b.Profile
		}
		if a.Operation != b.Operation {
			return a.Operation < b.Operation
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Producer != b.Producer {
			return a.Producer < b.Producer
		}
		if a.Disposition != b.Disposition {
			return a.Disposition < b.Disposition
		}
		return a.Message < b.Message
	})
	r.Issues = slices.Compact(r.Issues)
	r.Discrepancies, r.ProfileQualifications = 0, 0
	for _, issue := range r.Issues {
		if issue.Code == IssueCaseDiscrepancy || issue.Code == IssuePrecedenceDiscrepancy ||
			(issue.Code == IssueProfileUnsupported && issue.Disposition == DispositionViolation) {
			r.Discrepancies++
		}
		if strings.HasPrefix(issue.Code, "profile_") || issue.Code == IssueCaseMissingExecution || issue.Code == IssueCaseUnverified || issue.Code == IssuePrecedenceUnwitnessed || issue.Code == IssueCompositionUnmeasured || issue.Code == IssueProducerIdentityDiffers {
			r.ProfileQualifications++
		}
	}
}
