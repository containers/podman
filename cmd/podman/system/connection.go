package system

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"regexp"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/v2/cmd/podman/registry"
	"github.com/containers/libpod/v2/libpod/define"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/containers/libpod/v2/pkg/terminal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const schemaPattern = "^[A-Za-z][A-Za-z0-9+.-]*:"

var (
	// Skip creating engines since this command will obtain connection information to engine
	noOp = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	connectionCmd = &cobra.Command{
		Use:  "connection [flags] destination",
		Args: cobra.ExactArgs(1),
		Long: `Store ssh destination information in podman configuration.
  "destination" is of the form [user@]hostname or
  an URI of the form ssh://[user@]hostname[:port]
`,
		Short:              "Record remote ssh destination",
		PersistentPreRunE:  noOp,
		PersistentPostRunE: noOp,
		TraverseChildren:   false,
		RunE:               connection,
		Example: `podman system connection server.fubar.com
  podman system connection --identity ~/.ssh/dev_rsa ssh://root@server.fubar.com:2222
  podman system connection --identity ~/.ssh/dev_rsa --port 22 root@server.fubar.com`,
	}

	cOpts = struct {
		Identity string
		Port     int
		UDSPath  string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: connectionCmd,
		Parent:  systemCmd,
	})

	flags := connectionCmd.Flags()
	flags.IntVarP(&cOpts.Port, "port", "p", 22, "port number for destination")
	flags.StringVar(&cOpts.UDSPath, "socket-path", "", "path to podman socket on remote host. (default '/run/podman/podman.sock' or '/run/user/{uid}/podman/podman.sock)")
}

func connection(cmd *cobra.Command, args []string) error {
	// Default to ssh: schema if none given
	dest := []byte(args[0])
	if match, err := regexp.Match(schemaPattern, dest); err != nil {
		return errors.Wrapf(err, "internal regex error %q", schemaPattern)
	} else if !match {
		dest = append([]byte("ssh://"), dest...)
	}

	uri, err := url.Parse(string(dest))
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", string(dest))
	}

	if uri.User.Username() == "" {
		if uri.User, err = getUserInfo(uri); err != nil {
			return err
		}
	}

	if cmd.Flag("socket-path").Changed {
		uri.Path = cmd.Flag("socket-path").Value.String()
	}

	if cmd.Flag("port").Changed {
		uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").Value.String())
	}

	if uri.Port() == "" {
		uri.Host = net.JoinHostPort(uri.Hostname(), cmd.Flag("port").DefValue)
	}

	if uri.Path == "" {
		if uri.Path, err = getUDS(cmd, uri); err != nil {
			return errors.Wrapf(err, "failed to connect to  %q", uri.String())
		}
	}

	custom, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if cmd.Flag("identity").Changed {
		custom.Engine.RemoteIdentity = cOpts.Identity
	}

	custom.Engine.RemoteURI = uri.String()
	return custom.Write()
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

	ident := cmd.Flag("identity")
	if ident.Changed {
		auth, err := terminal.PublicKey(ident.Value.String(), []byte(passwd))
		if err != nil {
			return "", errors.Wrapf(err, "Failed to read identity %q", ident.Value.String())
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

	config := &ssh.ClientConfig{
		User:            uri.User.Username(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	dial, err := ssh.Dial("tcp", uri.Host, config)
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
