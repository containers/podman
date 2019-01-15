package main

import (
	"github.com/containers/libpod/libpod/adapter"
	"runtime"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	infoDescription = "Display podman system information"
	infoCommand     = cli.Command{
		Name:         "info",
		Usage:        infoDescription,
		Description:  `Information display here pertain to the host, current storage stats, and build of podman. Useful for the user and when reporting issues.`,
		Flags:        sortFlags(infoFlags),
		Action:       infoCmd,
		ArgsUsage:    "",
		OnUsageError: usageErrorHandler,
	}
	infoFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "display additional debug information",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
	}
)

func infoCmd(c *cli.Context) error {
	if err := validateFlags(c, infoFlags); err != nil {
		return err
	}
	info := map[string]interface{}{}

	runtime, err := adapter.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Runtime.Shutdown(false)

	infoArr, err := runtime.Runtime.Info()
	if err != nil {
		return errors.Wrapf(err, "error getting info")
	}

	// TODO This is no a problem child because we don't know if we should add information
	// TODO about the client or the backend.  Only do for traditional podman for now.
	if !runtime.Remote && c.Bool("debug") {
		debugInfo := debugInfo(c)
		infoArr = append(infoArr, libpod.InfoData{Type: "debug", Data: debugInfo})
	}

	for _, currInfo := range infoArr {
		info[currInfo.Type] = currInfo.Data
	}

	var out formats.Writer
	infoOutputFormat := c.String("format")
	switch infoOutputFormat {
	case formats.JSONString:
		out = formats.JSONStruct{Output: info}
	case "":
		out = formats.YAMLStruct{Output: info}
	default:
		out = formats.StdoutTemplate{Output: info, Template: infoOutputFormat}
	}

	formats.Writer(out).Out()

	return nil
}

// top-level "debug" info
func debugInfo(c *cli.Context) map[string]interface{} {
	info := map[string]interface{}{}
	info["compiler"] = runtime.Compiler
	info["go version"] = runtime.Version()
	info["podman version"] = c.App.Version
	version, _ := libpod.GetVersion()
	info["git commit"] = version.GitCommit
	return info
}
