package blabla

import (
	"strings"
	"testing"
)

// The new mapping format: CLDR categories written by name or by numeric alias.

func mappingBla(t *testing.T) *BlaBla {
	t.Helper()
	dir := t.TempDir()
	path := writeYAML(t, dir, "mapping.yml", `lieta:
  lv:
    zero:  "%d lietu"
    one:   "%d lieta"
    other: "%d lietas"
  en:
    - "%d item"
    - "%d items"

numeric:
  lv:
    0: "%d lietu"
    1: "%d lieta"
    2: "%d lietas"

mixed:
  lv:
    0:     ZERO
    one:   ONE
    other: OTHER

partial:
  lv:
    one:   "%d lieta"
    other: "%d lietas"

shouty:
  lv:
    ZERO:  UPPER-ZERO
    One:   UPPER-ONE
    OTHER: UPPER-OTHER

welsh:
  cy:
    zero:  CY-ZERO
    one:   CY-ONE
    two:   CY-TWO
    few:   CY-FEW
    many:  CY-MANY
    other: CY-OTHER
`)
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return bla
}

// The roadmap's Latvian example, end to end.
func TestMappingLatvian(t *testing.T) {
	bla := mappingBla(t)

	cases := map[int]string{
		0:   "0 lietu",
		1:   "1 lieta",
		5:   "5 lietas",
		10:  "10 lietu",
		15:  "15 lietu",
		20:  "20 lietu",
		21:  "21 lieta",
		22:  "22 lietas",
		101: "101 lieta",
		111: "111 lietu",
	}
	for count, want := range cases {
		if got := bla.Get("lv", "lieta", count); got != want {
			t.Errorf("Get(lv, lieta, %d) = `%s`, want `%s`", count, got, want)
		}
	}
}

// A mapping and a sequence under one key, each on its own engine.
func TestMappingAndSequenceCoexist(t *testing.T) {
	bla := mappingBla(t)

	// lv is a mapping -> CLDR: 21 is `one`
	if got := bla.Get("lv", "lieta", 21); got != "21 lieta" {
		t.Errorf("lv mapping: got `%s`", got)
	}
	// en is a sequence -> legacy: 21 is plural
	if got := bla.Get("en", "lieta", 21); got != "21 items" {
		t.Errorf("en sequence: got `%s`", got)
	}
}

func TestMappingNumericAliases(t *testing.T) {
	bla := mappingBla(t)

	cases := map[int]string{0: "0 lietu", 1: "1 lieta", 5: "5 lietas"}
	for count, want := range cases {
		if got := bla.Get("lv", "numeric", count); got != want {
			t.Errorf("numeric alias, count %d = `%s`, want `%s`", count, got, want)
		}
	}
}

func TestMappingMixedAndUppercaseKeys(t *testing.T) {
	bla := mappingBla(t)

	if got := bla.Get("lv", "mixed", 10); got != "ZERO" {
		t.Errorf("mixed numeric+named: got `%s`", got)
	}
	if got := bla.Get("lv", "mixed", 1); got != "ONE" {
		t.Errorf("mixed numeric+named: got `%s`", got)
	}
	if got := bla.Get("lv", "shouty", 10); got != "UPPER-ZERO" {
		t.Errorf("uppercase category key: got `%s`", got)
	}
}

// A category the file does not define falls back to `other`.
func TestMappingFallsBackToOther(t *testing.T) {
	bla := mappingBla(t)

	// 10 is `zero` in Latvian, and `partial` has no zero form.
	if got := bla.Get("lv", "partial", 10); got != "10 lietas" {
		t.Errorf("expected the `other` form, got `%s`", got)
	}
}

// No `other` either -> the existing sentinel.
func TestMappingSentinelWhenNothingMatches(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "thin.yml", "k:\n  lv:\n    one: ONLY-ONE\n")
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("lv", "k", 5); got != "(lv.k)" {
		t.Errorf("expected sentinel, got `%s`", got)
	}
	if got := bla.Get("lv", "k", 1); got != "ONLY-ONE" {
		t.Errorf("expected `ONLY-ONE`, got `%s`", got)
	}
}

// Forced getters name an exact form, so they must not fall back.
func TestMappingForcedGettersDoNotFallBack(t *testing.T) {
	bla := mappingBla(t)

	if got := bla.GetZero("lv", "lieta", 10); got != "10 lietu" {
		t.Errorf("GetZero: got `%s`", got)
	}
	// `partial` has no zero form, and `other` must not stand in for it.
	if got := bla.GetZero("lv", "partial", 10); got != "(lv.partial)" {
		t.Errorf("GetZero must not fall back, got `%s`", got)
	}

	if got := bla.GetSingle("lv", "lieta", 5); got != "5 lieta" {
		t.Errorf("GetSingle on a mapping: got `%s`", got)
	}
	if got := bla.GetPlural("lv", "lieta", 1); got != "1 lietas" {
		t.Errorf("GetPlural on a mapping: got `%s`", got)
	}
}

// Welsh reaches every category.
func TestMappingAllSixCategories(t *testing.T) {
	bla := mappingBla(t)

	cases := map[int]string{
		0: "CY-ZERO",
		1: "CY-ONE",
		2: "CY-TWO",
		3: "CY-FEW",
		6: "CY-MANY",
		4: "CY-OTHER",
	}
	for count, want := range cases {
		if got := bla.Get("cy", "welsh", count); got != want {
			t.Errorf("Get(cy, welsh, %d) = `%s`, want `%s`", count, got, want)
		}
	}

	getters := map[string]string{
		bla.GetZero("cy", "welsh"):   "CY-ZERO",
		bla.GetSingle("cy", "welsh"): "CY-ONE",
		bla.GetTwo("cy", "welsh"):    "CY-TWO",
		bla.GetFew("cy", "welsh"):    "CY-FEW",
		bla.GetMany("cy", "welsh"):   "CY-MANY",
		bla.GetPlural("cy", "welsh"): "CY-OTHER",
	}
	for got, want := range getters {
		if got != want {
			t.Errorf("forced getter returned `%s`, want `%s`", got, want)
		}
	}
}

// No count -> the singular form, same as the legacy path.
func TestMappingWithoutCount(t *testing.T) {
	bla := mappingBla(t)

	if got := bla.Get("lv", "mixed"); got != "ONE" {
		t.Errorf("expected the `one` form, got `%s`", got)
	}
}

func TestMappingRejectsUnknownCategoryKey(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "typo.yml", "k:\n  lv:\n    oher: typo\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for an unknown category key")
	}
	if !strings.Contains(err.Error(), "oher") {
		t.Errorf("error should name the bad key, got: %v", err)
	}
}

func TestMappingRejectsNonScalarForm(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "nested.yml", "k:\n  lv:\n    one:\n      - a\n      - b\n")

	if _, err := Load(path); err == nil {
		t.Error("expected an error for a non-scalar category value")
	}
}

// The `^` keyword works per form inside a mapping.
func TestMappingSameAsKey(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "caret.yml", "Item:\n  lv:\n    one: ^\n    other: DAUDZ\n")
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("lv", "Item", 1); got != "Item" {
		t.Errorf("`^` in a mapping: got `%s`", got)
	}
	if got := bla.Get("lv", "Item", 5); got != "DAUDZ" {
		t.Errorf("expected `DAUDZ`, got `%s`", got)
	}
}

// An unknown language keeps one/other, so a mapping still resolves.
func TestMappingUnknownLanguageKeepsOneOther(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "unknown.yml", "k:\n  ru:\n    one: ODIN\n    other: MNOGO\n")
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("ru", "k", 1); got != "ODIN" {
		t.Errorf("got `%s`", got)
	}
	if got := bla.Get("ru", "k", 5); got != "MNOGO" {
		t.Errorf("got `%s`", got)
	}
}

// A custom parser still receives one already-resolved line.
func TestMappingCustomParserStillFormattingOnly(t *testing.T) {
	bla := mappingBla(t)

	var gotTemplate string
	if err := bla.CustomParser("lv", func(s string, v ...any) string {
		gotTemplate = s
		return "CUSTOM"
	}); err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("lv", "lieta", 10); got != "CUSTOM" {
		t.Errorf("custom parser not applied, got `%s`", got)
	}
	if gotTemplate != "%d lietu" {
		t.Errorf("parser should get the resolved `zero` line, got `%s`", gotTemplate)
	}
}
