package validate

import (
	"fmt"
	"strings"
)

// Honors cobra.Value interface
type choiceValue struct {
	value   *string
	choices []string
}

// ChoiceValue may be used in cobra FlagSet methods Var/VarP/VarPF() to select from a set of values
//
// Example:
// 	created := validate.ChoiceValue(&opts.Sort, "command", "created", "id", "image", "names", "runningfor", "size", "status")
//	flags.Var(created, "sort", "Sort output by: "+created.Choices())
func ChoiceValue(p *string, choices ...string) *choiceValue {
	return &choiceValue{
		value:   p,
		choices: choices,
	}
}

func (c *choiceValue) String() string {
	return *c.value
}

func (c *choiceValue) Set(value string) error {
	for _, v := range c.choices {
		if v == value {
			*c.value = value
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid value.  Choose from: %q", value, c.Choices())
}

func (c *choiceValue) Choices() string {
	return strings.Join(c.choices, ", ")
}

func (c *choiceValue) Type() string {
	return "choice"
}
