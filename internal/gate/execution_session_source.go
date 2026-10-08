package gate

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// SourceIndexForGraph acquires the source phase without starting tests. Source
// validators and the later execution snapshot share this exact AST ownership.
func (s *ExecutionSession) SourceIndexForGraph(g *ontology.Graph) (*AtomSourceIndex, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.users.Done()
	if g == nil {
		return nil, errors.New("source snapshot graph is nil")
	}
	hash, err := hashExecutionInputs(g)
	if err != nil {
		return nil, fmt.Errorf("hash source snapshot inputs: %w", err)
	}
	return s.sourceIndexForGraph(g, hash)
}

func (s *ExecutionSession) sourceIndexForGraph(g *ontology.Graph, contentHash string) (*AtomSourceIndex, error) {
	configuration, err := json.Marshal(struct {
		Root               string
		Languages          []string
		DefaultLanguage    string
		Packages           []string
		RecorderImportPath string
		RuleCases          bool
	}{SpecRootForGraph(g), g.Languages, g.DefaultLanguage, g.SelfExecutingAtomPackages, g.AtomRecorderImportPath, g.Conformance != nil && g.Conformance.RuleCases})
	if err != nil {
		return nil, err
	}
	key := contentHash + string(configuration)
	s.sourceMu.Lock()
	defer s.sourceMu.Unlock()
	if s.sourceKey == key {
		return s.sourceIndex, s.sourceErr
	}
	s.sourceIndex, s.sourceErr = newAtomSourceIndex(SpecRootForGraph(g), atomSourceOptions{
		languages: g.Languages, defaultLanguage: g.DefaultLanguage,
		ruleCases: g.Conformance != nil && g.Conformance.RuleCases,
		packages:  g.SelfExecutingAtomPackages, recorderImportPath: g.AtomRecorderImportPath,
		parseSource: s.parseSourceFile,
	})
	s.sourceKey = key
	return s.sourceIndex, s.sourceErr
}
