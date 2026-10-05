package ontology

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
)

var (
	languageTagPattern = regexp.MustCompile(`^[a-z]{2,8}(?:-[a-z0-9]{1,8})*$`)
	integerPattern     = regexp.MustCompile(`^-?[0-9]+$`)
	floatBitsPattern   = regexp.MustCompile(`^[0-9a-fA-F]{16}$`)
)

// ValidateLocalizedText validates language keys and authored text values
// without imposing a domain-specific completeness requirement.
func ValidateLocalizedText(text LocalizedText) []ConformanceIssue {
	keys := make([]string, 0, len(text))
	for language := range text {
		keys = append(keys, language)
	}
	sort.Strings(keys)
	var issues []ConformanceIssue
	for _, language := range keys {
		if !languageTagPattern.MatchString(language) || !localization.Supported(language) {
			issues = append(issues, ConformanceIssue{ID: language, Kind: "language", Message: "localized text has an unsupported or unsafe language key"})
		}
		if strings.TrimSpace(text[language]) == "" {
			issues = append(issues, ConformanceIssue{ID: language, Kind: "language", Message: "localized text must be non-empty"})
		}
	}
	return issues
}

// ValidateConformance validates localized claims and the structural portions
// of declared conformance metadata. It does not audit authored source contents,
// execute cases, resolve test files, or claim semantic completeness.
func ValidateConformance(g *Graph) []ConformanceIssue {
	var issues []ConformanceIssue
	add := func(id, kind, format string, args ...any) {
		issues = append(issues, ConformanceIssue{ID: id, Kind: kind, Message: fmt.Sprintf(format, args...)})
	}
	if g == nil {
		return []ConformanceIssue{{ID: "graph", Kind: "graph", Message: "graph is nil"}}
	}

	langs := make(map[string]struct{}, len(g.Languages))
	if g.Languages != nil {
		if len(g.Languages) == 0 {
			add("languages", "language", "languages must contain at least one supported language")
		}
		for i, language := range g.Languages {
			if !languageTagPattern.MatchString(language) || strings.Contains(language, "..") || strings.ContainsAny(language, `/\\`) {
				add(fmt.Sprintf("languages[%d]", i), "language", "unsafe or non-lowercase language tag %q", language)
				continue
			}
			if !localization.Supported(language) {
				add(fmt.Sprintf("languages[%d]", i), "language", "language %q has no complete localization catalog", language)
			}
			if _, exists := langs[language]; exists {
				add(fmt.Sprintf("languages[%d]", i), "language", "duplicate language %q", language)
			}
			langs[language] = struct{}{}
		}
	}
	defaultLanguage := g.DefaultLanguage
	if g.Languages == nil {
		if defaultLanguage != "" {
			add("default_language", "language", "default_language requires a declared languages list")
		}
		if g.RenderLanguage != "" {
			add("render_language", "language", "render language %q is not declared", g.RenderLanguage)
		}
	} else {
		if defaultLanguage == "" && len(g.Languages) == 1 {
			defaultLanguage = g.Languages[0]
		}
		if len(g.Languages) > 1 && defaultLanguage == "" {
			add("default_language", "language", "default_language is required when multiple languages are declared")
		}
		if defaultLanguage != "" {
			if !languageTagPattern.MatchString(defaultLanguage) || strings.Contains(defaultLanguage, "..") || strings.ContainsAny(defaultLanguage, `/\\`) {
				add("default_language", "language", "unsafe or non-lowercase default_language %q", defaultLanguage)
			} else if !localization.Supported(defaultLanguage) {
				add("default_language", "language", "default_language %q has no complete localization catalog", defaultLanguage)
			}
			if _, ok := langs[defaultLanguage]; !ok {
				add("default_language", "language", "default_language %q is not listed in languages", defaultLanguage)
			}
		}
		if g.RenderLanguage != "" {
			if _, ok := langs[g.RenderLanguage]; !ok {
				add("render_language", "language", "render language %q is not declared", g.RenderLanguage)
			}
		}
	}

	reqByID := make(map[string]Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		if r.ID != "" {
			if _, exists := reqByID[r.ID]; exists {
				add(r.ID, "requirement", "duplicate requirement ID")
			}
			reqByID[r.ID] = r
		}
	}

	profiles := map[string]Profile{}
	compositions := map[string]Composition{}
	clauses := map[string]SourceClause{}
	if cfg := g.Conformance; cfg != nil {
		for i, p := range cfg.Profiles {
			if strings.TrimSpace(p.ID) == "" {
				add(fmt.Sprintf("profiles[%d]", i), "profile", "profile id is required")
				continue
			}
			if _, exists := profiles[p.ID]; exists {
				add(p.ID, "profile", "duplicate profile ID")
				continue
			}
			profiles[p.ID] = p
			validateProfile(p, add)
		}
		for i, c := range cfg.Compositions {
			if strings.TrimSpace(c.ID) == "" {
				add(fmt.Sprintf("compositions[%d]", i), "composition", "composition id is required")
				continue
			}
			if _, exists := compositions[c.ID]; exists {
				add(c.ID, "composition", "duplicate composition ID")
				continue
			}
			compositions[c.ID] = c
			validateComposition(c, add)
		}
		for i, c := range cfg.Clauses {
			if strings.TrimSpace(c.ID) == "" {
				add(fmt.Sprintf("clauses[%d]", i), "clause", "clause id is required")
				continue
			}
			if _, exists := clauses[c.ID]; exists {
				add(c.ID, "clause", "duplicate clause ID")
				continue
			}
			clauses[c.ID] = c
			validateClause(c, g.SpecificationSources, profiles, add)
		}
	}
	caseAtoms := map[string]struct{}{}
	for _, r := range g.Requirements {
		if r.Status == StatusREJECTED {
			continue
		}
		for _, c := range r.Cases {
			for _, atomID := range c.AtomIDs {
				caseAtoms[atomID] = struct{}{}
			}
		}
	}

	for _, r := range g.Requirements {
		validateClaimTexts(g, r, langs, defaultLanguage, add)
		retired := r.Status == StatusREJECTED
		if r.AtomKind != "" && r.AtomKind != "rule" {
			add(r.ID, "requirement", "atom_kind must be empty or %q, got %q", "rule", r.AtomKind)
		}
		if r.Strength != "" && !validStrength(r.Strength) {
			add(r.ID, "requirement", "strength must be MUST, SHOULD, or MAY, got %q", r.Strength)
		}
		if retired {
			continue
		}
		if r.AtomKind == "rule" && (g.Conformance == nil || !g.Conformance.RuleCases) {
			add(r.ID, "case", "rule atoms require the conformance.rule_cases trigger")
		}
		if r.AtomKind != "rule" && len(r.Cases) > 0 {
			add(r.ID, "case", "case definitions require atom_kind %q", "rule")
		}
		if r.AtomKind == "rule" && g.Conformance != nil && g.Conformance.RuleCases {
			if _, ok := caseAtoms[r.ID]; !ok {
				add(r.ID, "case", "rule atom has no case definitions under conformance.rule_cases")
			}
		}
		validateApplicability(r.ID, r.Applicability, profiles, add)
		type clauseLinkKey struct {
			clauseID string
			side     string
		}
		clauseLinks := map[clauseLinkKey]struct{}{}
		for i, link := range r.ClauseLinks {
			if strings.TrimSpace(link.ClauseID) == "" {
				add(r.ID, "clause", "clause_links[%d] clause_id is required", i)
			}
			key := clauseLinkKey{clauseID: link.ClauseID, side: link.Side}
			if _, exists := clauseLinks[key]; exists {
				add(r.ID, "clause", "duplicate clause link %q side %q", link.ClauseID, link.Side)
			}
			clauseLinks[key] = struct{}{}
		}
		for i, link := range r.ClauseLinks {
			clause, ok := clauses[link.ClauseID]
			if !ok {
				add(r.ID, "clause", "clause_links[%d] references unknown clause %q", i, link.ClauseID)
				continue
			}
			if len(clause.Sides) > 0 {
				if strings.TrimSpace(link.Side) == "" {
					add(r.ID, "clause", "clause_links[%d] must name a side of clause %q", i, link.ClauseID)
				} else if !containsString(clause.Sides, link.Side) {
					add(r.ID, "clause", "clause_links[%d] names undeclared side %q of clause %q", i, link.Side, link.ClauseID)
				}
			} else if link.Side != "" {
				add(r.ID, "clause", "clause_links[%d] names side %q but clause %q declares no sides", i, link.Side, link.ClauseID)
			}
		}
		validatePrecedence(r, reqByID, profiles, add)
	}

	caseByID := map[string]CaseDefinition{}
	caseOwner := map[string]string{}
	for _, r := range g.Requirements {
		for i, c := range r.Cases {
			path := fmt.Sprintf("%s.cases[%d]", r.ID, i)
			if strings.TrimSpace(c.ID) == "" {
				add(path, "case", "case id is required")
			} else if previous, exists := caseByID[c.ID]; exists {
				if !equalCaseDefinition(previous, c) {
					add(c.ID, "case", "conflicting metadata for duplicate case ID (first declared by %s)", caseOwner[c.ID])
				}
			} else {
				caseByID[c.ID] = c
				caseOwner[c.ID] = r.ID
			}
			active := r.Status != StatusREJECTED
			validateCase(path, c, reqByID, profiles, compositions, g.Conformance, active, add)
			if active && c.ID != "" && !containsString(c.AtomIDs, r.ID) {
				add(c.ID, "case", "case declared on requirement %q must include that requirement in atom_ids", r.ID)
			}
		}
	}

	validatePrecedenceCycles(g.Requirements, add)
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Kind != issues[j].Kind {
			return issues[i].Kind < issues[j].Kind
		}
		if issues[i].ID != issues[j].ID {
			return issues[i].ID < issues[j].ID
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

func validateClaimTexts(g *Graph, r Requirement, declared map[string]struct{}, defaultLanguage string, add func(string, string, string, ...any)) {
	if r.ClaimTexts == nil {
		if g.Languages != nil && len(g.Languages) > 1 {
			add(r.ID, "language", "claim_texts is required for every requirement in a multilingual domain")
		}
		return
	}
	if g.Languages == nil {
		add(r.ID, "language", "claim_texts requires an explicit languages declaration")
	}
	for _, issue := range ValidateLocalizedText(r.ClaimTexts) {
		add(r.ID, "language", "claim_texts[%q]: %s", issue.ID, issue.Message)
	}
	keys := make([]string, 0, len(r.ClaimTexts))
	for language := range r.ClaimTexts {
		keys = append(keys, language)
	}
	sort.Strings(keys)
	for _, language := range keys {
		if declared != nil {
			if _, ok := declared[language]; !ok {
				add(r.ID, "language", "claim_texts language %q is not declared", language)
			}
		}
	}
	if g.Languages != nil {
		for _, language := range g.Languages {
			text, ok := r.ClaimTexts[language]
			if !ok || strings.TrimSpace(text) == "" {
				add(r.ID, "language", "claim_texts has no non-empty text for declared language %q", language)
			}
		}
	}
	if defaultLanguage != "" {
		primary, ok := r.ClaimTexts[defaultLanguage]
		if !ok {
			add(r.ID, "language", "claim_texts has no primary text for default_language %q", defaultLanguage)
		} else if primary != r.Claim {
			add(r.ID, "language", "claim does not exactly match claim_texts[%q]", defaultLanguage)
		}
	}
}

func validateProfile(p Profile, add func(string, string, string, ...any)) {
	validateStringSet(p.ID, "operations", p.Operations, add)
	validateStringSet(p.ID, "features", p.Features, add)
	var min, max *big.Int
	if p.IntegerMin != "" {
		if !integerPattern.MatchString(p.IntegerMin) {
			add(p.ID, "profile", "integer_min %q is not a decimal integer", p.IntegerMin)
		} else {
			min, _ = new(big.Int).SetString(p.IntegerMin, 10)
		}
	}
	if p.IntegerMax != "" {
		if !integerPattern.MatchString(p.IntegerMax) {
			add(p.ID, "profile", "integer_max %q is not a decimal integer", p.IntegerMax)
		} else {
			max, _ = new(big.Int).SetString(p.IntegerMax, 10)
		}
	}
	if min != nil && max != nil && min.Cmp(max) > 0 {
		add(p.ID, "profile", "integer_min exceeds integer_max")
	}
	if p.FloatDomain != "" && strings.TrimSpace(p.FloatDomain) == "" {
		add(p.ID, "profile", "float_domain must not be whitespace")
	}
	if p.Rounding != "" && strings.TrimSpace(p.Rounding) == "" {
		add(p.ID, "profile", "rounding must not be whitespace")
	}
	for key := range p.Capabilities {
		if strings.TrimSpace(key) == "" {
			add(p.ID, "profile", "capability names must be non-empty")
		}
	}
}

func validateComposition(c Composition, add func(string, string, string, ...any)) {
	if len(c.Components) == 0 {
		add(c.ID, "composition", "composition must declare at least one component")
	}
	seen := map[string]struct{}{}
	for i, component := range c.Components {
		if strings.TrimSpace(component.ID) == "" {
			add(c.ID, "composition", "components[%d] id is required", i)
		} else if _, exists := seen[component.ID]; exists {
			add(c.ID, "composition", "duplicate component ID %q", component.ID)
		} else {
			seen[component.ID] = struct{}{}
		}
		if strings.TrimSpace(component.Role) == "" {
			add(c.ID, "composition", "components[%d] role is required", i)
		}
		validateSHA256(c.ID, fmt.Sprintf("components[%d].sha256", i), component.SHA256, "composition", add)
	}
}

func validateClause(c SourceClause, sources []SpecificationSource, profiles map[string]Profile, add func(string, string, string, ...any)) {
	if !validStrength(c.Strength) {
		add(c.ID, "clause", "strength must be MUST, SHOULD, or MAY, got %q", c.Strength)
	}
	validateStringSet(c.ID, "sides", c.Sides, add)
	validateApplicability(c.ID, c.Applicability, profiles, add)
	if len(c.SourceLinks) == 0 {
		add(c.ID, "clause", "source clause must declare at least one source link")
	}
	sourceIDs := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		sourceIDs[source.ID] = struct{}{}
	}
	for i, link := range c.SourceLinks {
		if strings.TrimSpace(link.SourceID) == "" {
			add(c.ID, "source", "source_links[%d] source_id is required", i)
		} else if _, exists := sourceIDs[link.SourceID]; !exists {
			add(c.ID, "source", "source_links[%d] references unknown specification source %q", i, link.SourceID)
		}
		if strings.TrimSpace(link.Anchor) == "" {
			add(c.ID, "source", "source_links[%d] anchor is required", i)
		}
	}
}

func validateCase(path string, c CaseDefinition, reqs map[string]Requirement, profiles map[string]Profile, compositions map[string]Composition, config *ConformanceConfig, active bool, add func(string, string, string, ...any)) {
	if active {
		if strings.TrimSpace(c.Test) == "" {
			add(path, "case", "test reference is required")
		} else if !isFileQualifiedTest(c.Test) {
			add(path, "case", "test reference %q must be file-qualified", c.Test)
		}
	}
	if strings.TrimSpace(c.Operation) != "" && strings.TrimSpace(c.Operation) != c.Operation {
		add(path, "case", "operation must not have surrounding whitespace")
	}
	if strings.TrimSpace(c.Producer) != "" && strings.TrimSpace(c.Producer) != c.Producer {
		add(path, "case", "producer must not have surrounding whitespace")
	}
	validateStringSet(path, "atom_ids", c.AtomIDs, add)
	if active {
		if len(c.AtomIDs) == 0 {
			add(path, "case", "case must identify at least one atom")
		}
		for _, atomID := range c.AtomIDs {
			r, ok := reqs[atomID]
			if !ok {
				add(path, "case", "atom_ids references unknown requirement %q", atomID)
			} else if r.AtomKind != "rule" {
				add(path, "case", "atom_ids requirement %q is not an explicit rule atom", atomID)
			}
		}
		if c.Profile != "" {
			if _, ok := profiles[c.Profile]; !ok {
				add(path, "case", "profile references unknown profile %q", c.Profile)
			}
		}
		if c.Target != "" && config != nil && config.Compositions != nil {
			if _, ok := compositions[c.Target]; !ok {
				add(path, "composition", "target references unknown composition %q", c.Target)
			}
		}
	}
	fixtures := map[string]struct{}{}
	validRoles := map[string]struct{}{"input": {}, "expected": {}, "canonical": {}, "error": {}, "extra": {}}
	for i, fixture := range c.Fixtures {
		if strings.TrimSpace(fixture.ID) == "" {
			add(path, "fixture", "fixtures[%d] id is required", i)
		} else if _, exists := fixtures[fixture.ID]; exists {
			add(path, "fixture", "duplicate fixture id %q", fixture.ID)
		} else {
			fixtures[fixture.ID] = struct{}{}
		}
		if strings.TrimSpace(fixture.Category) == "" {
			add(path, "fixture", "fixtures[%d] category is required", i)
		}
		if strings.TrimSpace(fixture.Path) == "" {
			add(path, "fixture", "fixtures[%d] path is required", i)
		}
		if _, ok := validRoles[fixture.Role]; !ok {
			add(path, "fixture", "fixtures[%d] has invalid role %q", i, fixture.Role)
		}
		if fixture.SHA256 == "" {
			add(path, "fixture", "fixtures[%d] sha256 is required", i)
		}
		validateSHA256(path, fmt.Sprintf("fixtures[%d].sha256", i), fixture.SHA256, "fixture", add)
	}
	conditionNames := make([]string, len(c.Conditions))
	for i, condition := range c.Conditions {
		conditionNames[i] = condition.Name
	}
	validateStringSet(path, "conditions", conditionNames, add)
	if c.Selection != nil {
		if c.Selection.Matched == nil {
			add(path, "selection", "matched must be an explicit list; use an empty list when nothing matched")
		}
		validateStringSet(path, "selection.matched", c.Selection.Matched, add)
		if c.Selection.Selected != "" && !containsString(c.Selection.Matched, c.Selection.Selected) {
			add(path, "selection", "selected branch %q is not in matched", c.Selection.Selected)
		}
	}
	if c.Input != nil {
		validateObservedValue(path+".input", c.Input, add)
	}
	if c.Expected != nil {
		validateObservedValue(path+".expected", c.Expected, add)
	}
	validateStringSet(path, "sides", c.Sides, add)
}

func validateObservedValue(path string, value *ObservedValue, add func(string, string, string, ...any)) {
	if value == nil {
		add(path, "value", "typed value must not be nil")
		return
	}
	present := 0
	count := func(ok bool) {
		if ok {
			present++
		}
	}
	count(value.Text != nil)
	count(value.Bool != nil)
	count(value.Encoding != "")
	count(value.Bytes != "" || value.hasBytes || (!value.decoded && value.Kind == "bytes"))
	count(value.Integer != "")
	count(value.FloatBits != "")
	count(value.Fields != nil || value.hasFields)
	count(value.Diagnostic != nil)
	scalarKindPresent := value.ScalarKind != "" || value.hasScalarKind
	if scalarKindPresent && strings.TrimSpace(value.ScalarKind) == "" {
		add(path, "value", "scalar_kind must be non-empty when present")
	}
	switch value.Kind {
	case "text":
		if value.Text == nil {
			add(path, "value", "text value requires a present text field (empty text is allowed)")
		}
		if present != 1 {
			add(path, "value", "text value has payload fields for other kinds")
		}
	case "bytes":
		if scalarKindPresent {
			add(path, "value", "scalar_kind is not valid for bytes values")
		}
		if value.Encoding != "base64" {
			add(path, "value", "bytes encoding must be %q", "base64")
		}
		if value.decoded && !value.hasBytes {
			add(path, "value", "bytes value requires a present bytes field (empty base64 is allowed)")
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(value.Bytes)
		if err != nil || base64.StdEncoding.EncodeToString(decoded) != value.Bytes {
			add(path, "value", "bytes payload is not canonical base64")
		}
		if present != 2 {
			add(path, "value", "bytes value has payload fields for other kinds")
		}
	case "bool":
		if value.Bool == nil {
			add(path, "value", "bool value requires a present bool field")
		}
		if present != 1 {
			add(path, "value", "bool value has payload fields for other kinds")
		}
	case "integer":
		if !integerPattern.MatchString(value.Integer) {
			add(path, "value", "integer must be a decimal integer string")
		}
		if present != 1 {
			add(path, "value", "integer value has payload fields for other kinds")
		}
	case "float":
		if !floatBitsPattern.MatchString(value.FloatBits) {
			add(path, "value", "float_bits must be exactly 16 hexadecimal digits")
		}
		if present != 1 {
			add(path, "value", "float value has payload fields for other kinds")
		}
	case "object":
		if scalarKindPresent {
			add(path, "value", "scalar_kind is not valid for object values")
		}
		if value.Fields == nil || (value.decoded && !value.hasFields) {
			add(path, "value", "object value requires a present fields object (an empty object is allowed)")
		}
		if present != 1 {
			add(path, "value", "object value has payload fields for other kinds")
		}
		keys := make([]string, 0, len(value.Fields))
		for key := range value.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := value.Fields[key]
			validateObservedValue(path+"."+key, &child, add)
		}
	case "null":
		if scalarKindPresent {
			add(path, "value", "scalar_kind is not valid for null values")
		}
		if present != 0 {
			add(path, "value", "null value must not carry a payload")
		}
	case "diagnostic":
		if scalarKindPresent {
			add(path, "value", "scalar_kind is not valid for diagnostic values")
		}
		if value.Diagnostic == nil {
			add(path, "value", "diagnostic value requires a diagnostic object")
		} else {
			validateDiagnostic(path+".diagnostic", value.Diagnostic, add)
		}
		if present != 1 {
			add(path, "value", "diagnostic value has payload fields for other kinds")
		}
	default:
		add(path, "value", "unknown observed value kind %q", value.Kind)
	}
}

func validateDiagnostic(path string, d *DiagnosticValue, add func(string, string, string, ...any)) {
	if strings.TrimSpace(d.Code) == "" {
		add(path, "value", "diagnostic code is required")
	}
	if d.Line != nil && *d.Line < 1 {
		add(path, "value", "diagnostic line must be one-based and positive")
	}
	if d.Span != nil {
		if d.Span.Start < 0 || d.Span.End < d.Span.Start {
			add(path, "value", "diagnostic span must be a non-negative half-open byte range")
		}
	}
}

func validateApplicability(id string, a *Applicability, profiles map[string]Profile, add func(string, string, string, ...any)) {
	if a == nil {
		return
	}
	validateStringSet(id, "applicability.operations", a.Operations, add)
	validateStringSet(id, "applicability.profiles", a.Profiles, add)
	validateStringSet(id, "applicability.features", a.Features, add)
	for _, profile := range a.Profiles {
		if _, ok := profiles[profile]; !ok {
			add(id, "profile", "applicability references unknown profile %q", profile)
		}
	}
}

func validatePrecedence(r Requirement, reqs map[string]Requirement, profiles map[string]Profile, add func(string, string, string, ...any)) {
	type precedenceKey struct {
		scope  string
		target string
	}
	seen := map[precedenceKey]struct{}{}
	for i, link := range r.Precedence {
		if strings.TrimSpace(link.Scope) == "" {
			add(r.ID, "precedence", "precedence[%d] scope is required", i)
		}
		if strings.TrimSpace(link.Target) == "" {
			add(r.ID, "precedence", "precedence[%d] target is required", i)
		} else if _, ok := reqs[link.Target]; !ok {
			add(r.ID, "precedence", "precedence[%d] references unknown target %q", i, link.Target)
		}
		key := precedenceKey{scope: link.Scope, target: link.Target}
		if _, exists := seen[key]; exists {
			add(r.ID, "precedence", "duplicate precedence target %q in scope %q", link.Target, link.Scope)
		}
		seen[key] = struct{}{}
		validateApplicability(r.ID, link.Applicability, profiles, add)
	}
}

func validatePrecedenceCycles(requirements []Requirement, add func(string, string, string, ...any)) {
	byScope := map[string]map[string][]string{}
	for _, r := range requirements {
		if r.Status == StatusREJECTED {
			continue
		}
		for _, link := range r.Precedence {
			if link.Scope == "" || link.Target == "" {
				continue
			}
			if byScope[link.Scope] == nil {
				byScope[link.Scope] = map[string][]string{}
			}
			byScope[link.Scope][r.ID] = append(byScope[link.Scope][r.ID], link.Target)
		}
	}
	scopes := make([]string, 0, len(byScope))
	for scope := range byScope {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	for _, scope := range scopes {
		adj := byScope[scope]
		state := map[string]uint8{}
		var visit func(string)
		visit = func(node string) {
			if state[node] == 2 {
				return
			}
			if state[node] == 1 {
				add(node, "precedence", "strict precedence cycle in scope %q", scope)
				return
			}
			state[node] = 1
			next := append([]string(nil), adj[node]...)
			sort.Strings(next)
			for _, target := range next {
				visit(target)
			}
			state[node] = 2
		}
		roots := make([]string, 0, len(adj))
		for node := range adj {
			roots = append(roots, node)
		}
		sort.Strings(roots)
		for _, node := range roots {
			visit(node)
		}
	}
}

func validateSHA256(id, field, value, kind string, add func(string, string, string, ...any)) {
	if value == "" {
		return
	}
	if len(value) != 64 {
		add(id, kind, "%s must be 64 hexadecimal characters", field)
		return
	}
	if _, err := hex.DecodeString(value); err != nil {
		add(id, kind, "%s must be 64 hexadecimal characters", field)
	}
}

func validateStringSet(id, field string, values []string, add func(string, string, string, ...any)) {
	seen := map[string]struct{}{}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			add(id, "schema", "%s[%d] must be non-empty", field, i)
			continue
		}
		if value != strings.TrimSpace(value) {
			add(id, "schema", "%s[%d] must not have surrounding whitespace", field, i)
		}
		if _, exists := seen[value]; exists {
			add(id, "schema", "%s contains duplicate value %q", field, value)
		}
		seen[value] = struct{}{}
	}
}

func validStrength(s string) bool { return s == "MUST" || s == "SHOULD" || s == "MAY" }

func isFileQualifiedTest(ref string) bool {
	colon := strings.LastIndexByte(ref, ':')
	if colon <= 0 || colon == len(ref)-1 {
		return false
	}
	file, test := ref[:colon], ref[colon+1:]
	return strings.TrimSpace(file) == file && strings.TrimSpace(test) == test && strings.Contains(file, ".go")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
