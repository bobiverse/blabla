package main

import (
	"fmt"

	"github.com/bobiverse/blabla"
)

func main() {
	bla := blabla.MustLoad("../tests/translations.yml") // panics if there's an error
	// bla, err := blabla.Load("..") // no panics

	lang := "lv"

	fmt.Println(bla.Get(lang, "hello"))                    // Outputs: "Sveiki"
	fmt.Println(bla.Get(lang, "params", 1, 2.02, "three")) // Outputs: "1=1, 2=2.02 3=three"
	fmt.Println(bla.Get(lang, "plural.demo", 5))           // Outputs: "5 items"

	fmt.Println(bla.Get("en", "Same English text from Key")) // Outputs: "Same English text from Key"

	pluralForms()
}

// pluralForms shows CLDR plural categories. Each language picks its own form
// for the same count, from one key.
func pluralForms() {
	bla := blabla.MustLoad("../tests/plural.yml")

	fmt.Printf("\n%6s   %-10s   %-10s   %-11s   %s\n", "count", "en", "et", "lv", "lt")
	for _, n := range []any{0, 1, 2, 10, 11, 21, 22, 101, "1.5"} {
		fmt.Printf("%6v   %-10s   %-10s   %-11s   %s\n", n,
			bla.Get("en", "items", n),
			bla.Get("et", "items", n),
			bla.Get("lv", "items", n),
			bla.Get("lt", "items", n))
	}

	// A form the language does not define is never invented.
	fmt.Println("\n" + bla.GetFew("lv", "items", 5)) // Outputs: "(lv.items)" -- lv has no `few` form

	// A language the key is not translated into returns the sentinel.
	fmt.Println(bla.Get("fr", "items", 2)) // Outputs: "(fr.items)" -- no `fr` block on this key
}
