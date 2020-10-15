package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"regexp"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/system"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/terminal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const schemaPattern = "^[A-Za-z][A-Za-z0-9+.-]*:"

var (
	addCmd = &cobra.Command{
		Use:   "add [options] NAME DESTINATION",
		Args:  cobra.ExactArgs(2),
		Short: "Record destination for the Podman service",
		Long: `Add destination to podman configuration.
  "destination" is of the form [user@]hostname or
  an URI of the form ssh://[user@]hostname[:port]
`,
		RunE: add,
		Example: `podman system connection add laptop server.fubar.com
  podman system connection add --identity ~/.ssh/dev_rsa testing ssh://root@server.fubar.com:2222
  podman system connection add --identity ~/.ssh/dev_rsa --port 22 production root@server.fubar.com
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
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: addCmd,
		Parent:  system.ConnectionCmd,
	})

	flags := addCmd.Flags()
	flags.IntVarP(&cOpts.Port, "port", "p", 22, "SSH port number for destination")
	flags.StringVar(&cOpts.Identity, "identity", "", "path to SSH identity file")
	flags.StringVar(&cOpts.UDSPath, "socket-path", "", "path to podman socket on remote host. (default '/run/podman/podman.sock' or '/run/user/{uid}/podman/podman.sock)")
	flags.BoolVarP(&cOpts.Default, "default", "d", false, "Set connection to be default")
}

func add(cmd *cobra.Command, args []string) error {
	// Default to ssh: schema if none given
	dest := args[1]
	if match, err := regexp.Match(schemaPattern, []byte(dest)); err != nil {
		return errors.Wrapf(err, "internal regex error %q", schemaPattern)
	} else if !match {
		dest = "ssh://" + dest
	}

	uri, err := url.Parse(dest)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", dest)
	}

	if uri.User.Username() == "" {
		if uri.User, err = getUserInfo(uri); err != nil {
			return err
		}
	}

	if cmd.Flags().Changed("socket-path") {
		uri.Path = cmd.Flag("socket-path").Value.String()
	}

	if cmd.Flags().Changed("port") {
		uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").Value.String())
	}

	if uri.Port() == "" {
		uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").DefValue)
	}

	if uri.Path == "" || uri.Path == "/" {
		if uri.Path, err = getUDS(cmd, uri); err != nil {
			return errors.Wrapf(err, "failed to connect to  %q", uri.String())
		}
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

func getUserInfo(uri *url.URL) (*url.Userinfo, error) {
	var (
		usr *user.User
		err error
	)
	if u, found := os.LookupEnv("_CONTAINERS_ROOTLESS_UID"); found {
		usr, err = user.LookupId(u)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find user %q", u)
		}
	} else {
		usr, err = user.Current()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to obtain current user")
		}
	}

	pw, set := uri.User.Password()
	if set {
		return url.UserPassword(usr.Username, pw), nil
	}
	return url.User(usr.Username), nil
}

func getUDS(cmd *cobra.Command, uri *url.URL) (string, error) {
	var authMethods []ssh.AuthMethod
	passwd, set := uri.User.Password()
	if set {
		authMethods = append(authMethods, ssh.Password(passwd))
	}

	if cmd.Flags().Changed("identity") {
		value := cmd.Flag("identity").Value.String()
		auth, err := terminal.PublicKey(value, []byte(passwd))
		if err != nil {
			return "", errors.Wrapf(err, "failed to read identity %q", value)
		}
		authMethods = append(authMethods, auth)
	}

	if sock, found := os.LookupEnv("SSH_AUTH_SOCK"); found {
		logrus.Debugf("Found SSH_AUTH_SOCK %q, ssh-agent signer enabled", sock)

		c, err := net.Dial("unix", sock)
		if err != nil {
			return "", err
		}
		a := agent.NewClient(c)
		authMethods = append(authMethods, ssh.PublicKeysCallback(a.Signers))
	}

	if len(authMethods) == 0 {
		pass, err := terminal.ReadPassword(fmt.Sprintf("%s's login password:", uri.User.Username()))
		if err != nil {
			return "", err
		}
		authMethods = append(authMethods, ssh.Password(string(pass)))
	}

	cfg := &ssh.ClientConfig{
		User:            uri.User.Username(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	dial, err := ssh.Dial("tcp", uri.Host, cfg)
	if err != nil {
		return "", errors.Wrapf(err, "failed to connect to %q", uri.Host)
	}
	defer dial.Close()

	session, err := dial.NewSession()
	if err != nil {
		return "", errors.Wrapf(err, "failed to create new ssh session on %q", uri.Host)
	}
	defer session.Close()

	// Override podman binary for testing etc
	podman := "podman"
	if v, found := os.LookupEnv("PODMAN_BINARY"); found {
		podman = v
	}
	run := podman + " info --format=json"

	var buffer bytes.Buffer
	session.Stdout = &buffer
	if err := session.Run(run); err != nil {
		return "", errors.Wrapf(err, "failed to run %q", run)
	}

	var info define.Info
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		return "", errors.Wrapf(err, "failed to parse 'podman info' results")
	}

	if info.Host.RemoteSocket == nil || len(info.Host.RemoteSocket.Path) == 0 {
		return "", fmt.Errorf("remote podman %q failed to report its UDS socket", uri.Host)
	}
	return info.Host.RemoteSocket.Path, nil
}
