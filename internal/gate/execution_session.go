package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// ExecutionSession owns source snapshots, compiled artifacts and observed results
// for one invocation. Close rejects new work, waits for existing users, then
// releases only this owner's paths. No verdict is persisted between sessions.
type ExecutionSession struct {
	mu                    sync.Mutex
	users                 sync.WaitGroup
	closing               bool
	closeMu               sync.Mutex
	paths                 map[string]struct{}
	compileTmpDir         string
	compileCache          sync.Map
	compileSingleflightMu sync.Mutex
	compileInFlight       map[compileCacheKey]*compileInFlightCall
	compileModuleHashMu   sync.Mutex
	compileModuleHash     map[string]string
	runCache              sync.Map
	inFlightMu            sync.Mutex
	inFlightCalls         map[cacheKey]*inFlightCall
	snapshotMu            sync.Mutex
	snapshot              *AtomExecutionSnapshot
	snapshotKey           string
	sourceMu              sync.Mutex
	sourceKey             string
	sourceIndex           *AtomSourceIndex
	sourceErr             error
	astMu                 sync.Mutex
	astFiles              map[string]parsedSourceFile
}

var executionSessions sync.Map // live owners, including owners with failed Close

func NewExecutionSession() *ExecutionSession {
	s := &ExecutionSession{
		paths:             make(map[string]struct{}),
		astFiles:          make(map[string]parsedSourceFile),
		compileInFlight:   make(map[compileCacheKey]*compileInFlightCall),
		compileModuleHash: make(map[string]string),
		inFlightCalls:     make(map[cacheKey]*inFlightCall),
	}
	executionSessions.Store(s, struct{}{})
	return s
}

func (s *ExecutionSession) begin() error {
	if s == nil {
		return errors.New("execution session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return errors.New("execution session is closed")
	}
	s.users.Add(1)
	return nil
}

func (s *ExecutionSession) makeTempDir(prefix string) (string, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.paths[dir] = struct{}{}
	s.mu.Unlock()
	return dir, nil
}

func (s *ExecutionSession) removeOwnedPath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove execution-session artifact directory %q: %w", path, err)
	}
	s.mu.Lock()
	delete(s.paths, path)
	s.mu.Unlock()
	return nil
}

// Close is idempotent. Failed deletions retain ownership and are retried on a
// later Close; errors identify each retained path, never another owner's files.
func (s *ExecutionSession) Close() error {
	if s == nil {
		return nil
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	s.users.Wait()
	s.compileCache.Clear()
	s.runCache.Clear()
	s.snapshot = nil
	s.snapshotKey = ""
	s.sourceIndex = nil
	s.sourceKey = ""
	s.sourceErr = nil
	s.astFiles = nil
	s.compileModuleHash = nil
	s.mu.Lock()
	paths := make([]string, 0, len(s.paths))
	for path := range s.paths {
		paths = append(paths, path)
	}
	s.mu.Unlock()
	sort.Strings(paths)
	var failures []error
	for _, path := range paths {
		if err := s.removeOwnedPath(path); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		s.compileTmpDir = ""
		executionSessions.Delete(s)
	}
	return errors.Join(failures...)
}

// CloseExecutionSessions is the process-owner backstop used after CLI dispatch
// and after m.Run. Explicit invocation owners normally Close their own session.
func CloseExecutionSessions() error {
	var failures []error
	executionSessions.Range(func(key, _ any) bool {
		if err := key.(*ExecutionSession).Close(); err != nil {
			failures = append(failures, err)
		}
		return true
	})
	return errors.Join(failures...)
}

// AtomExecutionSnapshot is an immutable projection input once collection returns.
// Rendering from this snapshot never executes code or rereads the AST.
type AtomExecutionSnapshot struct {
	PackageFiles map[string]string
	PackageRuns  map[string]RecordingResult
	SourceIndex  *AtomSourceIndex
	TestFiles    map[string]map[string]string
	SourceErr    error
	DiscoveryErr error
}

func CollectAtomExecutionSnapshot(g *ontology.Graph) (*AtomExecutionSnapshot, error) {
	return CollectAtomExecutionSnapshotForPackages(g, nil)
}

func CollectAtomExecutionSnapshotForPackages(g *ontology.Graph, packages []string) (snapshot *AtomExecutionSnapshot, err error) {
	s := NewExecutionSession()
	defer func() { err = errors.Join(err, s.Close()) }()
	return s.CollectAtomExecutionSnapshotForPackages(g, packages)
}

// CollectAtomExecutionSnapshotForPackages shares one source/run snapshot while
// content and execution profile remain unchanged, independently of wall-clock day.
func (s *ExecutionSession) CollectAtomExecutionSnapshotForPackages(g *ontology.Graph, packages []string) (*AtomExecutionSnapshot, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.users.Done()
	if g == nil {
		return nil, errors.New("atom execution snapshot graph is nil")
	}
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	root := SpecRootForGraph(g)
	hash, err := hashExecutionInputs(g)
	if err != nil {
		return nil, fmt.Errorf("hash execution snapshot inputs: %w", err)
	}
	profile := executionProfile()
	config, err := json.Marshal(struct {
		Graph              *ontology.Graph
		Conformance        *ontology.ConformanceConfig
		Languages          []string
		DefaultLanguage    string
		Packages           []string
		Profile            string
		Root               string
		SelfExecutingAtoms bool
		SelfHosting        bool
	}{g, g.Conformance, g.Languages, g.DefaultLanguage, packages, profile, root, g.SelfExecutingAtoms, g.SelfHosting})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append([]byte(hash), config...))
	key := hex.EncodeToString(digest[:])
	if s.snapshot != nil && s.snapshotKey == key {
		return s.snapshot, nil
	}
	snapshot := &AtomExecutionSnapshot{
		PackageFiles: make(map[string]string),
		PackageRuns:  make(map[string]RecordingResult),
		TestFiles:    make(map[string]map[string]string),
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, reference := range requirement.VerifiedBy {
			if file, _, ok := ParseFileColonSymbol(strings.TrimSpace(reference)); ok && snapshotFileInPackages(file, packages) {
				addSnapshotTestReference(snapshot, reference)
			}
		}
		for _, caseDef := range requirement.Cases {
			if file, _, ok := ParseFileColonSymbol(strings.TrimSpace(caseDef.Test)); ok && snapshotFileInPackages(file, packages) {
				addSnapshotTestReference(snapshot, caseDef.Test)
			}
		}
	}
	if g.SelfExecutingAtoms || (g.Conformance != nil && len(g.Conformance.DocumentSections) != 0) {
		snapshot.SourceIndex, snapshot.SourceErr = s.sourceIndexForGraph(g, hash)
		if snapshot.SourceErr == nil {
			for _, requirement := range g.Requirements {
				if requirement.Status == ontology.StatusREJECTED || (len(requirement.VerifiedBy) == 0 && len(requirement.Cases) == 0) || len(requirement.ImplementedBy) == 0 {
					continue
				}
				if snapshot.SourceErr = snapshot.SourceIndex.ValidatePhraseLanguages(requirement.ImplementedBy); snapshot.SourceErr != nil {
					break
				}
			}
		}
	}
	if g.SelfExecutingAtoms {
		snapshot.DiscoveryErr = discoverSnapshotAtomTests(root, g.SelfExecutingAtomPackages, snapshot)
	}
	if snapshot.SourceErr == nil && snapshot.DiscoveryErr == nil {
		coverageFiles := snapshotCoverageFiles(g)
		for _, dir := range sortedSnapshotDirs(snapshot.PackageFiles) {
			testPattern, admitted := snapshotTestPattern(snapshot.TestFiles[dir])
			if len(admitted) == 0 {
				snapshot.PackageRuns[dir] = RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("package %q has no admitted test functions", dir)}}
				continue
			}
			run := s.runAtomRecording(root, snapshot.PackageFiles[dir], testPattern, coverageFiles[dir], true)
			if run.Err == nil && !run.CompileFailed && !run.Skipped {
				for _, test := range admitted {
					if verdict := run.ForTest(test); verdict.Err != nil {
						run.Err = errors.Join(run.Err, verdict.Err)
						run.Passed = false
					}
				}
			}
			snapshot.PackageRuns[dir] = run
		}
	}
	currentHash, err := hashExecutionInputs(g)
	if err != nil {
		return nil, fmt.Errorf("confirm execution snapshot inputs: %w", err)
	}
	if currentHash != hash || executionProfile() != profile {
		return nil, errors.New("source or execution profile changed while collecting the execution snapshot")
	}
	s.snapshot, s.snapshotKey = snapshot, key
	return snapshot, nil
}

// executionProfile identifies compile/runtime-affecting ambient configuration.
// Publication/review dates deliberately do not enter artifact or verdict identity.
func executionProfile() string {
	environment := os.Environ()
	sort.Strings(environment)
	hash := sha256.New()
	for _, entry := range environment {
		hash.Write([]byte(entry))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *ExecutionSession) ensureCompileTmpDir() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.compileTmpDir != "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "hotam-compile-")
	if err != nil {
		return err
	}
	s.compileTmpDir = filepath.Clean(dir)
	s.paths[s.compileTmpDir] = struct{}{}
	return nil
}
