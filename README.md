# 💬 blabla

**blabla** is a lightweight translation package. 
It simplifies managing multilingual content in both Go code and templates, supporting YAML-based translation files and dynamic language switching.

This package is motivated by the need for a quick and simple translation solution, designed for easy integration into existing Go projects, using YAML as a straightforward and flexible source for translations.


## Translation YAML file

**YAML**-based translation file format for easy management.


```yaml
hello:
  en: Hello
  lv: Sveiki

plural.demo:
  en:
    - One item
    - "%d items"
  lv: 
    - Viena lieta
    - "%d lietas"
```

<details>
  <summary>YAML with all features (plural options, format params, includes)</summary>
  
<br/>

The `^` in a YAML file copies the text from the translation key. The key can either be used as the final text or as a unique token for translations below.

```yaml

params:
  en: "1=%d, 2=%0.2f 3=%s"
  lv: "1=%d, 2=%0.2f 3=%s"

Same English text from Key:
  en: ^
  lv: Tas pats teksts no key

# Include additional translation files
include:
  sub: sub.yml
  sub2: sub3.yml
```

### Plural forms

Two lines under a language mean singular and plural — enough for English, wrong
for most other languages. Write the language block as a **mapping** instead and
the form is picked by that language's own CLDR rules.

```yaml
items:
  en:                     # one, other
    one:   "%v item"
    other: "%v items"
  et:                     # same rules as English for Estonian
    one:   "%v asi"
    other: "%v asja"
  lv:                     # zero, one, other
    zero:  "%v lietu"     # 0, 10-20, 30, 100, 110..
    one:   "%v lieta"     # 1, 21, 31, 101..
    other: "%v lietas"    # 2-9, 22-29..
  lt:                     # one, few, many, other
    one:   "%v daiktas"   # 1, 21, 31..
    few:   "%v daiktai"   # 2-9, 22-29..
    many:  "%v daikto"    # decimals only
    other: "%v daiktų"    # 0, 10-19, 110-119..
```

```go
bla.Get("lv", "items", 1)    // "1 lieta"
bla.Get("lv", "items", 10)   // "10 lietu"
bla.Get("lv", "items", 21)   // "21 lieta"
bla.Get("lv", "items", 22)   // "22 lietas"
```

The six categories are `zero`, `one`, `two`, `few`, `many`, `other`. List only
the ones a language actually uses — Latvian has no `few`, English has no `zero`.

#### Short numeric keys

`0:` and `1:` are accepted as short forms of `zero:` and `one:`. Both spellings
may be mixed in the same block.

```yaml
# these two blocks behave identically
short:
  lv:
    0:     "%v lietu"
    1:     "%v lieta"
    other: "%v lietas"

full:
  lv:
    zero:  "%v lietu"
    one:   "%v lieta"
    other: "%v lietas"
```

`two:`, `few:`, `many:` and `other:` have no numeric alias — spell those by
name. There is deliberately no `2:`, because it would read as the `two`
category rather than as `other`.

```yaml
named:
  lt:
    1:     "%v daiktas"   # one
    few:   "%v daiktai"
    many:  "%v daikto"
    other: "%v daiktų"
```

> Careful: a number here is a **category name**, not a count and not a position.
> `0:` means the `zero` category, which Latvian uses for 0, 10-20, 30, 100 —
> not "when the count is 0". In the older sequence form the first line is the
> singular, so `0:` and the first list entry are *not* the same thing.

Language keys are [ISO 639 language codes](https://www.loc.gov/standards/iso639-2/php/code_list.php)
— two letters where one exists (`et` Estonian, `uk` Ukrainian), three otherwise
(`hsb` Upper Sorbian) — never ISO 3166 country codes.

A category the file does not define falls back to `other`, then to the
`(lang.key)` sentinel. The forced getters — `GetZero`, `GetSingle`, `GetTwo`,
`GetFew`, `GetMany`, `GetPlural` — never fall back, so asking for a form a
language does not have returns the sentinel rather than the wrong form.

> Sequences keep working exactly as before. Only a mapping opts into the plural
> rules, so existing translation files are unaffected.

</details>

## Load translations

```go
package main

import (
	"fmt"

	"github.com/bobiverse/blabla"
)

func main() {
	bla := blabla.MustLoad("tests/translations.yml") // panics if there's an error
	// bla, err := blabla.Load("..") // no panics

	lang := "lv"

	fmt.Println(bla.Get(lang, "hello"))                    // Outputs: "Sveiki"
	fmt.Println(bla.Get(lang, "params", 1, 2.02, "three")) // Outputs: "1=1, 2=2.02 3=three"
	fmt.Println(bla.Get(lang, "plural.demo", 5))           // Outputs: "5 items"

	fmt.Println(bla.Get("en", "Same English text from Key")) // Outputs: "Same English text from Key"
}
```

Check [examples/main.go](examples/main.go)!


## Use translations in Golang templates

You can also use the translation function directly inside Golang templates:

```go
template.FuncMap{
    "T":      bla.Get, // global `bla`
    // ...
}
```

```html
<p>{{ T "lv" "hello" }}</p>
<p>{{ T .User.Lang "hello" }}</p>
```

### Integrate into your struct
```go
type User struct {
    Lang string 
    // ..
}

func (user *User) T(s string) string {
    return bla.Get(user.Lang, s) // global `bla`
}
```

```html
<p>{{ .User.T "hello" }}</p>
```

This will output the translation based on the active language.

