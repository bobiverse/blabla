package blabla

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// Tests that pin the bugs found in the bug report. Each asserts the CORRECT
// behavior, so each fails until the matching bug is fixed.

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// BUG 1 (HIGH): get() Sprintf's the line whenever args are passed, even when
// the line has no verbs, so fmt appends `%!(EXTRA ...)`.
func TestBugSingularFormMustNotLeakExtraArgs(t *testing.T) {
	bla := MustLoad("tests/translations.yml")

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Get plural key with count 1", bla.Get("en", "plural.demo", 1), "One item"},
		{"GetSingle plural key with count 1", bla.GetSingle("en", "plural.demo", 1), "One item"},
		{"Get singular-only key with a count", bla.Get("en", "hello", 5), "Hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("expected `%s`, got `%s`", c.want, c.got)
			}
		})
	}
}

// BUG 2 (HIGH): a circular include recurses forever. It is a fatal stack
// overflow, not a returned error, so it must run in a child process or it
// takes the whole test binary down.
const cycleEnv = "BLABLA_TEST_CYCLE_DIR"

func TestBugCircularIncludeMustNotCrash(t *testing.T) {
	// Child half: attempt the load and exit normally whatever Load returns.
	if dir := os.Getenv(cycleEnv); dir != "" {
		// Cap the stack so the runaway recursion aborts in milliseconds
		// instead of growing to the 1GB default.
		debug.SetMaxStack(8 << 20)
		if _, err := Load(filepath.Join(dir, "a.yml")); err != nil {
			t.Logf("child: Load returned error (fine): %v", err)
		}
		return
	}

	dir := t.TempDir()
	writeYAML(t, dir, "a.yml", "a.key:\n  en: A\ninclude:\n  b: b.yml\n")
	writeYAML(t, dir, "b.yml", "b.key:\n  en: B\ninclude:\n  a: a.yml\n")

	cmd := exec.Command(os.Args[0], "-test.run=^TestBugCircularIncludeMustNotCrash$", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), cycleEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("circular include crashed the process (%v); Load must detect the cycle and return.\n%s",
			err, lastLines(string(out), 6))
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// BUG 3 (MEDIUM): plural is picked if ANY variadic arg parses as > 1, so a
// price or an id hijacks the form choice from the actual count.
// NOTE: conflicts with t_basic_test.go TestPluralRouting
// "mixed: numeric > 1 anywhere triggers plural", which freezes today's behavior.
func TestBugPluralMustFollowCountNotAnyNumericArg(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "cart.yml",
		"cart:\n  en:\n    - \"%d item costs %0.2f EUR\"\n    - \"%d items cost %0.2f EUR\"\n")

	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := bla.Get("en", "cart", 1, 2.50), "1 item costs 2.50 EUR"; got != want {
		t.Errorf("expected `%s`, got `%s`", want, got)
	}
	// Same call with a price <= 1 already gives the right form today.
	if got, want := bla.Get("en", "cart", 1, 0.50), "1 item costs 0.50 EUR"; got != want {
		t.Errorf("expected `%s`, got `%s`", want, got)
	}
}

// BUG 4 (MEDIUM): maps.Copy lets included files overwrite the including file's
// own keys, and the include loop walks a map so multiple includes are applied
// in random order (confirmed varying across processes).
func TestBugIncludeMustNotOverrideParentKeys(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "one.yml", "greeting:\n  en: from-one\n")
	writeYAML(t, dir, "two.yml", "greeting:\n  en: from-two\n")
	path := writeYAML(t, dir, "parent.yml",
		"greeting:\n  en: from-parent\ninclude:\n  x: one.yml\n  y: two.yml\n")

	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := bla.Get("en", "greeting"), "from-parent"; got != want {
		t.Errorf("the including file must win over its includes: expected `%s`, got `%s`", want, got)
	}
}

// BUG 5 (LOW): only fsubnames[0] is used, so an include written as a YAML
// sequence silently drops every file after the first.
func TestBugIncludeSequenceMustLoadEveryFile(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "one.yml", "k.one:\n  en: One\n")
	writeYAML(t, dir, "two.yml", "k.two:\n  en: Two\n")
	path := writeYAML(t, dir, "multi.yml",
		"top:\n  en: T\ninclude:\n  many:\n    - one.yml\n    - two.yml\n")

	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := bla.Get("en", "k.one"), "One"; got != want {
		t.Errorf("expected `%s`, got `%s`", want, got)
	}
	if got, want := bla.Get("en", "k.two"), "Two"; got != want {
		t.Errorf("expected `%s`, got `%s`", want, got)
	}
}

// BUG 6 (LOW): isPluralCount requires n > 1, so a count of 0 takes the
// singular form. English needs the plural for 0.
// NOTE: conflicts with t_basic_test.go TestIsPluralCount {0, false} and
// TestPluralRouting "int 0 -> singular", which freeze today's behavior.
func TestBugZeroCountMustUsePluralForm(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "routing.yml", "count:\n  en:\n    - SINGULAR\n    - PLURAL\n")

	bla, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := bla.Get("en", "count", 0); got != "PLURAL" {
		t.Errorf("count 0 must use the plural form, got `%s`", got)
	}
}
