package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// hashPackageInputs fingerprints actual contents, never publication dates,
// mtimes or file sizes. Streaming bounds memory even for large input fixtures;
// there is no process-lifetime copy of the source tree to become stale.
func hashPackageInputs(moduleRoot, pkgDir string) (string, error) {
	_ = pkgDir
	h := sha256.New()
	writeFile := func(relative string) error {
		file, err := os.Open(filepath.Join(moduleRoot, filepath.FromSlash(relative)))
		if err != nil {
			return err
		}
		defer file.Close()
		fileHash := sha256.New()
		if _, err := io.Copy(fileHash, file); err != nil {
			return err
		}
		h.Write([]byte(relative + "\x00"))
		h.Write(fileHash.Sum(nil))
		return nil
	}
	var paths []string
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != moduleRoot {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" {
					return filepath.SkipDir
				}
				relative, err := filepath.Rel(moduleRoot, path)
				if err != nil {
					return err
				}
				if !isAuthoredFixturePath(filepath.ToSlash(relative)) {
					boundary, err := unrelatedInputRoot(path)
					if err != nil {
						return err
					}
					if boundary {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}
		if !entry.Type().IsRegular() || isBuildOutputExtension(entry.Name()) {
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		if !isAuthoredFixturePath(filepath.ToSlash(rel)) && isPublishedOutput(filepath.ToSlash(rel), path, moduleRoot) {
			return nil
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, relative := range paths {
		if err := writeFile(relative); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func HashPackageInputs(moduleRoot, pkgDir string) (string, error) {
	return hashPackageInputs(moduleRoot, pkgDir)
}

var buildOutputExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".test":  true,
}

func isBuildOutputExtension(name string) bool {
	return buildOutputExtensions[strings.ToLower(filepath.Ext(name))]
}

func isAuthoredFixturePath(relative string) bool {
	for _, segment := range strings.Split(relative, "/") {
		if segment == "testdata" {
			return true
		}
	}
	return false
}

// A nested module/worktree is a separate owner, not this package's source.
func unrelatedInputRoot(directory string) (bool, error) {
	for _, marker := range []string{"go.mod", ".git"} {
		if _, err := os.Stat(filepath.Join(directory, marker)); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func isPublishedOutput(relative, path, root string) bool {
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		suffix := strings.Join(segments[index:], "/")
		if segment == "docs" && docbundle.OwnedDomainPath(suffix) {
			return true
		}
		if segment == "framework" && docbundle.OwnedProjectPath(suffix) {
			return true
		}
	}
	name := filepath.Base(path)
	crystal := name == "CLAUDE.md" || name == "AGENTS.md" || name == "GEMINI.md" || docbundle.OwnedCrystalPath(name)
	if !crystal {
		return false
	}
	directory := filepath.Dir(path)
	if directory == root {
		return true
	}
	_, err := os.Stat(filepath.Join(directory, paths.MarkerFilename))
	return err == nil
}

// A domain can live beneath another Go module while its executable spec owns a
// nested module. Fingerprint the actual declared owners, never the enclosing
// checkout merely because walking up from DomainDir happens to find its go.mod.
func hashExecutionInputs(graph *ontology.Graph) (string, error) {
	root := SpecRootForGraph(graph)
	owners := make(map[string]bool)
	addOwner := func(directory string) {
		if module, ok := ModuleRoot(directory); ok {
			owners[filepath.Clean(module)] = true
		}
	}
	specModule := filepath.Join(root, "spec")
	if len(graph.SelfExecutingAtomPackages) == 0 {
		if _, err := os.Stat(filepath.Join(specModule, "go.mod")); err == nil {
			owners[specModule] = true
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if len(owners) == 0 {
		addOwner(root)
	}
	for _, requirement := range graph.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, references := range [][]string{specTestReferences(requirement), requirement.ImplementedBy} {
			for _, reference := range references {
				if file, _, ok := ParseFileColonSymbol(strings.TrimSpace(reference)); ok {
					addOwner(filepath.Dir(filepath.Join(root, filepath.FromSlash(file))))
				}
			}
		}
	}
	if len(owners) == 0 {
		owners[root] = true
	}
	ordered := make([]string, 0, len(owners))
	for owner := range owners {
		ordered = append(ordered, owner)
	}
	sort.Strings(ordered)
	digest := sha256.New()
	for _, owner := range ordered {
		hash, err := hashPackageInputs(owner, owner)
		if err != nil {
			return "", err
		}
		digest.Write([]byte(owner + "\x00" + hash + "\n"))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// ExecutionInputsFingerprint identifies authored inputs, excluding publications.
func ExecutionInputsFingerprint(graph *ontology.Graph) (string, error) {
	return hashExecutionInputs(graph)
}
