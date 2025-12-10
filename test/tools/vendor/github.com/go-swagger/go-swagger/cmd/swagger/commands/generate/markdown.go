package generate

import (
	"github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/generator"
)

// Markdown generates a markdown representation of the spec
type Markdown struct {
	WithShared
	WithModels
	WithOperations

	Output flags.Filename `long:"output" short:"" description:"the file to write the generated markdown." default:"markdown.md"`
}

func (m Markdown) apply(opts *generator.GenOpts) {
	m.Shared.apply(opts)
	m.Models.apply(opts)
	m.Operations.apply(opts)
}

func (m *Markdown) generate(opts *generator.GenOpts) error {
	return generator.GenerateMarkdown(string(m.Output), m.Models.Models, m.Operations.Operations, opts)
}

func (m Markdown) log(_ string) {
}

// Execute runs this command
func (m *Markdown) Execute(_ []string) error {
	return createSwagger(m)
}
