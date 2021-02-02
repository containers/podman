package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/onsi/ginkgo/ginkgo/outline"
)

const (
	// indentWidth is the width used by the 'indent' output
	indentWidth = 4
	// stdinAlias is a portable alias for stdin. This convention is used in
	// other CLIs, e.g., kubectl.
	stdinAlias   = "-"
	usageCommand = "ginkgo outline <filename>"
)

func BuildOutlineCommand() *Command {
	const defaultFormat = "csv"
	var format string
	flagSet := flag.NewFlagSet("outline", flag.ExitOnError)
	flagSet.StringVar(&format, "format", defaultFormat, "Format of outline. Accepted: 'csv', 'indent', 'json'")
	return &Command{
		Name:         "outline",
		FlagSet:      flagSet,
		UsageCommand: usageCommand,
		Usage: []string{
			"Create an outline of Ginkgo symbols for a file",
			"To read from stdin, use: `ginkgo outline -`",
			"Accepts the following flags:",
		},
		Command: func(args []string, additionalArgs []string) {
			outlineFile(args, format)
		},
	}
}

func outlineFile(args []string, format string) {
	if len(args) != 1 {
		println(fmt.Sprintf("usage: %s", usageCommand))
		os.Exit(1)
	}

	filename := args[0]
	var src *os.File
	if filename == stdinAlias {
		src = os.Stdin
	} else {
		var err error
		src, err = os.Open(filename)
		if err != nil {
			println(fmt.Sprintf("error opening file: %s", err))
			os.Exit(1)
		}
	}

	fset := token.NewFileSet()

	parsedSrc, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		println(fmt.Sprintf("error parsing source: %s", err))
		os.Exit(1)
	}

	o, err := outline.FromASTFile(fset, parsedSrc)
	if err != nil {
		println(fmt.Sprintf("error creating outline: %s", err))
		os.Exit(1)
	}

	var oerr error
	switch format {
	case "csv":
		_, oerr = fmt.Print(o)
	case "indent":
		_, oerr = fmt.Print(o.StringIndent(indentWidth))
	case "json":
		b, err := json.Marshal(o)
		if err != nil {
			println(fmt.Sprintf("error marshalling to json: %s", err))
		}
		_, oerr = fmt.Println(string(b))
	default:
		complainAndQuit(fmt.Sprintf("format %s not accepted", format))
	}
	if oerr != nil {
		println(fmt.Sprintf("error writing outline: %s", oerr))
		os.Exit(1)
	}
}
