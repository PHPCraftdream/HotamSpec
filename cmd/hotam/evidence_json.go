package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Legacy automatic dumps are removable only when their report schema is present.
func removeGeneratedEvidenceJSON(genDir string) error {
	path := filepath.Join(genDir, "evidence.json")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var header struct {
		SchemaVersion int       `json:"schema_version"`
		Conformance   *struct{} `json:"conformance"`
	}
	decoder := json.NewDecoder(file)
	decodeErr := decoder.Decode(&header)
	var trailing any
	if decodeErr == nil {
		decodeErr = decoder.Decode(&trailing)
		if decodeErr == io.EOF {
			decodeErr = nil
		} else if decodeErr == nil {
			decodeErr = fmt.Errorf("multiple JSON documents")
		}
	}
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	if decodeErr != nil || header.SchemaVersion != 2 || header.Conformance == nil {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove legacy automatic evidence dump: %w", err)
	}
	return nil
}
