package system

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/systemd"
	"github.com/containers/libpod/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	srvDescription = `Run an API service

Enable a listening service for API access to Podman commands.
`

	srvCmd = &cobra.Command{
		Use:   "service [flags] [URI]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Run API service",
		Long:  srvDescription,
		RunE:  service,
		Example: `podman system service --time=0 unix:///tmp/podman.sock
  podman system service --varlink --time=0 unix:///tmp/podman.sock`,
	}

	srvArgs = struct {
		Timeout int64
		Varlink bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: srvCmd,
		Parent:  systemCmd,
	})

	flags := srvCmd.Flags()
	flags.Int64VarP(&srvArgs.Timeout, "time", "t", 5, "Time until the service session expires in seconds.  Use 0 to disable the timeout")
	flags.Int64Var(&srvArgs.Timeout, "timeout", 5, "Time until the service session expires in seconds.  Use 0 to disable the timeout")
	flags.BoolVar(&srvArgs.Varlink, "varlink", false, "Use legacy varlink service instead of REST")

	_ = flags.MarkDeprecated("varlink", "valink API is deprecated.")
}

func service(cmd *cobra.Command, args []string) error {
	apiURI, err := resolveApiURI(args)
	if err != nil {
		return err
	}
	logrus.Infof("using API endpoint: \"%s\"", apiURI)

	opts := entities.ServiceOptions{
		URI:     apiURI,
		Timeout: time.Duration(srvArgs.Timeout) * time.Second,
		Command: cmd,
	}

	if srvArgs.Varlink {
		return registry.ContainerEngine().VarlinkService(registry.GetContext(), opts)
	}

	logrus.Warn("This function is EXPERIMENTAL")
	fmt.Fprintf(os.Stderr, "This function is EXPERIMENTAL.\n")
	return registry.ContainerEngine().RestService(registry.GetContext(), opts)
}

func resolveApiURI(_url []string) (string, error) {

	// When determining _*THE*_ listening endpoint --
	// 1) User input wins always
	// 2) systemd socket activation
	// 3) rootless honors XDG_RUNTIME_DIR
	// 4) if varlink -- adapter.DefaultVarlinkAddress
	// 5) lastly adapter.DefaultAPIAddress

	if _url == nil {
		if v, found := os.LookupEnv("PODMAN_SOCKET"); found {
			_url = []string{v}
		}
	}

	switch {
	case len(_url) > 0:
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
		if srvArgs.Varlink {
			socketName = "io.podman"
		}
		socketDir := filepath.Join(xdg, "podman", socketName)
		if _, err := os.Stat(filepath.Dir(socketDir)); err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(filepath.Dir(socketDir), 0755); err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		}
		return "unix:" + socketDir, nil
	case srvArgs.Varlink:
		return registry.DefaultVarlinkAddress, nil
	default:
		return registry.DefaultAPIAddress, nil
	}
}
