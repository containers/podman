package registry

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type CliCommand struct {
	Mode    []entities.EngineMode
	Command *cobra.Command
	Parent  *cobra.Command
}

var (
	Commands []CliCommand

	imageEngine     entities.ImageEngine
	containerEngine entities.ContainerEngine
	cliCtx          context.Context

	EngineOptions entities.EngineOptions

	ExitCode = define.ExecErrorCodeGeneric
)

func SetExitCode(code int) {
	ExitCode = code
}

func GetExitCode() int {
	return ExitCode
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
