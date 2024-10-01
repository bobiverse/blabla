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

The `^` in a YAML file copies the text from the translation key. The key can either be used as the final text or as a unique token for translations below.

```yaml
hello:
  en: Hello
  lv: Sveiki

params:
  en: "1=%d, 2=%0.2f 3=%s"
  lv: "1=%d, 2=%0.2f 3=%s"

plural.demo:
  en:
    - One item
    - "%d items"
  lv: 
    - Viena lieta
    - "%d lietas"

Same English text from Key:
  en: ^
  lv: Tas pats teksts no key



# Include additional translation files
include:
  sub: sub.yml
  sub2: sub3.yml
```
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


## Roadmap

- [ ] **Implement advanced pluralization logic**  
  Enhance the pluralization logic to support language-specific rules. For example, in **Latvian**, numbers like `1` and `21` use the singular form (`lieta`), while `5` and `22` use the plural form (`lietas`), and numbers like `10` and `15` use a different genitive plural form (`lietu`). Similarly, **Icelandic** uses a singular form for `1`, dual forms for `2-4`, and a plural form for `5` and above. This logic should cover edge cases across all supported languages.

