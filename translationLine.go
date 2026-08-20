package blabla

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// line in yaml file under key -> lang -> ....
//
// `list` holds the forms in file order. A scalar gives one entry, a sequence
// gives the legacy positional forms, and `include:` reuses this type for its
// filenames -- so `list` stays a plain ordered slice.
//
// `byCat` is filled only by a mapping node. Its presence is what switches a
// language block from the legacy `n != 1` routing to the CLDR rules, so an
// existing file is never touched by the new engine.
type translationLines struct {
	list  []string
	byCat map[category]string
}

// UnmarshalYAML accepts a scalar, a sequence, or a mapping of plural categories
func (trline *translationLines) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		// Handle single string
		*trline = translationLines{list: []string{value.Value}}
		return nil
	}

	if value.Kind == yaml.MappingNode {
		return trline.unmarshalCategories(value)
	}

	// Handle array of strings
	var arr []string
	if err := value.Decode(&arr); err != nil {
		return err
	}
	*trline = translationLines{list: arr}
	return nil
}

// unmarshalCategories reads a `zero:`/`one:`/`other:` style mapping. Keys are
// read as raw text, so numeric aliases (`0:`) never hit yaml's int-vs-string
// strictness. Order in the file is irrelevant -- the category names carry it.
func (trline *translationLines) unmarshalCategories(value *yaml.Node) error {
	*trline = translationLines{byCat: map[category]string{}}

	// A mapping node stores keys and values as alternating Content entries.
	for i := 0; i+1 < len(value.Content); i += 2 {
		keynode, valnode := value.Content[i], value.Content[i+1]

		key := strings.ToLower(strings.TrimSpace(keynode.Value))
		cat, isCategory := categoryByKey[key]
		if !isCategory {
			return fmt.Errorf("unknown plural category `%s`: expected one of zero, one, two, few, many, other (or 0, 1)", keynode.Value)
		}

		if valnode.Kind != yaml.ScalarNode {
			return fmt.Errorf("plural category `%s` must hold a single string", key)
		}

		trline.byCat[cat] = valnode.Value
		trline.list = append(trline.list, valnode.Value)
	}

	return nil
}

// hasCategories reports whether this block opted into the CLDR rules.
func (trline translationLines) hasCategories() bool {
	return trline.byCat != nil
}
