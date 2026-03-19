package generate

import "github.com/go-swagger/go-swagger/generator"

type Cli struct {
	// generate a cli includes all client code
	Client
	// cmd/<cli-app-name>/main.go will be generated. This ensures that go install will compile the app with desired name.
	CliAppName string `long:"cli-app-name" description:"the app name for the cli executable. useful for go install." default:"cli"`
	CliPackage string `long:"cli-package" description:"the package to save the cli specific code" default:"cli"`
}

func (c Cli) apply(opts *generator.GenOpts) {
	c.Client.apply(opts)
	opts.IncludeCLi = true
	opts.CliPackage = c.CliPackage
	opts.CliAppName = c.CliAppName
}

func (c *Cli) generate(opts *generator.GenOpts) error {
	return c.Client.generate(opts)
}

// Execute runs this command
func (c *Cli) Execute(_ []string) error {
	return createSwagger(c)
}
