# blabla

**blabla** is a lightweight translation package. 
It simplifies managing multilingual content in both Go code and templates, supporting YAML-based translation files and dynamic language switching.




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

Same English text from Key:
  en: ^
  lv: Taspats teksts no key

# Include additional translation files
include:
  sub: sub.yml
  sub2: sub3.yml
```

## Load translations

```go
package main

import (
    "fmt"
    "blabla"
)

func main() {
    blabla.MustLoad("translations.yml") // panics if there's an error
    
    fmt.Println(blabla.Get("lv", "hello")) // Outputs: "Sveiki"
    fmt.Println(blabla.Get("en", "Same English text from Key")) // Outputs: "Same English text from Key"
    fmt.Println(blabla.Get("lv", "plural.demo", []any{5})) // Outputs: "5 items"
}
```

## Use translations in Golang templates

You can also use the translation function directly inside Golang templates:

```go
template.FuncMap{
    "T":      blabla.Get,
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
    return blabla.Get(user.Lang, s)
}
```

```html
<p>{{ .User.T "hello" }}</p>
```

This will output the translation based on the active language.
