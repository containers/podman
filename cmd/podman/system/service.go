// +build linux,!remote

package system

import (
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/systemd"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	srvDescription = `Run an API service

Enable a listening service for API access to Podman commands.
`

	srvCmd = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "service [options] [URI]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Run API service",
		Long:              srvDescription,
		RunE:              service,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example:           `podman system service --time=0 unix:///tmp/podman.sock`,
	}

	srvArgs = struct {
		Timeout     int64
		CorsHeaders string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: srvCmd,
		Parent:  systemCmd,
	})

	flags := srvCmd.Flags()

	timeFlagName := "time"
	flags.Int64VarP(&srvArgs.Timeout, timeFlagName, "t", 5, "Time until the service session expires in seconds.  Use 0 to disable the timeout")
	_ = srvCmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
	flags.StringVarP(&srvArgs.CorsHeaders, "cors", "", "", "Set CORS Headers")
	_ = srvCmd.RegisterFlagCompletionFunc("cors", completion.AutocompleteNone)

	flags.SetNormalizeFunc(aliasTimeoutFlag)
}

func aliasTimeoutFlag(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if name == "timeout" {
		name = "time"
	}
	return pflag.NormalizedName(name)
}

func service(cmd *cobra.Command, args []string) error {
	apiURI, err := resolveAPIURI(args)
	if err != nil {
		return err
	}
	logrus.Infof("using API endpoint: '%s'", apiURI)
	// Clean up any old existing unix domain socket
	if len(apiURI) > 0 {
		uri, err := url.Parse(apiURI)
		if err != nil {
			return err
		}

		// socket activation uses a unix:// socket in the shipped unit files but apiURI is coded as "" at this layer.
		if uri.Scheme == "unix" && !registry.IsRemote() {
			if err := syscall.Unlink(uri.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
			mask := syscall.Umask(0177)
			defer syscall.Umask(mask)
		}
	}

	opts := entities.ServiceOptions{
		URI:         apiURI,
		Command:     cmd,
		CorsHeaders: srvArgs.CorsHeaders,
	}

	opts.Timeout = time.Duration(srvArgs.Timeout) * time.Second
	return restService(opts, cmd.Flags(), registry.PodmanConfig())
}

func resolveAPIURI(_url []string) (string, error) {
	// When determining _*THE*_ listening endpoint --
	// 1) User input wins always
	// 2) systemd socket activation
	// 3) rootless honors XDG_RUNTIME_DIR
	// 4) lastly adapter.DefaultAPIAddress

	if len(_url) == 0 {
		if v, found := os.LookupEnv("PODMAN_SOCKET"); found {
			logrus.Debugf("PODMAN_SOCKET='%s' used to determine API endpoint", v)
			_url = []string{v}
		}
	}

	switch {
	case len(_url) > 0 && _url[0] != "":
		return _url[0], nil
	case systemd.SocketActivated():
		logrus.Info("using systemd socket activation to determine API endpoint")
		return "", nil
	case rootless.IsRootless():
		xdg, err := util.GetRuntimeDir()
		if err != nil {
			return "", err
		}

		socketName := "podman.sock"
		socketPath := filepath.Join(xdg, "podman", socketName)
		if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
			return "", err
		}
		return "unix:" + socketPath, nil
	default:
		if err := os.MkdirAll(filepath.Dir(registry.DefaultRootAPIPath), 0700); err != nil {
			return "", err
		}
		return registry.DefaultRootAPIAddress, nil
	}
}
