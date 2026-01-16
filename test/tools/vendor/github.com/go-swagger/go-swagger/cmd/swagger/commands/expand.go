package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	flags "github.com/jessevdk/go-flags"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

// ExpandSpec is a command that expands the $refs in a swagger document.
//
// There are no specific options for this expansion.
type ExpandSpec struct {
	Compact bool           `long:"compact" description:"applies to JSON formatted specs. When present, doesn't prettify the json"`
	Output  flags.Filename `long:"output" short:"o" description:"the file to write to"`
	Format  string         `long:"format" description:"the format for the spec document" default:"json" choice:"yaml" choice:"json"`
}

// Execute expands the spec
func (c *ExpandSpec) Execute(args []string) error {
	if len(args) != 1 {
		return errors.New("expand command requires the single swagger document url to be specified")
	}

	swaggerDoc := args[0]
	specDoc, err := loads.Spec(swaggerDoc)
	if err != nil {
		return err
	}

	exp, err := specDoc.Expanded()
	if err != nil {
		return err
	}

	return writeToFile(exp.Spec(), !c.Compact, c.Format, string(c.Output))
}

var defaultWriter io.Writer = os.Stdout

func writeToFile(swspec *spec.Swagger, pretty bool, format string, output string) error {
	var b []byte
	var err error
	asJSON := format == "json"

	log.Println("format = ", format)
	switch {
	case pretty && asJSON:
		b, err = json.MarshalIndent(swspec, "", "  ")
	case asJSON:
		b, err = json.Marshal(swspec)
	default:
		// marshals as YAML
		b, err = json.Marshal(swspec)
		if err == nil {
			var data swag.JSONMapSlice
			if erg := json.Unmarshal(b, &data); erg != nil {
				log.Fatalln(erg)
			}
			var bb any
			bb, err = data.MarshalYAML()
			if err == nil {
				b = bb.([]byte)
			}
		}
	}

	if err != nil {
		return err
	}

	switch output {
	case "", "-":
		_, e := fmt.Fprintf(defaultWriter, "%s\n", b)
		return e
	default:
		return os.WriteFile(output, b, 0o644) //#nosec
	}
}
