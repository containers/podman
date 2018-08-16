package main

import (
	"runtime"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	infoDescription = "Display podman system information"
	infoCommand     = cli.Command{
		Name:        "info",
		Usage:       infoDescription,
		Description: `Information display here pertain to the host, current storage stats, and build of podman. Useful for the user and when reporting issues.`,
		Flags:       infoFlags,
		Action:      infoCmd,
		ArgsUsage:   "",
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

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	infoArr, err := runtime.Info()
	if err != nil {
		return errors.Wrapf(err, "error getting info")
	}

	if c.Bool("debug") {
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
	info["git commit"] = libpod.GitCommit
	return info
}
