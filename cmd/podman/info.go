package main

import (
	"fmt"
	rt "runtime"

	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/libpod/version"
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
			Name:  "format, f",
			Usage: "Change the output format to JSON or a Go template",
		},
	}
)

func infoCmd(c *cli.Context) error {
	if err := validateFlags(c, infoFlags); err != nil {
		return err
	}
	info := map[string]interface{}{}
	remoteClientInfo := map[string]interface{}{}

	runtime, err := adapter.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	infoArr, err := runtime.Info()
	if err != nil {
		return errors.Wrapf(err, "error getting info")
	}
	if runtime.Remote {
		remoteClientInfo["RemoteAPI Version"] = version.RemoteAPIVersion
		remoteClientInfo["Podman Version"] = version.Version
		remoteClientInfo["OS Arch"] = fmt.Sprintf("%s/%s", rt.GOOS, rt.GOARCH)
		infoArr = append(infoArr, libpod.InfoData{Type: "client", Data: remoteClientInfo})
	}

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
	info["compiler"] = rt.Compiler
	info["go version"] = rt.Version()
	info["podman version"] = c.App.Version
	version, _ := libpod.GetVersion()
	info["git commit"] = version.GitCommit
	return info
}
