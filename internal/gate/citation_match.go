// citation_match.go holds the EXPORTED citation-resolution machinery BOTH
// internal/invariants (check_model_complete/check_public_surface_linked_or_marked)
// AND internal/query (task #399/W2.2's BriefCard evidence packet) call to answer
// the single question "does this implemented_by 'file:symbol' citation resolve
// to an EXPORTED receiver method, an EXPORTED interface method, or an EXPORTED
// top-level function in the domain's scanned model inventory?"
//
// LAYERING (why this lives in internal/gate, not internal/invariants, task #399
// /W2.2): this function was originally internal/invariants/model_complete.go's
// UNEXPORTED matchCitedSymbol (added by task #396/W1.4 to generalize the older
// matchCitedExportedMethod). internal/query is a read-only presentation layer
// that must NOT import internal/invariants (the enforcement/violation layer) --
// invariants is a layer ABOVE query conceptually, and there is a real (if
// currently absent) risk of an import cycle if invariants ever needs something
// from query later. internal/gate is a true leaf BOTH invariants (authored_links.
// go, scenario_discipline.go, ...) AND query (brief.go, evidence.go, ...) already
// depend on directly, and it already owns the ModelFile/ModelObject/ScanAuthoredModels
// scan this function operates over (model_scan.go) -- promoting the citation
// matcher here, once, lets both higher layers call the EXACT SAME code without
// either importing the other, and without forking a duplicate matcher into query.
//
// This file is read-only over the scanned model inventory: it never executes,
// type-checks, or mutates anything it reads.
package gate

import (
	"go/token"
	"path/filepath"
)

// SymbolKind classifies which of the three EXPORTED public-surface categories
// MatchCitedSymbol resolved an implemented_by citation to -- the same three
// categories task #396/W1.4's brief names as checkPublicSurfaceLinkedOrMarked's
// scope: a receiver method (ModelObject.Methods), an interface's own declared
// method (ModelObject.InterfaceMethods, Name != ""), or a top-level function/
// constructor (ModelFile.Funcs).
//
// This is the EXPORTED promotion of the former unexported symbolKind from
// internal/invariants/model_complete.go. The values are iota-ordered so the
// underlying int is stable for use as a map key (internal/invariants'
// citedSymbolKey formats it into a string key).
type SymbolKind int

const (
	// SymbolKindMethod is a receiver method (ModelObject.Methods).
	SymbolKindMethod SymbolKind = iota
	// SymbolKindInterfaceMethod is an interface's own declared method
	// (ModelObject.InterfaceMethods, Name != "").
	SymbolKindInterfaceMethod
	// SymbolKindFunc is a top-level function/constructor (ModelFile.Funcs).
	SymbolKindFunc
)

// MatchCitedSymbol resolves a cited implemented_by "file:symbol" entry against
// the scanned authored-model inventory and reports, when the citation names an
// EXPORTED receiver method, an EXPORTED interface method, or an EXPORTED
// top-level function, which category it is (SymbolKind) and its identity
// (objName -- "" for a func -- and symbolName). Returns found=false for
// anything that is not one of those three categories -- a type-only citation,
// an unexported symbol, an embedded interface entry (no Name of its own), or a
// citation into a file the scan did not parse (e.g. an out-of-scope path).
//
// This is the EXPORTED promotion of the former unexported matchCitedSymbol
// from internal/invariants/model_complete.go (task #396/W1.4 generalization of
// the former matchCitedExportedMethod). It preserves that exact matching
// behavior byte-for-byte, and adds two more categories beyond receiver methods,
// checked in this priority order per candidate object/file so a qualified
// "Type.Symbol" citation naming an INTERFACE never accidentally matches an
// unrelated top-level func of the same bare name:
//
//  1. obj.Methods (receiver methods) -- exactly matchCitedExportedMethod's
//     original rule.
//  2. obj.InterfaceMethods with Name != "" (an interface's own declared
//     method; Embedded entries, which have no Name, are never matched here
//     -- they are not this object's own declared method).
//  3. ModelFile.Funcs (top-level functions/constructors) -- matched only
//     when the citation is BARE (unqualified) or when NO object in the file
//     matched categories 1-2, since a func is not receiver-scoped and has no
//     "Type." qualifier of its own; a QUALIFIED citation whose Type does not
//     match any object in the file cannot legitimately be a func citation
//     either (the qualifier already commits to naming a type), so funcs are
//     tried only for a bare wantName in this pass.
//
// Matching rules for categories 1-2 mirror gate.ResolveSpecSymbol's own
// documented convention for implemented_by symbol names (spec_resolver.go):
//   - a QUALIFIED "Type.Method" symbol matches an object named "Type" in the
//     named file that declares an exported method "Method";
//   - a BARE "Method" symbol matches the FIRST object in the named file
//     (declaration order) that declares an exported method "Method".
//
// fileRel is the citation's domain-relative file path (forward-slash, as
// authored); each scanned ModelFile.RelPath is root-relative forward-slash
// -- both are normalized via normalizeRelPath (filepath.ToSlash +
// filepath.Clean) before comparison so platform separator/cleaning
// differences cannot cause a spurious mismatch.
func MatchCitedSymbol(files []ModelFile, fileRel, symbol string) (kind SymbolKind, objName, symbolName string, found bool) {
	wantFile := normalizeRelPath(fileRel)
	wantType, wantName, qualified := splitQualifiedSymbol(symbol)

	for _, f := range files {
		if normalizeRelPath(f.RelPath) != wantFile {
			continue
		}
		for _, obj := range f.Objects {
			if qualified && obj.Name != wantType {
				continue
			}
			// Category 1: receiver methods (matchCitedExportedMethod's
			// original, unchanged rule).
			for _, m := range obj.Methods {
				if m.Name != wantName {
					continue
				}
				if !token.IsExported(m.Name) {
					// Unexported methods are out of scope. Keep scanning -- a
					// different object in the same file may declare an
					// exported method of the same name.
					continue
				}
				return SymbolKindMethod, obj.Name, m.Name, true
			}
			// Category 2: interface methods (Name != "" only -- an Embedded
			// entry is not this object's own declared method).
			for _, im := range obj.InterfaceMethods {
				if im.Name != wantName || im.Name == "" {
					continue
				}
				if !token.IsExported(im.Name) {
					continue
				}
				return SymbolKindInterfaceMethod, obj.Name, im.Name, true
			}
		}
		// Category 3: top-level funcs -- only for a BARE citation (see doc
		// comment: a qualified citation already commits to naming a type, so
		// it cannot legitimately resolve to a func here).
		if !qualified {
			for _, fn := range f.Funcs {
				if fn.Name != wantName {
					continue
				}
				if !token.IsExported(fn.Name) {
					continue
				}
				return SymbolKindFunc, "", fn.Name, true
			}
		}
		return SymbolKindMethod, "", "", false
	}
	return SymbolKindMethod, "", "", false
}

// normalizeRelPath cleans a domain-relative path and forces forward slashes so
// an authored implemented_by file ("spec/model/risk.go") compares equal to a
// scanned ModelFile.RelPath regardless of OS separator or minor cleaning
// differences. Promoted alongside MatchCitedSymbol from internal/invariants
// (task #399/W2.2) -- it is the same byte-for-byte body the invariants copy
// carried.
func normalizeRelPath(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}
