// Package hotamontology is the CANONICAL source of the minimal vendored
// Requirement mirror + generic Registry a consumer domain's own spec/ Go
// module imports directly by file (task #365, RAC2 Phase A: "Go-code-only
// authority" for consumer-domain requirements, mirroring hotam-spec-self's
// own self-hosting shape -- internal/selfspec/requirements_*.go +
// `hotam sync-self`).
//
// THIS FILE IS THE CANON. It is never imported directly by a consumer
// domain's spec/ module (that would require a cross-module `replace`,
// forbidden per NEW-2-bis -- see internal/gate/test_exec.go's
// hashPackageInputs NEW-2 doc comment, and internal/recorder/canon/
// hotamspec.go's identical doc comment for the sibling precedent this
// package copies 1-in-1). Instead this file is VENDORED -- copied
// byte-for-byte, banner-stamped "do not edit", into each consumer spec/
// module as its own single-file `hotamontology` package
// (internal/ontology/vendor's Source does the banner-stamping; cmd/hotam's
// `vendor-ontology` command writes the vendored copy to
// <domainDir>/spec/hotamontology/requirement.go).
// internal/invariants/ontology_vendor_check.go's check_ontology_vendor_current
// invariant sha256-compares the vendored copy against this canonical file
// (post banner-strip) and fires a violation on drift.
//
// SCOPE: Requirement here is a MINIMAL, JSON-tag-identical mirror of ONLY the
// STRUCTURAL fields of internal/ontology.Requirement (internal/ontology/
// requirement.go) -- the fields internal/selfspec/merge.go's own doc comment
// already names "structural" (replaced wholesale from a registry entry),
// Why included (task #390, W0.3: Why is authored rationale, the same kind of
// registry-replaced field as Claim/Owner/Status, NOT an event log entry), in
// contrast to the "event" fields it passes through untouched from the
// existing graph node (History, GateSignoffs, LastReviewedAt, ReviewAfter,
// Evidence -- deliberately NOT mirrored here; a consumer domain authors
// identity/claim/owner/status/why/links in Go code, it never authors its own
// review timestamps or history log entries in code). Those event-adjacent
// fields (LastReviewedAt, ReviewAfter, Evidence, History, GateSignoffs) stay
// engine-side, populated only by the graph itself over time -- exactly the
// same split MergeIntoGraph already mechanizes for hotam-spec-self's own
// self-hosting path.
//
// The mirror provides strict codecs for its typed data. ObservedValue's
// lossless codec preserves empty bytes/objects and explicit false values;
// all codecs remain independent of engine internals.
package hotamontology

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

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

// LocalizedText mirrors ontology.LocalizedText. Missing language keys are
// absent data; consumers must not infer text from another language.
type LocalizedText map[string]string

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

// Relation mirrors ontology.Relation (internal/ontology/requirement.go) --
// same fields, same JSON tags, byte-identical wire shape.
type Relation struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

// SourceLink mirrors ontology.SourceLink's JSON shape without importing the
// engine package into a consumer domain's vendored spec module.
type SourceLink struct {
	SourceID string `json:"source_id"`
	Anchor   string `json:"anchor,omitempty"`
}

// CoverageDeclaration mirrors ontology.CoverageDeclaration's authored shape.
type CoverageDeclaration struct {
	Status    string `json:"status"`
	Rationale string `json:"rationale"`
	Profile   string `json:"profile,omitempty"`
}

// Requirement mirrors ONLY the structural fields of ontology.Requirement
// (internal/ontology/requirement.go) -- see this file's package doc comment
// for exactly which fields those are and why the event fields (History,
// GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are excluded. Field
// order and JSON tags match the source exactly, field-for-field, so a value
// of this type round-trips through JSON identically to the corresponding
// subset of an ontology.Requirement.
type Requirement struct {
	ID             string               `json:"id"`
	Claim          string               `json:"claim"`
	ClaimTexts     LocalizedText        `json:"claim_texts,omitempty"`
	Owner          string               `json:"owner"`
	Status         string               `json:"status"`
	Why            string               `json:"why"`
	Relations      []Relation           `json:"relations"`
	Assumptions    []string             `json:"assumptions"`
	Enforcement    string               `json:"enforcement"`
	EnforcedBy     []string             `json:"enforced_by"`
	Enforceability string               `json:"enforceability"`
	MTag           string               `json:"m_tag"`
	Summary        string               `json:"summary"`
	CreatedAt      string               `json:"created_at"`
	SettledAt      string               `json:"settled_at"`
	BlockedOn      string               `json:"blocked_on,omitempty"`
	ImplementedBy  []string             `json:"implemented_by,omitempty"`
	VerifiedBy     []string             `json:"verified_by,omitempty"`
	SourceLinks    []SourceLink         `json:"source_links,omitempty"`
	Coverage       *CoverageDeclaration `json:"coverage,omitempty"`
	SourceRefs     []string             `json:"source_refs"`
	DeclOrder      int                  `json:"decl_order"`
	AtomKind       string               `json:"atom_kind,omitempty"`
	Cases          []CaseDefinition     `json:"cases,omitempty"`
	ClauseLinks    []ClauseLink         `json:"clause_links,omitempty"`
	Strength       string               `json:"strength,omitempty"`
	Applicability  *Applicability       `json:"applicability,omitempty"`
	Precedence     []PrecedenceLink     `json:"precedence,omitempty"`
}

func (r *Requirement) UnmarshalJSON(data []byte) error {
	type wire Requirement
	var value wire
	if _, err := decodeStrictObject(data, "requirement", &value, nil,
		[]string{"claim_texts", "atom_kind", "cases", "clause_links", "strength", "applicability", "precedence"}); err != nil {
		return err
	}
	*r = Requirement(value)
	return nil
}

type ClauseLink struct {
	ClauseID string `json:"clause_id"`
	Side     string `json:"side,omitempty"`
}

type CaseDefinition struct {
	ID         string              `json:"id"`
	Test       string              `json:"test"`
	AtomIDs    []string            `json:"atom_ids,omitempty"`
	Profile    string              `json:"profile,omitempty"`
	Target     string              `json:"target,omitempty"`
	Operation  string              `json:"operation,omitempty"`
	Producer   string              `json:"producer,omitempty"`
	Input      *ObservedValue      `json:"input,omitempty"`
	Expected   *ObservedValue      `json:"expected,omitempty"`
	Fixtures   []FixtureRef        `json:"fixtures,omitempty"`
	Conditions []ConditionEvidence `json:"conditions,omitempty"`
	Sides      []string            `json:"sides,omitempty"`
	Selection  *SelectionEvidence  `json:"selection,omitempty"`
}

type FixtureRef struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Role     string `json:"role"`
	SHA256   string `json:"sha256"`
	RawBytes bool   `json:"raw_bytes"`
}

type ObservedValue struct {
	Kind          string                   `json:"kind"`
	Text          *string                  `json:"text,omitempty"`
	Bool          *bool                    `json:"bool,omitempty"`
	Encoding      string                   `json:"encoding,omitempty"`
	Bytes         string                   `json:"bytes,omitempty"`
	ScalarKind    string                   `json:"scalar_kind,omitempty"`
	Integer       string                   `json:"integer,omitempty"`
	FloatBits     string                   `json:"float_bits,omitempty"`
	Fields        map[string]ObservedValue `json:"fields,omitempty"`
	Diagnostic    *DiagnosticValue         `json:"diagnostic,omitempty"`
	decoded       bool
	hasBytes      bool
	hasFields     bool
	hasScalarKind bool
}

func (v *ObservedValue) UnmarshalJSON(data []byte) error {
	type wire ObservedValue
	var value wire
	fields, err := decodeStrictObject(data, "observed_value", &value,
		[]string{"kind"},
		[]string{"kind", "text", "bool", "encoding", "bytes", "scalar_kind", "integer", "float_bits", "fields", "diagnostic"})
	if err != nil {
		return err
	}
	*v = ObservedValue(value)
	v.decoded = true
	_, v.hasBytes = fields["bytes"]
	_, v.hasScalarKind = fields["scalar_kind"]
	_, v.hasFields = fields["fields"]
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
		Diagnostic *DiagnosticValue          `json:"diagnostic,omitempty"`
	}
	out := wire{Kind: v.Kind, Text: v.Text, Bool: v.Bool, Encoding: v.Encoding,
		Integer: v.Integer, FloatBits: v.FloatBits, Diagnostic: v.Diagnostic}
	if v.ScalarKind != "" || v.hasScalarKind {
		scalarKind := v.ScalarKind
		out.ScalarKind = &scalarKind
	}
	if v.Kind == "bytes" || v.Bytes != "" || v.hasBytes {
		payload := v.Bytes
		out.Bytes = &payload
	}
	if v.Kind == "object" || v.Fields != nil || v.hasFields {
		fields := v.Fields
		out.Fields = &fields
	}
	return json.Marshal(out)
}

type DiagnosticValue struct {
	Code   string    `json:"code"`
	Class  *string   `json:"class,omitempty"`
	Reason *string   `json:"reason,omitempty"`
	Line   *int      `json:"line,omitempty"`
	Span   *ByteSpan `json:"span,omitempty"`
}

type ByteSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ConditionEvidence struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

type SelectionEvidence struct {
	Matched  []string `json:"matched"`
	Selected string   `json:"selected"`
}

type Applicability struct {
	Operations []string `json:"operations,omitempty"`
	Profiles   []string `json:"profiles,omitempty"`
	Features   []string `json:"features,omitempty"`
}

type PrecedenceLink struct {
	Target        string         `json:"target"`
	Scope         string         `json:"scope"`
	Applicability *Applicability `json:"applicability,omitempty"`
}

func (c *CaseDefinition) UnmarshalJSON(data []byte) error {
	type wire CaseDefinition
	var value wire
	if _, err := decodeStrictObject(data, "case_definition", &value,
		[]string{"id"},
		[]string{"id", "test", "atom_ids", "profile", "target", "operation", "producer", "input", "expected", "fixtures", "conditions", "sides", "selection"}); err != nil {
		return err
	}
	*c = CaseDefinition(value)
	return nil
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
