package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// hashDirContent walks a single directory recursively and returns a sha256
// content-hash over every regular file found (rel-path + bytes, sorted), in
// the SAME algorithm philosophy hashPackageInputs (test_exec_inputs.go) already
// established — but scoped to ONE directory, not the whole module. This is a
// deliberately separate, scope-limited reimplementation: hashPackageInputs
// ignores its pkgDir parameter and hashes the entire module tree (it is
// load-bearing for runCache/coverageRunCache invalidation and MUST NOT be
// changed), so a per-package hash needs its own small function.
//
// Build-output extensions (.exe/.dll/.so/.dylib/.test) are excluded via the
// existing isBuildOutputExtension predicate. Dot-prefixed subdirectories are
// skipped. Non-regular files are skipped.
//
// A non-existent directory (os.IsNotExist) returns ("", nil) — an empty
// contribution with no error, so callers can combine per-package results
// without treating a missing package as a hard failure.
func hashDirContent(dir string) (string, error) {
	var relPaths []string
	fileData := map[string][]byte{}
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if isBuildOutputExtension(d.Name()) {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relPaths = append(relPaths, relSlash)
		fileData[relSlash] = data
		return nil
	})
	if walkErr != nil {
		if os.IsNotExist(walkErr) {
			return "", nil
		}
		return "", walkErr
	}

	sort.Strings(relPaths)
	h := sha256.New()
	for _, rel := range relPaths {
		h.Write([]byte(rel + "\n"))
		h.Write(fileData[rel])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// EngineDocsFingerprint computes a deterministic content-hash fingerprint of
// the three generator-relevant engine packages — internal/generator (the
// rendering logic), internal/ontology (the schema/types every renderer walks),
// and internal/loader (manifest/graph-loading resolution) — whose changes
// would actually affect a domain's generated-doc SHAPE or CONTENT.
//
// The fingerprint is a PURE function of the checked-out source tree at the
// moment the engine binary is running: two consecutive calls against the same
// unchanged tree produce the identical fingerprint, every time. It does NOT
// depend on map iteration order or any other non-deterministic source (the
// three per-package hashes are sorted lexicographically before combination, so
// the final digest is the same regardless of which package is hashed first).
//
// It is deliberately NOT tied to the repo's git commit SHA (which would
// invalidate on EVERY commit, including commits that touch nothing about doc
// generation — far too noisy) nor to a manually-bumped version number (which
// this project does not practice for every doc-shape-relevant change, and
// which defaults to "dev"/"unknown" for local builds that never set ldflags).
// Instead it is a content hash over exactly the packages whose source changes
// can flip what gen-spec produces.
//
// This function does NOT share code with hashPackageInputs (which ignores
// pkgDir and hashes the whole module for runCache/coverageRunCache cache
// invalidation): it is a scope-limited reimplementation of the same algorithm
// philosophy, not a fork of business logic.
//
// If none of the three package directories exist under moduleRoot (e.g. a
// consumer repo that has the engine as a compiled binary without source),
// EngineDocsFingerprint returns a non-nil error so the caller can degrade to
// an honest no-op rather than stamping a meaningless fingerprint.
func EngineDocsFingerprint(moduleRoot string) (string, error) {
	packages := []string{
		filepath.FromSlash("internal/generator"),
		filepath.FromSlash("internal/ontology"),
		filepath.FromSlash("internal/loader"),
	}
	var digests []string
	for _, pkg := range packages {
		d, err := hashDirContent(filepath.Join(moduleRoot, pkg))
		if err != nil {
			return "", fmt.Errorf("EngineDocsFingerprint: hashing %s: %w", pkg, err)
		}
		digests = append(digests, d)
	}
	// If every package directory was missing, there is nothing to fingerprint.
	allEmpty := true
	for _, d := range digests {
		if d != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return "", fmt.Errorf("EngineDocsFingerprint: none of the three engine packages (internal/generator, internal/ontology, internal/loader) were found under %s", moduleRoot)
	}

	sort.Strings(digests)
	combined := strings.Join(digests, "\n")
	h := sha256.New()
	h.Write([]byte(combined))
	return hex.EncodeToString(h.Sum(nil)), nil
}
