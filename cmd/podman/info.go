package main

import (
	"fmt"
	rt "runtime"
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	infoCommand cliconfig.InfoValues

	infoDescription = `Display information pertaining to the host, current storage stats, and build of podman.

  Useful for the user and when reporting issues.
`
	_infoCommand = &cobra.Command{
		Use:   "info",
		Args:  noSubArgs,
		Long:  infoDescription,
		Short: "Display podman system information",
		RunE: func(cmd *cobra.Command, args []string) error {
			infoCommand.InputArgs = args
			infoCommand.GlobalFlags = MainGlobalOpts
			infoCommand.Remote = remoteclient
			return infoCmd(&infoCommand)
		},
		Example: `podman info`,
	}
)

func init() {
	infoCommand.Command = _infoCommand
	infoCommand.SetHelpTemplate(HelpTemplate())
	infoCommand.SetUsageTemplate(UsageTemplate())
	flags := infoCommand.Flags()

	flags.BoolVarP(&infoCommand.Debug, "debug", "D", false, "Display additional debug information")
	flags.StringVarP(&infoCommand.Format, "format", "f", "", "Change the output format to JSON or a Go template")

}

func infoCmd(c *cliconfig.InfoValues) error {
	info := map[string]interface{}{}
	remoteClientInfo := map[string]interface{}{}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	infoArr, err := runtime.Info()
	if err != nil {
		return errors.Wrapf(err, "error getting info")
	}

	if runtime.Remote {
		endpoint, err := runtime.RemoteEndpoint()
		if err != nil {
			logrus.Errorf("Failed to obtain server connection: %s", err.Error())
		} else {
			remoteClientInfo["Connection"] = endpoint.Connection
			remoteClientInfo["Connection Type"] = endpoint.Type.String()
		}

		remoteClientInfo["RemoteAPI Version"] = version.RemoteAPIVersion
		remoteClientInfo["Podman Version"] = version.Version
		remoteClientInfo["OS Arch"] = fmt.Sprintf("%s/%s", rt.GOOS, rt.GOARCH)
		infoArr = append(infoArr, define.InfoData{Type: "client", Data: remoteClientInfo})
	}

	if !runtime.Remote && c.Debug {
		debugInfo := debugInfo(c)
		infoArr = append(infoArr, define.InfoData{Type: "debug", Data: debugInfo})
	}

	for _, currInfo := range infoArr {
		info[currInfo.Type] = currInfo.Data
	}

	var out formats.Writer
	infoOutputFormat := c.Format
	if strings.Join(strings.Fields(infoOutputFormat), "") == "{{json.}}" {
		infoOutputFormat = formats.JSONString
	}
	switch infoOutputFormat {
	case formats.JSONString:
		out = formats.JSONStruct{Output: info}
	case "":
		out = formats.YAMLStruct{Output: info}
	default:
		out = formats.StdoutTemplate{Output: info, Template: infoOutputFormat}
	}

	return out.Out()
}

// top-level "debug" info
func debugInfo(c *cliconfig.InfoValues) map[string]interface{} {
	info := map[string]interface{}{}
	info["compiler"] = rt.Compiler
	info["go version"] = rt.Version()
	info["podman version"] = version.Version
	version, _ := define.GetVersion()
	info["git commit"] = version.GitCommit
	return info
}
