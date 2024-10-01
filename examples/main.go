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
}
