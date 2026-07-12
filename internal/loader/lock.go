package loader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type lockData struct {
	SHA256    string `json:"sha256"`
	UpdatedAt string `json:"updated_at"`
	Note      string `json:"note"`
}

func LockPath(graphPath string) string {
	return filepath.Join(filepath.Dir(graphPath), "graph.lock")
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func WriteLock(graphPath string, note string) error {
	hash, err := sha256File(graphPath)
	if err != nil {
		return fmt.Errorf("write lock: hash %s: %w", graphPath, err)
	}
	lock := lockData{
		SHA256:    hash,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Note:      note,
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("write lock: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteFile(LockPath(graphPath), data); err != nil {
		return fmt.Errorf("write lock: %s: %w", LockPath(graphPath), err)
	}
	return nil
}

func VerifyLock(graphPath string) (bool, error) {
	lockPath := LockPath(graphPath)
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("verify lock: %s absent (run WriteGraph or WriteLock to create it)", lockPath)
		}
		return false, fmt.Errorf("verify lock: read %s: %w", lockPath, err)
	}
	var lock lockData
	if err := json.Unmarshal(raw, &lock); err != nil {
		return false, fmt.Errorf("verify lock: parse %s: %w", lockPath, err)
	}
	if lock.SHA256 == "" {
		return false, fmt.Errorf("verify lock: %s: empty sha256", lockPath)
	}
	actual, err := sha256File(graphPath)
	if err != nil {
		return false, fmt.Errorf("verify lock: hash %s: %w", graphPath, err)
	}
	return actual == lock.SHA256, nil
}
