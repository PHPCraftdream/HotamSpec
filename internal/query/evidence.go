// evidence.go builds task #399/W2.2's five-section evidence packet that extends
// BriefCard (brief.go) with the IMPLEMENTATION/PROOF/STRUCTURE context an agent
// orienting on one Requirement would otherwise have to assemble from 3-4 separate
// round-trips (the graph, the Go source, the generated docs, and the tests):
//
//  1. Object/method signatures -- every implemented_by entry resolved to its
//     owning ModelObject + the specific symbol's signature/doc.
//  2. Port/mock contract -- when a resolved object is a port (interface) or a
//     mock, its FULL method set as the contract it declares/stands in for.
//  3. Actual Given/When/Then -- the first passing scenario artifact's title +
//     ordered steps, from a real go-test execution of a scenario-carrying
//     verified_by entry.
//  4. Related entity/process -- reverse-match the requirement's implemented_by
//     files against EntityType.ModelSymbol fields, surfacing the entity types
//     and the processes that drive them.
//  5. Provenance -- each section carries a Source string so a reader can tell
//     where each fragment came from without re-deriving it.
//
// All five sections are Requirement-only (nil/omitempty for Conflict/Assumption
// anchors, exactly like Freshness already is). Each section is independently
// honest-absent: a requirement whose implemented_by does not resolve, that
// carries no scenario, or that touches no EntityType simply omits that section,
// never errors. The scan (gate.ScanAuthoredModels) and the test execution
// (gate.RunVerifiedByTestRecording) are bounded to ONE requirement per brief
// invocation -- the same acceptable cost class task #398's State field already
// pays (Decision 4).
package query

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// EvidenceSignature is one resolved implemented_by entry: the owning
// ModelObject's identity/classification/doc plus the specific resolved symbol's
// own signature and doc. Built only for entries that resolve via
// gate.MatchCitedSymbol -- a stale/malformed citation (already
// check_implemented_by_symbol_resolvable's own violation) is simply omitted
// from this section, never an error.
type EvidenceSignature struct {
	Citation        string `json:"citation"`          // the raw implemented_by entry
	File            string `json:"file"`              // domain-relative file path
	SymbolKind      string `json:"symbol_kind"`       // "method" | "interface-method" | "func"
	ObjectName      string `json:"object_name"`       // owning ModelObject name ("" for a func)
	ObjectKind      string `json:"object_kind"`       // "struct" | "interface" | "type"
	ObjectModelKind string `json:"object_model_kind"` // "object"|"value"|"port"|"mock"|"policy"
	ObjectDoc       string `json:"object_doc"`
	SymbolName      string `json:"symbol_name"`
	Signature       string `json:"signature"` // the resolved symbol's own signature
	SymbolDoc       string `json:"symbol_doc"`
	Source          string `json:"source"` // provenance
}

// EvidenceContractMethod is one method of a port/mock contract, mirroring
// gate.ModelInterfaceMethod's Name/Signature/Doc/Embedded shape.
type EvidenceContractMethod struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Doc       string `json:"doc"`
	Embedded  string `json:"embedded,omitempty"` // set instead of Name/Signature for an embedded interface
}

// EvidencePortContract is the full method set of a port (interface) or mock
// object the requirement touches. For a port, this is the interface's own
// declared InterfaceMethods (the contract it specifies); for a mock, this is
// the mock's own Methods (the executable surrogate's method set) -- a known
// simplification that does NOT cross-reference back to the specific port a
// mock implements (noted in task #399's Decision 3.2).
type EvidencePortContract struct {
	ObjectName string                   `json:"object_name"`
	File       string                   `json:"file"`
	ModelKind  string                   `json:"model_kind"` // "port" or "mock"
	Methods    []EvidenceContractMethod `json:"methods"`
	Source     string                   `json:"source"`
}

// EvidenceScenarioStep is one Given/When/Then/Value step from a recorded
// scenario artifact, mirroring internal/invariants/scenario_quality.go's own
// scenarioQualityArtifactStep shape (Kind/Desc only -- Values/Passed are
// irrelevant to presentation, same as they are to the quality gate). Defined
// as query's OWN local decode target per the established "each consumer
// package decodes its own local shape" precedent (scenario_quality.go's own
// doc comment), never imported from invariants.
type EvidenceScenarioStep struct {
	Kind string `json:"kind"`
	Desc string `json:"desc"`
}

// EvidenceScenario is the ACTUAL Given/When/Then narrative from the FIRST
// verified_by entry that AST-carries a scenario (gate.ResolveSpecTest's
// HasScenario) AND produces at least one Verdict=="pass" artifact via
// gate.RunVerifiedByTestRecording. This is a PRESENTATIONAL feature (show
// whatever scenario exists, even an imperfect one), NOT a compliance check --
// task #397's four-rule quality gate is a separate concern this task
// deliberately does not conflate (Decision 3.3).
type EvidenceScenario struct {
	Title  string                 `json:"title"`
	Steps  []EvidenceScenarioStep `json:"steps"`
	Test   string                 `json:"test"`   // the verified_by entry that produced this artifact
	Source string                 `json:"source"` // provenance
}

// EvidenceProcessRef is a short reference to a Process that drives a matched
// EntityType -- its ID and Why, not the full Process struct.
type EvidenceProcessRef struct {
	ID  string `json:"id"`
	Why string `json:"why"`
}

// EvidenceEntity is one reverse-matched EntityType (its ModelSymbol names a
// file the requirement's implemented_by also cites) plus the Processes whose
// DrivesEntities includes this EntityType's Slug.
type EvidenceEntity struct {
	EntityTypeSlug string               `json:"entity_type_slug"`
	ModelSymbol    string               `json:"model_symbol"`
	Processes      []EvidenceProcessRef `json:"processes,omitempty"`
}

// EvidenceEntityProcess is section 4's aggregate: every EntityType whose
// ModelSymbol matches one of the requirement's implemented_by files, each
// carrying the Processes that drive it. Empty/nil when nothing matches (the
// common case today -- zero live EntityTypes set ModelSymbol anywhere yet).
type EvidenceEntityProcess struct {
	Entities []EvidenceEntity `json:"entities,omitempty"`
	Source   string           `json:"source"`
}

// evidenceResult is the internal aggregate buildEvidence returns -- the four
// populated sections, each independently nil/empty when nothing was found.
type evidenceResult struct {
	signatures    []EvidenceSignature
	portContracts []EvidencePortContract
	scenario      *EvidenceScenario
	entityProcess *EvidenceEntityProcess
}

// briefArtifact is this file's own local decode target for one
// gate.RecordedArtifact's RawJSON bytes -- the same "equivalent local shape
// rather than widening gate's public API" precedent
// internal/invariants/scenario_quality.go's own scenarioQualityArtifact and
// internal/selfspec/claim_derive.go's own scenarioArtifact both establish.
// Only Title/Steps/Verdict are decoded -- the fields this presentational
// feature actually needs.
type briefArtifact struct {
	Title   string              `json:"title"`
	Steps   []briefArtifactStep `json:"steps"`
	Verdict string              `json:"verdict"`
}

type briefArtifactStep struct {
	Kind string `json:"kind"`
	Desc string `json:"desc"`
}

// buildEvidence computes all four evidence sections for one Requirement. It
// calls gate.ScanAuthoredModels ONCE (shared across sections 1, 2, and 4) and
// gate.RunVerifiedByTestRecording at most ONCE (section 3, first
// scenario-carrying verified_by entry only). Every section is independently
// honest-absent: a scan failure, an unresolvable citation, a missing scenario,
// or no entity match simply omits that section without erroring.
func buildEvidence(g *ontology.Graph, r ontology.Requirement) evidenceResult {
	specRoot := gate.SpecRootForGraph(g)

	files, err := gate.ScanAuthoredModels(g)
	if err != nil || len(files) == 0 {
		// No authored models to resolve against -- every section that depends
		// on the scan is honest-absent. Section 3 (scenario) does NOT depend
		// on the model scan, but a requirement with no resolvable
		// implemented_by is unlikely to have a meaningful scenario either --
		// and we still try it independently below.
		return evidenceResult{
			scenario: buildScenarioEvidence(g, r, specRoot),
		}
	}

	res := evidenceResult{}
	citedFiles := map[string]struct{}{} // files the requirement's implemented_by resolves into (for entity matching)

	// Sections 1 + 2: resolve each implemented_by entry.
	for _, ibRaw := range r.ImplementedBy {
		trimmed := strings.TrimSpace(ibRaw)
		file, symbol, ok := gate.ParseFileColonSymbol(trimmed)
		if !ok {
			continue
		}
		if scopeOK, _ := gate.EntryWithinSpecScope(specRoot, file, g.SelfHosting); !scopeOK {
			continue
		}

		kind, objName, symbolName, found := gate.MatchCitedSymbol(files, file, symbol)
		if !found {
			continue
		}

		obj, signature, symDoc, detailOK := resolveSymbolDetails(files, file, kind, objName, symbolName)
		if !detailOK {
			continue
		}

		citedFiles[normPath(file)] = struct{}{}

		res.signatures = append(res.signatures, EvidenceSignature{
			Citation:        trimmed,
			File:            file,
			SymbolKind:      symbolKindString(kind),
			ObjectName:      objName,
			ObjectKind:      obj.Kind,
			ObjectModelKind: obj.ModelKind,
			ObjectDoc:       obj.Doc,
			SymbolName:      symbolName,
			Signature:       signature,
			SymbolDoc:       symDoc,
			Source:          fmt.Sprintf("gate.ScanAuthoredModels + gate.MatchCitedSymbol of %s", file),
		})

		// Section 2: port/mock contract (deduplicated by object -- multiple
		// citations to the same port/mock produce one contract entry).
		if obj.ModelKind == "port" || obj.ModelKind == "mock" {
			if !contractAlreadySeen(res.portContracts, file, obj.Name) {
				res.portContracts = append(res.portContracts, buildContract(obj, file))
			}
		}
	}

	// Section 3: scenario (independent of the model scan).
	res.scenario = buildScenarioEvidence(g, r, specRoot)

	// Section 4: entity/process reverse-match.
	res.entityProcess = buildEntityProcessEvidence(g, citedFiles)

	return res
}

// resolveSymbolDetails looks up the owning ModelObject and the specific
// resolved symbol's signature+doc from the scanned inventory, given a
// successful gate.MatchCitedSymbol result. For a func (SymbolKindFunc), obj
// is returned zero-value (a top-level function has no owning object).
func resolveSymbolDetails(files []gate.ModelFile, file string, kind gate.SymbolKind, objName, symbolName string) (obj gate.ModelObject, signature, doc string, ok bool) {
	wantFile := normPath(file)
	for _, f := range files {
		if normPath(f.RelPath) != wantFile {
			continue
		}
		if kind == gate.SymbolKindFunc {
			for _, fn := range f.Funcs {
				if fn.Name == symbolName {
					return gate.ModelObject{}, fn.Signature, fn.Doc, true
				}
			}
			return gate.ModelObject{}, "", "", false
		}
		for _, o := range f.Objects {
			if o.Name != objName {
				continue
			}
			if kind == gate.SymbolKindMethod {
				for _, m := range o.Methods {
					if m.Name == symbolName {
						return o, m.Signature, m.Doc, true
					}
				}
			}
			if kind == gate.SymbolKindInterfaceMethod {
				for _, im := range o.InterfaceMethods {
					if im.Name == symbolName {
						return o, im.Signature, im.Doc, true
					}
				}
			}
		}
	}
	return gate.ModelObject{}, "", "", false
}

// symbolKindString renders a gate.SymbolKind as the JSON-friendly string this
// section's consumers expect.
func symbolKindString(k gate.SymbolKind) string {
	switch k {
	case gate.SymbolKindMethod:
		return "method"
	case gate.SymbolKindInterfaceMethod:
		return "interface-method"
	case gate.SymbolKindFunc:
		return "func"
	default:
		return "unknown"
	}
}

// contractAlreadySeen reports whether a contract for the same (file, object)
// pair is already in the list -- avoids duplicating a port/mock contract when
// multiple implemented_by entries resolve to different methods of the same
// object.
func contractAlreadySeen(contracts []EvidencePortContract, file, objName string) bool {
	for _, c := range contracts {
		if c.ObjectName == objName && normPath(c.File) == normPath(file) {
			return true
		}
	}
	return false
}

// buildContract builds one EvidencePortContract from a resolved port or mock
// object. For a port (interface), the contract is the object's own declared
// InterfaceMethods; for a mock, it is the mock's own Methods (the executable
// surrogate's method set) -- a known simplification per Decision 3.2.
func buildContract(obj gate.ModelObject, file string) EvidencePortContract {
	var methods []EvidenceContractMethod
	if obj.ModelKind == "port" {
		for _, im := range obj.InterfaceMethods {
			methods = append(methods, EvidenceContractMethod{
				Name:      im.Name,
				Signature: im.Signature,
				Doc:       im.Doc,
				Embedded:  im.Embedded,
			})
		}
	} else {
		// mock: show the mock's own receiver methods.
		for _, m := range obj.Methods {
			methods = append(methods, EvidenceContractMethod{
				Name:      m.Name,
				Signature: m.Signature,
				Doc:       m.Doc,
			})
		}
	}
	return EvidencePortContract{
		ObjectName: obj.Name,
		File:       file,
		ModelKind:  obj.ModelKind,
		Methods:    methods,
		Source:     fmt.Sprintf("gate.ScanAuthoredModels of %s (%s contract)", file, obj.ModelKind),
	}
}

// buildScenarioEvidence resolves the FIRST verified_by entry (in r.VerifiedBy
// order) that AST-carries a scenario (gate.ResolveSpecTest's HasScenario) and,
// once executed via gate.RunVerifiedByTestRecording, produces at least one
// Verdict=="pass" artifact. Returns nil (honest-absent) when no entry carries
// a scenario or none produces a passing artifact.
//
// This is a PRESENTATIONAL feature -- it shows whatever scenario exists, even
// an imperfect one, and deliberately does NOT apply task #397's four-rule
// quality gate (a compliance concern, not a presentation concern).
func buildScenarioEvidence(g *ontology.Graph, r ontology.Requirement, specRoot string) *EvidenceScenario {
	for _, vbRaw := range r.VerifiedBy {
		trimmed := strings.TrimSpace(vbRaw)
		file, testName, ok := gate.ParseFileColonSymbol(trimmed)
		if !ok || !strings.HasPrefix(testName, "Test") {
			continue
		}
		if scopeOK, _ := gate.EntryWithinSpecScope(specRoot, file, g.SelfHosting); !scopeOK {
			continue
		}
		result, err := gate.ResolveSpecTest(specRoot, file, testName)
		if err != nil || !result.Found || !result.HasScenario {
			continue
		}

		// This entry carries a scenario -- execute it.
		recResult := gate.RunVerifiedByTestRecording(specRoot, file, testName, "")
		if recResult.Skipped || recResult.Err != nil || recResult.CompileFailed || !recResult.Passed {
			continue
		}
		for _, art := range recResult.Artifacts {
			parsed, decodeOK := decodeBriefArtifact(art.RawJSON)
			if !decodeOK || parsed.Verdict != "pass" {
				continue
			}
			steps := make([]EvidenceScenarioStep, 0, len(parsed.Steps))
			for _, s := range parsed.Steps {
				steps = append(steps, EvidenceScenarioStep{Kind: s.Kind, Desc: s.Desc})
			}
			return &EvidenceScenario{
				Title:  parsed.Title,
				Steps:  steps,
				Test:   trimmed,
				Source: fmt.Sprintf("live go test execution of %s (verdict: pass)", testName),
			}
		}
	}
	return nil
}

// decodeBriefArtifact decodes one gate.RecordedArtifact.RawJSON into
// briefArtifact. Defensive: a malformed artifact returns ok=false (mirrors
// internal/invariants/scenario_quality.go's decodeScenarioQualityArtifact and
// internal/selfspec/claim_derive.go's own decode pattern).
func decodeBriefArtifact(raw []byte) (briefArtifact, bool) {
	var parsed briefArtifact
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return briefArtifact{}, false
	}
	return parsed, true
}

// buildEntityProcessEvidence reverse-matches the requirement's implemented_by
// files (citedFiles -- the set of domain-relative files its citations resolved
// into) against g.EntityTypes[].ModelSymbol. For every EntityType whose
// ModelSymbol names the same file as one of the requirement's citations, it
// surfaces the EntityType plus any g.Processes[] whose DrivesEntities includes
// that EntityType's Slug. Returns nil when nothing matches (the common case
// today -- zero live EntityTypes set ModelSymbol anywhere yet).
//
// Matching precision: file-level (the EntityType's ModelSymbol file must
// normalize-equal one of the requirement's cited files). When the EntityType's
// ModelSymbol also names a specific type symbol, that symbol is compared
// against the implemented_by citation's resolved object name as a refinement
// -- but a file-level match alone is sufficient, since a requirement citing a
// method "Risk.Validate" and an EntityType naming type "Risk" in the same file
// are clearly connected even though the symbols differ (one is a method, the
// other a type declaration).
func buildEntityProcessEvidence(g *ontology.Graph, citedFiles map[string]struct{}) *EvidenceEntityProcess {
	if len(citedFiles) == 0 {
		return nil
	}
	var entities []EvidenceEntity
	for _, et := range g.EntityTypes {
		raw := strings.TrimSpace(et.ModelSymbol)
		if raw == "" {
			continue
		}
		etFile, _, ok := gate.ParseFileColonSymbol(raw)
		if !ok {
			continue
		}
		if _, matches := citedFiles[normPath(etFile)]; !matches {
			continue
		}
		entity := EvidenceEntity{
			EntityTypeSlug: et.Slug,
			ModelSymbol:    raw,
		}
		// Find Processes that drive this EntityType.
		for _, p := range g.Processes {
			for _, slug := range p.DrivesEntities {
				if slug == et.Slug {
					entity.Processes = append(entity.Processes, EvidenceProcessRef{
						ID:  p.ID,
						Why: p.Why,
					})
					break
				}
			}
		}
		entities = append(entities, entity)
	}
	if len(entities) == 0 {
		return nil
	}
	return &EvidenceEntityProcess{
		Entities: entities,
		Source:   "graph.json EntityType.model_symbol reverse-match against implemented_by files",
	}
}

// normPath cleans a domain-relative path and forces forward slashes so an
// authored file path compares equal regardless of OS separator or cleaning
// differences. Same body as gate.normalizeRelPath (unexported there) and the
// former internal/invariants normalizeRelPath.
func normPath(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}
