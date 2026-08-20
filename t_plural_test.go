package blabla

import (
	"fmt"
	"math"
	"testing"
)

// Tests the CLDR rulesets directly, without going through Get(), so a failure
// points at the transcription rather than at the lookup around it.

func catFor(t *testing.T, p *pattern, count any) category {
	t.Helper()
	op, isNumber := parseOperands(fmt.Sprintf("%v", count))
	if !isNumber {
		t.Fatalf("parseOperands(%v) failed", count)
	}
	return p.fn(op)
}

func TestParseOperands(t *testing.T) {
	cases := []struct {
		in   string
		want operands
	}{
		{"1", operands{n: 1, i: 1}},
		{"0", operands{n: 0, i: 0}},
		{"-3", operands{n: 3, i: 3}},
		{"1.50", operands{n: 1.5, i: 1, v: 2, w: 1, f: 50, t: 5}},
		{"1.0", operands{n: 1, i: 1, v: 1, w: 0, f: 0, t: 0}},
		{"0.5", operands{n: 0.5, i: 0, v: 1, w: 1, f: 5, t: 5}},
		{"-2.30", operands{n: 2.3, i: 2, v: 2, w: 1, f: 30, t: 3}},
		{"1000000", operands{n: 1000000, i: 1000000}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, isNumber := parseOperands(c.in)
			if !isNumber {
				t.Fatalf("parseOperands(%q) reported not-a-number", c.in)
			}
			if got != c.want {
				t.Errorf("parseOperands(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}

	for _, in := range []string{"banana", "", "<nil>", "1.2.3"} {
		if _, isNumber := parseOperands(in); isNumber {
			t.Errorf("parseOperands(%q) should report not-a-number", in)
		}
	}
}

// Exponent form reaches parseOperands whenever fmt picks it for a float.
func TestParseOperandsExponentForm(t *testing.T) {
	got, isNumber := parseOperands(fmt.Sprintf("%v", 1e21))
	if !isNumber {
		t.Fatalf("exponent form rejected")
	}
	if got.v != 0 || got.n != 1e21 {
		t.Errorf("exponent form gave %+v, want v=0 n=1e21", got)
	}
	// The integer part saturates rather than failing the lookup.
	if got.i != math.MaxInt64 {
		t.Errorf("expected saturated i, got %d", got.i)
	}
	if cat := patternEnglish.fn(got); cat != catOther {
		t.Errorf("a huge count must not be singular, got %s", cat)
	}
}

func TestPatterns(t *testing.T) {
	cases := []struct {
		p     *pattern
		count any
		want  category
	}{
		// English: one only at exactly 1 with no visible decimals
		{patternEnglish, 0, catOther},
		{patternEnglish, 1, catOne},
		{patternEnglish, 2, catOther},
		{patternEnglish, "1.0", catOther},

		// Spanish shares English's category set but not its rule -- this pair
		// is the reason patterns are keyed on conditions, not on form count.
		{patternSpanish, 1, catOne},
		{patternSpanish, "1.0", catOne},

		// French: 0 is singular
		{patternFrench, 0, catOne},
		{patternFrench, 1, catOne},
		{patternFrench, 2, catOther},
		{patternFrench, "1.5", catOne},

		// Danish: fractions below 2 are singular
		{patternDanish, 1, catOne},
		{patternDanish, "0.5", catOne},
		{patternDanish, 2, catOther},

		// Icelandic: two categories, not a 2-4 dual
		{patternIcelandic, 1, catOne},
		{patternIcelandic, 2, catOther},
		{patternIcelandic, 3, catOther},
		{patternIcelandic, 4, catOther},
		{patternIcelandic, 5, catOther},
		{patternIcelandic, 11, catOther},
		{patternIcelandic, 21, catOne},
		{patternIcelandic, "1.5", catOther}, // only x.1 decimals are `one`

		// Latvian: the roadmap's example
		{patternLatvian, 0, catZero},
		{patternLatvian, 1, catOne},
		{patternLatvian, 5, catOther},
		{patternLatvian, 10, catZero},
		{patternLatvian, 11, catZero},
		{patternLatvian, 15, catZero},
		{patternLatvian, 20, catZero},
		{patternLatvian, 21, catOne},
		{patternLatvian, 22, catOther},
		{patternLatvian, 100, catZero},
		{patternLatvian, 101, catOne},
		{patternLatvian, 111, catZero},

		// Lithuanian
		{patternLithuanian, 1, catOne},
		{patternLithuanian, 2, catFew},
		{patternLithuanian, 9, catFew},
		{patternLithuanian, 10, catOther},
		{patternLithuanian, 11, catOther},
		{patternLithuanian, 21, catOne},
		{patternLithuanian, "1.5", catMany},

		// Ukrainian
		{patternUkrainian, 0, catMany},
		{patternUkrainian, 1, catOne},
		{patternUkrainian, 2, catFew},
		{patternUkrainian, 4, catFew},
		{patternUkrainian, 5, catMany},
		{patternUkrainian, 11, catMany},
		{patternUkrainian, 21, catOne},
		{patternUkrainian, 22, catFew},
		{patternUkrainian, "1.5", catOther},

		// Polish
		{patternPolish, 0, catMany},
		{patternPolish, 1, catOne},
		{patternPolish, 2, catFew},
		{patternPolish, 4, catFew},
		{patternPolish, 5, catMany},
		{patternPolish, 12, catMany},
		{patternPolish, 22, catFew},
		{patternPolish, 25, catMany},
		{patternPolish, "1.5", catOther},

		// Czech
		{patternCzech, 1, catOne},
		{patternCzech, 2, catFew},
		{patternCzech, 4, catFew},
		{patternCzech, 5, catOther},
		{patternCzech, "1.5", catMany},

		// Slovenian: the real 2-4 dual
		{patternSlovenian, 1, catOne},
		{patternSlovenian, 101, catOne},
		{patternSlovenian, 2, catTwo},
		{patternSlovenian, 3, catFew},
		{patternSlovenian, 4, catFew},
		{patternSlovenian, 5, catOther},
		{patternSlovenian, "1.5", catFew},

		// Romanian
		{patternRomanian, 0, catFew},
		{patternRomanian, 1, catOne},
		{patternRomanian, 2, catFew},
		{patternRomanian, 19, catFew},
		{patternRomanian, 20, catOther},
		{patternRomanian, 101, catFew}, // n != 1 and n % 100 = 1..19

		// Welsh: all six categories in one language
		{patternWelsh, 0, catZero},
		{patternWelsh, 1, catOne},
		{patternWelsh, 2, catTwo},
		{patternWelsh, 3, catFew},
		{patternWelsh, 4, catOther},
		{patternWelsh, 6, catMany},
		{patternWelsh, 7, catOther},

		// Irish
		{patternIrish, 1, catOne},
		{patternIrish, 2, catTwo},
		{patternIrish, 3, catFew},
		{patternIrish, 6, catFew},
		{patternIrish, 7, catMany},
		{patternIrish, 10, catMany},
		{patternIrish, 11, catOther},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s_%v", c.p.name, c.count), func(t *testing.T) {
			if got := catFor(t, c.p, c.count); got != c.want {
				t.Errorf("%s(%v) = %s, want %s", c.p.name, c.count, got, c.want)
			}
		})
	}
}

// Every category a pattern can actually return must be declared in `cats`,
// because Validate reports missing forms from that list.
func TestPatternCategoriesAreDeclared(t *testing.T) {
	for _, p := range patterns {
		declared := map[category]bool{}
		for _, cat := range p.cats {
			declared[cat] = true
		}

		for n := 0; n <= 200; n++ {
			cat := catFor(t, p, n)
			if !declared[cat] {
				t.Errorf("%s: count %d returns `%s`, which is not in cats", p.name, n, cat)
			}
		}
	}
}

func TestPatternForLanguage(t *testing.T) {
	cases := map[string]*pattern{
		"lv":      patternLatvian,
		"uk":      patternUkrainian,
		"en":      patternEnglish,
		"pt-BR":   patternFrench,  // region suffix falls back to the base language
		"en_US":   patternEnglish, // underscore form too
		"ru":      patternEnglish, // not transcribed; keeps one/other
		"klingon": patternEnglish, // unknown
		"ua":      patternEnglish, // country code, not a language code
	}
	for lang, want := range cases {
		if got := patternFor(lang); got != want {
			t.Errorf("patternFor(%q) = %s, want %s", lang, got.name, want.name)
		}
	}
}
