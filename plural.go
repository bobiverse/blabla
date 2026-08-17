package blabla

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// category is a CLDR plural category. Unexported on purpose: a caller names a
// form by calling its getter (GetZero, GetFew, ..), so no constants leak.
type category uint8

const (
	catZero category = iota
	catOne
	catTwo
	catFew
	catMany
	catOther
)

// categoryByKey maps a YAML mapping key to its category. Numeric aliases are
// accepted alongside the names: `0` zero, `1` one, `2` other.
var categoryByKey = map[string]category{
	"zero":  catZero,
	"one":   catOne,
	"two":   catTwo,
	"few":   catFew,
	"many":  catMany,
	"other": catOther,
	"0":     catZero,
	"1":     catOne,
	"2":     catOther,
}

func (cat category) String() string {
	switch cat {
	case catZero:
		return "zero"
	case catOne:
		return "one"
	case catTwo:
		return "two"
	case catFew:
		return "few"
	case catMany:
		return "many"
	}
	return "other"
}

// operands are the CLDR plural operands, derived from the count's decimal text
// rather than its numeric value -- `1` and `1.0` are different to CLDR.
type operands struct {
	n    float64 // absolute value
	i    int64   // integer part
	v, w int     // fraction digit count, with and without trailing zeros
	f, t int64   // fraction digits as an integer, with and without trailing zeros
}

// parseOperands reads the operands off a number's decimal text.
func parseOperands(s string) (operands, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return operands{}, false
	}

	// `%v` renders some floats in exponent form (`1e+06`). CLDR operands are
	// defined on the decimal text, so re-render without an exponent.
	if strings.ContainsAny(s, "eE") {
		s = strconv.FormatFloat(f, 'f', -1, 64)
	}
	s = strings.TrimLeft(s, "+-")

	intText, fracText, _ := strings.Cut(s, ".")
	if intText == "" {
		intText = "0" // `.5` parses as a float but not as an int
	}

	i, err := strconv.ParseInt(intText, 10, 64)
	if errors.Is(err, strconv.ErrRange) {
		// A number too big for int64 is far past any rule's boundary. Saturate
		// so `i = 1` style checks stay false and the modulo checks land in the
		// plural branches, instead of failing the whole lookup.
		i = math.MaxInt64
	} else if err != nil {
		return operands{}, false
	}

	op := operands{n: math.Abs(f), i: i}
	if fracText == "" {
		return op, true
	}

	op.v = len(fracText)
	if op.f, err = strconv.ParseInt(fracText, 10, 64); err != nil {
		return operands{}, false
	}

	trimmed := strings.TrimRight(fracText, "0")
	op.w = len(trimmed)
	if trimmed == "" {
		return op, true
	}
	if op.t, err = strconv.ParseInt(trimmed, 10, 64); err != nil {
		return operands{}, false
	}

	return op, true
}

// countOperands returns the operands of the first numeric argument -- the count
// that decides the form. Later numbers are format params (a price, an id).
func countOperands(v []any) (operands, bool) {
	for _, arg := range v {
		if op, isNumber := parseOperands(fmt.Sprintf("%v", arg)); isNumber {
			return op, true
		}
	}
	return operands{}, false
}

// pattern is one CLDR ruleset together with the languages that use it.
//
// Grouped this way on purpose. CLDR's own plurals.xml stores rulesets carrying
// a `locales` list, so transcribing in the same direction is a copy rather than
// a per-language re-derivation -- which removes the whole class of assignment
// error. It also makes "which languages did we miss" a set difference.
//
// Do not file a language under a pattern by counting its forms. Several
// rulesets share the category set {one, other} and differ only on decimals.
type pattern struct {
	name  string
	langs []string
	cats  []category // the categories fn can return; Validate needs this
	fn    func(op operands) category
}

// inRange reports whether x is within lo..hi inclusive.
func inRange(x, lo, hi int64) bool {
	return x >= lo && x <= hi
}

// inRangeF is inRange for the `n` operand, which may be fractional.
func inRangeF(x, lo, hi float64) bool {
	return x >= lo && x <= hi && x == math.Trunc(x)
}

var (
	// one: i = 1 and v = 0
	patternEnglish = &pattern{
		name:  "english",
		langs: []string{"en", "de", "nl", "sv", "nb", "nn", "et", "fi", "el", "it", "bg", "hu", "tr", "sq", "ka", "eu"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.i == 1 && op.v == 0 {
				return catOne
			}
			return catOther
		},
	}

	// one: n = 1
	patternSpanish = &pattern{
		name:  "spanish",
		langs: []string{"es"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.n == 1 {
				return catOne
			}
			return catOther
		},
	}

	// one: i in 0..1
	patternFrench = &pattern{
		name:  "french",
		langs: []string{"fr", "pt"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if inRange(op.i, 0, 1) {
				return catOne
			}
			return catOther
		},
	}

	// one: n = 1 or (t != 0 and i in 0..1)
	patternDanish = &pattern{
		name:  "danish",
		langs: []string{"da"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.n == 1 || (op.t != 0 && inRange(op.i, 0, 1)) {
				return catOne
			}
			return catOther
		},
	}

	// one: (t = 0 and i % 10 = 1 and i % 100 != 11) or t != 0
	patternIcelandic = &pattern{
		name:  "icelandic",
		langs: []string{"is", "mk"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.t != 0 || (op.i%10 == 1 && op.i%100 != 11) {
				return catOne
			}
			return catOther
		},
	}

	// zero: n % 10 = 0, or n % 100 in 11..19, or (v = 2 and f % 100 in 11..19)
	// one:  (n % 10 = 1 and n % 100 != 11)
	//       or (v = 2 and f % 10 = 1 and f % 100 != 11)
	//       or (v != 2 and f % 10 = 1)
	patternLatvian = &pattern{
		name:  "latvian",
		langs: []string{"lv", "prg"},
		cats:  []category{catZero, catOne, catOther},
		fn: func(op operands) category {
			n10, n100 := math.Mod(op.n, 10), math.Mod(op.n, 100)

			if n10 == 0 || inRangeF(n100, 11, 19) || (op.v == 2 && inRange(op.f%100, 11, 19)) {
				return catZero
			}
			if n10 == 1 && n100 != 11 {
				return catOne
			}
			if op.v == 2 && op.f%10 == 1 && op.f%100 != 11 {
				return catOne
			}
			if op.v != 2 && op.f%10 == 1 {
				return catOne
			}
			return catOther
		},
	}

	// one: n % 10 = 1 and n % 100 not in 11..19
	// few: n % 10 in 2..9 and n % 100 not in 11..19
	// many: f != 0
	patternLithuanian = &pattern{
		name:  "lithuanian",
		langs: []string{"lt"},
		cats:  []category{catOne, catFew, catMany, catOther},
		fn: func(op operands) category {
			n10, n100 := math.Mod(op.n, 10), math.Mod(op.n, 100)

			if n10 == 1 && !inRangeF(n100, 11, 19) {
				return catOne
			}
			if inRangeF(n10, 2, 9) && !inRangeF(n100, 11, 19) {
				return catFew
			}
			if op.f != 0 {
				return catMany
			}
			return catOther
		},
	}

	// one:  v = 0 and i % 10 = 1 and i % 100 != 11
	// few:  v = 0 and i % 10 in 2..4 and i % 100 not in 12..14
	// many: v = 0 and (i % 10 = 0 or i % 10 in 5..9 or i % 100 in 11..14)
	patternUkrainian = &pattern{
		name:  "ukrainian",
		langs: []string{"uk", "be"},
		cats:  []category{catOne, catFew, catMany, catOther},
		fn: func(op operands) category {
			if op.v != 0 {
				return catOther
			}
			if op.i%10 == 1 && op.i%100 != 11 {
				return catOne
			}
			if inRange(op.i%10, 2, 4) && !inRange(op.i%100, 12, 14) {
				return catFew
			}
			if op.i%10 == 0 || inRange(op.i%10, 5, 9) || inRange(op.i%100, 11, 14) {
				return catMany
			}
			return catOther
		},
	}

	// one:  i = 1 and v = 0
	// few:  v = 0 and i % 10 in 2..4 and i % 100 not in 12..14
	// many: v = 0 and i != 1 and (i % 10 in 0..1 or i % 10 in 5..9 or i % 100 in 12..14)
	patternPolish = &pattern{
		name:  "polish",
		langs: []string{"pl"},
		cats:  []category{catOne, catFew, catMany, catOther},
		fn: func(op operands) category {
			if op.v != 0 {
				return catOther
			}
			if op.i == 1 {
				return catOne
			}
			if inRange(op.i%10, 2, 4) && !inRange(op.i%100, 12, 14) {
				return catFew
			}
			if inRange(op.i%10, 0, 1) || inRange(op.i%10, 5, 9) || inRange(op.i%100, 12, 14) {
				return catMany
			}
			return catOther
		},
	}

	// one: i = 1 and v = 0 · few: i in 2..4 and v = 0 · many: v != 0
	patternCzech = &pattern{
		name:  "czech",
		langs: []string{"cs", "sk"},
		cats:  []category{catOne, catFew, catMany, catOther},
		fn: func(op operands) category {
			if op.v != 0 {
				return catMany
			}
			if op.i == 1 {
				return catOne
			}
			if inRange(op.i, 2, 4) {
				return catFew
			}
			return catOther
		},
	}

	// one: v = 0 and i % 100 = 1 · two: v = 0 and i % 100 = 2
	// few: (v = 0 and i % 100 in 3..4) or v != 0
	patternSlovenian = &pattern{
		name:  "slovenian",
		langs: []string{"sl", "hsb", "dsb"},
		cats:  []category{catOne, catTwo, catFew, catOther},
		fn: func(op operands) category {
			if op.v != 0 {
				return catFew
			}
			switch {
			case op.i%100 == 1:
				return catOne
			case op.i%100 == 2:
				return catTwo
			case inRange(op.i%100, 3, 4):
				return catFew
			}
			return catOther
		},
	}

	// one: i = 1 and v = 0 · few: v != 0 or n = 0 or n % 100 in 2..19
	patternRomanian = &pattern{
		name:  "romanian",
		langs: []string{"ro"},
		cats:  []category{catOne, catFew, catOther},
		fn: func(op operands) category {
			if op.i == 1 && op.v == 0 {
				return catOne
			}
			if op.v != 0 || op.n == 0 || inRangeF(math.Mod(op.n, 100), 2, 19) {
				return catFew
			}
			return catOther
		},
	}

	// zero: n = 0 · one: n = 1 · two: n = 2 · few: n = 3 · many: n = 6
	patternWelsh = &pattern{
		name:  "welsh",
		langs: []string{"cy"},
		cats:  []category{catZero, catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			switch op.n {
			case 0:
				return catZero
			case 1:
				return catOne
			case 2:
				return catTwo
			case 3:
				return catFew
			case 6:
				return catMany
			}
			return catOther
		},
	}

	// one: n = 1 · two: n = 2 · few: n in 3..6 · many: n in 7..10
	patternIrish = &pattern{
		name:  "irish",
		langs: []string{"ga"},
		cats:  []category{catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			switch {
			case op.n == 1:
				return catOne
			case op.n == 2:
				return catTwo
			case inRangeF(op.n, 3, 6):
				return catFew
			case inRangeF(op.n, 7, 10):
				return catMany
			}
			return catOther
		},
	}
)

// patterns is the transcribed set. Prototype scope: the European rulesets that
// exercise every category. The remaining one/other languages are listed in the
// plan and behave correctly under patternEnglish anyway.
var patterns = []*pattern{
	patternEnglish,
	patternSpanish,
	patternFrench,
	patternDanish,
	patternIcelandic,
	patternLatvian,
	patternLithuanian,
	patternUkrainian,
	patternPolish,
	patternCzech,
	patternSlovenian,
	patternRomanian,
	patternWelsh,
	patternIrish,
}

// patternByLang is the flattened lookup, built once from patterns.
var patternByLang = map[string]*pattern{}

func init() {
	for _, p := range patterns {
		for _, lang := range p.langs {
			// A language filed under two patterns is a transcription mistake,
			// and silently letting the last one win would hide it forever.
			if already, isDup := patternByLang[lang]; isDup {
				panic(fmt.Sprintf("blabla: language `%s` is in both `%s` and `%s` patterns", lang, already.name, p.name))
			}
			patternByLang[lang] = p
		}
	}
}

// patternFor finds the ruleset for a language code. A region or script suffix
// is retried on the base language, so `pt-BR` uses the `pt` rules.
func patternFor(lang string) *pattern {
	if p, isKnown := patternByLang[lang]; isKnown {
		return p
	}

	if cut := strings.IndexAny(lang, "-_"); cut > 0 {
		if p, isKnown := patternByLang[lang[:cut]]; isKnown {
			return p
		}
	}

	// Unknown languages keep one/other, so a legacy two-form file still works.
	return patternEnglish
}
