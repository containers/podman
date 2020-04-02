package registry

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// DefaultAPIAddress is the default address of the REST socket
const DefaultAPIAddress = "unix:/run/podman/podman.sock"

// DefaultVarlinkAddress is the default address of the varlink socket
const DefaultVarlinkAddress = "unix:/run/podman/io.podman"

type CliCommand struct {
	Mode    []entities.EngineMode
	Command *cobra.Command
	Parent  *cobra.Command
}

const ExecErrorCodeGeneric = 125

var (
	cliCtx          context.Context
	containerEngine entities.ContainerEngine
	exitCode        = ExecErrorCodeGeneric
	imageEngine     entities.ImageEngine

	Commands      []CliCommand
	EngineOptions entities.EngineOptions
)

func SetExitCode(code int) {
	exitCode = code
}

func GetExitCode() int {
	return exitCode
}

// HelpTemplate returns the help template for podman commands
// This uses the short and long options.
// command should not use this.
func HelpTemplate() string {
	return `{{.Short}}

Description:
  {{.Long}}

{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// UsageTemplate returns the usage template for podman commands
// This blocks the displaying of the global options. The main podman
// command should not use this.
func UsageTemplate() string {
	return `Usage(v2):{{if (and .Runnable (not .HasAvailableSubCommands))}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}
`
}

func ImageEngine() entities.ImageEngine {
	return imageEngine
}

// NewImageEngine is a wrapper for building an ImageEngine to be used for PreRunE functions
func NewImageEngine(cmd *cobra.Command, args []string) (entities.ImageEngine, error) {
	if imageEngine == nil {
		EngineOptions.FlagSet = cmd.Flags()
		engine, err := infra.NewImageEngine(EngineOptions)
		if err != nil {
			return nil, err
		}
		imageEngine = engine
	}
	return imageEngine, nil
}

func ContainerEngine() entities.ContainerEngine {
	return containerEngine
}

// NewContainerEngine is a wrapper for building an ContainerEngine to be used for PreRunE functions
func NewContainerEngine(cmd *cobra.Command, args []string) (entities.ContainerEngine, error) {
	if containerEngine == nil {
		EngineOptions.FlagSet = cmd.Flags()
		engine, err := infra.NewContainerEngine(EngineOptions)
		if err != nil {
			return nil, err
		}
		containerEngine = engine
	}
	return containerEngine, nil
}

func SubCommandExists(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf("unrecognized command `%[1]s %[2]s`\nTry '%[1]s --help' for more information.", cmd.CommandPath(), args[0])
	}
	return errors.Errorf("missing command '%[1]s COMMAND'\nTry '%[1]s --help' for more information.", cmd.CommandPath())
}

// IdOrLatestArgs used to validate a nameOrId was provided or the "--latest" flag
func IdOrLatestArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 0 && !cmd.Flag("latest").Changed) {
		return errors.New(`command requires a name, id  or the "--latest" flag`)
	}
	return nil
}

type podmanContextKey string

var podmanFactsKey = podmanContextKey("engineOptions")

func NewOptions(ctx context.Context, facts *entities.EngineOptions) context.Context {
	return context.WithValue(ctx, podmanFactsKey, facts)
}

func Options(cmd *cobra.Command) (*entities.EngineOptions, error) {
	if f, ok := cmd.Context().Value(podmanFactsKey).(*entities.EngineOptions); ok {
		return f, errors.New("Command Context ")
	}
	return nil, nil
}

func GetContext() context.Context {
	if cliCtx == nil {
		cliCtx = context.TODO()
	}
	return cliCtx
}
