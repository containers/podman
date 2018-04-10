package formats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
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

func setJSONFormatEncoder(isTerminal bool, w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	if isTerminal {
		enc.SetEscapeHTML(false)
	}
	return enc
}

// Out method for JSON Arrays
func (j JSONStructArray) Out() error {
	buf := bytes.NewBuffer(nil)
	enc := setJSONFormatEncoder(terminal.IsTerminal(int(os.Stdout.Fd())), buf)
	if err := enc.Encode(j.Output); err != nil {
		return err
	}
	data := buf.Bytes()

	// JSON returns a byte array with a literal null [110 117 108 108] in it
	// if it is passed empty data.  We used bytes.Compare to see if that is
	// the case.
	if diff := bytes.Compare(data, []byte("null")); diff == 0 {
		data = []byte("[]")
	}

	// If the we did get NULL back, we should spit out {} which is
	// at least valid JSON for the consumer.
	fmt.Printf("%s", data)
	humanNewLine()
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
	for i, img := range t.Output {
		basicTmpl := tmpl.Funcs(basicFunctions)
		err = basicTmpl.Execute(w, img)
		if err != nil {
			return err
		}
		if i != len(t.Output)-1 {
			fmt.Fprintln(w, "")
			continue
		}
		// Only print new line at the end of the output if stdout is the terminal
		if terminal.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Fprintln(w, "")
		}
	}
	return w.Flush()
}

// Out method for JSON struct
func (j JSONStruct) Out() error {
	data, err := json.MarshalIndent(j.Output, "", "    ")
	if err != nil {
		return err
	}
	fmt.Printf("%s", data)
	humanNewLine()
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
	humanNewLine()
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
	fmt.Printf("%s", string(buf))
	humanNewLine()
	return nil
}

// humanNewLine prints a new line at the end of the output only if stdout is the terminal
func humanNewLine() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println()
	}
}
