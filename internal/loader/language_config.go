package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var languageTagPattern = regexp.MustCompile(`^[a-z]{2,8}(?:-[a-z0-9]{1,8})*$`)

// validateDomainManifest enforces the language catalog/default contract and
// structural conformance declaration validity before manifest data is used.
func validateDomainManifest(m *DomainManifest) error {
	if m == nil {
		return fmt.Errorf("nil manifest")
	}
	if err := validateLanguages(m.Languages, m.DefaultLanguage); err != nil {
		return err
	}
	issues := ontology.ValidateConformance(&ontology.Graph{
		Conformance:          m.Conformance,
		SpecificationSources: m.SpecificationSources,
	})
	if len(issues) > 0 {
		parts := make([]string, len(issues))
		for i, issue := range issues {
			parts[i] = fmt.Sprintf("%s: %s", issue.ID, issue.Message)
		}
		return fmt.Errorf("invalid manifest conformance: %s", strings.Join(parts, "; "))
	}
	return nil
}
func validateAtomicManifestFieldNames(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("manifest must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return nil
	}
	for key := range fields {
		for _, expected := range []string{"languages", "default_language", "conformance"} {
			if strings.EqualFold(key, expected) && key != expected {
				return fmt.Errorf("manifest field %q must use the exact snake_case name %q", key, expected)
			}
		}
	}
	for _, key := range []string{"languages", "default_language", "conformance"} {
		if raw, exists := fields[key]; exists && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("manifest field %q must not be null", key)
		}
	}
	if raw, exists := fields["default_language"]; exists {
		var value string
		if json.Unmarshal(raw, &value) == nil && value == "" {
			return fmt.Errorf("default_language must not be empty when declared")
		}
	}
	return nil
}

func validateLanguages(languages []string, defaultLanguage string) error {
	if languages == nil {
		if defaultLanguage != "" {
			return fmt.Errorf("default_language requires an explicit non-empty languages list")
		}
		return nil
	}
	if len(languages) == 0 {
		return fmt.Errorf("languages must contain at least one supported language")
	}
	seen := make(map[string]struct{}, len(languages))
	for i, language := range languages {
		if !languageTagPattern.MatchString(language) || strings.Contains(language, "..") || strings.ContainsAny(language, `/\\`) {
			return fmt.Errorf("languages[%d] %q is not a safe lowercase language tag", i, language)
		}
		if !localization.Supported(language) {
			return fmt.Errorf("languages[%d] %q has no complete localization catalog", i, language)
		}
		if _, exists := seen[language]; exists {
			return fmt.Errorf("languages[%d] duplicates language %q", i, language)
		}
		seen[language] = struct{}{}
	}
	if defaultLanguage != "" {
		if !languageTagPattern.MatchString(defaultLanguage) || strings.Contains(defaultLanguage, "..") || strings.ContainsAny(defaultLanguage, `/\\`) {
			return fmt.Errorf("default_language %q is not a safe lowercase language tag", defaultLanguage)
		}
		if !localization.Supported(defaultLanguage) {
			return fmt.Errorf("default_language %q has no complete localization catalog", defaultLanguage)
		}
		if _, declared := seen[defaultLanguage]; !declared {
			return fmt.Errorf("default_language %q is not listed in languages", defaultLanguage)
		}
	}
	if len(languages) > 1 && defaultLanguage == "" {
		return fmt.Errorf("default_language is required when multiple languages are declared")
	}
	return nil
}
