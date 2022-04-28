package common_test

import (
	"testing"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type Car struct {
	Brand string
	Stats struct {
		HP           *int
		Displacement int
	}
	Extras map[string]string
}

type Anonymous struct {
	Hello string
}

func (c Car) Type() string {
	return ""
}

func (c Car) Color() string {
	return ""
}

func TestAutocompleteFormat(t *testing.T) {
	testStruct := struct {
		Name string
		Age  int
		Car  *Car
		Car2 *Car
		*Anonymous
	}{}

	testStruct.Car = &Car{}
	testStruct.Car.Extras = map[string]string{"test": "1"}

	tests := []struct {
		name       string
		toComplete string
		expected   []string
	}{
		{
			"empty completion",
			"",
			[]string{"json"},
		},
		{
			"json completion",
			"json",
			[]string{"json"},
		},
		{
			"invalid completion",
			"blahblah",
			nil,
		},
		{
			"invalid completion",
			"{{",
			nil,
		},
		{
			"invalid completion",
			"{{  ",
			nil,
		},
		{
			"invalid completion",
			"{{  ..",
			nil,
		},
		{
			"fist level struct field name",
			"{{.",
			[]string{"{{.Name}}", "{{.Age}}", "{{.Car.", "{{.Car2.", "{{.Hello}}"},
		},
		{
			"fist level struct field name",
			"{{ .",
			[]string{"{{ .Name}}", "{{ .Age}}", "{{ .Car.", "{{ .Car2.", "{{ .Hello}}"},
		},
		{
			"fist level struct field name",
			"{{ .N",
			[]string{"{{ .Name}}"},
		},
		{
			"second level struct field name",
			"{{ .Car.",
			[]string{"{{ .Car.Brand}}", "{{ .Car.Stats.", "{{ .Car.Extras}}", "{{ .Car.Color}}", "{{ .Car.Type}}"},
		},
		{
			"second level struct field name",
			"{{ .Car.B",
			[]string{"{{ .Car.Brand}}"},
		},
		{
			"second level nil struct field name",
			"{{ .Car2.",
			[]string{"{{ .Car2.Brand}}", "{{ .Car2.Stats.", "{{ .Car2.Extras}}", "{{ .Car2.Color}}", "{{ .Car2.Type}}"},
		},
		{
			"three level struct field name",
			"{{ .Car.Stats.",
			[]string{"{{ .Car.Stats.HP}}", "{{ .Car.Stats.Displacement}}"},
		},
		{
			"three level struct field name",
			"{{ .Car.Stats.D",
			[]string{"{{ .Car.Stats.Displacement}}"},
		},
		{
			"second level struct field name",
			"{{ .Car.B",
			[]string{"{{ .Car.Brand}}"},
		},
		{
			"invalid field name",
			"{{ .Ca.B",
			nil,
		},
		{
			"map key names don't work",
			"{{ .Car.Extras.",
			nil,
		},
		{
			"two variables struct field name",
			"{{ .Car.Brand }} {{ .Car.",
			[]string{"{{ .Car.Brand }} {{ .Car.Brand}}", "{{ .Car.Brand }} {{ .Car.Stats.", "{{ .Car.Brand }} {{ .Car.Extras}}",
				"{{ .Car.Brand }} {{ .Car.Color}}", "{{ .Car.Brand }} {{ .Car.Type}}"},
		},
		{
			"only dot without variable",
			".",
			nil,
		},
	}

	for _, test := range tests {
		completion, directive := common.AutocompleteFormat(testStruct)(nil, nil, test.toComplete)
		// directive should always be greater than ShellCompDirectiveNoFileComp
		assert.GreaterOrEqual(t, directive, cobra.ShellCompDirectiveNoFileComp, "unexpected ShellCompDirective")
		assert.Equal(t, test.expected, completion, test.name)
	}
}
