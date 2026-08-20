package blabla

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain silences the package logger for the whole run. A failed include is
// logged and tolerated by design, and two tests exercise that path on purpose,
// so their output is expected noise rather than a problem. Tests that care
// about what was logged capture it with captureLog.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	code := m.Run()
	log.SetOutput(os.Stderr)
	os.Exit(code)
}

// captureLog redirects the standard logger into a buffer for one test and
// restores the previous writer afterwards.
func captureLog(t *testing.T) func() string {
	t.Helper()

	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prev)
		log.SetFlags(flags)
	})

	return buf.String
}

// Freezes the behavior of scalar and sequence blocks. None of it may change
// when the CLDR engine lands: a file that does not use a mapping must take the
// same code path it always took. If any of this goes red, the change is wrong.

func legacyBla(t *testing.T) *BlaBla {
	t.Helper()
	dir := t.TempDir()
	path := writeYAML(t, dir, "legacy.yml", `count:
  en:
    - "%d SINGULAR"
    - "%d PLURAL"
  lv:
    - "%d lieta"
    - "%d lietas"

hello:
  en: Hello
  lv: Sveiki

three:
  en:
    - ONE
    - TWO
    - THREE
`)
	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return bla
}

// The important half: Latvian counts where CLDR disagrees with `n != 1`.
// A sequence must keep answering the old way.
func TestLegacySequenceRoutingUnchanged(t *testing.T) {
	bla := legacyBla(t)

	cases := []struct {
		lang   string
		count  any
		plural bool
	}{
		{"en", 0, true},
		{"en", 1, false},
		{"en", 2, true},
		{"en", 5, true},

		// CLDR Latvian puts these in `one`; the legacy engine must not.
		{"lv", 21, true},
		{"lv", 31, true},
		{"lv", 101, true},
		// CLDR Latvian puts these in `zero`; still plural here.
		{"lv", 10, true},
		{"lv", 15, true},
		{"lv", 0, true},
		// And the forms that agree either way.
		{"lv", 1, false},
		{"lv", 5, true},
	}

	for _, c := range cases {
		got := bla.Get(c.lang, "count", c.count)
		isPlural := strings.Contains(got, "PLURAL") || strings.Contains(got, "lietas")
		if isPlural != c.plural {
			t.Errorf("Get(%q, count, %v) = `%s`; plural=%v, want %v", c.lang, c.count, got, isPlural, c.plural)
		}
	}
}

func TestLegacyArgHandlingUnchanged(t *testing.T) {
	bla := legacyBla(t)

	cases := []struct {
		name string
		args []any
		want string
	}{
		{"no args", nil, "%d SINGULAR"}, // no args -> no Sprintf at all
		{"non-numeric", []any{"banana"}, "%!d(string=banana) SINGULAR"},
		{"numeric string", []any{"5"}, "%!d(string=5) PLURAL"},
		{"negative", []any{-3}, "-3 PLURAL"},
		{"fraction", []any{0.5}, "%!d(float64=0.5) PLURAL"},
		{"first numeric decides", []any{1, 99}, "1 SINGULAR%!(EXTRA int=99)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bla.Get("en", "count", c.args...); got != c.want {
				t.Errorf("expected `%s`, got `%s`", c.want, got)
			}
		})
	}
}

// A single-form key with a count must not route by plural at all.
func TestLegacyScalarWithCountUnchanged(t *testing.T) {
	bla := legacyBla(t)

	if got := bla.Get("en", "hello", 5); got != "Hello" {
		t.Errorf("expected `Hello`, got `%s`", got)
	}
	if got := bla.Get("lv", "hello", 0); got != "Sveiki" {
		t.Errorf("expected `Sveiki`, got `%s`", got)
	}
}

func TestLegacyForcedGettersUnchanged(t *testing.T) {
	bla := legacyBla(t)

	if got := bla.GetSingle("en", "count", 7); got != "7 SINGULAR" {
		t.Errorf("GetSingle: got `%s`", got)
	}
	if got := bla.GetPlural("en", "count", 1); got != "1 PLURAL" {
		t.Errorf("GetPlural: got `%s`", got)
	}
	// No plural form -> sentinel, as before.
	if got := bla.GetPlural("en", "hello"); got != "(en.hello)" {
		t.Errorf("GetPlural on scalar: got `%s`", got)
	}
	// The category getters have nothing to read on a legacy block.
	if got := bla.GetZero("lv", "count", 0); got != "(lv.count)" {
		t.Errorf("GetZero on a sequence should be a sentinel, got `%s`", got)
	}
}

// Entries past index 1 stay unreachable, and produce no new diagnostic.
func TestLegacyOverlongSequenceUnchanged(t *testing.T) {
	bla := legacyBla(t)

	if got := bla.Get("en", "three", 1); got != "ONE" {
		t.Errorf("expected `ONE`, got `%s`", got)
	}
	if got := bla.Get("en", "three", 5); got != "TWO" {
		t.Errorf("expected `TWO`, got `%s`", got)
	}
}

// NSingle/NMany keep their uint type -- assigning them here is the guard.
func TestLegacyConstantsAreStillUint(t *testing.T) {
	var single uint = NSingle
	var many uint = NMany

	if single != 0 || many != 1 {
		t.Errorf("NSingle/NMany changed value: %d/%d", single, many)
	}
}

// A file with no mapping block must report exactly what it reports today.
func TestLegacyValidationUnchanged(t *testing.T) {
	bla := MustLoad("tests/translations.yml")
	if bla.Errors != nil {
		t.Fatalf("fixture should validate clean, got: %v", bla.Errors)
	}

	broken, err := Load("tests/translations_broken.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(broken.Errors) != 1 {
		t.Errorf("expected exactly 1 validation error, got %d: %v", len(broken.Errors), broken.Errors)
	}
}

// `include:` reuses translationLines for filenames, so the struct change is
// the risky part here.
func TestLegacyIncludeUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "one.yml", "k.one:\n  en: One\n")
	writeYAML(t, dir, "two.yml", "k.two:\n  en: Two\n")
	path := writeYAML(t, dir, "main.yml", `top:
  en: T
include:
  seq:
    - one.yml
    - two.yml
  gone: nope.yml
  empty: []
`)

	logged := captureLog(t)

	bla, err := Load(path)
	if err != nil {
		t.Fatalf("Load should tolerate a missing include: %v", err)
	}

	// The missing include must be reported, not swallowed.
	if out := logged(); !strings.Contains(out, "nope.yml") {
		t.Errorf("a failed include should be logged, got: %q", out)
	}

	for key, want := range map[string]string{"top": "T", "k.one": "One", "k.two": "Two"} {
		if got := bla.Get("en", key); got != want {
			t.Errorf("Get(en, %s) = `%s`, want `%s`", key, got, want)
		}
	}
}

func TestLegacySameAsKeyUnchanged(t *testing.T) {
	bla := MustLoad("tests/translations.yml")
	if got := bla.Get("en", "Same English text from Key"); got != "Same English text from Key" {
		t.Errorf("`^` keyword: got `%s`", got)
	}
}

func TestLegacyCustomParserUnchanged(t *testing.T) {
	bla := legacyBla(t)

	var gotTemplate string
	var gotArgs []any
	if err := bla.CustomParser("en", func(s string, v ...any) string {
		gotTemplate, gotArgs = s, v
		return "CUSTOM"
	}); err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("en", "count", 5); got != "CUSTOM" {
		t.Errorf("custom parser not applied, got `%s`", got)
	}
	if gotTemplate != "%d PLURAL" || len(gotArgs) != 1 || gotArgs[0] != 5 {
		t.Errorf("parser got template `%s` args %v", gotTemplate, gotArgs)
	}
}

func TestLegacySentinelUnchanged(t *testing.T) {
	bla := legacyBla(t)

	cases := map[string]string{
		"fr|hello": "(fr.hello)",
		"en|nope":  "(en.nope)",
		"|hello":   "(.hello)",
	}
	for in, want := range cases {
		lang, key, _ := strings.Cut(in, "|")
		if got := bla.Get(lang, key); got != want {
			t.Errorf("Get(%q, %q) = `%s`, want `%s`", lang, key, got, want)
		}
	}
}

func TestLegacyLangCaseUnchanged(t *testing.T) {
	bla := legacyBla(t)
	for _, lang := range []string{"LV", "Lv", "lV"} {
		if got := bla.Get(lang, "hello"); got != "Sveiki" {
			t.Errorf("Get(%q, hello) = `%s`", lang, got)
		}
	}
}

func TestLegacyLoadErrorsUnchanged(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Error("expected an error for a missing file")
	}

	dir := t.TempDir()
	path := writeYAML(t, dir, "bad.yml", "broken:\n  en:\n    - good\n    - {nested: object}\n")
	if _, err := Load(path); err == nil {
		t.Error("expected a decode error for a non-scalar sequence element")
	}
}
