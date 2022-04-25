package connection

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"regexp"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/terminal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
		return errors.Wrapf(err, "invalid destination")
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
			if uri.User, err = GetUserInfo(uri); err != nil {
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

func GetUserInfo(uri *url.URL) (*url.Userinfo, error) {
	var (
		usr *user.User
		err error
	)
	if u, found := os.LookupEnv("_CONTAINERS_ROOTLESS_UID"); found {
		usr, err = user.LookupId(u)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to lookup rootless user")
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

func getUDS(uri *url.URL, iden string) (string, error) {
	cfg, err := ValidateAndConfigure(uri, iden)
	if err != nil {
		return "", errors.Wrapf(err, "failed to validate")
	}
	dial, err := ssh.Dial("tcp", uri.Host, cfg)
	if err != nil {
		return "", errors.Wrapf(err, "failed to connect")
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
	infoJSON, err := ExecRemoteCommand(dial, podman+" info --format=json")
	if err != nil {
		return "", err
	}

	var info define.Info
	if err := json.Unmarshal(infoJSON, &info); err != nil {
		return "", errors.Wrapf(err, "failed to parse 'podman info' results")
	}

	if info.Host.RemoteSocket == nil || len(info.Host.RemoteSocket.Path) == 0 {
		return "", errors.Errorf("remote podman %q failed to report its UDS socket", uri.Host)
	}
	return info.Host.RemoteSocket.Path, nil
}

// ValidateAndConfigure will take a ssh url and an identity key (rsa and the like) and ensure the information given is valid
// iden iden can be blank to mean no identity key
// once the function validates the information it creates and returns an ssh.ClientConfig.
func ValidateAndConfigure(uri *url.URL, iden string) (*ssh.ClientConfig, error) {
	var signers []ssh.Signer
	passwd, passwdSet := uri.User.Password()
	if iden != "" { // iden might be blank if coming from image scp or if no validation is needed
		value := iden
		s, err := terminal.PublicKey(value, []byte(passwd))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read identity %q", value)
		}
		signers = append(signers, s)
		logrus.Debugf("SSH Ident Key %q %s %s", value, ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
	}
	if sock, found := os.LookupEnv("SSH_AUTH_SOCK"); found { // validate ssh information, specifically the unix file socket used by the ssh agent.
		logrus.Debugf("Found SSH_AUTH_SOCK %q, ssh-agent signer enabled", sock)

		c, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}
		agentSigners, err := agent.NewClient(c).Signers()
		if err != nil {
			return nil, err
		}

		signers = append(signers, agentSigners...)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			for _, s := range agentSigners {
				logrus.Debugf("SSH Agent Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
		}
	}
	var authMethods []ssh.AuthMethod // now we validate and check for the authorization methods, most notaibly public key authorization
	if len(signers) > 0 {
		var dedup = make(map[string]ssh.Signer)
		for _, s := range signers {
			fp := ssh.FingerprintSHA256(s.PublicKey())
			if _, found := dedup[fp]; found {
				logrus.Debugf("Dedup SSH Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
			dedup[fp] = s
		}

		var uniq []ssh.Signer
		for _, s := range dedup {
			uniq = append(uniq, s)
		}
		authMethods = append(authMethods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return uniq, nil
		}))
	}
	if passwdSet { // if password authentication is given and valid, add to the list
		authMethods = append(authMethods, ssh.Password(passwd))
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.PasswordCallback(func() (string, error) {
			pass, err := terminal.ReadPassword(fmt.Sprintf("%s's login password:", uri.User.Username()))
			return string(pass), err
		}))
	}
	tick, err := time.ParseDuration("40s")
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            uri.User.Username(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         tick,
	}
	return cfg, nil
}
