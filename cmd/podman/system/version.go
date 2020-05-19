package system

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	versionCommand = &cobra.Command{
		Use:   "version",
		Args:  validate.NoArgs,
		Short: "Display the Podman Version Information",
		RunE:  version,
	}
	versionFormat string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: versionCommand,
	})
	flags := versionCommand.Flags()
	flags.StringVarP(&versionFormat, "format", "f", "", "Change the output format to JSON or a Go template")
}

func version(cmd *cobra.Command, args []string) error {
	versions, err := registry.ContainerEngine().Version(registry.Context())
	if err != nil {
		return err
	}

	switch {
	case versionFormat == "json", versionFormat == "{{ json .}}":
		s, err := json.MarshalToString(versions)
		if err != nil {
			return err
		}
		_, err = io.WriteString(os.Stdout, s)
		return err
	case cmd.Flag("format").Changed:
		if !strings.HasSuffix(versionFormat, "\n") {
			versionFormat += "\n"
		}
		out := formats.StdoutTemplate{Output: versions, Template: versionFormat}
		err := out.Out()
		if err != nil {
			// On Failure, assume user is using older version of podman version --format and check client
			versionFormat = strings.Replace(versionFormat, ".Server.", ".", 1)
			out = formats.StdoutTemplate{Output: versions.Client, Template: versionFormat}
			if err1 := out.Out(); err1 != nil {
				return err
			}
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

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

func formatVersion(writer io.Writer, version *define.Version) {
	fmt.Fprintf(writer, "Version:\t%s\n", version.Version)
	fmt.Fprintf(writer, "API Version:\t%d\n", version.APIVersion)
	fmt.Fprintf(writer, "Go Version:\t%s\n", version.GoVersion)
	if version.GitCommit != "" {
		fmt.Fprintf(writer, "Git Commit:\t%s\n", version.GitCommit)
	}
	fmt.Fprintf(writer, "Built:\t%s\n", version.BuiltTime)
	fmt.Fprintf(writer, "OS/Arch:\t%s\n", version.OsArch)
}
