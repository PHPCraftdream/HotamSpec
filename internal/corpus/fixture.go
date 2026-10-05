// Package corpus reads explicitly referenced fixture bytes without interpreting
// consumer-specific categories or formats.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var fixtureRoles = map[string]struct{}{
	"input": {}, "expected": {}, "canonical": {}, "error": {}, "extra": {},
}

// Read loads the exact bytes named by ref, checking its role, path and SHA-256
// before returning them. Relative paths are confined to root, including when
// symlinks are present. Explicit absolute paths are intentionally permitted.
func Read(root string, ref ontology.FixtureRef) ([]byte, error) {
	if strings.TrimSpace(ref.ID) == "" {
		return nil, fmt.Errorf("fixture ID is empty")
	}
	if _, ok := fixtureRoles[ref.Role]; !ok {
		return nil, fmt.Errorf("fixture %q has unsupported role %q", ref.ID, ref.Role)
	}
	if strings.TrimSpace(ref.Path) == "" {
		return nil, fmt.Errorf("fixture %q path is empty", ref.ID)
	}
	expected, err := hex.DecodeString(ref.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return nil, fmt.Errorf("fixture %q sha256 must be exactly 64 hexadecimal characters", ref.ID)
	}

	path := filepath.Clean(ref.Path)
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(root) == "" {
			return nil, fmt.Errorf("fixture %q has a relative path but no corpus root", ref.ID)
		}
		rootPath, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("fixture %q resolve root: %w", ref.ID, err)
		}
		rootPath, err = filepath.EvalSymlinks(rootPath)
		if err != nil {
			return nil, fmt.Errorf("fixture %q resolve root: %w", ref.ID, err)
		}
		path = filepath.Join(rootPath, path)
		relative, err := filepath.Rel(rootPath, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("fixture %q path escapes corpus root", ref.ID)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fixtureReadError(ref.ID, err)
		}
		relative, err = filepath.Rel(rootPath, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("fixture %q path resolves outside corpus root", ref.ID)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fixtureReadError(ref.ID, err)
	}
	actual := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(actual[:]), ref.SHA256) {
		return nil, fmt.Errorf("fixture %q bytes do not match declared sha256", ref.ID)
	}
	return data, nil
}

func fixtureReadError(id string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fixture %q does not exist", id)
	}
	return fmt.Errorf("fixture %q could not be read: %w", id, err)
}
