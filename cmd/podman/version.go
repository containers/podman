package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	versionCommand  cliconfig.VersionValues
	_versionCommand = &cobra.Command{
		Use:   "version",
		Args:  noSubArgs,
		Short: "Display the Podman Version Information",
		RunE: func(cmd *cobra.Command, args []string) error {
			versionCommand.InputArgs = args
			versionCommand.GlobalFlags = MainGlobalOpts
			return versionCmd(&versionCommand)
		},
	}
)

func init() {
	versionCommand.Command = _versionCommand
	versionCommand.SetUsageTemplate(UsageTemplate())
	flags := versionCommand.Flags()
	flags.StringVarP(&versionCommand.Format, "format", "f", "", "Change the output format to JSON or a Go template")
}

// versionCmd gets and prints version info for version command
func versionCmd(c *cliconfig.VersionValues) error {
	output, err := libpod.GetVersion()
	if err != nil {
		errors.Wrapf(err, "unable to determine version")
	}

	versionOutputFormat := c.Format
	if versionOutputFormat != "" {
		if strings.Join(strings.Fields(versionOutputFormat), "") == "{{json.}}" {
			versionOutputFormat = formats.JSONString
		}
		var out formats.Writer
		switch versionOutputFormat {
		case formats.JSONString:
			out = formats.JSONStruct{Output: output}
		default:
			out = formats.StdoutTemplate{Output: output, Template: versionOutputFormat}
		}
		return formats.Writer(out).Out()
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "Version:\t%s\n", output.Version)
	fmt.Fprintf(w, "RemoteAPI Version:\t%d\n", output.RemoteAPIVersion)
	fmt.Fprintf(w, "Go Version:\t%s\n", output.GoVersion)
	if output.GitCommit != "" {
		fmt.Fprintf(w, "Git Commit:\t%s\n", output.GitCommit)
	}
	// Prints out the build time in readable format
	if output.Built != 0 {
		fmt.Fprintf(w, "Built:\t%s\n", time.Unix(output.Built, 0).Format(time.ANSIC))
	}

	fmt.Fprintf(w, "OS/Arch:\t%s\n", output.OsArch)
	return nil
}
