package ontology

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// LocalizedText carries authored text by declared language tag. A missing key
// is missing data; consumers must never infer or fall back to another language.
type LocalizedText map[string]string

func decodeStrictObject(data []byte, label string, target any, required, nonNull []string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return nil, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%s has trailing JSON data", label)
		}
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("%s.%s is required", label, name)
		}
	}
	for _, name := range nonNull {
		if raw, ok := fields[name]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, fmt.Errorf("%s.%s must be omitted or carry a typed value, not null", label, name)
		}
	}
	return fields, nil
}

func (text *LocalizedText) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("localized text must be an object, not null")
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	for language, value := range encoded {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("localized text %q must not be null", language)
		}
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*text = LocalizedText(values)
	return nil
}

// ConformanceConfig contains independently triggered conformance declarations.
type ConformanceConfig struct {
	RuleCases        bool              `json:"rule_cases,omitempty"`
	Clauses          []SourceClause    `json:"clauses,omitempty"`
	Profiles         []Profile         `json:"profiles,omitempty"`
	Compositions     []Composition     `json:"compositions,omitempty"`
	DocumentSections []DocumentSection `json:"document_sections,omitempty"`
}

func (c *ConformanceConfig) UnmarshalJSON(data []byte) error {
	type wire struct {
		RuleCases        bool              `json:"rule_cases,omitempty"`
		Clauses          []SourceClause    `json:"clauses,omitempty"`
		Profiles         []Profile         `json:"profiles,omitempty"`
		Compositions     []Composition     `json:"compositions,omitempty"`
		DocumentSections []DocumentSection `json:"document_sections,omitempty"`
	}
	var decoded wire
	if _, err := decodeStrictObject(data, "conformance", &decoded, nil,
		[]string{"rule_cases", "clauses", "profiles", "compositions", "document_sections"}); err != nil {
		return err
	}
	*c = ConformanceConfig{
		RuleCases: decoded.RuleCases, Clauses: decoded.Clauses,
		Profiles: decoded.Profiles, Compositions: decoded.Compositions,
		DocumentSections: decoded.DocumentSections,
	}
	return nil
}

func (c ConformanceConfig) MarshalJSON() ([]byte, error) {
	type wire struct {
		RuleCases        bool               `json:"rule_cases,omitempty"`
		Clauses          *[]SourceClause    `json:"clauses,omitempty"`
		Profiles         *[]Profile         `json:"profiles,omitempty"`
		Compositions     *[]Composition     `json:"compositions,omitempty"`
		DocumentSections *[]DocumentSection `json:"document_sections,omitempty"`
	}
	out := wire{RuleCases: c.RuleCases}
	if c.Clauses != nil {
		clauses := c.Clauses
		out.Clauses = &clauses
	}
	if c.Profiles != nil {
		profiles := c.Profiles
		out.Profiles = &profiles
	}
	if c.Compositions != nil {
		compositions := c.Compositions
		out.Compositions = &compositions
	}
	if c.DocumentSections != nil {
		sections := c.DocumentSections
		out.DocumentSections = &sections
	}
	return json.Marshal(out)
}

// DocumentSection declares reader-facing structure independently of execution.
type DocumentSection struct {
	ID         string           `json:"id"`
	ParentID   string           `json:"parent_id,omitempty"`
	Order      int              `json:"order"`
	TitleTexts LocalizedText    `json:"title_texts"`
	Blocks     []NormativeBlock `json:"blocks,omitempty"`
}

// NormativeBlock binds authored prose to explicit obligations and examples.
// Role is normative, informative, recommendation, or profile_qualification.
type NormativeBlock struct {
	ID                 string            `json:"id"`
	TextRef            string            `json:"text_ref"`
	ClauseIDs          []string          `json:"clause_ids,omitempty"`
	Role               string            `json:"role"`
	Examples           []DocumentExample `json:"examples,omitempty"`
	QualificationTexts LocalizedText     `json:"qualification_texts,omitempty"`
}

// DocumentExample selects uniquely named comparisons from one case.
type DocumentExample struct {
	CaseID          string   `json:"case_id"`
	ComparisonNames []string `json:"comparison_names"`
}

func (s *DocumentSection) UnmarshalJSON(data []byte) error {
	type wire DocumentSection
	var value wire
	if _, err := decodeStrictObject(data, "document_section", &value,
		[]string{"id", "order", "title_texts"},
		[]string{"id", "parent_id", "order", "title_texts", "blocks"}); err != nil {
		return err
	}
	*s = DocumentSection(value)
	return nil
}

func (b *NormativeBlock) UnmarshalJSON(data []byte) error {
	type wire NormativeBlock
	var value wire
	if _, err := decodeStrictObject(data, "normative_block", &value,
		[]string{"id", "text_ref", "role"},
		[]string{"id", "text_ref", "clause_ids", "role", "examples", "qualification_texts"}); err != nil {
		return err
	}
	*b = NormativeBlock(value)
	return nil
}

func (e *DocumentExample) UnmarshalJSON(data []byte) error {
	type wire DocumentExample
	var value wire
	if _, err := decodeStrictObject(data, "document_example", &value,
		[]string{"case_id", "comparison_names"}, []string{"case_id", "comparison_names"}); err != nil {
		return err
	}
	if value.CaseID == "" || strings.TrimSpace(value.CaseID) != value.CaseID {
		return fmt.Errorf("document example case_id %q is empty or has surrounding whitespace", value.CaseID)
	}
	if len(value.ComparisonNames) == 0 {
		return fmt.Errorf("document example %q requires explicit comparison_names", value.CaseID)
	}
	seen := make(map[string]bool, len(value.ComparisonNames))
	for _, name := range value.ComparisonNames {
		if name == "" || strings.TrimSpace(name) != name || seen[name] {
			return fmt.Errorf("document example %q has empty, malformed or duplicate comparison name %q", value.CaseID, name)
		}
		seen[name] = true
	}
	*e = DocumentExample(value)
	return nil
}

// SourceClause identifies one source-owned normative unit and the obligations
// it requires an implementation to address.
type SourceClause struct {
	ID            string         `json:"id"`
	SourceLinks   []SourceLink   `json:"source_links,omitempty"`
	Sides         []string       `json:"sides,omitempty"`
	Strength      string         `json:"strength,omitempty"`
	Applicability *Applicability `json:"applicability,omitempty"`
}

// ClauseLink connects a requirement to one side of a declared source clause.
type ClauseLink struct {
	ClauseID string `json:"clause_id"`
	Side     string `json:"side,omitempty"`
}

// CaseDefinition holds persistent case metadata. Test is the file-qualified
// execution reference; Input and Expected are the declared input/oracle, not
// the SUT's actual result. Runtime observations are stored separately.
type CaseDefinition struct {
	ID        string   `json:"id"`
	Test      string   `json:"test"`
	AtomIDs   []string `json:"atom_ids,omitempty"`
	ClauseIDs []string `json:"clause_ids,omitempty"`
	Profile   string   `json:"profile,omitempty"`
	Target    string   `json:"target,omitempty"`
	// Operation and Producer are declared case context, never inferred from a result.
	Operation  string              `json:"operation,omitempty"`
	Producer   string              `json:"producer,omitempty"`
	Input      *ObservedValue      `json:"input,omitempty"`
	Expected   *ObservedValue      `json:"expected,omitempty"`
	Fixtures   []FixtureRef        `json:"fixtures,omitempty"`
	Conditions []ConditionEvidence `json:"conditions,omitempty"`
	Sides      []string            `json:"sides,omitempty"`
	Selection  *SelectionEvidence  `json:"selection,omitempty"`
}

// FixtureRef identifies domain-owned corpus material by category and ID,
// pins exact content bytes with SHA-256, and is resolved by the corpus reader.
type FixtureRef struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Role     string `json:"role"`
	SHA256   string `json:"sha256"`
	RawBytes bool   `json:"raw_bytes"`
}

// ObservedValue is the transport-neutral exact value envelope used by authored
// expectations and recorded observations. Payload fields are interpreted by
// Kind; nil pointers distinguish absent values from explicit empty values.
type ObservedValue struct {
	Kind string  `json:"kind"`
	Text *string `json:"text,omitempty"`
	// Bool is a pointer so an observed false remains distinct from missing data.
	Bool     *bool  `json:"bool,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Bytes    string `json:"bytes,omitempty"`
	// ScalarKind records the producer-supplied scalar type, not its value.
	ScalarKind    string                   `json:"scalar_kind,omitempty"`
	Integer       string                   `json:"integer,omitempty"`
	FloatBits     string                   `json:"float_bits,omitempty"`
	Fields        map[string]ObservedValue `json:"fields,omitempty"`
	Items         []ObservedValue          `json:"items,omitempty"`
	Diagnostic    *DiagnosticValue         `json:"diagnostic,omitempty"`
	decoded       bool
	hasBytes      bool
	hasFields     bool
	hasItems      bool
	hasScalarKind bool
}

func (v *ObservedValue) UnmarshalJSON(data []byte) error {
	type wire ObservedValue
	var value wire
	fields, err := decodeStrictObject(data, "observed_value", &value,
		[]string{"kind"},
		[]string{"kind", "text", "bool", "encoding", "bytes", "scalar_kind", "integer", "float_bits", "fields", "items", "diagnostic"})
	if err != nil {
		return err
	}
	*v = ObservedValue(value)
	v.decoded = true
	_, v.hasBytes = fields["bytes"]
	_, v.hasScalarKind = fields["scalar_kind"]
	_, v.hasFields = fields["fields"]
	_, v.hasItems = fields["items"]
	return nil
}

func (v ObservedValue) MarshalJSON() ([]byte, error) {
	type wire struct {
		Kind       string                    `json:"kind"`
		Text       *string                   `json:"text,omitempty"`
		Bool       *bool                     `json:"bool,omitempty"`
		Encoding   string                    `json:"encoding,omitempty"`
		Bytes      *string                   `json:"bytes,omitempty"`
		ScalarKind *string                   `json:"scalar_kind,omitempty"`
		Integer    string                    `json:"integer,omitempty"`
		FloatBits  string                    `json:"float_bits,omitempty"`
		Fields     *map[string]ObservedValue `json:"fields,omitempty"`
		Items      *[]ObservedValue          `json:"items,omitempty"`
		Diagnostic *DiagnosticValue          `json:"diagnostic,omitempty"`
	}
	out := wire{
		Kind: v.Kind, Text: v.Text, Bool: v.Bool, Encoding: v.Encoding,
		Integer: v.Integer, FloatBits: v.FloatBits, Diagnostic: v.Diagnostic,
	}
	if v.ScalarKind != "" || v.hasScalarKind {
		scalarKind := v.ScalarKind
		out.ScalarKind = &scalarKind
	}
	if v.Kind == "bytes" || v.Bytes != "" || v.hasBytes {
		value := v.Bytes
		out.Bytes = &value
	}
	if v.Kind == "object" || v.Fields != nil || v.hasFields {
		fields := v.Fields
		out.Fields = &fields
	}
	if v.Kind == "array" || v.Items != nil || v.hasItems {
		items := v.Items
		out.Items = &items
	}
	return json.Marshal(out)
}

// Validate checks the kind-specific payload and explicit absence semantics.
func (v ObservedValue) Validate() error {
	var problems []string
	validateObservedValue("value", &v, func(_ string, _ string, format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	})
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// DiagnosticValue preserves only diagnostic fields supplied by the producer.
type DiagnosticValue struct {
	Code   string    `json:"code"`
	Class  *string   `json:"class,omitempty"`
	Reason *string   `json:"reason,omitempty"`
	Line   *int      `json:"line,omitempty"`
	Span   *ByteSpan `json:"span,omitempty"`
}

func (d *DiagnosticValue) UnmarshalJSON(data []byte) error {
	type wire DiagnosticValue
	var value wire
	if _, err := decodeStrictObject(data, "diagnostic", &value,
		[]string{"code"}, []string{"code", "class", "reason", "line", "span"}); err != nil {
		return err
	}
	*d = DiagnosticValue(value)
	return nil
}

// ByteSpan is a half-open byte range [Start, End).
type ByteSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (span *ByteSpan) UnmarshalJSON(data []byte) error {
	type wire ByteSpan
	var value wire
	if _, err := decodeStrictObject(data, "byte_span", &value,
		[]string{"start", "end"}, []string{"start", "end"}); err != nil {
		return err
	}
	*span = ByteSpan(value)
	return nil
}

// ConditionEvidence records a producer-observed condition result.
type ConditionEvidence struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

func (e *ConditionEvidence) UnmarshalJSON(data []byte) error {
	type wire ConditionEvidence
	var value wire
	if _, err := decodeStrictObject(data, "condition_evidence", &value,
		[]string{"name", "matched"}, []string{"name", "matched"}); err != nil {
		return err
	}
	*e = ConditionEvidence(value)
	return nil
}

// SelectionEvidence records producer-observed branch selection. Empty Matched
// and Selected values are meaningful when no branch was observed to match.
type SelectionEvidence struct {
	Matched  []string `json:"matched"`
	Selected string   `json:"selected"`
}

// Applicability describes the profile/case context where an obligation applies.
type Applicability struct {
	Operations []string `json:"operations,omitempty"`
	Profiles   []string `json:"profiles,omitempty"`
	Features   []string `json:"features,omitempty"`
}

func (e *SelectionEvidence) UnmarshalJSON(data []byte) error {
	type wire SelectionEvidence
	var value wire
	if _, err := decodeStrictObject(data, "selection_evidence", &value,
		[]string{"matched", "selected"}, []string{"matched", "selected"}); err != nil {
		return err
	}
	*e = SelectionEvidence(value)
	return nil
}

// Profile declares a reusable implementation capability context.
type Profile struct {
	ID             string            `json:"id"`
	Operations     []string          `json:"operations,omitempty"`
	Features       []string          `json:"features,omitempty"`
	IntegerMin     string            `json:"integer_min,omitempty"`
	IntegerMax     string            `json:"integer_max,omitempty"`
	FloatDomain    string            `json:"float_domain,omitempty"`
	Rounding       string            `json:"rounding,omitempty"`
	PreservesOrder *bool             `json:"preserves_order,omitempty"`
	Capabilities   map[string]string `json:"capabilities,omitempty"`
}

// Composition declares a named implementation target and its components.
type Composition struct {
	ID         string      `json:"id"`
	Components []Component `json:"components,omitempty"`
}

// Component records a declared or measured implementation component.
type Component struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Version  string `json:"version,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	Measured bool   `json:"measured"`
}

// PrecedenceLink states that the owning requirement strictly precedes Target
// within Scope; it is not a dependency or source-order relationship.
type PrecedenceLink struct {
	Target        string         `json:"target"`
	Scope         string         `json:"scope"`
	Applicability *Applicability `json:"applicability,omitempty"`
}

// ConformanceIssue is a stable, machine-readable schema/audit issue.
type ConformanceIssue struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

// CloneRequirementMetadata deep-copies the localized and conformance fields
// of a Requirement while leaving its existing fields unchanged.
func CloneRequirementMetadata(r Requirement) Requirement {
	if r.ClaimTexts != nil {
		texts := make(LocalizedText, len(r.ClaimTexts))
		for language, text := range r.ClaimTexts {
			texts[language] = text
		}
		r.ClaimTexts = texts
	}
	r.Cases = CloneCaseDefinitions(r.Cases)
	r.ClauseLinks = cloneSlice(r.ClauseLinks)
	r.Applicability = cloneApplicability(r.Applicability)
	r.Precedence = clonePrecedenceLinks(r.Precedence)
	return r
}

// EqualCaseDefinitions compares persisted case metadata. Typed values use
// EqualObservedValues so private decoder bookkeeping is not structural data.
func EqualCaseDefinitions(a, b []CaseDefinition) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalCaseDefinition(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalCaseDefinition(a, b CaseDefinition) bool {
	if a.ID != b.ID || a.Test != b.Test ||
		a.Profile != b.Profile || a.Target != b.Target ||
		a.Operation != b.Operation || a.Producer != b.Producer ||
		!slices.Equal(a.AtomIDs, b.AtomIDs) ||
		!sameSliceValues(a.ClauseIDs, b.ClauseIDs) ||
		!EqualObservedValues(a.Input, b.Input) ||
		!EqualObservedValues(a.Expected, b.Expected) ||
		!slices.Equal(a.Fixtures, b.Fixtures) ||
		!slices.Equal(a.Conditions, b.Conditions) ||
		!slices.Equal(a.Sides, b.Sides) {
		return false
	}
	if (a.Selection == nil) != (b.Selection == nil) {
		return false
	}
	return a.Selection == nil ||
		(a.Selection.Selected == b.Selection.Selected &&
			sameSliceValues(a.Selection.Matched, b.Selection.Matched))
}

func sameSliceValues[T comparable](a, b []T) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CloneCaseDefinitions preserves transport presence and isolates nested metadata.
func CloneCaseDefinitions(in []CaseDefinition) []CaseDefinition {
	if in == nil {
		return nil
	}
	out := make([]CaseDefinition, len(in))
	copy(out, in)
	for i := range out {
		out[i].AtomIDs = cloneSlice(in[i].AtomIDs)
		out[i].ClauseIDs = cloneSlice(in[i].ClauseIDs)
		out[i].Fixtures = cloneSlice(in[i].Fixtures)
		out[i].Conditions = cloneSlice(in[i].Conditions)
		out[i].Sides = cloneSlice(in[i].Sides)
		out[i].Input = CloneObservedValue(in[i].Input)
		out[i].Expected = CloneObservedValue(in[i].Expected)
		if in[i].Selection != nil {
			selection := *in[i].Selection
			selection.Matched = cloneSlice(in[i].Selection.Matched)
			out[i].Selection = &selection
		}
	}
	return out
}

// CloneObservedValue preserves transport presence and isolates mutable payloads.
func CloneObservedValue(in *ObservedValue) *ObservedValue {
	if in == nil {
		return nil
	}
	out := cloneObservedPayload(in)
	return &out
}

func cloneObservedPayload(in *ObservedValue) ObservedValue {
	out := *in
	if in.Text != nil {
		text := *in.Text
		out.Text = &text
	}
	if in.Bool != nil {
		boolean := *in.Bool
		out.Bool = &boolean
	}
	if in.Fields != nil {
		out.Fields = make(map[string]ObservedValue, len(in.Fields))
		for key, field := range in.Fields {
			out.Fields[key] = cloneObservedPayload(&field)
		}
	}
	if in.Items != nil {
		out.Items = make([]ObservedValue, len(in.Items))
		for i := range in.Items {
			out.Items[i] = cloneObservedPayload(&in.Items[i])
		}
	}
	if in.Diagnostic != nil {
		diagnostic := *in.Diagnostic
		if in.Diagnostic.Class != nil {
			class := *in.Diagnostic.Class
			diagnostic.Class = &class
		}
		if in.Diagnostic.Reason != nil {
			reason := *in.Diagnostic.Reason
			diagnostic.Reason = &reason
		}
		if in.Diagnostic.Line != nil {
			line := *in.Diagnostic.Line
			diagnostic.Line = &line
		}
		if in.Diagnostic.Span != nil {
			span := *in.Diagnostic.Span
			diagnostic.Span = &span
		}
		out.Diagnostic = &diagnostic
	}
	return out
}

func cloneApplicability(in *Applicability) *Applicability {
	if in == nil {
		return nil
	}
	out := *in
	out.Operations = cloneSlice(in.Operations)
	out.Profiles = cloneSlice(in.Profiles)
	out.Features = cloneSlice(in.Features)
	return &out
}

func clonePrecedenceLinks(in []PrecedenceLink) []PrecedenceLink {
	if in == nil {
		return nil
	}
	out := make([]PrecedenceLink, len(in))
	copy(out, in)
	for i := range out {
		out[i].Applicability = cloneApplicability(in[i].Applicability)
	}
	return out
}

func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}
func (c *CaseDefinition) UnmarshalJSON(data []byte) error {
	type wire CaseDefinition
	var value wire
	if _, err := decodeStrictObject(data, "case_definition", &value,
		[]string{"id"},
		[]string{"id", "test", "atom_ids", "clause_ids", "profile", "target", "operation", "producer", "input", "expected", "fixtures", "conditions", "sides", "selection"}); err != nil {
		return err
	}
	*c = CaseDefinition(value)
	return nil
}

// MarshalJSON retains an explicitly empty proof scope instead of reverting to
// the legacy absent-scope interpretation after a persisted round trip.
func (c CaseDefinition) MarshalJSON() ([]byte, error) {
	type plain CaseDefinition
	wire := struct {
		plain
		ClauseIDs *[]string `json:"clause_ids,omitempty"`
	}{plain: plain(c)}
	if c.ClauseIDs != nil {
		wire.ClauseIDs = &c.ClauseIDs
	}
	return json.Marshal(wire)
}

func (f *FixtureRef) UnmarshalJSON(data []byte) error {
	type wire FixtureRef
	var value wire
	if _, err := decodeStrictObject(data, "fixture", &value,
		[]string{"id", "category", "path", "role", "sha256", "raw_bytes"},
		[]string{"id", "category", "path", "role", "sha256", "raw_bytes"}); err != nil {
		return err
	}
	*f = FixtureRef(value)
	return nil
}

func (l *ClauseLink) UnmarshalJSON(data []byte) error {
	type wire ClauseLink
	var value wire
	if _, err := decodeStrictObject(data, "clause_link", &value,
		[]string{"clause_id"}, []string{"clause_id", "side"}); err != nil {
		return err
	}
	*l = ClauseLink(value)
	return nil
}

func (c *SourceClause) UnmarshalJSON(data []byte) error {
	type wire SourceClause
	var value wire
	if _, err := decodeStrictObject(data, "source_clause", &value,
		[]string{"id"}, []string{"id", "source_links", "sides", "strength", "applicability"}); err != nil {
		return err
	}
	*c = SourceClause(value)
	return nil
}

func (a *Applicability) UnmarshalJSON(data []byte) error {
	type wire Applicability
	var value wire
	if _, err := decodeStrictObject(data, "applicability", &value, nil,
		[]string{"operations", "profiles", "features"}); err != nil {
		return err
	}
	*a = Applicability(value)
	return nil
}

func (p *Profile) UnmarshalJSON(data []byte) error {
	type wire Profile
	var value wire
	fields, err := decodeStrictObject(data, "profile", &value,
		[]string{"id"},
		[]string{"id", "operations", "features", "integer_min", "integer_max", "float_domain", "rounding", "preserves_order", "capabilities"})
	if err != nil {
		return err
	}
	if raw, ok := fields["capabilities"]; ok {
		var capabilities map[string]json.RawMessage
		if err := json.Unmarshal(raw, &capabilities); err != nil {
			return err
		}
		for name, capability := range capabilities {
			if bytes.Equal(bytes.TrimSpace(capability), []byte("null")) {
				return fmt.Errorf("profile.capabilities[%q] must not be null", name)
			}
		}
	}
	*p = Profile(value)
	return nil
}

func (c *Composition) UnmarshalJSON(data []byte) error {
	type wire Composition
	var value wire
	if _, err := decodeStrictObject(data, "composition", &value,
		[]string{"id"}, []string{"id", "components"}); err != nil {
		return err
	}
	*c = Composition(value)
	return nil
}

func (c *Component) UnmarshalJSON(data []byte) error {
	type wire Component
	var value wire
	if _, err := decodeStrictObject(data, "component", &value,
		[]string{"id", "role", "measured"}, []string{"id", "role", "version", "sha256", "measured"}); err != nil {
		return err
	}
	*c = Component(value)
	return nil
}

func (p *PrecedenceLink) UnmarshalJSON(data []byte) error {
	type wire PrecedenceLink
	var value wire
	if _, err := decodeStrictObject(data, "precedence_link", &value,
		[]string{"target", "scope"}, []string{"target", "scope", "applicability"}); err != nil {
		return err
	}
	*p = PrecedenceLink(value)
	return nil
}
