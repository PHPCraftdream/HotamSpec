package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveGeneratedEvidenceJSONPreservesAuthoredAndUnknownFiles(t *testing.T) {
	for _, data := range []string{
		`{"notes":"authored"}`,
		`{"schema_version":99,"conformance":{},"notes":"unknown schema"}`,
		`{"schema_version":2,"conformance":null,"notes":"not a report"}`,
		`{"schema_version":2,"conformance":{}} {"notes":"second document"}`,
	} {
		t.Run(data, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "evidence.json")
			if err := os.WriteFile(path, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := removeGeneratedEvidenceJSON(dir); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Fatalf("non-owned packet was removed or changed: data=%q err=%v", got, err)
			}
		})
	}
}

func TestRemoveGeneratedEvidenceJSONRemovesOwnedLegacyPacket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":2,"requirements":[],"findings":[],"sources":[],"conformance":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := removeGeneratedEvidenceJSON(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy automatic packet remains: %v", err)
	}
}
