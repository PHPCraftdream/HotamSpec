package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestReadPreservesRawBytesAndChecksFixtureIdentity(t *testing.T) {
	root := t.TempDir()
	payload := []byte{0xff, 0x00, '\r', '\n', 0x80}
	path := filepath.Join(root, "fixtures", "bytes.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	ref := ontology.FixtureRef{ID: "case-invalid-bytes", Path: "fixtures/bytes.bin", Role: "input", SHA256: hex.EncodeToString(digest[:]), RawBytes: true}
	got, err := Read(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Read changed raw payload bytes: got %v, want %v", got, payload)
	}

	ref.SHA256 = strings.Repeat("0", sha256.Size*2)
	if _, err := Read(root, ref); err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("hash mismatch error = %v, want a content-integrity error", err)
	}
}

func TestReadRejectsInvalidRoleAndRelativePathEscape(t *testing.T) {
	root := t.TempDir()
	ref := ontology.FixtureRef{ID: "case", Path: "fixture.dat", Role: "input", SHA256: strings.Repeat("0", sha256.Size*2)}
	if _, err := Read(root, ref); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing fixture error = %v, want missing-file diagnosis", err)
	}
	ref.Path = "../outside.dat"
	if _, err := Read(root, ref); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("path escape error = %v, want root-confinement diagnosis", err)
	}
	ref.Path = "fixture.dat"
	ref.Role = "ktav-invalid"
	if _, err := Read(root, ref); err == nil || !strings.Contains(err.Error(), "unsupported role") {
		t.Fatalf("unknown role error = %v, want generic role validation", err)
	}
}
