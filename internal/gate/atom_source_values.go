package gate

import (
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"go/ast"
	"go/token"
)

// atom_source_values.go resolves atom method return types and named value constants used by value atoms.
// atomValueConstant is a model package constant whose doc may carry
// language blocks or a verbatim `>>>>> lang=*` marker.
type atomValueConstant struct {
	name         string
	translations ontology.LocalizedText
	verbatim     bool
	isString     bool
	position     token.Position
}

func (c atomValueConstant) untranslatedString() bool {
	return c.isString && !c.verbatim && len(c.translations) == 0
}

// returnTypeName names the method's sole return type when it is a plain
// identifier; value-to-constant matching keys on this name.
func (s AtomSource) returnTypeName() string {
	typeName, _ := s.returnType()
	return typeName
}

// returnType names the method's sole return type: a plain identifier yields
// (name, ""), a selector `pkg.Type` yields (Type, pkg).
func (s AtomSource) returnType() (typeName, pkgName string) {
	if s.Method == nil || s.Method.Type == nil || s.Method.Type.Results == nil || len(s.Method.Type.Results.List) != 1 {
		return "", ""
	}
	field := s.Method.Type.Results.List[0]
	if len(field.Names) > 1 {
		return "", ""
	}
	switch typ := field.Type.(type) {
	case *ast.Ident:
		return typ.Name, ""
	case *ast.SelectorExpr:
		if pkg, ok := typ.X.(*ast.Ident); ok {
			return typ.Sel.Name, pkg.Name
		}
	}
	return "", ""
}

// valueConstant resolves the constant for the subject's return type within
// the subject's own model package. A selector return type `pkg.Type` resolves
// through the subject file's imports to that model package's constants.
func (index *AtomSourceIndex) valueConstant(source AtomSource, value string) (atomValueConstant, bool) {
	typeName, pkgName := source.returnType()
	if typeName == "" {
		return atomValueConstant{}, false
	}
	packagePath := source.PackagePath
	if pkgName != "" {
		if source.PackageImports == nil {
			return atomValueConstant{}, false
		}
		packagePath = source.PackageImports[pkgName]
	}
	byValue, ok := index.constants[packagePath]
	if !ok {
		return atomValueConstant{}, false
	}
	entry, ok := byValue[typeName][value]
	return entry, ok
}

// returnsBool reports whether the subject method returns exactly one bare
// `bool`. Aliases (`type Bit bool`) are intentionally not recognized: typed
// values from the recorder are absent for plain fact/holds steps, so the AST
// return type is the only reliable bool signal.
func (s AtomSource) returnsBool() bool {
	if s.Method == nil || s.Method.Type == nil || s.Method.Type.Results == nil || len(s.Method.Type.Results.List) != 1 {
		return false
	}
	field := s.Method.Type.Results.List[0]
	if len(field.Names) > 1 {
		return false
	}
	ident, ok := field.Type.(*ast.Ident)
	return ok && ident.Name == "bool"
}

// atomPhraseText renders a fact/holds step: a bool method renders the bare
// phrase on "true" and needs an authored `not:` negation on "false", other
// methods render "phrase — value.".
