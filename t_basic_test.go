package blabla

import (
	"errors"
	"testing"
)

func TestBasic(t *testing.T) {

	// Define test cases
	testCases := []struct {
		lang, key, expected string
		params              []any
	}{
		{"en", "hello", "Hello", nil},
		{"lv", "hello", "Sveiki", nil},

		{"en", "plural.demo", "One item", nil},
		{"en", "plural.demo", "5 items", []any{5}},

		{"en", "sub.hello", "Sub Hello", nil},
		{"lv", "sub.hello", "Sub Sveiki", nil},

		{"en", "sub3.hello", "Sub3 Hello", nil},
		{"lv", "sub3.hello", "Sub3 Sveiki", nil},

		{"en", "Same English text from Key", "Same English text from Key", nil},
		{"lv", "Same English text from Key", "Tas pats teksts no key", nil},

		{"en", "params", "1=1, 2=2.02 3=three", []any{1, 2.02, "three"}}, // test default case

		{"en", "EMPTY", "(en.EMPTY)", nil}, // test default case

		{"en", "haiku", "An old silent pond\nA frog jumps into the pond—\nSplash! Silence again.\n", nil}, // note `\n` nee lines
	}

	bla := MustLoad("tests/translations.yml")

	if bla.Errors != nil {
		t.Fatalf("Validation errors: %s", errors.Join(bla.Errors...))
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.lang+"_"+tc.key, func(t *testing.T) {
			result := bla.Get(tc.lang, tc.key, tc.params...)
			if result != tc.expected {
				t.Errorf("Expected `%s` but got `%s`", tc.expected, result)
			}
		})
	}
}
