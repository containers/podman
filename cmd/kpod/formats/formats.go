package formats

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"bytes"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

const (
	// JSONString const to save on duplicate variable names
	JSONString = "json"
	// IDString const to save on duplicates for Go templates
	IDString = "{{.ID}}"
)

// Writer interface for outputs
type Writer interface {
	Out() error
}

// JSONStructArray for JSON output
type JSONStructArray struct {
	Output []interface{}
}

// StdoutTemplateArray for Go template output
type StdoutTemplateArray struct {
	Output   []interface{}
	Template string
	Fields   map[string]string
}

// JSONStruct for JSON output
type JSONStruct struct {
	Output interface{}
}

// StdoutTemplate for Go template output
type StdoutTemplate struct {
	Output   interface{}
	Template string
	Fields   map[string]string
}

// YAMLStruct for YAML output
type YAMLStruct struct {
	Output interface{}
}

// Out method for JSON Arrays
func (j JSONStructArray) Out() error {
	data, err := json.MarshalIndent(j.Output, "", "    ")
	if err != nil {
		return err
	}

	// JSON returns a byte array with a literal null [110 117 108 108] in it
	// if it is passed empty data.  We used bytes.Compare to see if that is
	// the case.
	if diff := bytes.Compare(data, []byte("null")); diff == 0 {
		data = []byte("[]")
	}

	// If the we did get NULL back, we should spit out {} which is
	// at least valid JSON for the consumer.
	fmt.Printf("%s\n", data)
	return nil
}

// Out method for Go templates
func (t StdoutTemplateArray) Out() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if strings.HasPrefix(t.Template, "table") {
		// replace any spaces with tabs in template so that tabwriter can align it
		t.Template = strings.Replace(strings.TrimSpace(t.Template[5:]), " ", "\t", -1)
		headerTmpl, err := template.New("header").Funcs(headerFunctions).Parse(t.Template)
		if err != nil {
			return errors.Wrapf(err, "Template parsing error")
		}
		err = headerTmpl.Execute(w, t.Fields)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}
	t.Template = strings.Replace(t.Template, " ", "\t", -1)
	tmpl, err := template.New("image").Funcs(basicFunctions).Parse(t.Template)
	if err != nil {
		return errors.Wrapf(err, "Template parsing error")
	}
	for _, img := range t.Output {
		basicTmpl := tmpl.Funcs(basicFunctions)
		err = basicTmpl.Execute(w, img)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}
	return w.Flush()
}

// Out method for JSON struct
func (j JSONStruct) Out() error {
	data, err := json.MarshalIndent(j.Output, "", "    ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return nil
}

//Out method for Go templates
func (t StdoutTemplate) Out() error {
	tmpl, err := template.New("image").Parse(t.Template)
	if err != nil {
		return errors.Wrapf(err, "template parsing error")
	}
	err = tmpl.Execute(os.Stdout, t.Output)
	if err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// Out method for YAML
func (y YAMLStruct) Out() error {
	var buf []byte
	var err error
	buf, err = yaml.Marshal(y.Output)
	if err != nil {
		return err
	}
	fmt.Println(string(buf))
	return nil
}
