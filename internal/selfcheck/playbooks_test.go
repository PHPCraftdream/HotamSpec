package selfcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// bandPlaybookRE matches the band-specific playbook naming convention
// P<band>-<NAME>.md (e.g. P4-OPEN-ITEM.md). It deliberately excludes generic
// docs like README.md so the check asserts a BAND-SPECIFIC playbook exists, not
// just any markdown file under docs/playbooks/.
var bandPlaybookRE = regexp.MustCompile(`^P\d+-.+\.md$`)

// TestActiveLoopPlaybook_AtLeastOneBandPlaybook enforces
// R-active-loop-playbook-doc: at least one band-specific playbook shall exist
// under docs/playbooks/ describing the agent's role for that band.
//
// EXACT RULE (mechanically checked): docs/playbooks/ is readable and contains at
// least one NON-EMPTY .md file whose name matches the band-playbook convention
// P<digit>-<name>.md. It fails the moment the last band playbook is deleted or
// emptied — the active-loop documentation substrate would then be gone.
func TestActiveLoopPlaybook_AtLeastOneBandPlaybook(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(repoRoot(t), "docs", "playbooks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("R-active-loop-playbook-doc: docs/playbooks/ not readable: %v", err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() || !bandPlaybookRE.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			continue
		}
		found = append(found, e.Name())
	}
	if len(found) == 0 {
		t.Errorf("R-active-loop-playbook-doc: no non-empty band playbook (P<digit>-*.md) under docs/playbooks/")
	}
}
