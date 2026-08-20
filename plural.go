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
//
// The `e` operand (compact decimal exponent) is not modelled. blabla has no
// compact notation, so the rules that read it are unreachable here.
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

	// `%v` renders some floats in exponent form (`1e+21`). CLDR operands are
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
// error. Every list below is copied from that attribute, European members only.
//
// Do not file a language under a pattern by counting its forms. Four rulesets
// share the category set {one, other} and differ only on decimals.
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

// inRangeF is inRange for the `n` operand, which may be fractional. A
// fractional n is never inside a CLDR integer range.
func inRangeF(x, lo, hi float64) bool {
	return x >= lo && x <= hi && x == math.Trunc(x)
}

// eqAny reports whether x equals any of the listed values.
func eqAny(x float64, vals ...float64) bool {
	for _, val := range vals {
		if x == val {
			return true
		}
	}
	return false
}

// isRoundMillion reports the reachable half of the romance `many` rule:
// `e = 0 and i != 0 and i % 1000000 = 0 and v = 0`.
//
// The other half (`e != 0..5`) needs the compact-decimal exponent, which
// blabla does not have -- but this half fires on a plain 1000000, so `many`
// is NOT unreachable here. CLDR's own samples caught that assumption.
func isRoundMillion(op operands) bool {
	return op.v == 0 && op.i != 0 && op.i%1000000 == 0
}

var (
	// one: i = 1 and v = 0
	patternEnglish = &pattern{
		name:  "english",
		langs: []string{"ast", "de", "en", "et", "fi", "fy", "ia", "ie", "io", "lij", "nl", "sc", "sv"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.i == 1 && op.v == 0 {
				return catOne
			}
			return catOther
		},
	}

	// one: n = 1 -- the largest CLDR ruleset
	patternGreek = &pattern{
		name: "greek",
		langs: []string{"af", "an", "bg", "ce", "el", "eo", "eu", "fo", "fur", "gsw", "hu", "ka",
			"kk", "kl", "ku", "ky", "lb", "mn", "nb", "nd", "nn", "no", "nr", "os", "rm", "sq",
			"ss", "st", "tk", "tn", "tr", "ts", "uz", "ve", "wae", "xh"},
		cats: []category{catOne, catOther},
		fn: func(op operands) category {
			if op.n == 1 {
				return catOne
			}
			return catOther
		},
	}

	// one: n = 1 or t != 0 and i = 0,1
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

	// one: t = 0 and i % 10 = 1 and i % 100 != 11
	//      or t % 10 = 1 and t % 100 != 11
	patternIcelandic = &pattern{
		name:  "icelandic",
		langs: []string{"is"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.t == 0 && op.i%10 == 1 && op.i%100 != 11 {
				return catOne
			}
			if op.t%10 == 1 && op.t%100 != 11 {
				return catOne
			}
			return catOther
		},
	}

	// one: v = 0 and i % 10 = 1 and i % 100 != 11
	//      or f % 10 = 1 and f % 100 != 11
	patternMacedonian = &pattern{
		name:  "macedonian",
		langs: []string{"mk"},
		cats:  []category{catOne, catOther},
		fn: func(op operands) category {
			if op.v == 0 && op.i%10 == 1 && op.i%100 != 11 {
				return catOne
			}
			if op.f%10 == 1 && op.f%100 != 11 {
				return catOne
			}
			return catOther
		},
	}

	// one:  i = 0..1
	// many: e = 0 and i != 0 and i % 1000000 = 0 and v = 0 or e != 0..5
	patternFrench = &pattern{
		name:  "french",
		langs: []string{"fr", "pt"},
		cats:  []category{catOne, catMany, catOther},
		fn: func(op operands) category {
			if inRange(op.i, 0, 1) {
				return catOne
			}
			if isRoundMillion(op) {
				return catMany
			}
			return catOther
		},
	}

	// one:  i = 1 and v = 0 · many: same rule as french
	// A separate ruleset from english in CLDR: english has no `many` at all.
	patternCatalan = &pattern{
		name:  "catalan",
		langs: []string{"ca", "gl", "it", "lld", "scn", "vec"},
		cats:  []category{catOne, catMany, catOther},
		fn: func(op operands) category {
			if op.i == 1 && op.v == 0 {
				return catOne
			}
			if isRoundMillion(op) {
				return catMany
			}
			return catOther
		},
	}

	// one: n = 1 · many: same rule as french
	patternSpanish = &pattern{
		name:  "spanish",
		langs: []string{"es"},
		cats:  []category{catOne, catMany, catOther},
		fn: func(op operands) category {
			if op.n == 1 {
				return catOne
			}
			if isRoundMillion(op) {
				return catMany
			}
			return catOther
		},
	}

	// zero: n % 10 = 0 or n % 100 = 11..19 or v = 2 and f % 100 = 11..19
	// one:  n % 10 = 1 and n % 100 != 11
	//       or v = 2 and f % 10 = 1 and f % 100 != 11
	//       or v != 2 and f % 10 = 1
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

	// one: n % 10 = 1 and n % 100 != 11..19
	// few: n % 10 = 2..9 and n % 100 != 11..19
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

	// one: n % 10 = 1 and n % 100 != 11 · two: n = 2
	// few: n != 2 and n % 10 = 2..9 and n % 100 != 11..19 · many: f != 0
	patternSamogitian = &pattern{
		name:  "samogitian",
		langs: []string{"sgs"},
		cats:  []category{catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			n10, n100 := math.Mod(op.n, 10), math.Mod(op.n, 100)

			if n10 == 1 && n100 != 11 {
				return catOne
			}
			if op.n == 2 {
				return catTwo
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
	// few:  v = 0 and i % 10 = 2..4 and i % 100 != 12..14
	// many: v = 0 and (i % 10 = 0 or i % 10 = 5..9 or i % 100 = 11..14)
	//
	// CLDR files `ru` under this ruleset too. Not registered here.
	patternUkrainian = &pattern{
		name:  "ukrainian",
		langs: []string{"uk"},
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

	// one:  n % 10 = 1 and n % 100 != 11
	// few:  n % 10 = 2..4 and n % 100 != 12..14
	// many: n % 10 = 0 or n % 10 = 5..9 or n % 100 = 11..14
	//
	// Reads `n`, not `i`/`v`, so it is a different ruleset from ukrainian.
	patternBelarusian = &pattern{
		name:  "belarusian",
		langs: []string{"be"},
		cats:  []category{catOne, catFew, catMany, catOther},
		fn: func(op operands) category {
			n10, n100 := math.Mod(op.n, 10), math.Mod(op.n, 100)

			if n10 == 1 && n100 != 11 {
				return catOne
			}
			if inRangeF(n10, 2, 4) && !inRangeF(n100, 12, 14) {
				return catFew
			}
			if n10 == 0 || inRangeF(n10, 5, 9) || inRangeF(n100, 11, 14) {
				return catMany
			}
			return catOther
		},
	}

	// one:  i = 1 and v = 0
	// few:  v = 0 and i % 10 = 2..4 and i % 100 != 12..14
	// many: v = 0 and i != 1 and i % 10 = 0..1
	//       or v = 0 and i % 10 = 5..9
	//       or v = 0 and i % 100 = 12..14
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

	// one: i = 1 and v = 0 · few: i = 2..4 and v = 0 · many: v != 0
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
	// few: v = 0 and i % 100 = 3..4 or v != 0
	patternSlovenian = &pattern{
		name:  "slovenian",
		langs: []string{"sl"},
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

	// one: v = 0 and i % 100 = 1 or f % 100 = 1
	// two: v = 0 and i % 100 = 2 or f % 100 = 2
	// few: v = 0 and i % 100 = 3..4 or f % 100 = 3..4
	patternSorbian = &pattern{
		name:  "sorbian",
		langs: []string{"dsb", "hsb"},
		cats:  []category{catOne, catTwo, catFew, catOther},
		fn: func(op operands) category {
			switch {
			case (op.v == 0 && op.i%100 == 1) || op.f%100 == 1:
				return catOne
			case (op.v == 0 && op.i%100 == 2) || op.f%100 == 2:
				return catTwo
			case (op.v == 0 && inRange(op.i%100, 3, 4)) || inRange(op.f%100, 3, 4):
				return catFew
			}
			return catOther
		},
	}

	// one: v = 0 and i % 10 = 1 and i % 100 != 11 or f % 10 = 1 and f % 100 != 11
	// few: v = 0 and i % 10 = 2..4 and i % 100 != 12..14
	//      or f % 10 = 2..4 and f % 100 != 12..14
	patternCroatian = &pattern{
		name:  "croatian",
		langs: []string{"bs", "hr", "sr"}, // `sh` (Serbo-Croatian) withdrawn in 2000
		cats:  []category{catOne, catFew, catOther},
		fn: func(op operands) category {
			if op.v == 0 && op.i%10 == 1 && op.i%100 != 11 {
				return catOne
			}
			if op.f%10 == 1 && op.f%100 != 11 {
				return catOne
			}
			if op.v == 0 && inRange(op.i%10, 2, 4) && !inRange(op.i%100, 12, 14) {
				return catFew
			}
			if inRange(op.f%10, 2, 4) && !inRange(op.f%100, 12, 14) {
				return catFew
			}
			return catOther
		},
	}

	// one: i = 1 and v = 0
	// few: v != 0 or n = 0 or n != 1 and n % 100 = 1..19
	patternRomanian = &pattern{
		name:  "romanian",
		langs: []string{"ro"}, // `mo` (Moldavian) withdrawn from ISO 639-1 in 2008, merged into `ro`
		cats:  []category{catOne, catFew, catOther},
		fn: func(op operands) category {
			if op.i == 1 && op.v == 0 {
				return catOne
			}
			if op.v != 0 || op.n == 0 || (op.n != 1 && inRangeF(math.Mod(op.n, 100), 1, 19)) {
				return catFew
			}
			return catOther
		},
	}

	// one: n = 1 · two: n = 2 · few: n = 3..6 · many: n = 7..10
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

	// one: n = 1,11 · two: n = 2,12 · few: n = 3..10,13..19
	patternScottishGaelic = &pattern{
		name:  "scottish-gaelic",
		langs: []string{"gd"},
		cats:  []category{catOne, catTwo, catFew, catOther},
		fn: func(op operands) category {
			switch {
			case eqAny(op.n, 1, 11):
				return catOne
			case eqAny(op.n, 2, 12):
				return catTwo
			case inRangeF(op.n, 3, 10) || inRangeF(op.n, 13, 19):
				return catFew
			}
			return catOther
		},
	}

	// one: v = 0 and i % 10 = 1 · two: v = 0 and i % 10 = 2
	// few: v = 0 and i % 100 = 0,20,40,60,80 · many: v != 0
	patternManx = &pattern{
		name:  "manx",
		langs: []string{"gv"},
		cats:  []category{catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			if op.v != 0 {
				return catMany
			}
			switch {
			case op.i%10 == 1:
				return catOne
			case op.i%10 == 2:
				return catTwo
			case eqAny(float64(op.i%100), 0, 20, 40, 60, 80):
				return catFew
			}
			return catOther
		},
	}

	// one:  n % 10 = 1 and n % 100 != 11,71,91
	// two:  n % 10 = 2 and n % 100 != 12,72,92
	// few:  n % 10 = 3..4,9 and n % 100 != 10..19,70..79,90..99
	// many: n != 0 and n % 1000000 = 0
	patternBreton = &pattern{
		name:  "breton",
		langs: []string{"br"},
		cats:  []category{catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			n10, n100 := math.Mod(op.n, 10), math.Mod(op.n, 100)

			if n10 == 1 && !eqAny(n100, 11, 71, 91) {
				return catOne
			}
			if n10 == 2 && !eqAny(n100, 12, 72, 92) {
				return catTwo
			}
			if (inRangeF(n10, 3, 4) || n10 == 9) &&
				!inRangeF(n100, 10, 19) && !inRangeF(n100, 70, 79) && !inRangeF(n100, 90, 99) {
				return catFew
			}
			if op.n != 0 && math.Mod(op.n, 1000000) == 0 {
				return catMany
			}
			return catOther
		},
	}

	// one: n = 1 · two: n = 2 · few: n = 0 or n % 100 = 3..10
	// many: n % 100 = 11..19
	patternMaltese = &pattern{
		name:  "maltese",
		langs: []string{"mt"},
		cats:  []category{catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			n100 := math.Mod(op.n, 100)

			switch {
			case op.n == 1:
				return catOne
			case op.n == 2:
				return catTwo
			case op.n == 0 || inRangeF(n100, 3, 10):
				return catFew
			case inRangeF(n100, 11, 19):
				return catMany
			}
			return catOther
		},
	}

	// zero: n = 0 · one: n = 1
	// two:  n % 100 = 2,22,42,62,82
	//       or n % 1000 = 0 and n % 100000 = 1000..20000,40000,60000,80000
	//       or n != 0 and n % 1000000 = 100000
	// few:  n % 100 = 3,23,43,63,83
	// many: n != 1 and n % 100 = 1,21,41,61,81
	patternCornish = &pattern{
		name:  "cornish",
		langs: []string{"kw"},
		cats:  []category{catZero, catOne, catTwo, catFew, catMany, catOther},
		fn: func(op operands) category {
			n100, n1000 := math.Mod(op.n, 100), math.Mod(op.n, 1000)
			n100000, n1000000 := math.Mod(op.n, 100000), math.Mod(op.n, 1000000)

			switch {
			case op.n == 0:
				return catZero
			case op.n == 1:
				return catOne
			case eqAny(n100, 2, 22, 42, 62, 82):
				return catTwo
			case n1000 == 0 && (inRangeF(n100000, 1000, 20000) || eqAny(n100000, 40000, 60000, 80000)):
				return catTwo
			case op.n != 0 && n1000000 == 100000:
				return catTwo
			case eqAny(n100, 3, 23, 43, 63, 83):
				return catFew
			case op.n != 1 && eqAny(n100, 1, 21, 41, 61, 81):
				return catMany
			}
			return catOther
		},
	}

	// one: n = 1 · two: n = 2
	// The Sami languages of Norway, Sweden and Finland. CLDR files the
	// non-European iu/naq/sat under this same ruleset; they are not registered.
	patternNorthernSami = &pattern{
		name:  "northern-sami",
		langs: []string{"se", "sma", "smi", "smj", "smn", "sms"},
		cats:  []category{catOne, catTwo, catOther},
		fn: func(op operands) category {
			switch op.n {
			case 1:
				return catOne
			case 2:
				return catTwo
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
)

// patterns holds every transcribed ruleset. European languages only -- see the
// plan's scope. `ru` is not registered.
var patterns = []*pattern{
	patternEnglish,
	patternGreek,
	patternDanish,
	patternIcelandic,
	patternMacedonian,
	patternFrench,
	patternCatalan,
	patternSpanish,
	patternLatvian,
	patternLithuanian,
	patternSamogitian,
	patternUkrainian,
	patternBelarusian,
	patternPolish,
	patternCzech,
	patternSlovenian,
	patternSorbian,
	patternCroatian,
	patternRomanian,
	patternIrish,
	patternScottishGaelic,
	patternManx,
	patternBreton,
	patternMaltese,
	patternCornish,
	patternNorthernSami,
	patternWelsh,
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
