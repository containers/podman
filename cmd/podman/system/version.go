package system

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	versionCommand = &cobra.Command{
		Use:               "version [options]",
		Args:              validate.NoArgs,
		Short:             "Display the Podman version information",
		RunE:              version,
		ValidArgsFunction: completion.AutocompleteNone,
	}
	versionFormat string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: versionCommand,
	})
	flags := versionCommand.Flags()

	formatFlagName := "format"
	flags.StringVarP(&versionFormat, formatFlagName, "f", "", "Change the output format to JSON or a Go template")
	_ = versionCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(entities.SystemVersionReport{}))
}

func version(cmd *cobra.Command, args []string) error {
	versions, err := registry.ContainerEngine().Version(registry.Context())
	if err != nil {
		return err
	}

	if report.IsJSON(versionFormat) {
		s, err := json.MarshalToString(versions)
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	}

	if cmd.Flag("format").Changed {
		// Cannot use report.New() as it enforces {{range .}} for OriginUser templates
		tmpl := template.New(cmd.Name()).Funcs(template.FuncMap(report.DefaultFuncs))

		versionFormat = report.NormalizeFormat(versionFormat)
		tmpl, err := tmpl.Parse(versionFormat)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(os.Stdout, versions); err != nil {
			// On Failure, assume user is using older version of podman version --format and check client
			versionFormat = strings.ReplaceAll(versionFormat, ".Server.", ".")
			tmpl, err := tmpl.Parse(versionFormat)
			if err != nil {
				return err
			}
			if err := tmpl.Execute(os.Stdout, versions.Client); err != nil {
				return err
			}
		}
		return nil
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()
	rpt, err = rpt.Parse(report.OriginPodman, versionTemplate)
	if err != nil {
		return err
	}
	return rpt.Execute(versions)
}

const versionTemplate = `{{with .Client -}}
Client:\tPodman Engine
Version:\t{{.Version}}
API Version:\t{{.APIVersion}}
Go Version:\t{{.GoVersion}}
{{if .GitCommit -}}
  Git Commit:\t{{.GitCommit}}
{{- end}}
Built:\t{{.BuiltTime}}
OS/Arch:\t{{.OsArch}}
{{- end}}

{{- if .Server }}{{with .Server}}

Server:\tPodman Engine
Version:\t{{.Version}}
API Version:\t{{.APIVersion}}
Go Version:\t{{.GoVersion}}
{{if .GitCommit -}}
  Git Commit:\t{{.GitCommit}}
{{- end}}
Built:\t{{.BuiltTime}}
OS/Arch:\t{{.OsArch}}
{{- end}}{{- end}}
`
