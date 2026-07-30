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

// lockData is the on-disk shape of graph.lock. SHA256/UpdatedAt/Note are the
// original graph.json content pin (R-no-hand-edit-graph). DisciplineFullObserved
// is the F2 discipline ratchet (task W7.2, @fx finding F2): once a domain's
// manifest.json has EVER been observed with discipline:"full", this field is
// pinned true and NEVER goes back to false -- a resolver who silently removes
// or downgrades the discipline key from manifest.json cannot un-pin it, so
// check_discipline_ratchet (internal/invariants) fires on the regression. This
// field is ADDITIVE (json:"discipline_full_observed,omitempty"): an existing
// graph.lock written before F2 lacks the field entirely and decodes with the
// Go zero value (false), preserving 100% backward compatibility with every
// graph.lock already on disk -- the same additive-field convention
// Requirement.blocked_on/implemented_by/verified_by already established for
// graph.json's own schema migrations.
//
// ClaimAuthorityScenarioObserved is the F2-shaped ratchet pin for
// claim_authority:"scenario" (task #388/W0.1), symmetric with
// DisciplineFullObserved above: once a domain's manifest.json has EVER been
// observed with claim_authority:"scenario", this field is pinned true and
// NEVER goes back to false, closing the identical silent-regression gap F2
// closed for discipline:"full" (see check_claim_authority_ratchet,
// internal/invariants/claim_authority_ratchet.go). ADDITIVE
// (json:"claim_authority_scenario_observed,omitempty"): an existing
// graph.lock written before this field existed lacks it entirely and decodes
// with the Go zero value (false) -- 100% backward compatible with every
// graph.lock already on disk, the same convention DisciplineFullObserved
// itself established.
type lockData struct {
	SHA256                         string `json:"sha256"`
	UpdatedAt                      string `json:"updated_at"`
	Note                           string `json:"note"`
	DisciplineFullObserved         bool   `json:"discipline_full_observed,omitempty"`
	ClaimAuthorityScenarioObserved bool   `json:"claim_authority_scenario_observed,omitempty"`
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

// WriteLock writes (or refreshes) graph.lock next to graphPath. It computes
// the sha256 of graph.json and writes it together with a timestamp and note.
//
// F2 DISCIPLINE RATCHET (task W7.2, @fx finding F2): WriteLock ALSO reads the
// manifest.json sitting next to graph.json (via ResolveDiscipline) and, if the
// live discipline is "full", pins DisciplineFullObserved=true in the lock. The
// ratchet is ONE-WAY: if the EXISTING lock already has DisciplineFullObserved=
// true, it stays true even if the live manifest's discipline is no longer "full"
// (the resolver removed or downgraded the key) -- exactly the one-way-door
// semantics PLAN-scenario-generated-spec.md §2 D4 specifies for discipline:full.
// This means a domain that was EVER discipline:full and then silently regressed
// will have lock.DisciplineFullObserved=true but live discipline!="" --
// check_discipline_ratchet (internal/invariants) detects that mismatch and
// fires a violation. A domain that has NEVER been discipline:full leaves
// DisciplineFullObserved at false (or absent in older locks), and the ratchet
// is a no-op for it.
func WriteLock(graphPath string, note string) error {
	hash, err := sha256File(graphPath)
	if err != nil {
		return fmt.Errorf("write lock: hash %s: %w", graphPath, err)
	}
	// F2 ratchet: preserve a previously-pinned discipline:full observation.
	// Read the existing lock (if any) to get the old DisciplineFullObserved /
	// ClaimAuthorityScenarioObserved.
	prevFullObserved := false
	prevClaimAuthorityObserved := false
	if prevLock, pErr := readLockData(graphPath); pErr == nil {
		prevFullObserved = prevLock.DisciplineFullObserved
		prevClaimAuthorityObserved = prevLock.ClaimAuthorityScenarioObserved
	}
	// Observe the live discipline. If it is "full" now, pin it; otherwise
	// preserve the previous pin (ratchet: once true, always true).
	liveFull := ResolveDiscipline(graphPath) == DisciplineFull
	// task #388/W0.1 ratchet: same one-way pin, for claim_authority:"scenario".
	liveClaimAuthorityScenario := resolveClaimAuthorityScenario(graphPath)
	lock := lockData{
		SHA256:                         hash,
		UpdatedAt:                      time.Now().UTC().Format(time.RFC3339),
		Note:                           note,
		DisciplineFullObserved:         prevFullObserved || liveFull,
		ClaimAuthorityScenarioObserved: prevClaimAuthorityObserved || liveClaimAuthorityScenario,
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

// readLockData reads and parses the graph.lock sitting next to graphPath.
// Returns an error if the lock does not exist or cannot be parsed -- callers
// that want to treat absence as "no pin yet" should check the error and fall
// back to the zero value (DisciplineFullObserved=false), which WriteLock's
// own ratchet logic already does.
func readLockData(graphPath string) (*lockData, error) {
	lockPath := LockPath(graphPath)
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}
	var lock lockData
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

// ReadDisciplinePin reads graph.lock next to graphPath and returns whether
// discipline:"full" was ever observed (the F2 ratchet pin). Returns
// (false, false) when the lock does not exist or cannot be parsed -- the
// caller (check_discipline_ratchet) treats absence as "no pin yet" (honest
// no-op, same shape as check_graph_lock_pins_graph_json's own absent-lock
// bail). Returns (pinValue, true) when the lock exists and was parsed.
func ReadDisciplinePin(graphPath string) (fullObserved bool, exists bool) {
	lock, err := readLockData(graphPath)
	if err != nil {
		return false, false
	}
	return lock.DisciplineFullObserved, true
}

// ReadClaimAuthorityPin reads graph.lock next to graphPath and returns
// whether claim_authority:"scenario" was ever observed (task #388/W0.1's
// ratchet pin), mirroring ReadDisciplinePin exactly. Returns (false, false)
// when the lock does not exist or cannot be parsed -- the caller
// (check_claim_authority_ratchet) treats absence as "no pin yet" (honest
// no-op, same shape as ReadDisciplinePin's own absent-lock bail). Returns
// (pinValue, true) when the lock exists and was parsed.
func ReadClaimAuthorityPin(graphPath string) (scenarioObserved bool, exists bool) {
	lock, err := readLockData(graphPath)
	if err != nil {
		return false, false
	}
	return lock.ClaimAuthorityScenarioObserved, true
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
