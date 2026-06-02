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
	bla := &BlaBla{
		raw:       map[string]map[string]translationLines{},
		languages: map[string]func(str string, v ...any) string{},
	}

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("reading YAML file: %v", err)
	}

	// Unmarshal the YAML into a generic map
	err = yaml.Unmarshal(data, bla.raw)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML `%s` file: %v", fname, err)
	}

	// structure/prepare data
	for key, langs := range bla.raw {

		// include another file
		if key == keywordInclude {
			basedir := filepath.Dir(fname)
			for _, fsubnames := range langs {
				if len(fsubnames) == 0 {
					continue
				}

				fsubname := filepath.Join(basedir, fsubnames[0])

				// fmt.Printf("- INCLUDE: %s %v\n", key, fsubname)

				subbla, err2 := Load(fsubname)
				if err2 != nil {
					log.Printf("Error: include failed: %s", err2)
					continue
				}
				maps.Copy(bla.raw, subbla.raw)
			}
			delete(bla.raw, key)
			continue
		}
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

			if len(trline) > 0 && trline[0] == keywordSameAsKey {
				trline[0] = key
			}

			if _, isAlready := bla.languages[lang]; !isAlready {
				bla.languages[lang] = nil
			}
		}
	}

	bla.Validate()
	return bla, nil
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

	if uint(len(line)) < index+1 {
		return missingTranslation(lang, key)
	}

	if fn := bla.languages[lang]; fn != nil {
		return fn(line[index], v...)
	}

	if len(v) > 0 {
		return fmt.Sprintf(line[index], v...)
	}

	return line[index]
}

// isPluralCount reports whether v looks like a numeric count > 1.
// Uses fmt %v + ParseFloat (rather than reflect or a type switch) so it
// stays untyped — see CLAUDE.md. This is the seam a future language-aware
// plural-rules engine should replace.
func isPluralCount(v any) bool {
	s := fmt.Sprintf("%v", v)
	n, err := strconv.ParseFloat(s, 64)
	return err == nil && n > 1
}

// Get translation by guessing single/plural
func (bla *BlaBla) Get(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	if len(bla.raw[key][lang]) >= 2 && slices.ContainsFunc(v, isPluralCount) {
		return bla.get(lang, key, NMany, v...)
	}

	return bla.get(lang, key, NSingle, v...)
}

// GetSingle translation forced to be singular
func (bla *BlaBla) GetSingle(lang, key string, v ...any) string {
	return bla.get(lang, key, NSingle, v...)
}

// GetPlural translation forced to be plural
func (bla *BlaBla) GetPlural(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	if len(bla.raw[key][lang]) < 2 {
		return missingTranslation(lang, key)
	}

	return bla.get(lang, key, NMany, v...)
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
