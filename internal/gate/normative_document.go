package gate

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var normativeDocumentID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateNormativeDocument validates authored document identity, localization,
// source resolution and explicit case proof scope. Method links are traceability
// only: they never supply missing CaseDefinition.ClauseIDs or example bindings.
// Domains that do not opt into document metadata keep their existing contract.
func ValidateNormativeDocument(g *ontology.Graph, index *AtomSourceIndex) error {
	if g == nil {
		return fmt.Errorf("normative document graph is nil")
	}
	if g.Conformance == nil || len(g.Conformance.DocumentSections) == 0 {
		return nil
	}
	languages := g.Languages
	if len(languages) == 0 && index != nil {
		languages = index.claimLanguages()
	}
	if len(languages) == 0 {
		return fmt.Errorf("normative document has no declared languages")
	}
	validateTexts := func(label string, texts ontology.LocalizedText, required bool) error {
		if len(texts) == 0 && !required {
			return nil
		}
		if len(texts) != len(languages) {
			return fmt.Errorf("%s translations do not match declared languages", label)
		}
		for _, language := range languages {
			if strings.TrimSpace(texts[language]) == "" {
				return fmt.Errorf("%s has no non-empty translation for language %q", label, language)
			}
		}
		return nil
	}
	clauses := make(map[string]bool, len(g.Conformance.Clauses))
	clauseDefinitions := make(map[string]*ontology.SourceClause, len(g.Conformance.Clauses))
	for position := range g.Conformance.Clauses {
		clause := &g.Conformance.Clauses[position]
		if !normativeDocumentID.MatchString(clause.ID) || clauses[clause.ID] {
			return fmt.Errorf("normative document has invalid or duplicate clause ID %q", clause.ID)
		}
		clauses[clause.ID] = true
		clauseDefinitions[clause.ID] = clause
	}
	// A case can intentionally be shared by several property requirements, but
	// those declarations must be identical. Duplicates within one owner are not
	// shared bindings and cannot be silently merged.
	cases := make(map[string]ontology.CaseDefinition)
	caseLinks := make(map[string]map[string]bool)
	caseOwners := make(map[string][]*ontology.Requirement)
	for position := range g.Requirements {
		requirement := &g.Requirements[position]
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		seen := make(map[string]bool)
		for _, definition := range requirement.Cases {
			if strings.TrimSpace(definition.ID) == "" || seen[definition.ID] {
				return fmt.Errorf("requirement %q has empty or duplicate case ID %q", requirement.ID, definition.ID)
			}
			seen[definition.ID] = true
			if prior, exists := cases[definition.ID]; exists && !reflect.DeepEqual(prior, definition) {
				return fmt.Errorf("case %q has ambiguous declarations", definition.ID)
			}
			cases[definition.ID] = definition
			caseOwners[definition.ID] = append(caseOwners[definition.ID], requirement)
			if caseLinks[definition.ID] == nil {
				caseLinks[definition.ID] = make(map[string]bool)
			}
			for _, link := range requirement.ClauseLinks {
				if !clauses[link.ClauseID] {
					return fmt.Errorf("requirement %q links unknown clause %q", requirement.ID, link.ClauseID)
				}
				caseLinks[definition.ID][link.ClauseID] = true
			}
		}
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, definition := range requirement.Cases {
			seen := make(map[string]bool)
			for _, clauseID := range definition.ClauseIDs {
				if !clauses[clauseID] || seen[clauseID] {
					return fmt.Errorf("case %q has unknown or duplicate clause ID %q", definition.ID, clauseID)
				}
				if !caseLinks[definition.ID][clauseID] {
					return fmt.Errorf("case %q claims clause %q not linked by its declared property owners", definition.ID, clauseID)
				}
				seen[clauseID] = true
			}
		}
	}
	sections := make(map[string]ontology.DocumentSection)
	anchors := make(map[string]bool)
	orders := make(map[string]map[int]bool)
	for _, section := range g.Conformance.DocumentSections {
		if !normativeDocumentID.MatchString(section.ID) || anchors[section.ID] {
			return fmt.Errorf("normative document has invalid or duplicate section ID %q", section.ID)
		}
		anchors[section.ID] = true
		sections[section.ID] = section
		if section.Order < 0 {
			return fmt.Errorf("section %q has negative order", section.ID)
		}
		if orders[section.ParentID] == nil {
			orders[section.ParentID] = make(map[int]bool)
		}
		if orders[section.ParentID][section.Order] {
			return fmt.Errorf("section %q has ambiguous sibling order %d", section.ID, section.Order)
		}
		orders[section.ParentID][section.Order] = true
		if err := validateTexts("section "+section.ID+" title", section.TitleTexts, true); err != nil {
			return err
		}
	}
	for _, section := range g.Conformance.DocumentSections {
		seen := make(map[string]bool)
		for id := section.ID; id != ""; {
			if seen[id] {
				return fmt.Errorf("normative document section cycle at %q", id)
			}
			seen[id] = true
			parent, exists := sections[id]
			if !exists {
				return fmt.Errorf("section %q names unknown parent %q", section.ID, id)
			}
			id = parent.ParentID
		}
	}
	textOwners := make(map[string]string)
	clauseOwners := make(map[string]string)
	exampleOwners := make(map[string]string)
	for _, section := range g.Conformance.DocumentSections {
		for _, block := range section.Blocks {
			if !normativeDocumentID.MatchString(block.ID) || anchors[block.ID] {
				return fmt.Errorf("section %q has invalid or duplicate block ID %q", section.ID, block.ID)
			}
			anchors[block.ID] = true
			switch block.Role {
			case "normative":
				if len(block.ClauseIDs) == 0 {
					return fmt.Errorf("normative block %q has no explicit clause IDs", block.ID)
				}
			case "informative", "recommendation", "profile_qualification":
			default:
				return fmt.Errorf("block %q has unknown role %q", block.ID, block.Role)
			}
			if owner, exists := textOwners[block.TextRef]; exists {
				return fmt.Errorf("blocks %q and %q duplicate text reference %q", owner, block.ID, block.TextRef)
			}
			textOwners[block.TextRef] = block.ID
			texts, err := index.ResolveNormativeText(block.TextRef)
			if err != nil {
				return fmt.Errorf("section %q block %q: %w", section.ID, block.ID, err)
			}
			if err := validateTexts("block "+block.ID, texts, true); err != nil {
				return err
			}
			if err := validateTexts("block "+block.ID+" qualification", block.QualificationTexts, false); err != nil {
				return err
			}
			seenClauses := make(map[string]bool)
			for _, clauseID := range block.ClauseIDs {
				if !clauses[clauseID] || seenClauses[clauseID] {
					return fmt.Errorf("block %q has unknown or duplicate clause ID %q", block.ID, clauseID)
				}
				if prior, exists := clauseOwners[clauseID]; exists {
					return fmt.Errorf("clause %q has ambiguous document owners %q and %q", clauseID, prior, block.ID)
				}
				seenClauses[clauseID] = true
				clauseOwners[clauseID] = block.ID
			}
			for _, example := range block.Examples {
				caseID := example.CaseID
				if caseID == "" || strings.TrimSpace(caseID) != caseID {
					return fmt.Errorf("block %q has empty or malformed example case ID %q", block.ID, caseID)
				}
				if len(example.ComparisonNames) == 0 {
					return fmt.Errorf("example case %q requires explicit comparison names", caseID)
				}
				names := make(map[string]bool, len(example.ComparisonNames))
				for _, name := range example.ComparisonNames {
					if name == "" || strings.TrimSpace(name) != name || names[name] {
						return fmt.Errorf("example case %q has empty, malformed or duplicate comparison name %q", caseID, name)
					}
					names[name] = true
				}
				definition, exists := cases[caseID]
				if !exists {
					return fmt.Errorf("block %q names unknown or stale example case %q", block.ID, caseID)
				}
				if prior, duplicate := exampleOwners[caseID]; duplicate {
					return fmt.Errorf("example case %q is duplicated by blocks %q and %q", caseID, prior, block.ID)
				}
				relevant := false
				for _, clauseID := range definition.ClauseIDs {
					relevant = relevant || seenClauses[clauseID]
				}
				if !relevant {
					return fmt.Errorf("example case %q has no explicit clause binding relevant to block %q", caseID, block.ID)
				}
				if err := validateNormativeExampleApplicability(g, definition, clauseDefinitions, caseOwners[caseID]); err != nil {
					return err
				}
				exampleOwners[caseID] = block.ID
			}
		}
	}
	for _, clause := range g.Conformance.Clauses {
		if clauseOwners[clause.ID] == "" {
			return fmt.Errorf("source clause %q has no authored document block", clause.ID)
		}
	}
	return nil
}

func validateNormativeExampleApplicability(g *ontology.Graph, definition ontology.CaseDefinition, clauses map[string]*ontology.SourceClause, owners []*ontology.Requirement) error {
	var profile ontology.Profile
	hasProfile := false
	for _, declared := range g.Conformance.Profiles {
		if declared.ID == definition.Profile {
			profile, hasProfile = declared, true
			break
		}
	}
	if len(g.Conformance.Profiles) > 0 && !hasProfile {
		return fmt.Errorf("example case %q names unknown profile %q", definition.ID, definition.Profile)
	}
	for _, clauseID := range definition.ClauseIDs {
		clause := clauses[clauseID]
		if !conformance.ApplicabilityMatches(clause.Applicability, definition.Profile, definition.Operation, profile, hasProfile) {
			return fmt.Errorf("example case %q is inapplicable to scoped clause %q", definition.ID, clauseID)
		}
	}
	for _, owner := range owners {
		if !conformance.ApplicabilityMatches(owner.Applicability, definition.Profile, definition.Operation, profile, hasProfile) {
			return fmt.Errorf("example case %q is inapplicable to property %q", definition.ID, owner.ID)
		}
		if owner.Coverage != nil && owner.Coverage.Status != "" && (owner.Coverage.Profile == "" || owner.Coverage.Profile == definition.Profile) {
			return fmt.Errorf("example case %q has qualified %q coverage in property %q", definition.ID, owner.Coverage.Status, owner.ID)
		}
	}
	return nil
}
