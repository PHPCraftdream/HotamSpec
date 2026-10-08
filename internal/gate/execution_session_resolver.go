package gate

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type parsedSourceFile struct {
	hash    [32]byte
	fileSet *token.FileSet
	file    *ast.File
	err     error
}

// Standalone resolvers always read and parse current bytes. The only reuse is
// owned by an explicit execution session, keyed by content rather than stat data.
func parseSourceFileFresh(path string) (*token.FileSet, *ast.File, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	return fileSet, file, err
}

func (s *ExecutionSession) parseSourceFile(path string) (*token.FileSet, *ast.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	contents, err := os.ReadFile(absolute)
	if err != nil {
		return nil, nil, err
	}
	hash := sha256.Sum256(contents)
	s.astMu.Lock()
	defer s.astMu.Unlock()
	if previous, ok := s.astFiles[absolute]; ok && previous.hash == hash {
		return previous.fileSet, previous.file, previous.err
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, absolute, contents, parser.ParseComments)
	s.astFiles[absolute] = parsedSourceFile{hash, fileSet, file, err}
	return fileSet, file, err
}

func (s *ExecutionSession) ResolveSpecSymbol(specRoot, file, symbol string) (SpecSymbolResult, error) {
	if err := s.begin(); err != nil {
		return SpecSymbolResult{}, err
	}
	defer s.users.Done()
	path := filepath.Join(specRoot, filepath.FromSlash(file))
	_, parsed, err := s.parseSourceFile(path)
	if err != nil {
		return SpecSymbolResult{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return resolveSpecSymbol(parsed, symbol), nil
}

func (s *ExecutionSession) ResolveSpecSymbolRange(specRoot, file, symbol string) (SymbolRange, bool, error) {
	if err := s.begin(); err != nil {
		return SymbolRange{}, false, err
	}
	defer s.users.Done()
	path, err := filepath.Abs(filepath.Join(specRoot, filepath.FromSlash(file)))
	if err != nil {
		return SymbolRange{}, false, err
	}
	fileSet, parsed, err := s.parseSourceFile(path)
	if err != nil {
		return SymbolRange{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	range_, found := resolveSpecSymbolRange(fileSet, parsed, path, symbol)
	return range_, found, nil
}

func (s *ExecutionSession) ResolveSpecTest(specRoot, file, testName string, declaredRecorderImportPaths ...string) (SpecTestResult, error) {
	if err := s.begin(); err != nil {
		return SpecTestResult{}, err
	}
	defer s.users.Done()
	path := filepath.Join(specRoot, filepath.FromSlash(file))
	_, parsed, err := s.parseSourceFile(path)
	if err != nil {
		return SpecTestResult{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return resolveSpecTest(parsed, testName, declaredRecorderImportPaths...), nil
}

func (s *ExecutionSession) parseTestFile(path string) (parsedTestFile, error) {
	_, file, err := s.parseSourceFile(path)
	if err != nil {
		return parsedTestFile{}, err
	}
	return extractTestFile(file), nil
}

func executionSessionForGraph(g *ontology.Graph) (*ExecutionSession, bool) {
	if owner, ok := g.InvocationState.(interface{ ExecutionSession() *ExecutionSession }); ok {
		return owner.ExecutionSession(), false
	}
	return NewExecutionSession(), true
}

// SourceFile returns an invocation-owned, read-only AST and its exact file set.
func (s *ExecutionSession) SourceFile(path string) (*token.FileSet, *ast.File, error) {
	if err := s.begin(); err != nil {
		return nil, nil, err
	}
	defer s.users.Done()
	return s.parseSourceFile(path)
}
