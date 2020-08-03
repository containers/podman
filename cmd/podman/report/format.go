package report

import (
	"reflect"
	"strings"
)

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
	f := format
	// two replacers used so we only remove the prefix keyword `table`
	if strings.HasPrefix(f, "table ") {
		f = tableReplacer.Replace(f)
	} else {
		f = escapedReplacer.Replace(format)
	}

	if !strings.HasSuffix(f, "\n") {
		f += "\n"
	}

	return f
}

// Headers queries the interface for field names
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
		headers[field.Name] = strings.ToUpper(field.Name)
	}

	if len(overrides) > 0 {
		// Override column header as provided
		for k, v := range overrides {
			headers[k] = strings.ToUpper(v)
		}
	}
	return []map[string]string{headers}
}
