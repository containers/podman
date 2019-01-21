package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// versionCmd gets and prints version info for version command
func versionCmd(c *cli.Context) error {
	output, err := libpod.GetVersion()
	if err != nil {
		errors.Wrapf(err, "unable to determine version")
	}

	versionOutputFormat := c.String("format")
	if versionOutputFormat != "" {
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

// Cli command to print out the full version of podman
var (
	versionCommand = cli.Command{
		Name:   "version",
		Usage:  "Display the Podman Version Information",
		Action: versionCmd,
		Flags:  versionFlags,
	}
	versionFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
	}
)
