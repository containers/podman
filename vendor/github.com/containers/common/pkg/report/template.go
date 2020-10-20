package report

import (
	"reflect"
	"strings"
	"text/template"

	"github.com/containers/common/pkg/report/camelcase"
)

// Template embeds template.Template to add functionality to methods
type Template struct {
	*template.Template
	isTable bool
}

// FuncMap is aliased from template.FuncMap
type FuncMap template.FuncMap

// tableReplacer will remove 'table ' prefix and clean up tabs
var tableReplacer = strings.NewReplacer(
	"table ", "",
	`\t`, "\t",
	`\n`, "\n",
	" ", "\t",
)

// escapedReplacer will clean up escaped characters from CLI
var escapedReplacer = strings.NewReplacer(
	`\t`, "\t",
	`\n`, "\n",
)

// NormalizeFormat reads given go template format provided by CLI and munges it into what we need
func NormalizeFormat(format string) string {
	var f string
	// two replacers used so we only remove the prefix keyword `table`
	if strings.HasPrefix(format, "table ") {
		f = tableReplacer.Replace(format)
	} else {
		f = escapedReplacer.Replace(format)
	}

	if !strings.HasSuffix(f, "\n") {
		f += "\n"
	}

	return f
}

// Headers queries the interface for field names.
// Array of map is returned to support range templates
// Note: unexported fields can be supported by adding field to overrides
// Note: It is left to the developer to write out said headers
//       Podman commands use the general rules of:
//       1) unchanged --format includes headers
//       2) --format '{{.ID}"        # no headers
//       3) --format 'table {{.ID}}' # includes headers
func Headers(object interface{}, overrides map[string]string) []map[string]string {
	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Column header will be field name upper-cased.
	headers := make(map[string]string, value.NumField())
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		// Recurse to find field names from promoted structs
		if field.Type.Kind() == reflect.Struct && field.Anonymous {
			h := Headers(reflect.New(field.Type).Interface(), nil)
			for k, v := range h[0] {
				headers[k] = v
			}
			continue
		}
		name := strings.Join(camelcase.Split(field.Name), " ")
		headers[field.Name] = strings.ToUpper(name)
	}

	if len(overrides) > 0 {
		// Override column header as provided
		for k, v := range overrides {
			headers[k] = strings.ToUpper(v)
		}
	}
	return []map[string]string{headers}
}

// NewTemplate creates a new template object
func NewTemplate(name string) *Template {
	return &Template{template.New(name), false}
}

// Parse parses text as a template body for t
func (t *Template) Parse(text string) (*Template, error) {
	if strings.HasPrefix(text, "table ") {
		t.isTable = true
		text = "{{range .}}" + NormalizeFormat(text) + "{{end}}"
	}

	tt, err := t.Template.Parse(text)
	return &Template{tt, t.isTable}, err
}

// Funcs adds the elements of the argument map to the template's function map
func (t *Template) Funcs(funcMap FuncMap) *Template {
	return &Template{t.Template.Funcs(template.FuncMap(funcMap)), t.isTable}
}

// IsTable returns true if format string defines a "table"
func (t *Template) IsTable() bool {
	return t.isTable
}
