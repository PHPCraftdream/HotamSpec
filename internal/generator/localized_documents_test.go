package generator

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/docbundle"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestLocalizedBundleSelectsMatchingViewPathsAndPreservesAuthoredData(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	g.Languages = []string{"ru", "en"}
	g.DefaultLanguage = "ru"
	g.RenderLanguage = "caller-view"
	g.SelfExecutingAtoms = true
	g.Requirements[0].ImplementedBy = []string{"spec/model/person.go:Person"}
	claimRU := "Preserve exact SDK output: A/B; code `R-X`"
	claimEN := "Preserve exact SDK output: English; code `R-X`"
	for i := range g.Requirements {
		if g.Requirements[i].ClaimTexts == nil {
			g.Requirements[i].ClaimTexts = make(ontology.LocalizedText)
		}
		g.Requirements[i].ClaimTexts["ru"] = g.Requirements[i].Claim
		g.Requirements[i].ClaimTexts["en"] = g.Requirements[i].Claim
	}
	g.Requirements[0].Claim = claimRU
	g.Requirements[0].ClaimTexts["ru"] = claimRU
	g.Requirements[0].ClaimTexts["en"] = claimEN

	layout, err := docbundle.NewLayout(g.Languages, g.DefaultLanguage)
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	specDocs := make(map[string]string)
	for _, language := range g.Languages {
		indexPath, err := layout.SpecIndexPath(language)
		if err != nil {
			t.Fatalf("SpecIndexPath(%s): %v", language, err)
		}
		shardPath, err := layout.SpecShardPath(language, gate.SpecPackage(g.Requirements[0])+".md")
		if err != nil {
			t.Fatalf("SpecShardPath(%s): %v", language, err)
		}
		for path, content := range map[string]string{
			indexPath: "# SPEC index " + language + "\n",
			shardPath: "# Person " + language + "\n",
		} {
			relative, err := filepath.Rel(filepath.FromSlash("docs/gen"), filepath.FromSlash(path))
			if err != nil {
				t.Fatalf("SPEC path %q is outside docs/gen: %v", path, err)
			}
			specDocs[filepath.ToSlash(relative)] = content
		}
	}
	docs, err := BuildLocalizedDocumentsWithSnapshot(g, "fixture-domain", t.TempDir(), "2026-10-04", nil, specDocs)
	if err != nil {
		t.Fatalf("BuildLocalizedDocumentsWithSnapshot: %v", err)
	}

	requirementsRU, ok := docs["docs/gen/REQUIREMENTS.ru.md"]
	if !ok {
		t.Fatal("localized bundle omitted Russian REQUIREMENTS projection")
	}
	requirementsEN, ok := docs["docs/gen/REQUIREMENTS.en.md"]
	if !ok {
		t.Fatal("localized bundle omitted English REQUIREMENTS projection")
	}
	for language, content := range map[string]string{"ru": requirementsRU, "en": requirementsEN} {
		want := claimRU
		other := claimEN
		if language == "en" {
			want, other = claimEN, claimRU
		}
		if !strings.Contains(content, want) {
			t.Errorf("%s view omitted its authored claim text: %q", language, want)
		}
		if strings.Contains(content, other) {
			t.Errorf("%s view included another language's authored claim text: %q", language, other)
		}
	}

	repoMapRU, ok := docs["docs/gen/REPO-MAP.ru.md"]
	if !ok {
		t.Fatal("localized bundle omitted Russian repository map")
	}
	repoMapEN, ok := docs["docs/gen/REPO-MAP.en.md"]
	if !ok {
		t.Fatal("localized bundle omitted English repository map")
	}
	for language, content := range map[string]string{"ru": repoMapRU, "en": repoMapEN} {
		indexPath, err := layout.SpecIndexPath(language)
		if err != nil {
			t.Fatalf("SpecIndexPath(%s): %v", language, err)
		}
		shardPath, err := layout.SpecShardPath(language, gate.SpecPackage(g.Requirements[0])+".md")
		if err != nil {
			t.Fatalf("SpecShardPath(%s): %v", language, err)
		}
		for _, want := range []string{"domains/fixture-domain/" + filepath.ToSlash(indexPath), "domains/fixture-domain/" + filepath.ToSlash(shardPath)} {
			if !strings.Contains(content, want) {
				t.Errorf("%s repository map does not point to its own SPEC view %q", language, want)
			}
		}
	}
	indexEN, _ := layout.SpecIndexPath("en")
	indexRU, _ := layout.SpecIndexPath("ru")
	if strings.Contains(repoMapRU, "domains/fixture-domain/"+filepath.ToSlash(indexEN)) || strings.Contains(repoMapEN, "domains/fixture-domain/"+filepath.ToSlash(indexRU)) {
		t.Fatal("a repository map linked into a different language's SPEC family")
	}
	if g.RenderLanguage != "caller-view" {
		t.Fatalf("localized rendering mutated the shared graph locale: %q", g.RenderLanguage)
	}
}

func TestLocalizedBundleRefusesUnsupportedLanguageBeforeReturningDocs(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	g.Languages = []string{"fr"}
	g.DefaultLanguage = "fr"
	_, err := BuildLocalizedDocumentsWithSnapshot(g, "fixture-domain", t.TempDir(), "2026-10-04", nil, nil)
	var missing *localization.MissingTranslation
	if !errors.As(err, &missing) || missing.Language != "fr" {
		t.Fatalf("localized bundle error = %#v, want typed missing-catalog refusal", err)
	}
}

func TestLocalizedSingleLanguageWithoutDefaultKeepsPlainClaims(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	g.Languages = []string{"ru"}
	g.DefaultLanguage = ""
	for i := range g.Requirements {
		g.Requirements[i].ClaimTexts = nil
	}
	claim := "простая одноязычная норма без таблицы переводов"
	g.Requirements[0].Claim = claim
	docs, err := BuildLocalizedDocumentsWithSnapshot(g, "fixture-domain", t.TempDir(), "2026-10-04", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docs["docs/gen/REQUIREMENTS.md"], claim) {
		t.Fatal("single-language bundle lost its primary plain claim")
	}
	if _, exists := docs["docs/gen/REQUIREMENTS.ru.md"]; exists {
		t.Fatal("single-language bundle unexpectedly switched to suffixed layout")
	}
}

func TestLocalizedBundleRefusesEmptySecondaryClaim(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	g.Languages = []string{"en", "ru"}
	g.DefaultLanguage = "en"
	for i := range g.Requirements {
		g.Requirements[i].ClaimTexts = ontology.LocalizedText{"en": g.Requirements[i].Claim, "ru": " "}
	}
	docs, err := BuildLocalizedDocumentsWithSnapshot(g, "fixture-domain", t.TempDir(), "2026-10-04", nil, nil)
	var missing *localization.MissingTranslation
	if !errors.As(err, &missing) || missing.Language != "ru" || docs != nil {
		t.Fatalf("empty secondary claim did not refuse the entire bundle: docs=%v err=%v", docs, err)
	}
}
