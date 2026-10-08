package docbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func (l Layout) EvidenceRequirementPath(language, requirementID string) (string, error) {
	return l.evidencePath(language, "requirements", requirementID)
}

func (l Layout) EvidenceCasePath(language, atomID, caseID string) (string, error) {
	return l.evidencePath(language, "cases", atomID+"\x00"+caseID)
}

func (l Layout) evidencePath(language, kind, identity string) (string, error) {
	if !l.hasLanguage(language) {
		return "", fmt.Errorf("language %q is not declared in the output layout", language)
	}
	if identity == "" {
		return "", fmt.Errorf("evidence identity is empty")
	}
	digest := sha256.Sum256([]byte(identity))
	prefix := "docs/gen/evidence/"
	if l.Multilingual() {
		prefix += language + "/"
	}
	return prefix + kind + "/" + hex.EncodeToString(digest[:]) + ".md", nil
}

// IsEvidenceShardPath recognizes only identity-addressed report pages.
func IsEvidenceShardPath(relative string) bool {
	relative = strings.TrimPrefix(relative, "docs/gen/")
	parts := strings.Split(relative, "/")
	if len(parts) != 3 && len(parts) != 4 || parts[0] != "evidence" {
		return false
	}
	if len(parts) == 4 && !knownLocale(parts[1]) {
		return false
	}
	kind := parts[len(parts)-2]
	if kind != "requirements" && kind != "cases" {
		return false
	}
	name := parts[len(parts)-1]
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	digest := strings.TrimSuffix(name, ".md")
	if len(digest) != 64 {
		return false
	}
	for _, character := range digest {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
