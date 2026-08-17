package blabla

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// keywords reserved in YML file for special operations
const keywordInclude = "include"

// string in value to copy same as key
const keywordSameAsKey string = "^"

const (
	NSingle uint = 0
	NMany   uint = 1
)

// BlaBla main type struct
type BlaBla struct {
	raw       map[string]map[string]translationLines
	languages map[string]func(str string, v ...any) string
	Errors    []error
}

// Load ..
func Load(fname string) (*BlaBla, error) {
	return load(fname, map[string]bool{})
}

// load is the base loader. `chain` holds the absolute paths of the files
// currently being loaded, so a file that includes an ancestor is rejected
// instead of recursing until the stack overflows.
func load(fname string, chain map[string]bool) (*BlaBla, error) {
	bla := &BlaBla{
		raw:       map[string]map[string]translationLines{},
		languages: map[string]func(str string, v ...any) string{},
	}

	abspath, err := filepath.Abs(fname)
	if err != nil {
		return nil, fmt.Errorf("resolving path `%s`: %v", fname, err)
	}
	if chain[abspath] {
		return nil, fmt.Errorf("circular include of `%s`", abspath)
	}
	chain[abspath] = true
	defer delete(chain, abspath) // siblings may include the same file, ancestors may not

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("reading YAML file: %v", err)
	}

	// Unmarshal the YAML into a generic map
	err = yaml.Unmarshal(data, bla.raw)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML `%s` file: %v", fname, err)
	}

	if subnamesByKey, isInclude := bla.raw[keywordInclude]; isInclude {
		delete(bla.raw, keywordInclude)
		bla.loadIncludes(filepath.Dir(fname), subnamesByKey, chain)
	}

	// collect translations
	for key, langs := range bla.raw {
		for lang, trline := range langs {
			_lang := strings.ToLower(lang)

			// normalize lang if needed `LV: hello` --> `lv: hello`
			if _lang != lang {
				delete(bla.raw[key], lang)   // delete invalid
				bla.raw[key][_lang] = trline // assign value
			}
			lang = _lang

			if len(trline.list) > 0 && trline.list[0] == keywordSameAsKey {
				trline.list[0] = key
			}

			// `^` in a mapping applies per form, not just at index 0.
			for cat, text := range trline.byCat {
				if text == keywordSameAsKey {
					trline.byCat[cat] = key
				}
			}

			if _, isAlready := bla.languages[lang]; !isAlready {
				bla.languages[lang] = nil
			}
		}
	}

	bla.Validate()
	return bla, nil
}

// loadIncludes merges every file listed under the `include:` key.
// Include keys are visited in sorted order so two includes defining the same
// translation key resolve the same way on every run.
func (bla *BlaBla) loadIncludes(basedir string, subnamesByKey map[string]translationLines, chain map[string]bool) {
	for _, subkey := range slices.Sorted(maps.Keys(subnamesByKey)) {
		for _, fsubname := range subnamesByKey[subkey].list {
			subbla, err := load(filepath.Join(basedir, fsubname), chain)
			if err != nil {
				log.Printf("Error: include failed: %s", err)
				continue
			}
			bla.mergeMissing(subbla.raw)
		}
	}
}

// mergeMissing adds translations that this file does not define itself.
// The including file always wins over the files it includes.
func (bla *BlaBla) mergeMissing(raw map[string]map[string]translationLines) {
	for key, langs := range raw {
		if _, isAlready := bla.raw[key]; !isAlready {
			bla.raw[key] = langs
		}
	}
}

// MustLoad finds and parse without errors
func MustLoad(fname string) *BlaBla {
	bla, err := Load(fname)
	if err != nil {
		log.Fatal(err)
	}
	return bla
}

// String ..
func (bla *BlaBla) String() string {
	s := "\n"

	s += strings.Repeat("-", 10) + "\n"
	s += fmt.Sprintf("Languages:\t %d %v\n", len(bla.languages), slices.Sorted(maps.Keys(bla.languages)))
	s += fmt.Sprintf("Translations:\t %d\n", len(bla.raw))
	s += fmt.Sprintf("Errors:\t %d\n", len(bla.Errors))
	s += strings.Repeat("-", 10) + "\n"

	return s
}

// Validate bla translations consistency
func (bla *BlaBla) Validate() []error {
	var langcounts = map[string]int{} // unique langs

	// collect language keys
	for _, langs := range bla.raw {
		for lang := range langs {
			langcounts[lang]++
		}
	}

	// check for missing translations
	bla.Errors = nil
	for key, langs := range bla.raw {
		for lang := range langcounts {
			if _, is := langs[lang]; !is {
				bla.Errors = append(bla.Errors, fmt.Errorf("Missing `%s` translation for `%s`", lang, key))
			}
		}
	}

	if len(bla.Errors) > 0 {
		return bla.Errors
	}

	return nil
}

// missingTranslation is the sentinel returned when a key/lang/index lookup misses.
// Tests assert on this exact format — see t_basic_test.go.
func missingTranslation(lang, key string) string {
	return "(" + lang + "." + key + ")"
}

func (bla *BlaBla) get(lang, key string, index uint, v ...any) string {
	lang = strings.ToLower(lang)
	line := bla.raw[key][lang]

	if uint(len(line.list)) < index+1 {
		return missingTranslation(lang, key)
	}

	return bla.format(lang, line.list[index], v...)
}

// format applies the language's custom parser, or the fmt verbs, to one line.
func (bla *BlaBla) format(lang, text string, v ...any) string {
	if fn := bla.languages[lang]; fn != nil {
		return fn(text, v...)
	}

	if len(v) > 0 && hasFormatVerb(text) {
		return fmt.Sprintf(text, v...)
	}

	return text
}

// getCategory is the base for every category-aware lookup.
//
// `fallback` is on only for Get: there the category was inferred from a count,
// so landing on `other` beats a sentinel. A forced getter passes it off --
// the caller named an exact form, and quietly returning a different one is a
// mistake nobody can see in the output.
func (bla *BlaBla) getCategory(lang, key string, cat category, fallback bool, v ...any) string {
	lang = strings.ToLower(lang)
	line := bla.raw[key][lang]

	text, isThere := line.byCat[cat]
	if !isThere && fallback {
		text, isThere = line.byCat[catOther]
	}
	if !isThere {
		return missingTranslation(lang, key)
	}

	return bla.format(lang, text, v...)
}

// categoryFor resolves the count in v to a category for lang.
func categoryFor(lang string, v []any) category {
	op, isCount := countOperands(v)
	if !isCount {
		return catOne // no count -> singular, same as the legacy path
	}

	return patternFor(lang).fn(op)
}

// hasFormatVerb reports whether s contains a fmt verb (`%d`, `%s`, ..).
// A singular form usually has none ("One item"), and Sprintf would then
// append `%!(EXTRA ..)` to it — so such lines are returned untouched.
func hasFormatVerb(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '%' {
			continue
		}
		if s[i+1] == '%' {
			i++ // escaped literal `%%`
			continue
		}
		return true
	}
	return false
}

// countArg returns the first numeric argument — the count that decides
// singular vs plural. Later numbers are format params (a price, an id) and
// must not hijack the choice. Uses fmt %v + ParseFloat (rather than reflect
// or a type switch) so it stays untyped — see CLAUDE.md.
func countArg(v []any) (float64, bool) {
	for _, arg := range v {
		n, err := strconv.ParseFloat(fmt.Sprintf("%v", arg), 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

// isPluralCount reports whether count n needs the plural form.
// Everything but exactly 1 is plural (0 items, -3 items, 0.5 items).
// This is the seam a future language-aware plural-rules engine should replace.
func isPluralCount(n float64) bool {
	return n != 1
}

// Get translation by guessing the plural form from the count
func (bla *BlaBla) Get(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	// Only a mapping block uses the CLDR rules. A scalar or a sequence keeps
	// the legacy `n != 1` routing, so no existing file changes its output.
	if bla.raw[key][lang].hasCategories() {
		return bla.getCategory(lang, key, categoryFor(lang, v), true, v...)
	}

	if len(bla.raw[key][lang].list) >= 2 {
		if n, isCount := countArg(v); isCount && isPluralCount(n) {
			return bla.get(lang, key, NMany, v...)
		}
	}

	return bla.get(lang, key, NSingle, v...)
}

// GetSingle translation forced to be singular
func (bla *BlaBla) GetSingle(lang, key string, v ...any) string {
	if bla.raw[key][strings.ToLower(lang)].hasCategories() {
		return bla.getCategory(lang, key, catOne, false, v...)
	}

	return bla.get(lang, key, NSingle, v...)
}

// GetPlural translation forced to be plural
func (bla *BlaBla) GetPlural(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	if bla.raw[key][lang].hasCategories() {
		return bla.getCategory(lang, key, catOther, false, v...)
	}

	if len(bla.raw[key][lang].list) < 2 {
		return missingTranslation(lang, key)
	}

	return bla.get(lang, key, NMany, v...)
}

// GetZero translation forced to the `zero` form
func (bla *BlaBla) GetZero(lang, key string, v ...any) string {
	return bla.getCategory(lang, key, catZero, false, v...)
}

// GetTwo translation forced to the `two` form
func (bla *BlaBla) GetTwo(lang, key string, v ...any) string {
	return bla.getCategory(lang, key, catTwo, false, v...)
}

// GetFew translation forced to the `few` form
func (bla *BlaBla) GetFew(lang, key string, v ...any) string {
	return bla.getCategory(lang, key, catFew, false, v...)
}

// GetMany translation forced to the `many` form
func (bla *BlaBla) GetMany(lang, key string, v ...any) string {
	return bla.getCategory(lang, key, catMany, false, v...)
}

// CustomParser ..
func (bla *BlaBla) CustomParser(lang string, fn func(str string, v ...any) string) error {
	lang = strings.ToLower(lang)

	if _, exists := bla.languages[lang]; !exists {
		return fmt.Errorf("no such language `%s` to add custom parser", lang)
	}

	bla.languages[lang] = fn
	return nil
}
