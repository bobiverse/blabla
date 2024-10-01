package blabla

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"

	"gopkg.in/yaml.v3"
)

// keywords reserved in YML file for special operations
const keywordInclude = "include"

// string in value to copy same as key
const keywordSameAsKey string = "^"

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
				for key2, langs2 := range subbla.raw {
					bla.raw[key2] = langs2
				}
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
	s += fmt.Sprintf("Languages:\t %d %v\n", len(bla.languages), maps.Keys(bla.languages))
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
			lang = strings.ToLower(lang)
			if _, is := langcounts[lang]; !is {
				langcounts[lang] = 0
			}
			langcounts[lang]++
		}
	}

	// check for missing translations
	bla.Errors = nil
	for key, langs := range bla.raw {
		for lang := range langcounts {
			lang = strings.ToLower(lang)
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

func (bla *BlaBla) get(lang, key string, index int, v ...any) string {
	lang = strings.ToLower(lang)

	if fn, _ := bla.languages[lang]; fn != nil {
		return fn(bla.raw[key][lang][index], v...)
	}

	if len(v) > 0 {
		return fmt.Sprintf(bla.raw[key][lang][index], v...)
	}

	if len(bla.raw[key][lang]) < index+1 {
		return "(" + lang + "." + key + ")"
	}

	return bla.raw[key][lang][index]
}

// Get translation by guessign single/plural
func (bla *BlaBla) Get(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	// Check if any of the variadic arguments is numeric and greater than 1
	for _, value := range v {
		switch v := value.(type) {
		case int, int8, int16, int32, int64:
			if reflect.ValueOf(v).Int() > 1 {
				return bla.GetPlural(lang, key, v)
			}
		case uint, uint8, uint16, uint32, uint64:
			if reflect.ValueOf(v).Uint() > 1 {
				return bla.GetPlural(lang, key, v)
			}
		case float32, float64:
			if reflect.ValueOf(v).Float() > 1.0 {
				return bla.GetPlural(lang, key, v)
			}
		}
	}

	return bla.get(lang, key, 0, v...)
}

// GetSingle translation forced to be single
func (bla *BlaBla) GetSingle(lang, key string, v ...any) string {
	return bla.get(lang, key, 0, v...) // plural
}

// GetPlural translation forced to be plural
func (bla *BlaBla) GetPlural(lang, key string, v ...any) string {
	lang = strings.ToLower(lang)

	if len(bla.raw[key][lang]) != 2 {
		return "(" + lang + "." + key + ")"
	}

	return bla.get(lang, key, 1, v...) // plural
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
