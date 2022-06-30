package connection

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	addCmd = &cobra.Command{
		Use:   "add [options] NAME DESTINATION",
		Args:  cobra.ExactArgs(2),
		Short: "Record destination for the Podman service",
		Long: `Add destination to podman configuration.
  "destination" is one of the form:
    [user@]hostname (will default to ssh)
    ssh://[user@]hostname[:port][/path] (will obtain socket path from service, if not given.)
    tcp://hostname:port (not secured)
    unix://path (absolute path required)
`,
		RunE:              add,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman system connection add laptop server.fubar.com
  podman system connection add --identity ~/.ssh/dev_rsa testing ssh://root@server.fubar.com:2222
  podman system connection add --identity ~/.ssh/dev_rsa --port 22 production root@server.fubar.com
  podman system connection add debug tcp://localhost:8080
  `,
	}

	cOpts = struct {
		Identity string
		Port     int
		UDSPath  string
		Default  bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  system.ConnectionCmd,
	})

	flags := addCmd.Flags()

	portFlagName := "port"
	flags.IntVarP(&cOpts.Port, portFlagName, "p", 22, "SSH port number for destination")
	_ = addCmd.RegisterFlagCompletionFunc(portFlagName, completion.AutocompleteNone)

	identityFlagName := "identity"
	flags.StringVar(&cOpts.Identity, identityFlagName, "", "path to SSH identity file")
	_ = addCmd.RegisterFlagCompletionFunc(identityFlagName, completion.AutocompleteDefault)

	socketPathFlagName := "socket-path"
	flags.StringVar(&cOpts.UDSPath, socketPathFlagName, "", "path to podman socket on remote host. (default '/run/podman/podman.sock' or '/run/user/{uid}/podman/podman.sock)")
	_ = addCmd.RegisterFlagCompletionFunc(socketPathFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&cOpts.Default, "default", "d", false, "Set connection to be default")
}

func add(cmd *cobra.Command, args []string) error {
	// Default to ssh schema if none given
	dest := args[1]
	if match, err := regexp.Match("^[A-Za-z][A-Za-z0-9+.-]*://", []byte(dest)); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	} else if !match {
		dest = "ssh://" + dest
	}
	uri, err := url.Parse(dest)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("socket-path") {
		uri.Path = cmd.Flag("socket-path").Value.String()
	}

	switch uri.Scheme {
	case "ssh":
		if uri.User.Username() == "" {
			if uri.User, err = utils.GetUserInfo(uri); err != nil {
				return err
			}
		}

		if cmd.Flags().Changed("port") {
			uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").Value.String())
		}

		if uri.Port() == "" {
			uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").DefValue)
		}
		iden := ""
		if cmd.Flags().Changed("identity") {
			iden = cOpts.Identity
		}
		if uri.Path == "" || uri.Path == "/" {
			if uri.Path, err = getUDS(uri, iden); err != nil {
				return err
			}
		}
	case "unix":
		if cmd.Flags().Changed("identity") {
			return errors.New("--identity option not supported for unix scheme")
		}

		if cmd.Flags().Changed("socket-path") {
			uri.Path = cmd.Flag("socket-path").Value.String()
		}

		info, err := os.Stat(uri.Path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			logrus.Warnf("%q does not exists", uri.Path)
		case errors.Is(err, os.ErrPermission):
			logrus.Warnf("You do not have permission to read %q", uri.Path)
		case err != nil:
			return err
		case info.Mode()&os.ModeSocket == 0:
			return fmt.Errorf("%q exists and is not a unix domain socket", uri.Path)
		}
	case "tcp":
		if cmd.Flags().Changed("socket-path") {
			return errors.New("--socket-path option not supported for tcp scheme")
		}
		if cmd.Flags().Changed("identity") {
			return errors.New("--identity option not supported for tcp scheme")
		}
		if uri.Port() == "" {
			return errors.New("tcp scheme requires a port either via --port or in destination URL")
		}
	default:
		logrus.Warnf("%q unknown scheme, no validation provided", uri.Scheme)
	}

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("default") {
		if cOpts.Default {
			cfg.Engine.ActiveService = args[0]
		}
	}

	dst := config.Destination{
		URI: uri.String(),
	}

	if cmd.Flags().Changed("identity") {
		dst.Identity = cOpts.Identity
	}

	if cfg.Engine.ServiceDestinations == nil {
		cfg.Engine.ServiceDestinations = map[string]config.Destination{
			args[0]: dst,
		}
		cfg.Engine.ActiveService = args[0]
	} else {
		cfg.Engine.ServiceDestinations[args[0]] = dst
	}
	return cfg.Write()
}

func getUDS(uri *url.URL, iden string) (string, error) {
	cfg, err := utils.ValidateAndConfigure(uri, iden)
	if err != nil {
		return "", fmt.Errorf("failed to validate: %w", err)
	}
	dial, err := ssh.Dial("tcp", uri.Host, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w", err)
	}
	defer dial.Close()

	session, err := dial.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create new ssh session on %q: %w", uri.Host, err)
	}
	defer session.Close()

	// Override podman binary for testing etc
	podman := "podman"
	if v, found := os.LookupEnv("PODMAN_BINARY"); found {
		podman = v
	}
	infoJSON, err := utils.ExecRemoteCommand(dial, podman+" info --format=json")
	if err != nil {
		return "", err
	}

	var info define.Info
	if err := json.Unmarshal(infoJSON, &info); err != nil {
		return "", fmt.Errorf("failed to parse 'podman info' results: %w", err)
	}

	if info.Host.RemoteSocket == nil || len(info.Host.RemoteSocket.Path) == 0 {
		return "", fmt.Errorf("remote podman %q failed to report its UDS socket", uri.Host)
	}
	return info.Host.RemoteSocket.Path, nil
}
