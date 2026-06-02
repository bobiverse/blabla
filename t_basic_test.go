package blabla

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBasic(t *testing.T) {

	// Define test cases
	testCases := []struct {
		lang, key, expected string
		params              []any
	}{
		{"en", "hello", "Hello", nil},
		{"lv", "hello", "Sveiki", nil},

		{"en", "plural.demo", "One item", nil},
		{"en", "plural.demo", "5 items", []any{5}},

		{"en", "sub.hello", "Sub Hello", nil},
		{"lv", "sub.hello", "Sub Sveiki", nil},

		{"en", "sub3.hello", "Sub3 Hello", nil},
		{"lv", "sub3.hello", "Sub3 Sveiki", nil},

		{"en", "Same English text from Key", "Same English text from Key", nil},
		{"lv", "Same English text from Key", "Tas pats teksts no key", nil},

		{"en", "params", "1=1, 2=2.02 3=three", []any{1, 2.02, "three"}}, // test default case

		{"en", "EMPTY", "(en.EMPTY)", nil}, // test default case

		{"en", "haiku", "An old silent pond\nA frog jumps into the pond—\nSplash! Silence again.\n", nil}, // note `\n` nee lines
	}

	bla := MustLoad("tests/translations.yml")

	if bla.Errors != nil {
		t.Fatalf("Validation errors: %s", errors.Join(bla.Errors...))
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.lang+"_"+tc.key, func(t *testing.T) {
			result := bla.Get(tc.lang, tc.key, tc.params...)
			if result != tc.expected {
				t.Errorf("Expected `%s` but got `%s`", tc.expected, result)
			}
		})
	}
}

func TestGetSingleAndPlural(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	// GetSingle picks index 0 even when a plural form exists
	if got := bla.GetSingle("en", "plural.demo"); got != "One item" {
		t.Errorf("GetSingle: expected `One item`, got `%s`", got)
	}

	// GetPlural forces plural even when count <= 1
	if got := bla.GetPlural("en", "plural.demo", 1); got != "1 items" {
		t.Errorf("GetPlural: expected `1 items`, got `%s`", got)
	}

	// GetPlural on a key without a plural form falls back to sentinel
	if got := bla.GetPlural("en", "hello"); got != "(en.hello)" {
		t.Errorf("GetPlural missing form: expected `(en.hello)`, got `%s`", got)
	}
}

func TestCaseInsensitiveLang(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	cases := map[string]string{
		"EN": "Hello",
		"En": "Hello",
		"Lv": "Sveiki",
		"LV": "Sveiki",
	}
	for lang, want := range cases {
		if got := bla.Get(lang, "hello"); got != want {
			t.Errorf("Get(%q, hello): expected `%s`, got `%s`", lang, want, got)
		}
	}
}

func TestMissingLang(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	if got := bla.Get("fr", "hello"); got != "(fr.hello)" {
		t.Errorf("missing lang: expected `(fr.hello)`, got `%s`", got)
	}
}

func TestCustomParser(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	// Unknown lang → error, no registration
	if err := bla.CustomParser("xx", func(s string, v ...any) string { return s }); err == nil {
		t.Errorf("expected error registering parser for unknown lang")
	}

	called := 0
	err := bla.CustomParser("EN", func(s string, v ...any) string {
		called++
		return "CUSTOM:" + s
	})
	if err != nil {
		t.Fatalf("CustomParser(en) returned error: %v", err)
	}

	if got := bla.Get("en", "hello"); got != "CUSTOM:Hello" {
		t.Errorf("custom parser: expected `CUSTOM:Hello`, got `%s`", got)
	}
	if called == 0 {
		t.Errorf("custom parser was never invoked")
	}

	// Other langs are unaffected
	if got := bla.Get("lv", "hello"); got != "Sveiki" {
		t.Errorf("lv path leaked through custom parser: got `%s`", got)
	}
}

func TestValidateReportsMissingTranslation(t *testing.T) {
	bla, err := Load("tests/translations_broken.yml")
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if len(bla.Errors) == 0 {
		t.Fatalf("expected at least one validation error, got none")
	}

	joined := errors.Join(bla.Errors...).Error()
	if !strings.Contains(joined, "lv") || !strings.Contains(joined, "goodbye") {
		t.Errorf("validation error should mention missing `lv` for `goodbye`, got: %s", joined)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	if _, err := Load("tests/does_not_exist.yml"); err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestIsPluralCount(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{0, false},
		{1, false},
		{2, true},
		{1.5, true},
		{0.5, false},
		{-3, false},
		{"5", true},
		{"1", false},
		{"abc", false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isPluralCount(c.in); got != c.want {
			t.Errorf("isPluralCount(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	bla := MustLoad("tests/translations.yml")
	s := bla.String()

	for _, want := range []string{"Languages:", "Translations:", "Errors:", "en", "lv"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() missing %q; got:\n%s", want, s)
		}
	}
}

func TestStringReflectsValidationErrors(t *testing.T) {
	bla, err := Load("tests/translations_broken.yml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(bla.String(), "Errors:\t 1") {
		t.Errorf("String() should report 1 error; got:\n%s", bla.String())
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	if err := os.WriteFile(path, []byte("key:\n  en: hello\n: : : not valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Errorf("expected Load to return an error for malformed YAML")
	}
}

func TestLoadMissingIncludeIsTolerated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.yml")
	content := `greeting:
  en: Hi
  lv: Sveiks

include:
  missing: nope.yml
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	bla, err := Load(path)
	if err != nil {
		t.Fatalf("Load should tolerate a missing include, got: %v", err)
	}
	if got := bla.Get("en", "greeting"); got != "Hi" {
		t.Errorf("non-include keys should still load: got `%s`", got)
	}
}

func TestIncludeKeyIsRemoved(t *testing.T) {
	bla := MustLoad("tests/translations.yml")
	if got := bla.Get("en", keywordInclude); got != "(en.include)" {
		t.Errorf("`include` key should be stripped after Load, got `%s`", got)
	}
}

func TestEmptyKeyAndLang(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	cases := []struct {
		lang, key, want string
	}{
		{"", "hello", "(.hello)"},
		{"en", "", "(en.)"},
		{"", "", "(.)"},
	}
	for _, c := range cases {
		if got := bla.Get(c.lang, c.key); got != c.want {
			t.Errorf("Get(%q, %q): expected `%s`, got `%s`", c.lang, c.key, c.want, got)
		}
	}
}

func TestPluralRouting(t *testing.T) {
	// Sentinel templates so we can assert which form was picked
	// without fighting fmt's verb/arg arithmetic.
	path := filepath.Join(t.TempDir(), "routing.yml")
	if err := os.WriteFile(path, []byte("count:\n  en:\n    - SINGULAR\n    - PLURAL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		args   []any
		plural bool
	}{
		{"no args -> singular", nil, false},
		{"int 0 -> singular", []any{0}, false},
		{"int 1 -> singular", []any{1}, false},
		{"int 2 -> plural", []any{2}, true},
		{"negative -> singular", []any{-3}, false},
		{"fractional > 1 -> plural", []any{1.5}, true},
		{"fractional <= 1 -> singular", []any{0.5}, false},
		{"non-numeric -> singular", []any{"banana"}, false},
		{"numeric string > 1 -> plural", []any{"5"}, true},
		{"numeric string == 1 -> singular", []any{"1"}, false},
		{"nil arg -> singular", []any{nil}, false},
		{"mixed: numeric > 1 anywhere triggers plural", []any{"tag", 5}, true},
		{"mixed: all non-plural stay singular", []any{"tag", 1, "more"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bla.Get("en", "count", c.args...)
			wantPrefix := "SINGULAR"
			if c.plural {
				wantPrefix = "PLURAL"
			}
			if !strings.HasPrefix(got, wantPrefix) {
				t.Errorf("expected %s form, got `%s`", wantPrefix, got)
			}
		})
	}
}

func TestPluralIgnoredWhenKeyHasNoPluralForm(t *testing.T) {
	// `hello` only has a single form; passing a count > 1 must NOT crash
	// and must NOT route to a non-existent plural slot.
	bla := MustLoad("tests/translations.yml")
	got := bla.Get("en", "hello", 5)
	if !strings.HasPrefix(got, "Hello") {
		t.Errorf("singular-only key should stay on form 0, got `%s`", got)
	}
}

func TestCustomParserReceivesArgs(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	var (
		gotStr  string
		gotArgs []any
	)
	if err := bla.CustomParser("en", func(s string, v ...any) string {
		gotStr = s
		gotArgs = v
		return "OK"
	}); err != nil {
		t.Fatal(err)
	}

	bla.Get("en", "params", 1, 2.02, "three")

	if gotStr != "1=%d, 2=%0.2f 3=%s" {
		t.Errorf("custom parser got template `%s`", gotStr)
	}
	if len(gotArgs) != 3 || gotArgs[0] != 1 || gotArgs[1] != 2.02 || gotArgs[2] != "three" {
		t.Errorf("custom parser args mismatch: %v", gotArgs)
	}
}

func TestCustomParserDoesNotMaskSentinel(t *testing.T) {
	bla := MustLoad("tests/translations.yml")
	if err := bla.CustomParser("en", func(s string, v ...any) string {
		return "WRAPPED:" + s
	}); err != nil {
		t.Fatal(err)
	}
	if got := bla.Get("en", "NONEXISTENT"); got != "(en.NONEXISTENT)" {
		t.Errorf("missing key should bypass custom parser, got `%s`", got)
	}
}

func TestLoadEmptyIncludeValue(t *testing.T) {
	// An empty sequence for an include lang must be skipped silently.
	path := filepath.Join(t.TempDir(), "main.yml")
	content := `greeting:
  en: Hi

include:
  sub: []
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	bla, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := bla.Get("en", "greeting"); got != "Hi" {
		t.Errorf("expected `Hi`, got `%s`", got)
	}
}

func TestUnmarshalYAMLRejectsNonStringSequenceElement(t *testing.T) {
	// A sequence whose elements aren't scalars can't decode into []string.
	path := filepath.Join(t.TempDir(), "bad.yml")
	content := `broken:
  en:
    - good
    - {nested: object}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Errorf("expected decode error for non-scalar sequence element")
	}
}

func TestValidateIsIdempotent(t *testing.T) {
	bla, err := Load("tests/translations_broken.yml")
	if err != nil {
		t.Fatal(err)
	}
	first := len(bla.Errors)
	if first == 0 {
		t.Fatalf("expected validation errors on first call")
	}
	bla.Validate()
	if len(bla.Errors) != first {
		t.Errorf("re-running Validate changed error count: %d -> %d", first, len(bla.Errors))
	}
}
