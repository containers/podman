package main

import (
	"fmt"
	"html/template"
	"os"
	rt "runtime"
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
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
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	i, err := runtime.Info()
	if err != nil {
		return errors.Wrapf(err, "error getting info")
	}

	info := infoWithExtra{Info: i}
	if runtime.Remote {
		endpoint, err := runtime.RemoteEndpoint()
		if err != nil {
			return err
		}
		info.Remote = getRemote(endpoint)
	}

	if !runtime.Remote && c.Debug {
		d, err := getDebug()
		if err != nil {
			return err
		}
		info.Debug = d
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
		tmpl, err := template.New("info").Parse(c.Format)
		if err != nil {
			return err
		}
		err = tmpl.Execute(os.Stdout, info)
		return err
	}

	return out.Out()
}

// top-level "debug" info
func getDebug() (*debugInfo, error) {
	v, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	return &debugInfo{
		Compiler:      rt.Compiler,
		GoVersion:     rt.Version(),
		PodmanVersion: v.Version,
		GitCommit:     v.GitCommit,
	}, nil
}

func getRemote(endpoint *adapter.Endpoint) *remoteInfo {
	return &remoteInfo{
		Connection:       endpoint.Connection,
		ConnectionType:   endpoint.Type.String(),
		RemoteAPIVersion: string(version.RemoteAPIVersion),
		PodmanVersion:    version.Version,
		OSArch:           fmt.Sprintf("%s/%s", rt.GOOS, rt.GOARCH),
	}
}

type infoWithExtra struct {
	*define.Info
	Remote *remoteInfo `json:"remote,omitempty"`
	Debug  *debugInfo  `json:"debug,omitempty"`
}

type remoteInfo struct {
	Connection       string `json:"connection"`
	ConnectionType   string `json:"connectionType"`
	RemoteAPIVersion string `json:"remoteAPIVersion"`
	PodmanVersion    string `json:"podmanVersion"`
	OSArch           string `json:"OSArch"`
}

type debugInfo struct {
	Compiler      string `json:"compiler"`
	GoVersion     string `json:"goVersion"`
	PodmanVersion string `json:"podmanVersion"`
	GitCommit     string `json:"gitCommit"`
}
