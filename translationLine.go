package blabla

import (
	"gopkg.in/yaml.v3"
)

// line in yaml file under key -> lang -> ....
type translationLines []string

// UnmarshalYAML is a custom unmarshaller for StringOrArray
func (trline *translationLines) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		// Handle single string
		*trline = translationLines{value.Value}
		return nil
	}

	// if value.Kind == yaml.SequenceNode {
	// Handle array of strings
	var arr []string
	if err := value.Decode(&arr); err != nil {
		return err
	}
	*trline = arr
	return nil
}
