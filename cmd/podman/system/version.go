package system

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	versionCommand = &cobra.Command{
		Use:               "version [options]",
		Args:              validate.NoArgs,
		Short:             "Display the Podman Version Information",
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	if cmd.Flag("format").Changed {
		row := report.NormalizeFormat(versionFormat)
		tmpl, err := template.New("version 2.0.0").Parse(row)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(w, versions); err != nil {
			// On Failure, assume user is using older version of podman version --format and check client
			row = strings.Replace(row, ".Server.", ".", 1)
			tmpl, err := template.New("version 1.0.0").Parse(row)
			if err != nil {
				return err
			}
			if err := tmpl.Execute(w, versions.Client); err != nil {
				return err
			}
		}
		return nil
	}

	if versions.Server != nil {
		if _, err := fmt.Fprintf(w, "Client:\n"); err != nil {
			return err
		}
		formatVersion(w, versions.Client)
		if _, err := fmt.Fprintf(w, "\nServer:\n"); err != nil {
			return err
		}
		formatVersion(w, versions.Server)
	} else {
		formatVersion(w, versions.Client)
	}
	return nil
}

func formatVersion(w io.Writer, version *define.Version) {
	fmt.Fprintf(w, "Version:\t%s\n", version.Version)
	fmt.Fprintf(w, "API Version:\t%s\n", version.APIVersion)
	fmt.Fprintf(w, "Go Version:\t%s\n", version.GoVersion)
	if version.GitCommit != "" {
		fmt.Fprintf(w, "Git Commit:\t%s\n", version.GitCommit)
	}
	fmt.Fprintf(w, "Built:\t%s\n", version.BuiltTime)
	fmt.Fprintf(w, "OS/Arch:\t%s\n", version.OsArch)
}
