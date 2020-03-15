package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
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
			versionCommand.Remote = remoteclient
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
func getRemoteVersion(c *cliconfig.VersionValues) (version define.Version, err error) {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return version, errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	return runtime.GetVersion()
}

type versionStruct struct {
	Client define.Version
	Server define.Version
}

// versionCmd gets and prints version info for version command
func versionCmd(c *cliconfig.VersionValues) error {

	var (
		v   versionStruct
		err error
	)
	v.Client, err = define.GetVersion()
	if err != nil {
		return errors.Wrapf(err, "unable to determine version")
	}
	if remote {
		v.Server, err = getRemoteVersion(c)
		if err != nil {
			return err
		}
	} else {
		v.Server = v.Client
	}

	versionOutputFormat := c.Format
	if versionOutputFormat != "" {
		if strings.Join(strings.Fields(versionOutputFormat), "") == "{{json.}}" {
			versionOutputFormat = formats.JSONString
		}
		var out formats.Writer
		switch versionOutputFormat {
		case formats.JSONString:
			out = formats.JSONStruct{Output: v}
			return out.Out()
		default:
			out = formats.StdoutTemplate{Output: v, Template: versionOutputFormat}
			err := out.Out()
			if err != nil {
				// On Failure, assume user is using older version of podman version --format and check client
				out = formats.StdoutTemplate{Output: v.Client, Template: versionOutputFormat}
				if err1 := out.Out(); err1 != nil {
					return err
				}
			}
		}
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	if remote {
		if _, err := fmt.Fprintf(w, "Client:\n"); err != nil {
			return err
		}
		formatVersion(w, v.Client)
		if _, err := fmt.Fprintf(w, "\nServer:\n"); err != nil {
			return err
		}
		formatVersion(w, v.Server)
	} else {
		formatVersion(w, v.Client)
	}
	return nil
}

func formatVersion(writer io.Writer, version define.Version) {
	fmt.Fprintf(writer, "Version:\t%s\n", version.Version)
	fmt.Fprintf(writer, "RemoteAPI Version:\t%d\n", version.RemoteAPIVersion)
	fmt.Fprintf(writer, "Go Version:\t%s\n", version.GoVersion)
	if version.GitCommit != "" {
		fmt.Fprintf(writer, "Git Commit:\t%s\n", version.GitCommit)
	}
	// Prints out the build time in readable format
	if version.Built != 0 {
		fmt.Fprintf(writer, "Built:\t%s\n", time.Unix(version.Built, 0).Format(time.ANSIC))
	}

	fmt.Fprintf(writer, "OS/Arch:\t%s\n", version.OsArch)
}
