// Package hotamspec is the CANONICAL source of the scenario-recorder API
// authored verified_by tests use to narrate a requirement's proof in the
// domain's own words (PLAN-scenario-generated-spec.md §1/§2 D1, task W1.1).
//
// THIS FILE IS THE CANON. It is never imported directly by a consumer
// domain's spec/ module (that would require a cross-module `replace`,
// forbidden per NEW-2-bis -- see internal/gate/test_exec.go's
// hashPackageInputs NEW-2 doc comment for the sibling precedent on why
// cross-module coupling is treated as a structural hazard in this engine).
// Instead this file is VENDORED -- copied byte-for-byte, banner-stamped
// "do not edit", into each consumer spec/ module as its own single-file
// `hotamspec` package (internal/generator/recorder_vendor.go's
// VendoredRecorderSource does the copying; cmd/hotam's `gen-spec` writes the
// vendored copy to <domainDir>/spec/hotamspec/hotamspec.go, mirroring the
// existing crystal-vendoring precedent in claudemd_static.go for markdown,
// applied here to Go source instead). internal/invariants/recorder_check.go's
// check_recorder_current invariant sha256-compares the vendored copy against
// this canonical file (post banner-strip) and fires a violation on drift.
//
// API SHAPE (PLAN §2 D1 sketch, confirmed against the pilot's actual
// authored-test style -- domains/prat/spec/model/brd_package_test.go /
// forecast_test.go -- both currently plain `func Test*(t *testing.T)` using
// bare t.Fatalf/t.Errorf, NOT any assertion library):
//
//	s := hotamspec.Scenario(t, "R-brd-integrity-zero-blockers", "BRD sign-off requires zero blockers")
//	s.Given("a BRD package with one outstanding blocker", "rule_id", "ac-orphan")
//	err := p.SignOffP_G3()
//	s.When("SignOffP_G3 is called")
//	s.Then("sign-off is rejected with ErrBrdHasBlockers", errors.Is(err, ErrBrdHasBlockers))
//	s.Value("blocker_count", p.BlockerCount())
//
// A plain `go test` run is PURE ASSERTS: Then still calls t.Errorf/t.Fatalf
// exactly like the hand-rolled tests it replaces (so an author who deletes
// every hotamspec.* call except Then loses nothing -- the recorder is a
// strict superset of "the test still asserts", never a replacement asserter
// bolted on top of a separately-passing test). No artifact is written in
// this mode: the Scenario only starts writing an artifact when the ENGINE
// (never the author, never the test itself) sets an env var requesting one
// -- that record-mode wiring is W1.2's job (test_exec.go), NOT this file's;
// this file only defines the recording MECHANISM (Steps accumulate, Render
// produces canonical bytes) that a future env-gated writer in W1.2 can call.
// The env var name and the actual write-on-request wiring are intentionally
// NOT present here yet, so this task cannot accidentally pre-empt W1.2's own
// design decisions about where/how the artifact lands on disk.
//
// DETERMINISM (mandatory: this becomes the source for byte-identical SPEC.md
// generation in W1.3): every Step's rendered form is a pure function of its
// own fields, values are canonically rendered (renderValue below), Given/
// Value keyed maps are impossible by construction (kv pairs are stored as an
// ORDERED slice, never a map, so there is no map-iteration order to sort
// away), and no wall-clock time, random number, pointer address, or other
// non-reproducible quantity is ever captured into a Step. Callers who pass a
// pointer/struct value to Given/Value get its %v/%q rendering, which for the
// float/map/pointer-address hazards this package explicitly avoids (see
// renderValue) is stable across runs on the same inputs.
package hotamspec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode"
)

// T is the minimal subset of *testing.T the Scenario needs. Scenario is
// constructed with a real *testing.T in every real test; this interface
// exists only so this package's OWN tests can substitute a fake recorder
// without depending on testing.T's full surface, and so a future non-testing
// driver (unlikely, but not foreclosed) is not structurally prevented.
//
// recordT is an OPTIONAL extension T may additionally satisfy: a real
// *testing.T always does (Cleanup/Name/Failed are all part of its real
// surface), so record-mode (see NewScenario's env-gated Cleanup below) works
// unconditionally for genuine tests. This package's own fakeT double
// (hotamspec_test.go) does NOT implement recordT, which is deliberate: this
// package's own unit tests exercise the Given/When/Then/Value mechanism
// directly via Steps(), never through the env-gated artifact writer, so they
// have no need to fake Cleanup/Name/Failed too.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// recordT is the extra surface NewScenario needs ONLY to support record-mode
// (writing a canonical JSON artifact when HOTAM_RECORD_DIR is set). Declared
// as a separate, optional interface -- checked via a type assertion in
// NewScenario, never required by T itself -- so record-mode support layers
// cleanly on top of the pre-existing plain-assert contract instead of
// widening what every caller (including this package's own fakeT-based
// tests) must implement.
type recordT interface {
	T
	Cleanup(func())
	Name() string
	Failed() bool
}

// StepKind classifies one recorded Step.
type StepKind string

const (
	// StepGiven records a precondition: a fact about the world the scenario
	// starts from, with zero or more supporting key/value facts.
	StepGiven StepKind = "given"
	// StepWhen records the action under test: the one thing the scenario
	// does that the Then steps judge.
	StepWhen StepKind = "when"
	// StepThen records an assertion: a claim that must hold, judged via the
	// underlying *testing.T exactly like a hand-written t.Errorf/t.Fatalf
	// would (see Then's doc comment for the pass/fail contract).
	StepThen StepKind = "then"
	// StepValue records a bare fact captured for narration (e.g. an
	// intermediate value worth showing in the generated prose) without
	// itself asserting anything.
	StepValue StepKind = "value"
)

// kv is one ordered key/value pair attached to a Step. A slice, never a map:
// map iteration order is randomized per Go process (deliberately, since
// Go 1), so ANY map-typed field on Step would make Render's output
// non-deterministic run-to-run -- exactly the hazard PLAN-scenario-generated
// -spec.md §5 names first ("Детерминизм записанных значений (map-order,
// float-формат, адреса)"). Keeping kv as an explicit slice sidesteps the
// hazard structurally: there is no map to accidentally range over.
type kv struct {
	key      string
	rendered string
}

// Step is one recorded moment in a Scenario's narrative -- a Given
// precondition, the When action, a Then assertion (with its outcome), or a
// bare Value capture. Exported so a future artifact writer (W1.2) and SPEC.md
// generator (W1.3) can walk s.Steps() without this package growing a second,
// parallel serialization path.
type Step struct {
	Kind StepKind
	Desc string
	// Values is Given/Value's ordered key/value payload -- always in the
	// order the caller passed them (see kv's doc comment for why this is a
	// slice, not a map). Empty for When/Then.
	Values []ValueFact
	// Passed is only meaningful for StepThen: whether the asserted condition
	// held. Zero value (false) for every other Kind.
	Passed       bool
	Subject      string
	Value        string
	Input        string
	HasInput     bool
	Expected     string
	HasExpected  bool
	Observations []Observation
	Context      *ArtifactContext
}

// Observation holds the readable projection and optional exact typed payload
// for one real comparison made during a method execution.
type Observation struct {
	Name        string      `json:"name"`
	Input       string      `json:"input,omitempty"`
	Actual      string      `json:"actual"`
	Expected    string      `json:"expected"`
	Passed      bool        `json:"passed"`
	RawInput    *TypedValue `json:"raw_input,omitempty"`
	RawActual   *TypedValue `json:"raw_actual,omitempty"`
	RawExpected *TypedValue `json:"raw_expected,omitempty"`
}

// FixtureRef mirrors the portable shared conformance fixture reference.
type FixtureRef struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Role     string `json:"role"`
	SHA256   string `json:"sha256"`
	RawBytes bool   `json:"raw_bytes"`
}

// ConditionEvidence records a condition actually observed by the producer.
type ConditionEvidence struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

// SelectionEvidence records producer-observed matched branches and selection.
type SelectionEvidence struct {
	Matched  []string `json:"matched"`
	Selected string   `json:"selected"`
}

// ByteSpan is a half-open byte range.
type ByteSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// DiagnosticValue contains only diagnostic fields supplied by its producer.
type DiagnosticValue struct {
	Code   string    `json:"code"`
	Class  *string   `json:"class,omitempty"`
	Reason *string   `json:"reason,omitempty"`
	Line   *int      `json:"line,omitempty"`
	Span   *ByteSpan `json:"span,omitempty"`
}

func decodeTypedObject(data []byte, label string, target any, required ...string) (map[string]json.RawMessage, error) {
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
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
	for name, raw := range fields {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, fmt.Errorf("%s.%s must not be null", label, name)
		}
	}
	return fields, nil
}

func (span *ByteSpan) UnmarshalJSON(data []byte) error {
	type plain ByteSpan
	var decoded plain
	if _, err := decodeTypedObject(data, "byte_span", &decoded, "start", "end"); err != nil {
		return err
	}
	*span = ByteSpan(decoded)
	return nil
}

func (value *DiagnosticValue) UnmarshalJSON(data []byte) error {
	type plain DiagnosticValue
	var decoded plain
	if _, err := decodeTypedObject(data, "diagnostic", &decoded, "code"); err != nil {
		return err
	}
	*value = DiagnosticValue(decoded)
	return nil
}

// TypedValue mirrors the portable observed-value schema. Bytes contains
// base64 when Kind is "bytes".
type TypedValue struct {
	Kind          string                `json:"kind"`
	Bool          *bool                 `json:"bool,omitempty"`
	Text          *string               `json:"text,omitempty"`
	Encoding      string                `json:"encoding,omitempty"`
	Bytes         string                `json:"bytes,omitempty"`
	ScalarKind    string                `json:"scalar_kind,omitempty"`
	Integer       string                `json:"integer,omitempty"`
	FloatBits     string                `json:"float_bits,omitempty"`
	Fields        map[string]TypedValue `json:"fields,omitempty"`
	Items         []TypedValue          `json:"items,omitempty"`
	Diagnostic    *DiagnosticValue      `json:"diagnostic,omitempty"`
	decoded       bool
	hasBytes      bool
	hasFields     bool
	hasItems      bool
	hasScalarKind bool
}

// UnmarshalJSON accepts only the explicit typed envelope, never user-key wrappers.
func (value *TypedValue) UnmarshalJSON(data []byte) error {
	type plain TypedValue
	var decoded plain
	fields, err := decodeTypedObject(data, "typed_value", &decoded, "kind")
	if err != nil {
		return err
	}
	*value = TypedValue(decoded)
	value.decoded = true
	_, value.hasBytes = fields["bytes"]
	_, value.hasFields = fields["fields"]
	_, value.hasItems = fields["items"]
	_, value.hasScalarKind = fields["scalar_kind"]
	return nil
}

// MarshalJSON preserves explicit empty byte, object, and ordered array payloads.
func (value TypedValue) MarshalJSON() ([]byte, error) {
	type plain TypedValue
	wire := struct {
		plain
		Bytes      *string                `json:"bytes,omitempty"`
		ScalarKind *string                `json:"scalar_kind,omitempty"`
		Fields     *map[string]TypedValue `json:"fields,omitempty"`
		Items      *[]TypedValue          `json:"items,omitempty"`
	}{plain: plain(value)}
	if value.Kind == "bytes" || value.Bytes != "" || value.hasBytes {
		wire.Bytes = &value.Bytes
	}
	if value.ScalarKind != "" || value.hasScalarKind {
		wire.ScalarKind = &value.ScalarKind
	}
	if value.Kind == "object" || value.Fields != nil || value.hasFields {
		wire.Fields = &value.Fields
	}
	if value.Kind == "array" || value.Items != nil || value.hasItems {
		wire.Items = &value.Items
	}
	return json.Marshal(wire)
}

// CaseContext carries shared case metadata. Test and Input are recorder-derived;
// Expected optionally supplies an independent oracle shared by property atoms.
type CaseContext struct {
	ID         string              `json:"id"`
	AtomIDs    []string            `json:"atom_ids,omitempty"`
	ClauseIDs  []string            `json:"clause_ids,omitempty"`
	Profile    string              `json:"profile,omitempty"`
	Target     string              `json:"target,omitempty"`
	Fixtures   []FixtureRef        `json:"fixtures,omitempty"`
	Conditions []ConditionEvidence `json:"conditions,omitempty"`
	Sides      []string            `json:"sides,omitempty"`
	Selection  *SelectionEvidence  `json:"selection,omitempty"`
	Expected   *TypedValue         `json:"-"`
	Operation  string              `json:"operation,omitempty"`
	Producer   string              `json:"producer,omitempty"`
}

func (ctx CaseContext) MarshalJSON() ([]byte, error) {
	type plain CaseContext
	wire := struct {
		plain
		ClauseIDs *[]string `json:"clause_ids,omitempty"`
	}{plain: plain(ctx)}
	if ctx.ClauseIDs != nil {
		wire.ClauseIDs = &ctx.ClauseIDs
	}
	return json.Marshal(wire)
}

// Bytes records exact bytes as base64 without decoding them as text.
func Bytes(value []byte) TypedValue {
	return TypedValue{Kind: "bytes", Encoding: "base64", Bytes: base64.StdEncoding.EncodeToString(value)}
}

// Text records an exact string, including an explicitly empty string.
func Text(value string) TypedValue {
	return TypedValue{Kind: "text", Text: stringPointer(value), ScalarKind: "string"}
}

// Integer records an exact decimal integer string.
func Integer(value string) TypedValue {
	return TypedValue{Kind: "integer", Integer: value, ScalarKind: "integer"}
}

// Float64Bits records the exact IEEE754 binary64 bits, including signed zero
// and NaN payload bits.
func Float64Bits(value float64) TypedValue {
	return TypedValue{Kind: "float", FloatBits: fmt.Sprintf("%016x", math.Float64bits(value)), ScalarKind: "float64"}
}

// Scalar records a primitive value with an explicit scalar kind. Known schema
// kinds are text/string, bool, integer, float/float64, and null; other scalar
// labels retain the same exact value under that label. Composite values panic
// instead of being silently flattened to text.
func Scalar(kind string, value any) TypedValue {
	if value == nil {
		if kind == "" || kind == "null" {
			return TypedValue{Kind: "null"}
		}
		panic(fmt.Sprintf("hotamspec: Scalar(%q): nil does not match the declared scalar kind", kind))
	}
	rv := reflect.ValueOf(value)
	typed := typedScalar(rv, kind)
	valid := false
	switch kind {
	case "text", "string":
		if typed.Kind == "text" {
			typed.ScalarKind = "string"
			valid = true
		}
	case "bool":
		if typed.Kind == "bool" {
			typed.ScalarKind = "bool"
			valid = true
		}
	case "integer":
		if text, ok := value.(string); ok {
			return Integer(text)
		}
		if typed.Kind == "integer" {
			typed.ScalarKind = "integer"
			valid = true
		}
	case "float", "float64":
		if typed.Kind == "float" {
			typed.ScalarKind = "float64"
			valid = true
		}
	case "null":
		break
	default:
		typed.ScalarKind = kind
		valid = true
	}
	if valid {
		return typed
	}
	panic(fmt.Sprintf("hotamspec: Scalar(%q): value of type %T has an incompatible scalar kind", kind, value))
}

func typedScalar(value reflect.Value, scalarKind string) TypedValue {
	switch value.Kind() {
	case reflect.String:
		text := value.String()
		return TypedValue{Kind: "text", Text: &text, ScalarKind: scalarKind}
	case reflect.Bool:
		boolean := value.Bool()
		return TypedValue{Kind: "bool", Bool: &boolean, ScalarKind: scalarKind}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TypedValue{Kind: "integer", Integer: strconv.FormatInt(value.Int(), 10), ScalarKind: scalarKind}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return TypedValue{Kind: "integer", Integer: strconv.FormatUint(value.Uint(), 10), ScalarKind: scalarKind}
	case reflect.Float32, reflect.Float64:
		return TypedValue{Kind: "float", FloatBits: fmt.Sprintf("%016x", math.Float64bits(value.Float())), ScalarKind: scalarKind}
	default:
		panic(fmt.Sprintf("hotamspec: Scalar(%q): unsupported scalar type %T", scalarKind, value.Interface()))
	}
}

// Diagnostic records structured producer-supplied diagnostic data.
func Diagnostic(value DiagnosticValue) TypedValue {
	return TypedValue{Kind: "diagnostic", Diagnostic: copyDiagnostic(&value)}
}

// Object records a defensive copy of typed object fields.
func Object(fields map[string]TypedValue) TypedValue {
	copy := make(map[string]TypedValue, len(fields))
	for key, value := range fields {
		copy[key] = cloneTypedValue(value)
	}
	return TypedValue{Kind: "object", Fields: copy}
}

// Array records a defensive copy of ordered items, including an explicit empty array.
func Array(items []TypedValue) TypedValue {
	copy := make([]TypedValue, len(items))
	for i := range items {
		copy[i] = cloneTypedValue(items[i])
	}
	return TypedValue{Kind: "array", Items: copy}
}

func stringPointer(value string) *string { return &value }

func copyDiagnostic(value *DiagnosticValue) *DiagnosticValue {
	if value == nil {
		return nil
	}
	copy := *value
	if value.Class != nil {
		class := *value.Class
		copy.Class = &class
	}
	if value.Reason != nil {
		reason := *value.Reason
		copy.Reason = &reason
	}
	if value.Line != nil {
		line := *value.Line
		copy.Line = &line
	}
	if value.Span != nil {
		span := *value.Span
		copy.Span = &span
	}
	return &copy
}

func cloneTypedValue(value TypedValue) TypedValue {
	copy := value
	if value.Bool != nil {
		boolean := *value.Bool
		copy.Bool = &boolean
	}
	if value.Text != nil {
		text := *value.Text
		copy.Text = &text
	}
	if value.Fields != nil {
		copy.Fields = make(map[string]TypedValue, len(value.Fields))
		for key, field := range value.Fields {
			copy.Fields[key] = cloneTypedValue(field)
		}
	}
	if value.Items != nil {
		copy.Items = make([]TypedValue, len(value.Items))
		for i := range value.Items {
			copy.Items[i] = cloneTypedValue(value.Items[i])
		}
	}
	copy.Diagnostic = copyDiagnostic(value.Diagnostic)
	return copy
}
func cloneObservations(observations []Observation) []Observation {
	copy := append([]Observation(nil), observations...)
	for i := range copy {
		copy[i].RawInput = cloneTypedValuePointer(copy[i].RawInput)
		copy[i].RawActual = cloneTypedValuePointer(copy[i].RawActual)
		copy[i].RawExpected = cloneTypedValuePointer(copy[i].RawExpected)
	}
	return copy
}

func cloneTypedValuePointer(value *TypedValue) *TypedValue {
	if value == nil {
		return nil
	}
	copy := cloneTypedValue(*value)
	return &copy
}

func cloneCaseContext(value *CaseContext) *CaseContext {
	if value == nil {
		return nil
	}
	copy := *value
	copy.AtomIDs = append([]string(nil), value.AtomIDs...)
	if value.ClauseIDs != nil {
		copy.ClauseIDs = append([]string{}, value.ClauseIDs...)
	}
	copy.Fixtures = append([]FixtureRef(nil), value.Fixtures...)
	copy.Conditions = append([]ConditionEvidence(nil), value.Conditions...)
	copy.Sides = append([]string(nil), value.Sides...)
	if value.Selection != nil {
		selection := *value.Selection
		if value.Selection.Matched != nil {
			selection.Matched = append([]string{}, value.Selection.Matched...)
		}
		copy.Selection = &selection
	}
	copy.Expected = cloneTypedValuePointer(value.Expected)
	return &copy
}
func observationsHaveRawValues(observations []Observation) bool {
	for _, observation := range observations {
		if observation.RawInput != nil || observation.RawActual != nil || observation.RawExpected != nil {
			return true
		}
	}
	return false
}

// Component is the local structural mirror of ontology.Component; Measured is
// explicit even when false, and is never inferred from a declared version.
type Component struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Version  string `json:"version,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	Measured bool   `json:"measured"`
}

// ArtifactContext carries explicitly supplied implementation and runtime
// producer/target/component provenance for an observation.
type ArtifactContext struct {
	Implementation        string      `json:"implementation,omitempty"`
	ImplementationVersion string      `json:"implementation_version,omitempty"`
	SpecVersion           string      `json:"spec_version,omitempty"`
	Profile               string      `json:"profile,omitempty"`
	Target                string      `json:"target,omitempty"`
	Operation             string      `json:"operation,omitempty"`
	Producer              string      `json:"producer,omitempty"`
	Components            []Component `json:"components,omitempty"`
}

func copyArtifactContext(ctx *ArtifactContext) *ArtifactContext {
	if ctx == nil {
		return nil
	}
	copy := *ctx
	copy.Components = append([]Component(nil), ctx.Components...)
	sort.Slice(copy.Components, func(i, j int) bool {
		left, right := copy.Components[i], copy.Components[j]
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		if left.Version != right.Version {
			return left.Version < right.Version
		}
		if left.SHA256 != right.SHA256 {
			return left.SHA256 < right.SHA256
		}
		return !left.Measured && right.Measured
	})
	return &copy
}

type observedCarrier interface {
	hotamObservation() (any, []Observation)
}

// ObservedValue wraps a method's return value together with comparisons made
// during that same execution. TypedValue is the separate raw schema payload.
type ObservedValue[V any] struct {
	Value        V
	Observations []Observation
}

func (o ObservedValue[V]) hotamObservation() (any, []Observation) {
	return o.Value, cloneObservations(o.Observations)
}

// Observed binds method output to comparisons without executing the method.
func Observed[V any](value V, nested ...Observation) ObservedValue[V] {
	return ObservedValue[V]{Value: value, Observations: cloneObservations(nested)}
}

// Observe describes an actual comparison using Scenario.Eq's equality and
// lossless-constant coercion rules. Fact reports a failed comparison to its
// real testing.T when it receives it through Observed.
func Observe(name string, input, actual, expected any) Observation {
	if !hasTypedValue(input) && !hasTypedValue(actual) && !hasTypedValue(expected) {
		passed := equalValues(actual, expected)
		return Observation{
			Name: name, Input: renderValue(input), Actual: renderValue(actual),
			Expected: renderExpected(actual, expected), Passed: passed,
		}
	}
	rawInput := typedValue(input)
	rawActual := typedValue(actual)
	rawExpected := typedValue(expected)
	return Observation{
		Name: name, Input: renderTypedValue(rawInput), Actual: renderTypedValue(rawActual),
		Expected: renderTypedValue(rawExpected),
		Passed:   equalTypedValues(&rawActual, &rawExpected),
		RawInput: &rawInput, RawActual: &rawActual, RawExpected: &rawExpected,
	}
}

// ValueFact is one exported (key, canonically-rendered value) pair from a Given
// or Value step.
type ValueFact struct {
	Key   string
	Value string
}

// Scenario narrates one verified_by test's proof, in Given/When/Then/Value
// steps, while asserting through the underlying T exactly as a hand-written
// test would. Construct with Scenario(t, reqID, title); call Given/When/
// Then/Value as the test proceeds.
type Scenario struct {
	t     T
	reqID string
	title string
	steps []Step

	defaultWhen  string // WithWhen; "" = none
	whenDone     bool   // explicit When seen, or default already resolved
	mode         string
	fileSuffix   string
	caseContext  *CaseContext
	caseInput    *TypedValue
	caseExpected *TypedValue
}

// Option configures NewScenario.
type Option func(*Scenario)

// WithWhen sets a default When narration: if the test never calls s.When
// before its first Then/Eq/Value, the recorder inserts StepWhen(desc) right
// there -- the recorded steps are exactly those of an explicit s.When(desc)
// at that point. An explicit s.When always takes precedence (no duplicate).
// Removes the `s.When("init")` boilerplate repeated across a file's scenarios.
func WithWhen(desc string) Option {
	return func(s *Scenario) { s.defaultWhen = desc }
}

// RecordDirEnv is the environment variable that switches Scenario into
// record-mode: WRITTEN ONLY BY THE ENGINE (internal/gate/test_exec.go's
// record-mode runner, PLAN-scenario-generated-spec.md §2 D1/§3 W1.2), never
// by the author of a verified_by test and never by the test's own code. When
// set to a non-empty directory path, every Scenario constructed via
// NewScenario in that `go test` process registers a t.Cleanup that
// serializes its recorded steps to a canonical JSON artifact under that
// directory once the test finishes -- see NewScenario's doc comment for the
// exact file-naming and content contract. When unset (the default -- what
// every plain `go test` invocation sees, including a human running `go test
// ./...` locally), NewScenario writes nothing at all: a Scenario-based test
// is byte-for-byte the same pure-asserts run W1.1 already established,
// record-mode is purely additive.
const RecordDirEnv = "HOTAM_RECORD_DIR"

// Artifact is the canonical JSON shape written to
// <HOTAM_RECORD_DIR>/<reqID>__<TestName>.json in record-mode -- one artifact
// per Scenario instance. Struct fields retain declaration order; the only
// object maps in typed values have string keys, which encoding/json sorts.
// Thus identical scenario inputs produce byte-identical JSON.
type Artifact struct {
	ReqID        string         `json:"req_id"`
	Test         string         `json:"test"`
	Title        string         `json:"title"`
	Steps        []ArtifactStep `json:"steps"`
	Verdict      string         `json:"verdict"`
	Mode         string         `json:"mode,omitempty"`
	Case         *CaseContext   `json:"case,omitempty"`
	CaseInput    *TypedValue    `json:"case_input,omitempty"`
	CaseExpected *TypedValue    `json:"case_expected,omitempty"`
}

// ArtifactStep is one Step's JSON projection. New detail fields are omitted
// when unused so existing Given/When/Value artifacts retain their shape.
type ArtifactStep struct {
	Kind         StepKind         `json:"kind"`
	Desc         string           `json:"desc"`
	Values       []ArtifactFact   `json:"values,omitempty"`
	Passed       bool             `json:"passed,omitempty"`
	Subject      string           `json:"subject,omitempty"`
	Value        string           `json:"value,omitempty"`
	Input        *string          `json:"input,omitempty"`
	Expected     *string          `json:"expected,omitempty"`
	Observations []Observation    `json:"observations,omitempty"`
	Context      *ArtifactContext `json:"context,omitempty"`
}

// ArtifactFact is one Fact's JSON projection -- Key/Value, in the exact
// order Step.Values already holds them (call order, per kv's own doc
// comment: never re-sorted, since Given/Value are ordered slices, not maps).
type ArtifactFact struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

// NewScenario starts a new Scenario bound to t, narrating the proof of
// requirement reqID under the human-readable title. reqID is expected to be
// an R-anchor (e.g. "R-brd-integrity-zero-blockers") matching the
// requirement this test's verified_by entry is cited from, but this package
// does not itself validate that shape or cross-check it against a graph --
// that linkage is the mirror audit's job (PLAN-authored-spec-discipline.md
// §6's HONESTY BOUNDARY), not a mechanical property this recorder enforces.
//
// Named NewScenario (not the shorter Scenario used in the package doc
// comment's illustrative sketch) because Scenario is also this file's
// exported TYPE name -- Go does not allow a function and a type to share one
// identifier in the same package, so the constructor takes the New-prefixed
// form used throughout this codebase's own conventions (see
// model.NewBrdPackage / model.NewForecast in the pilot's authored spec/, the
// exact style this recorder is designed to sit alongside).
//
// RECORD-MODE: if RecordDirEnv is set (non-empty) in this process's
// environment AND t additionally satisfies recordT (a real *testing.T always
// does), NewScenario registers a t.Cleanup that -- after the test function
// itself has returned, so every Given/When/Then/Value call the test makes has
// already landed in s.steps -- serializes this Scenario to an Artifact and
// writes it as canonical JSON to
// <RecordDirEnv>/<reqID>__<sanitized test name>.json. This is purely
// ADDITIVE to the plain-asserts contract: Then still calls t.Errorf exactly
// as before, record-mode never changes whether the test itself passes or
// fails, only whether a side artifact also gets written. Any error while
// writing the artifact (directory unwritable or marshal failure) is reported
// via t.Errorf, not silently swallowed. Typed raw values are JSON data with
// deterministic string-key maps. A record-mode failure is visible to callers,
// never a quiet no-op that looks identical to successful artifact production.
//
// Options: WithWhen(desc) sets a default When step for scenarios that all
// repeat the same action narration -- see WithWhen.
func NewScenario(t T, reqID, title string, opts ...Option) *Scenario {
	t.Helper()
	s := &Scenario{t: t, reqID: reqID, title: title}
	for _, o := range opts {
		o(s)
	}
	if dir := os.Getenv(RecordDirEnv); dir != "" || os.Getenv("HOTAM_RECORD_STDOUT") == "1" {
		if rt, ok := t.(recordT); ok {
			rt.Cleanup(func() { s.writeArtifact(rt, dir) })
		}
	}
	return s
}

// writeArtifact renders s as canonical JSON and writes it to
// <dir>/<reqID>__<sanitized test name>.json, called from the t.Cleanup
// NewScenario registers in record-mode. verdict is "pass" unless rt reports
// the test already Failed() by the time Cleanup runs (t.Failed() reflects
// every t.Error/t.Errorf/t.Fatal call made during the test, including ones
// this Scenario's own Then steps made, and any the test's surrounding code
// made directly) -- so verdict is a DERIVED summary of the same signal
// go test's own PASS/FAIL line already carries, never a second, independently
// -trackable notion of success a forged artifact could disagree with go
// test's real exit code about.
func (s *Scenario) writeArtifact(rt recordT, dir string) {
	rt.Helper()
	verdict := "pass"
	if rt.Failed() {
		verdict = "fail"
	}
	steps := make([]ArtifactStep, 0, len(s.steps))
	for _, st := range s.steps {
		values := make([]ArtifactFact, 0, len(st.Values))
		for _, f := range st.Values {
			values = append(values, ArtifactFact{Key: f.Key, Value: f.Value})
		}
		artifactStep := ArtifactStep{
			Kind: st.Kind, Desc: st.Desc, Values: values, Passed: st.Passed,
			Subject: st.Subject, Value: st.Value,
		}
		if st.HasInput {
			input := st.Input
			artifactStep.Input = &input
		}
		if st.HasExpected {
			expected := st.Expected
			artifactStep.Expected = &expected
		}
		artifactStep.Observations = cloneObservations(st.Observations)
		artifactStep.Context = copyArtifactContext(st.Context)
		steps = append(steps, artifactStep)
	}
	art := Artifact{
		ReqID:        s.reqID,
		Test:         rt.Name(),
		Title:        s.title,
		Steps:        steps,
		Verdict:      verdict,
		Mode:         s.mode,
		Case:         cloneCaseContext(s.caseContext),
		CaseInput:    cloneTypedValuePointer(s.caseInput),
		CaseExpected: cloneTypedValuePointer(s.caseExpected),
	}
	if os.Getenv("HOTAM_RECORD_STDOUT") == "1" {
		line, err := json.Marshal(art)
		if err != nil {
			rt.Errorf("hotamspec: marshal artifact: %v", err)
			return
		}
		fmt.Printf("HOTAMSPEC_ARTIFACT:%s\n", line)
	}
	if dir == "" {
		return
	}
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		rt.Errorf("hotamspec: record-mode: could not marshal artifact for %s/%s: %v", s.reqID, rt.Name(), err)
		return
	}
	data = append(data, '\n')
	name := artifactFileName(s.reqID, rt.Name())
	if s.fileSuffix != "" {
		name = strings.TrimSuffix(name, ".json") + s.fileSuffix + ".json"
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		rt.Errorf("hotamspec: record-mode: could not write artifact %s: %v", name, err)
	}
}

// artifactFileName builds the <reqID>__<sanitized test name>.json file name
// record-mode writes into RecordDirEnv. testName comes from *testing.T.Name(),
// which for a top-level test is just "TestFoo" but for a subtest (t.Run)
// includes a "/"-separated path (e.g. "TestFoo/case_1") -- "/" is not a valid
// path SEPARATOR-FREE filename character on every OS this engine targets
// (Windows rejects it outright), so it is replaced with "_" here, the same
// convention Go's own `go test -run` pattern matching and `t.TempDir()`
// naming already use for subtest names in generated paths.
func artifactFileName(reqID, testName string) string {
	safe := make([]byte, 0, len(testName))
	for i := 0; i < len(testName); i++ {
		c := testName[i]
		if c == '/' || c == '\\' {
			safe = append(safe, '_')
			continue
		}
		safe = append(safe, c)
	}
	return reqID + "__" + string(safe) + ".json"
}

// ReqID returns the requirement anchor this Scenario narrates.
func (s *Scenario) ReqID() string { return s.reqID }

// Title returns the scenario's human-readable title.
func (s *Scenario) Title() string { return s.title }

// Steps returns the recorded steps in call order -- a defensive copy, so a
// caller (e.g. a future artifact writer) cannot mutate the Scenario's own
// internal slice.
func (s *Scenario) Steps() []Step {
	out := make([]Step, len(s.steps))
	copy(out, s.steps)
	for i := range out {
		out[i].Values = append([]ValueFact(nil), s.steps[i].Values...)
		out[i].Observations = cloneObservations(s.steps[i].Observations)
		out[i].Context = copyArtifactContext(s.steps[i].Context)
	}
	return out
}

// Given records a precondition the scenario starts from. kv is an optional,
// even-length list of alternating key, value, key, value, ... pairs (the
// same convention log/slog uses) -- e.g.
// s.Given("a BRD package with one outstanding blocker", "rule_id", "ac-orphan").
// An odd-length kv is a caller bug: Given reports it via t.Fatalf immediately
// (fail loudly at the call site, not silently drop the dangling key) rather
// than guessing which half of the pair was meant.
func (s *Scenario) Given(desc string, kvPairs ...any) {
	s.t.Helper()
	facts, ok := pairsToFacts(kvPairs)
	if !ok {
		s.t.Fatalf("hotamspec: Given(%q, ...) called with an odd number of key/value arguments (%d) -- pass alternating key, value pairs", desc, len(kvPairs))
		return
	}
	s.steps = append(s.steps, Step{Kind: StepGiven, Desc: desc, Values: facts})
}

// When records the action under test -- the one thing the scenario does
// that the subsequent Then steps judge. When does not itself take a
// value/error to record: the pilot's own authored-test style
// (brd_package_test.go/forecast_test.go) calls the real method directly
// (err := p.SignOffP_G3()) and asserts on its result via a plain `if`, so
// When's job is purely narrative -- naming what just happened -- while the
// actual value flows into the following Then's condition exactly as it
// already does in a hand-written test. This keeps When from becoming a
// second, parallel calling convention alongside the direct method call the
// test already makes.
func (s *Scenario) When(desc string) {
	s.whenDone = true
	s.steps = append(s.steps, Step{Kind: StepWhen, Desc: desc})
}

// ensureWhen records the default When (WithWhen) exactly once, immediately
// before the first Then/Eq/Value step, unless the test already called When
// explicitly (an explicit When always wins; a later explicit When appends
// normally). The result is identical to an explicit s.When(desc) call placed
// there.
func (s *Scenario) ensureWhen() {
	if s.whenDone {
		return
	}
	s.whenDone = true
	if s.defaultWhen != "" {
		s.steps = append(s.steps, Step{Kind: StepWhen, Desc: s.defaultWhen})
	}
}

// Then asserts cond and records the outcome. On cond==false it reports a
// failure via t.Errorf(desc) -- non-fatal, exactly like a hand-written
// `t.Errorf(...)` in the pilot's existing tests (brd_package_test.go uses
// t.Fatalf for setup-invariant violations but the assertion pattern
// throughout is "compute, then judge with a plain if + t.Errorf/t.Fatalf");
// Then intentionally uses the non-fatal Errorf form (not Fatalf) so multiple
// Then steps in one Scenario all get a chance to report, matching Go's own
// t.Error-over-t.Fatal convention for "more than one thing worth telling the
// caller about in one test run" (see TestForecastVersion_String_
// MatchesClaimNaming's t.Errorf-in-a-loop precedent in forecast_test.go). A
// caller who needs fatal-on-failure semantics can call t.Fatal itself
// immediately after inspecting Then's own return (bool, so this is
// possible) instead of Then trying to guess when fatal is appropriate.
func (s *Scenario) Then(desc string, cond bool) bool {
	s.t.Helper()
	s.ensureWhen()
	if !cond {
		s.t.Errorf("hotamspec: Then(%q) failed for %s (%s)", desc, s.reqID, s.title)
	}
	s.steps = append(s.steps, Step{Kind: StepThen, Desc: desc, Passed: cond})
	return cond
}

// Eq asserts got == want and records the outcome with the VALUE taken from
// execution, not from the author's hand: the recorded StepThen's Desc is
// `label + " " + renderValue(got)`, so the narrated number/string is whatever
// the code actually produced -- an author cannot write "born 1990" in the text
// while the code asserts 1987.
//
// EQUALITY RULE (strict on type, canonical on value): after dereferencing
// non-nil pointers on both sides, got and want must have the SAME dynamic type
// AND the same canonical rendering (renderValue: float shortest form, maps
// key-sorted, errors via Error()). So Eq("x", 1, 1.0) FAILS (int vs float64),
// as do two distinct struct types that happen to render alike.
// CONVENIENCE FOR CONSTANTS: Go erases "untyped constant" at runtime, so a
// want whose type is a predeclared default constant type (bool, int, rune/
// int32, float64, string) is converted to got's type when got's basic kind is
// in the same family (bool; any int/uint kind; float32/float64; string) and
// the conversion is lossless (round-trips; no overflow/truncation). Hence
// Eq("year", BirthYear(1987), 1987) and Eq("kind", Kind("a"), "a") pass, while
// int vs float never converts across families. On mismatch it reports via
// t.Errorf (non-fatal, like Then) including label, both renderings and types.
// A comparison involving a TypedValue instead uses exact structural identity,
// so byte payloads, scalar kinds, diagnostic-field presence, and float bits
// remain distinct without changing ordinary Fact behavior.
// Prefer Eq for value facts ("blocker_count is 0"), Then for boolean
// predicates ("sign-off is rejected").
func (s *Scenario) Eq(label string, got, want any) bool {
	s.t.Helper()
	equal, _, _ := s.eq(label, got, want)
	return equal
}

func (s *Scenario) eq(label string, got, want any) (bool, string, string) {
	s.t.Helper()
	s.ensureWhen()
	g, w := derefValue(got), derefValue(want)
	w = coerceConst(g, w)
	var gotS, wantS string
	var equal bool
	typedComparison := hasTypedValue(got) || hasTypedValue(want)
	var rawGot, rawWant TypedValue
	if typedComparison {
		rawGot, rawWant = typedValue(got), typedValue(want)
		gotS, wantS = renderTypedValue(rawGot), renderTypedValue(rawWant)
		equal = equalTypedValues(&rawGot, &rawWant)
	} else {
		gotS, wantS = renderValue(got), renderValue(w)
		equal = reflect.TypeOf(g) == reflect.TypeOf(w) && renderValue(g) == renderValue(w)
	}
	if !equal {
		s.t.Errorf("hotamspec: Eq(%q) failed for %s (%s): got %s (%T), want %s (%T)", label, s.reqID, s.title, gotS, g, wantS, w)
	}
	observation := Observation{Name: label, Actual: gotS, Expected: wantS, Passed: equal}
	if typedComparison {
		observation.RawActual, observation.RawExpected = &rawGot, &rawWant
	}
	s.steps = append(s.steps, Step{
		Kind: StepThen, Desc: label + " " + gotS, Passed: equal,
		Subject: label, Value: gotS, Expected: wantS, HasExpected: true,
		Observations: []Observation{observation},
	})
	return equal, gotS, wantS
}

func renderExpected(got, want any) string {
	return renderValue(coerceConst(derefValue(got), derefValue(want)))
}

func derefValue(value any) any {
	for value != nil {
		reflected := reflect.ValueOf(value)
		if reflected.Kind() != reflect.Ptr || reflected.IsNil() {
			break
		}
		value = reflected.Elem().Interface()
	}
	return value
}

func equalValues(got, want any) bool {
	if hasTypedValue(got) || hasTypedValue(want) {
		actual, expected := typedValue(got), typedValue(want)
		return equalTypedValues(&actual, &expected)
	}
	g, w := derefValue(got), derefValue(want)
	w = coerceConst(g, w)
	return reflect.TypeOf(g) == reflect.TypeOf(w) && renderValue(g) == renderValue(w)
}

// equalTypedValues compares exact payloads, not private decoder bookkeeping.
func equalTypedValues(a, b *TypedValue) bool {
	if !validTypedCollection(a) || !validTypedCollection(b) {
		return false
	}
	bytesA := a.Bytes != "" || a.hasBytes || (!a.decoded && a.Kind == "bytes")
	bytesB := b.Bytes != "" || b.hasBytes || (!b.decoded && b.Kind == "bytes")
	if a.Kind != b.Kind || a.Encoding != b.Encoding || a.Bytes != b.Bytes ||
		bytesA != bytesB || a.ScalarKind != b.ScalarKind ||
		a.Integer != b.Integer || a.FloatBits != b.FloatBits ||
		!reflect.DeepEqual(a.Text, b.Text) || !reflect.DeepEqual(a.Bool, b.Bool) ||
		!reflect.DeepEqual(a.Diagnostic, b.Diagnostic) ||
		(a.Fields == nil) != (b.Fields == nil) || len(a.Fields) != len(b.Fields) ||
		(a.Items == nil) != (b.Items == nil) || len(a.Items) != len(b.Items) {
		return false
	}
	for key, field := range a.Fields {
		other, ok := b.Fields[key]
		if !ok || !equalTypedValues(&field, &other) {
			return false
		}
	}
	for i := range a.Items {
		if !equalTypedValues(&a.Items[i], &b.Items[i]) {
			return false
		}
	}
	return true
}

func validTypedCollection(value *TypedValue) bool {
	if value.Kind != "object" && value.Kind != "array" {
		return true
	}
	if value.Text != nil || value.Bool != nil || value.Encoding != "" ||
		value.Bytes != "" || value.hasBytes || value.ScalarKind != "" || value.hasScalarKind ||
		value.Integer != "" || value.FloatBits != "" || value.Diagnostic != nil {
		return false
	}
	if value.Kind == "object" {
		return value.Fields != nil && value.Items == nil && !value.hasItems
	}
	return value.Items != nil && value.Fields == nil && !value.hasFields
}

func hasTypedValue(value any) bool {
	switch value.(type) {
	case TypedValue, *TypedValue:
		return true
	default:
		return false
	}
}

func typedValuePointer(value any) *TypedValue {
	typed := typedValue(value)
	return &typed
}

func typedValue(value any) TypedValue {
	if typed, ok := value.(TypedValue); ok {
		return cloneTypedValue(typed)
	}
	if typed, ok := value.(*TypedValue); ok {
		if typed == nil {
			return TypedValue{Kind: "null"}
		}
		return cloneTypedValue(*typed)
	}
	if value == nil {
		return TypedValue{Kind: "null"}
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Ptr:
		if rv.IsNil() {
			return TypedValue{Kind: "null"}
		}
		return typedValue(rv.Elem().Interface())
	case reflect.String:
		return Text(rv.String())
	case reflect.Bool:
		boolean := rv.Bool()
		return TypedValue{Kind: "bool", Bool: &boolean, ScalarKind: "bool"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Integer(strconv.FormatInt(rv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return Integer(strconv.FormatUint(rv.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		return Float64Bits(rv.Float())
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return Bytes(rv.Bytes())
		}
		items := make([]TypedValue, rv.Len())
		for i := range items {
			items[i] = typedValue(rv.Index(i).Interface())
		}
		return TypedValue{Kind: "array", Items: items}
	case reflect.Array:
		items := make([]TypedValue, rv.Len())
		for i := range items {
			items[i] = typedValue(rv.Index(i).Interface())
		}
		return TypedValue{Kind: "array", Items: items}
	case reflect.Map:
		if rv.Type().Key().Kind() == reflect.String {
			fields := make(map[string]TypedValue, rv.Len())
			iter := rv.MapRange()
			for iter.Next() {
				fields[iter.Key().String()] = typedValue(iter.Value().Interface())
			}
			return TypedValue{Kind: "object", Fields: fields}
		}
	}
	text := renderValue(value)
	return TypedValue{Kind: "text", Text: &text, ScalarKind: rv.Type().String()}
}

// constFamily classifies a basic kind for constant coercion; "" = none.
func constFamily(k reflect.Kind) string {
	switch k {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.String:
		return "string"
	}
	return ""
}

// coerceConst converts want to got's type when want is of a predeclared
// default constant type, got is a same-family basic kind of a different type,
// and the conversion is lossless; otherwise returns want unchanged.
func coerceConst(got, want any) any {
	if got == nil || want == nil {
		return want
	}
	gt, wt := reflect.TypeOf(got), reflect.TypeOf(want)
	if gt == wt || wt.PkgPath() != "" {
		return want
	}
	switch wt {
	case reflect.TypeOf(false), reflect.TypeOf(0), reflect.TypeOf(int32(0)),
		reflect.TypeOf(0.0), reflect.TypeOf(""):
	default:
		return want
	}
	fam := constFamily(gt.Kind())
	if fam == "" || fam != constFamily(wt.Kind()) {
		return want
	}
	cv := reflect.ValueOf(want).Convert(gt)
	if cv.Convert(wt).Interface() != want {
		return want
	}
	return cv.Interface()
}

// Value records a bare fact for narration -- an intermediate or final value
// worth showing in the generated prose -- without itself asserting
// anything. v is canonically rendered via renderValue at call time (not
// lazily), so what gets narrated is exactly the value AS OF this call, never
// re-evaluated later.
func (s *Scenario) Value(key string, v any) {
	s.ensureWhen()
	s.steps = append(s.steps, Step{Kind: StepValue, Values: []ValueFact{{Key: key, Value: renderValue(v)}}})
}

// pairsToFacts converts an alternating key,value,... list into an ordered
// []ValueFact, canonically rendering each value via renderValue. Returns
// ok=false if kvPairs has odd length (a caller bug -- see Given's doc
// comment for why this is a hard failure, not a silent drop). A non-string
// key is rendered via fmt.Sprintf("%v", ...) rather than rejected outright,
// since Go's log/slog accepts this too and rejecting it would make Given
// pickier than the convention it deliberately mirrors -- but the common,
// expected case is a string literal key.
func pairsToFacts(kvPairs []any) ([]ValueFact, bool) {
	if len(kvPairs)%2 != 0 {
		return nil, false
	}
	facts := make([]ValueFact, 0, len(kvPairs)/2)
	for i := 0; i < len(kvPairs); i += 2 {
		key := fmt.Sprintf("%v", kvPairs[i])
		facts = append(facts, ValueFact{Key: key, Value: renderValue(kvPairs[i+1])})
	}
	return facts, true
}

// renderValue canonically renders v to a deterministic string, closing the
// three hazards PLAN-scenario-generated-spec.md §5 names by name
// (map-order, float-format, addresses):
//
//   - map-order: a map[K]V value has its keys sorted (by their %v
//     rendering) before rendering "k1:v1, k2:v2, ..." -- Go's own %v on a
//     map already sorts keys as of Go 1.12+ for BUILT-IN fmt verbs, but this
//     function does not rely on that fmt-internal behavior remaining true
//     forever; it sorts explicitly via reflection so the guarantee lives in
//     THIS package's own contract, not an incidental fmt implementation
//     detail this package happens to depend on.
//   - float format: a float32/float64 is rendered via strconv.FormatFloat
//     with the 'g' verb and -1 precision (shortest round-trippable decimal
//     representation) rather than %v's default %g-with-implementation-
//     chosen-precision, so the same float64 value renders identically
//     regardless of which Go version or platform produced it.
//   - addresses: a pointer is DEREFERENCED and its pointee rendered
//     recursively (never rendered as "0xc000...", which changes every
//     process run and would make two otherwise-identical scenario runs
//     produce different artifact bytes); a nil pointer renders as the
//     literal "<nil>". error values render via .Error() (not %v's default,
//     which for a *fmt.wrapError etc. already calls Error() but this makes
//     the choice explicit rather than incidental).
//
// Anything else (string, bool, int*, uint*, a struct/slice with no pointer/
// float inside) falls through to fmt.Sprintf("%v", v), which is already
// deterministic for those kinds.
func renderValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch typed := v.(type) {
	case TypedValue:
		return renderTypedValue(typed)
	case *TypedValue:
		if typed == nil {
			return "<nil>"
		}
		return renderTypedValue(*typed)
	}
	if err, ok := v.(error); ok {
		return err.Error()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			return "<nil>"
		}
		return renderValue(rv.Elem().Interface())
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Map:
		return renderMap(rv)
	default:
		return fmt.Sprintf("%v", v)
	}
}
func renderTypedValue(value TypedValue) string {
	switch value.Kind {
	case "text":
		if value.Text == nil {
			return "<absent>"
		}
		return *value.Text
	case "bytes":
		return "base64:" + value.Bytes
	case "bool":
		if value.Bool == nil {
			return "<absent>"
		}
		return strconv.FormatBool(*value.Bool)
	case "integer":
		if value.Integer == "" {
			return "<absent>"
		}
		return value.Integer
	case "float":
		bits, err := strconv.ParseUint(value.FloatBits, 16, 64)
		if err != nil {
			return "float-bits:" + value.FloatBits
		}
		return strconv.FormatFloat(math.Float64frombits(bits), 'g', -1, 64)
	case "object":
		keys := make([]string, 0, len(value.Fields))
		for key := range value.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('{')
		for i, key := range keys {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(key)
			b.WriteByte(':')
			b.WriteString(renderTypedValue(value.Fields[key]))
		}
		b.WriteByte('}')
		return b.String()
	case "array":
		var b strings.Builder
		b.WriteByte('[')
		for i := range value.Items {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderTypedValue(value.Items[i]))
		}
		b.WriteByte(']')
		return b.String()
	case "null":
		return "<nil>"
	case "diagnostic":
		if value.Diagnostic == nil {
			return "diagnostic:<absent>"
		}
		var b strings.Builder
		b.WriteString(value.Diagnostic.Code)
		if value.Diagnostic.Class != nil {
			b.WriteString(" class=")
			b.WriteString(*value.Diagnostic.Class)
		}
		if value.Diagnostic.Reason != nil {
			b.WriteString(" reason=")
			b.WriteString(*value.Diagnostic.Reason)
		}
		if value.Diagnostic.Line != nil {
			b.WriteString(" line=")
			b.WriteString(strconv.Itoa(*value.Diagnostic.Line))
		}
		if value.Diagnostic.Span != nil {
			b.WriteString(" span=")
			b.WriteString(strconv.Itoa(value.Diagnostic.Span.Start))
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(value.Diagnostic.Span.End))
		}
		return b.String()
	default:
		return value.Kind
	}
}

// renderMap renders a reflect.Value of Kind Map deterministically: every key
// is rendered via renderValue, the (renderedKey, renderedValue) pairs are
// sorted by renderedKey, then joined as "k1:v1, k2:v2, ...". Sorting by the
// RENDERED key string (not the raw key, which may not even be orderable --
// e.g. a struct key) is what makes this total for any map key type Go
// allows (comparable, but not necessarily ordered).
func renderMap(rv reflect.Value) string {
	type entry struct{ k, v string }
	entries := make([]entry, 0, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		entries = append(entries, entry{
			k: renderValue(iter.Key().Interface()),
			v: renderValue(iter.Value().Interface()),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].k < entries[j].k })
	out := "map["
	for i, e := range entries {
		if i > 0 {
			out += " "
		}
		out += e.k + ":" + e.v
	}
	out += "]"
	return out
}

// compile-time assertion that *testing.T satisfies T -- if the standard
// library ever changes Helper/Errorf/Fatalf's signatures this file fails to
// build instead of failing mysteriously at a real call site.
var _ T = (*testing.T)(nil)

// compile-time assertion that *testing.T also satisfies recordT -- a real
// test always supports record-mode; only this package's own fakeT double
// (hotamspec_test.go) deliberately does not.
var _ recordT = (*testing.T)(nil)

// Evidence is an executed method result. Holds consumes it without executing
// the supporting method again.
type Evidence struct {
	Subject      string
	Value        string
	Passed       bool
	Observations []Observation
	Input        string
	HasInput     bool
	Context      *ArtifactContext
	Case         *CaseContext
	CaseInput    *TypedValue
	CaseExpected *TypedValue

	hasExpectation bool
	want           bool
	isOption       bool
	hasContext     bool
	hasCase        bool
	rawInput       *TypedValue
}

// EvidenceOption shares the Evidence carrier so Holds can consume execution
// evidence and options in one ordered argument list.
type EvidenceOption = Evidence

// Expect overrides Holds' default expected predicate value of true.
func Expect(want bool) Evidence { return Evidence{hasExpectation: true, want: want} }

// WithInput attaches the real input to a Fact or Holds observation.
func WithInput(input any) EvidenceOption {
	return Evidence{Input: renderValue(input), HasInput: true, isOption: true, rawInput: typedValuePointer(input)}
}

// WithContext attaches explicitly supplied version/profile/target and producer
// metadata. Component declarations are copied and deterministically ordered;
// no component identity or measurement is inferred from process state.
func WithContext(ctx ArtifactContext) EvidenceOption {
	return Evidence{Context: copyArtifactContext(&ctx), hasContext: true, isOption: true}
}

// WithCase explicitly selects rule mode and supplies case metadata. The real
// test and input are recorder-derived; Expected may supply a shared root oracle.
func WithCase(ctx CaseContext) EvidenceOption {
	return Evidence{Case: cloneCaseContext(&ctx), hasCase: true, isOption: true}
}

func validateCaseContext(t T, caller string, ctx *CaseContext) bool {
	if ctx == nil || strings.TrimSpace(ctx.ID) == "" {
		t.Fatalf("hotamspec: %s: WithCase requires a non-empty case ID", caller)
		return false
	}
	rt, ok := t.(recordT)
	if !ok || strings.TrimSpace(rt.Name()) == "" {
		t.Fatalf("hotamspec: %s: WithCase requires a real test name from T.Name()", caller)
		return false
	}
	return true
}

// Fact executes a bound zero-argument value method once and asserts Eq
// semantics. Input and context are opt-in trailing values from WithInput and
// WithContext. WithCase selects rule mode: test/input derive from the real
// invocation, want remains this property's comparison expectation, and an
// optional CaseContext.Expected supplies the shared root oracle. A method may
// return Observed(value, comparisons...) to carry nested comparisons.
func Fact[V any](t T, method func() V, want any, options ...EvidenceOption) Evidence {
	t.Helper()
	subject, err := methodSubject(method)
	if err != nil {
		t.Fatalf("hotamspec: Fact: %v", err)
		return Evidence{}
	}
	var input string
	var hasInput bool
	var rawInput *TypedValue
	var ctx *ArtifactContext
	var caseContext *CaseContext
	caseSeen := false
	for _, option := range options {
		if !option.isOption || option.hasExpectation {
			t.Fatalf("hotamspec: Fact: trailing values must be WithInput, WithContext, or WithCase options")
			return Evidence{}
		}
		if option.HasInput {
			if hasInput {
				t.Fatalf("hotamspec: Fact: duplicate input")
				return Evidence{}
			}
			input, hasInput = option.Input, true
			rawInput = option.rawInput
		}
		if option.hasContext {
			if ctx != nil {
				t.Fatalf("hotamspec: Fact: duplicate context")
				return Evidence{}
			}
			ctx = option.Context
		}
		if option.hasCase {
			if caseSeen {
				t.Fatalf("hotamspec: Fact: duplicate case")
				return Evidence{}
			}
			caseSeen = true
			caseContext = option.Case
		}
	}
	if caseSeen && !validateCaseContext(t, "Fact", caseContext) {
		return Evidence{}
	}

	result := method()
	got := any(result)
	var nested []Observation
	if carrier, ok := any(result).(observedCarrier); ok {
		got, nested = carrier.hotamObservation()
	}
	mode := "fact"
	if caseContext != nil {
		mode = "rule"
	}
	s := atomScenario(t, subject, mode)
	if caseContext != nil {
		s.caseContext = cloneCaseContext(caseContext)
		s.caseInput = rawInput
		if s.caseContext.Expected != nil {
			s.caseExpected = cloneTypedValuePointer(s.caseContext.Expected)
		} else {
			s.caseExpected = typedValuePointer(want)
		}
	}
	passed, rendered, expected := s.eq(subject, got, want)
	eqObservation := s.steps[0].Observations[0]
	step := &s.steps[0]
	step.Subject, step.Value = subject, rendered
	step.Observations = cloneObservations(nested)
	if hasInput {
		step.Input, step.HasInput = input, true
	}
	step.Context = copyArtifactContext(ctx)
	for i := range step.Observations {
		if !step.Observations[i].Passed {
			passed = false
			t.Errorf("hotamspec: nested comparison %q failed for %s: got %s, want %s", step.Observations[i].Name, subject, step.Observations[i].Actual, step.Observations[i].Expected)
		}
	}
	ownObservation := Observation{
		Name: subject, Input: input, Actual: rendered, Expected: expected, Passed: s.steps[0].Passed,
		RawInput: rawInput,
	}
	if caseContext != nil || hasTypedValue(got) || hasTypedValue(want) {
		ownObservation.RawActual = eqObservation.RawActual
		ownObservation.RawExpected = eqObservation.RawExpected
		if ownObservation.RawActual == nil {
			ownObservation.RawActual = typedValuePointer(got)
		}
		if ownObservation.RawExpected == nil {
			ownObservation.RawExpected = typedValuePointer(want)
		}
	}
	step.Observations = append(step.Observations, ownObservation)
	step.Passed = passed
	observations := cloneObservations(step.Observations)
	return Evidence{
		Subject: subject, Value: rendered, Passed: passed, Observations: observations,
		Input: input, HasInput: hasInput, Context: copyArtifactContext(ctx),
		Case: cloneCaseContext(s.caseContext), CaseInput: cloneTypedValuePointer(s.caseInput),
		CaseExpected: cloneTypedValuePointer(s.caseExpected),
	}
}

// Holds executes a bound predicate once and records already executed evidence.
// WithCase selects rule mode: Expect remains the predicate comparison value,
// while CaseContext.Expected optionally supplies a shared root oracle.
func Holds(t T, predicate func() bool, evidence ...Evidence) Evidence {
	t.Helper()
	subject, err := methodSubject(predicate)
	if err != nil {
		t.Fatalf("hotamspec: Holds: %v", err)
		return Evidence{}
	}
	want, seen := true, false
	var input string
	var hasInput bool
	var rawInput *TypedValue
	var ctx *ArtifactContext
	var caseContext *CaseContext
	caseSeen := false
	for _, e := range evidence {
		if e.hasExpectation {
			if seen {
				t.Fatalf("hotamspec: Holds: duplicate expectation")
				return Evidence{}
			}
			want, seen = e.want, true
		}
		if e.isOption {
			if e.HasInput {
				if hasInput {
					t.Fatalf("hotamspec: Holds: duplicate input")
					return Evidence{}
				}
				input, hasInput = e.Input, true
				rawInput = e.rawInput
			}
			if e.hasContext {
				if ctx != nil {
					t.Fatalf("hotamspec: Holds: duplicate context")
					return Evidence{}
				}
				ctx = e.Context
			}
			if e.hasCase {
				if caseSeen {
					t.Fatalf("hotamspec: Holds: duplicate case")
					return Evidence{}
				}
				caseSeen = true
				caseContext = e.Case
			}
		}
	}
	if caseSeen && !validateCaseContext(t, "Holds", caseContext) {
		return Evidence{}
	}
	got := predicate()
	rendered := strconv.FormatBool(got)
	expected := strconv.FormatBool(want)
	passed := got == want
	mode := "holds"
	if caseContext != nil {
		mode = "rule"
	}
	s := atomScenario(t, subject, mode)
	if caseContext != nil {
		s.caseContext = cloneCaseContext(caseContext)
		s.caseInput = rawInput
		if s.caseContext.Expected != nil {
			s.caseExpected = cloneTypedValuePointer(s.caseContext.Expected)
		} else {
			s.caseExpected = typedValuePointer(want)
		}
	}
	ownObservation := Observation{
		Name: subject, Input: input, Actual: rendered, Expected: expected, Passed: passed,
		RawInput: rawInput,
	}
	if caseContext != nil {
		ownObservation.RawActual = typedValuePointer(got)
		ownObservation.RawExpected = typedValuePointer(want)
	}
	s.steps = append(s.steps, Step{
		Kind: StepThen, Desc: subject + " " + rendered, Subject: subject, Value: rendered,
		Passed: passed, Input: input, HasInput: hasInput, Expected: expected, HasExpected: true,
		Observations: []Observation{ownObservation}, Context: copyArtifactContext(ctx),
	})
	observations := []Observation{ownObservation}
	for _, e := range evidence {
		if e.hasExpectation || e.isOption {
			continue
		}
		if e.Subject == "" || !e.Passed {
			passed = false
		}
		s.steps = append(s.steps, Step{
			Kind: StepValue, Desc: e.Subject + " " + e.Value, Subject: e.Subject,
			Value: e.Value, Passed: e.Passed, Input: e.Input, HasInput: e.HasInput,
			Observations: cloneObservations(e.Observations), Context: copyArtifactContext(e.Context),
		})
		observations = append(observations, e.Observations...)
	}
	s.steps[0].Passed = passed
	if !passed {
		t.Errorf("hotamspec: Holds(%s) failed: got %t, want %t, supporting evidence must pass", subject, got, want)
	}
	if caseContext != nil || rawInput != nil || observationsHaveRawValues(observations) {
		observations = cloneObservations(observations)
	}
	return Evidence{
		Subject: subject, Value: rendered, Passed: passed, Observations: observations,
		Input: input, HasInput: hasInput, Context: copyArtifactContext(ctx),
		Case: cloneCaseContext(s.caseContext), CaseInput: cloneTypedValuePointer(s.caseInput),
		CaseExpected: cloneTypedValuePointer(s.caseExpected),
	}
}

var atomFiles = struct {
	sync.Mutex
	counts map[recordT]map[string]int
}{counts: make(map[recordT]map[string]int)}

func atomScenario(t T, subject, mode string) *Scenario {
	methodAt := strings.LastIndexByte(subject, '.')
	receiverAt := strings.LastIndexByte(subject[:methodAt], '.')
	reqID := "R-" + atomKebab(subject[receiverAt+1:methodAt]) + "-" + atomKebab(subject[methodAt+1:])
	s := NewScenario(t, reqID, "")
	s.mode = mode
	if rt, ok := t.(recordT); ok && (os.Getenv(RecordDirEnv) != "" || os.Getenv("HOTAM_RECORD_STDOUT") == "1") {
		atomFiles.Lock()
		counts := atomFiles.counts[rt]
		if counts == nil {
			counts = make(map[string]int)
			atomFiles.counts[rt] = counts
			rt.Cleanup(func() {
				atomFiles.Lock()
				delete(atomFiles.counts, rt)
				atomFiles.Unlock()
			})
		}
		counts[reqID]++
		s.fileSuffix = fmt.Sprintf("__atom-%03d", counts[reqID])
		atomFiles.Unlock()
	}
	return s
}

func methodSubject(method any) (string, error) {
	v := reflect.ValueOf(method)
	if !v.IsValid() || v.Kind() != reflect.Func || v.IsNil() {
		return "", fmt.Errorf("expected a bound method")
	}
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return "", fmt.Errorf("method has no runtime name")
	}
	return normalizeMethodName(fn.Name())
}

func normalizeMethodName(name string) (string, error) {
	if !strings.HasSuffix(name, "-fm") {
		return "", fmt.Errorf("%q is not a bound method", name)
	}
	name = strings.TrimSuffix(name, "-fm")
	var b strings.Builder
	depth := 0
	for _, r := range name {
		if r == '[' {
			depth++
			continue
		}
		if r == ']' {
			depth--
			continue
		}
		if depth == 0 && r != '(' && r != ')' && r != '*' {
			b.WriteRune(r)
		}
	}
	name = b.String()
	last := strings.LastIndexByte(name, '.')
	if last < 0 {
		return "", fmt.Errorf("%q has no method", name)
	}
	receiver := strings.LastIndexByte(name[:last], '.')
	if receiver < strings.LastIndexByte(name[:last], '/') || receiver < 0 {
		return "", fmt.Errorf("%q has no receiver", name)
	}
	if depth != 0 {
		return "", fmt.Errorf("%q is not a named method", name)
	}
	return name, nil
}

func atomKebab(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
