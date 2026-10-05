package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAtomValueFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["spec/go.mod"] = "module example.test/values\n\ngo 1.22\n"
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const sexValueModel = `package model

// Sex — пол.
type Sex string

const (
	// >>>>> lang=en
	// male
	//
	// >>>>> lang=ru
	// М
	constMale Sex = "М"
	// >>>>> lang=en
	// female
	//
	// >>>>> lang=ru
	// Ж
	constFemale Sex = "Ж"
)

type Person struct{}

// >>>>> lang=en
// sex of the person
//
// >>>>> lang=ru
// пол человека
func (p Person) Sex() Sex { return constMale }
`

func TestAtomValueTranslationsSubstituteInProjections(t *testing.T) {
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": sexValueModel})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	artifact := AtomArtifact{ReqID: "R-person-sex", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Sex", Value: "М"}}}
	claims, err := index.DeriveClaims(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Sex of the person — male." {
		t.Fatalf("English claim = %q", claims["en"])
	}
	if claims["ru"] != "Пол человека — М." {
		t.Fatalf("Russian claim = %q", claims["ru"])
	}
	artifact.Steps[0].Value = "Ж"
	claims, err = index.DeriveClaims(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Sex of the person — female." || claims["ru"] != "Пол человека — Ж." {
		t.Fatalf("female claims = %q / %q", claims["en"], claims["ru"])
	}
}

func TestAtomValueSameNamedTypesAcrossPackagesNotConfused(t *testing.T) {
	work := `package work

// Kind — срочность задачи.
type Kind string

const (
	// >>>>> lang=en
	// urgent
	//
	// >>>>> lang=ru
	// срочная
	constUrgent Kind = "срочный"
)

type Task struct{}

// >>>>> lang=en
// kind of the task
//
// >>>>> lang=ru
// срочность задачи
func (t Task) Kind() Kind { return constUrgent }
`
	hobby := `package hobby

// Kind — увлечённость человека.
type Kind string

const (
	// >>>>> lang=en
	// serious
	//
	// >>>>> lang=ru
	// серьёзное
	constSerious Kind = "срочный"
)

type Interest struct{}

// >>>>> lang=en
// kind of the interest
//
// >>>>> lang=ru
// увлечённость
func (i Interest) Kind() Kind { return constSerious }
`
	root := writeAtomValueFixture(t, map[string]string{
		"spec/model/work/work.go":   work,
		"spec/model/hobby/hobby.go": hobby,
	})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	task := AtomArtifact{ReqID: "R-task-kind", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "work.Task.Kind", Value: "срочный"}}}
	claims, err := index.DeriveClaims(task)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Kind of the task — urgent." || claims["ru"] != "Срочность задачи — срочная." {
		t.Fatalf("work claims = %q / %q", claims["en"], claims["ru"])
	}
	interest := AtomArtifact{ReqID: "R-interest-kind", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "hobby.Interest.Kind", Value: "срочный"}}}
	claims, err = index.DeriveClaims(interest)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Kind of the interest — serious." || claims["ru"] != "Увлечённость — серьёзное." {
		t.Fatalf("hobby claims = %q / %q", claims["en"], claims["ru"])
	}
}

func TestAtomValueCrossPackageSelectorReturnResolved(t *testing.T) {
	hobby := `package hobby

// Kind — увлечённость человека.
type Kind string

const (
	// >>>>> lang=en
	// serious
	//
	// >>>>> lang=ru
	// серьёзное
	constSerious Kind = "срочный"
)
`
	work := `package work

import "example.test/values/model/hobby"

type Task struct{}

// >>>>> lang=en
// kind of the task
//
// >>>>> lang=ru
// срочность задачи
func (t Task) Kind() hobby.Kind { return hobby.ConstSerious }
`
	root := writeAtomValueFixture(t, map[string]string{
		"spec/model/hobby/hobby.go": hobby,
		"spec/model/work/work.go":   work,
	})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	task := AtomArtifact{ReqID: "R-task-kind", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "work.Task.Kind", Value: "срочный"}}}
	claims, err := index.DeriveClaims(task)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Kind of the task — serious." || claims["ru"] != "Срочность задачи — серьёзное." {
		t.Fatalf("cross-package claims = %q / %q", claims["en"], claims["ru"])
	}
}

func TestAtomValueIncompleteLanguageCoverageRejected(t *testing.T) {
	source := `package model

type Sex string

// >>>>> lang=en
// male
const constMale Sex = "М"

type Person struct{}

// >>>>> lang=en
// sex
//
// >>>>> lang=ru
// пол
func (p Person) Sex() Sex { return constMale }
`
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
	_, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err == nil {
		t.Fatal("constant with incomplete language coverage accepted")
	}
	for _, part := range []string{"person.go:7:", "constMale", `language "ru"`, "missing language block"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("diagnostic %q missing %q", err, part)
		}
	}
}

func TestAtomValueUnknownAndDuplicateConstantBlocksRejected(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		parts []string
	}{
		{"unknown", "// >>>>> lang=en\n// male\n// >>>>> lang=fr\n// français\n", []string{"constMale", `language "fr"`, "not declared"}},
		{"duplicate", "// >>>>> lang=en\n// male\n// >>>>> lang=en\n// male again\n// >>>>> lang=ru\n// М\n", []string{"constMale", "duplicate language block"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "package model\n\ntype Sex string\n\n" + tc.doc + "const constMale Sex = \"М\"\n"
			root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
			_, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
			if err == nil {
				t.Fatal("invalid constant language blocks accepted")
			}
			for _, part := range tc.parts {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("diagnostic %q missing %q", err, part)
				}
			}
		})
	}
}

func TestAtomValueUntranslatedStringConstantRejectedInMultilingualDomain(t *testing.T) {
	source := `package model

type Sex string

const constMale Sex = "М"

type Person struct{}

// >>>>> lang=en
// sex of the person
//
// >>>>> lang=ru
// пол человека
func (p Person) Sex() Sex { return constMale }
`
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	artifact := AtomArtifact{ReqID: "R-person-sex", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Sex", Value: "М"}}}
	_, err = index.DeriveClaims(artifact)
	if err == nil || !strings.Contains(err.Error(), "string constant value without translation in a multilingual domain") || !strings.Contains(err.Error(), "person.go:5: constant constMale") {
		t.Fatalf("untranslated matched string constant should reject, got %v", err)
	}
}

func TestAtomValueVerbatimMarkerAccepted(t *testing.T) {
	source := `package model

type Sex string

// >>>>> lang=*
const constMale Sex = "М"

type Person struct{}

// >>>>> lang=en
// sex of the person
//
// >>>>> lang=ru
// пол человека
func (p Person) Sex() Sex { return constMale }
`
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	artifact := AtomArtifact{ReqID: "R-person-sex", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Sex", Value: "М"}}}
	claims, err := index.DeriveClaims(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Sex of the person — М." || claims["ru"] != "Пол человека — М." {
		t.Fatalf("verbatim claims = %q / %q", claims["en"], claims["ru"])
	}
}

func TestAtomValueMonolingualDomainUnchanged(t *testing.T) {
	source := `package model

type Sex string

// >>>>> lang=en
// male
const constMale Sex = "М"

type Person struct{}

// plain single-language phrase
func (p Person) Sex() Sex { return constMale }
`
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
	index, err := NewAtomSourceIndex(root)
	if err == nil || !strings.Contains(err.Error(), "language markers require a multilingual languages configuration") {
		t.Fatalf("constant language markers must require a multilingual configuration, got %v", err)
	}
	plain := `package model

type Sex string

const constMale Sex = "М"

type Person struct{}

// пол человека
func (p Person) Sex() Sex { return constMale }
`
	root = writeAtomValueFixture(t, map[string]string{"spec/model/person.go": plain})
	index, err = NewAtomSourceIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	artifact := AtomArtifact{ReqID: "R-person-sex", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Sex", Value: "М"}}}
	claim, err := index.DeriveClaim(artifact)
	if err != nil || claim != "Пол человека — М." {
		t.Fatalf("monolingual claim = %q, %v", claim, err)
	}
}

func TestAtomValueNumbersAndBoolSemanticsUnchanged(t *testing.T) {
	source := `package model

type Person struct{ adult bool }

// >>>>> lang=en
// age of the person
//
// >>>>> lang=ru
// возраст человека
func (p Person) Age() int { return 7 }

// >>>>> lang=en
// person is adult
// not: person is minor
//
// >>>>> lang=ru
// человек совершеннолетний
// not: человек несовершеннолетний
func (p Person) Adult() bool { return p.adult }
`
	root := writeAtomValueFixture(t, map[string]string{"spec/model/person.go": source})
	index, err := newAtomSourceIndex(root, []string{"en", "ru"}, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	age := AtomArtifact{ReqID: "R-person-age", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Age", Value: "7"}}}
	claims, err := index.DeriveClaims(age)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Age of the person — 7." || claims["ru"] != "Возраст человека — 7." {
		t.Fatalf("numeric claims = %q / %q", claims["en"], claims["ru"])
	}
	adult := AtomArtifact{ReqID: "R-person-adult", Mode: "fact", Verdict: "pass", Steps: []AtomStep{{Subject: "model.Person.Adult", Value: "false"}}}
	claims, err = index.DeriveClaims(adult)
	if err != nil {
		t.Fatal(err)
	}
	if claims["en"] != "Person is minor." || claims["ru"] != "Человек несовершеннолетний." {
		t.Fatalf("bool claims = %q / %q", claims["en"], claims["ru"])
	}
}
