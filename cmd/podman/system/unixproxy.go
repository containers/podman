package system

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	unixProxyDescription = `Proxy a remote podman service

	Proxies a remote podman service over a local unix domain socket.
`

	upCmd = &cobra.Command{
		Use:               "unix-proxy [options] [URI]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Proxies a remote podman service over a local unix domain socket",
		Long:              unixProxyDescription,
		RunE:              proxy,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example:           `podman system unix-proxy unix:///tmp/podman.sock`,
	}

	upArgs = struct {
		PidFile string
		Quiet   bool
	}{}
)

type CloseWriteStream interface {
	io.Reader
	io.WriteCloser
	CloseWrite() error
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: upCmd,
		Parent:  systemCmd,
	})

	flags := upCmd.Flags()

	flags.StringVarP(&upArgs.PidFile, "pid-file", "", "", "File to save PID")
	_ = upCmd.RegisterFlagCompletionFunc("pid-file", completion.AutocompleteNone)
	flags.BoolVarP(&upArgs.Quiet, "quiet", "q", false, "Suppress printed output")
	_ = upCmd.RegisterFlagCompletionFunc("quiet", completion.AutocompleteNone)
}

func proxy(cmd *cobra.Command, args []string) error {
	apiURI, err := resolveUnixURI(args)
	if err != nil {
		return err
	}
	logrus.Infof("using API endpoint: '%s'", apiURI)
	// Clean up any old existing unix domain socket
	var uri *url.URL
	if len(apiURI) > 0 {
		var err error
		uri, err = url.Parse(apiURI)
		if err != nil {
			return err
		}

		// socket activation uses a unix:// socket in the shipped unit files but apiURI is coded as "" at this layer.
		if uri.Scheme == "unix" {
			if err := os.Remove(uri.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
		} else {
			return errors.Errorf("Only unix domain sockets are supported as a proxy address: %s", uri)
		}
	}

	if len(upArgs.PidFile) > 0 {
		f, err := os.Create(upArgs.PidFile)
		if err != nil {
			errors.Wrap(err, "Error creating pid")
		}
		defer os.Remove(upArgs.PidFile)
		pid := os.Getpid()
		if _, err := f.WriteString(strconv.Itoa(pid)); err != nil {
			errors.Wrap(err, "Error creating pid")
		}
	}

	return setupProxy(uri)
}

func connectForward(bastion *bindings.Bastion) (CloseWriteStream, error) {
	for retries := 1; ; retries++ {
		forward, err := bastion.Client.Dial("unix", bastion.URI.Path)
		if err == nil {
			return forward.(ssh.Channel), nil
		}
		// Check if ssh connection is still alive
		_, _, err2 := bastion.Client.Conn.SendRequest("alive@podman", true, nil)
		if err2 != nil || retries > 2 {
			// couldn't reconnect ssh tunnel, or the destination is unreachable
			return nil, errors.Wrapf(err, "Couldn't reestablish ssh connection: %s", bastion.URI)
		}

		bastion.Reconnect()
	}
}

func listenUnix(socketURI *url.URL) (net.Listener, error) {
	oldmask := umask(0177)
	defer umask(oldmask)
	listener, err := net.Listen("unix", socketURI.Path)
	if err != nil {
		return listener, errors.Wrapf(err, "Error listening on socket: %s", socketURI.Path)
	}

	return listener, nil
}

func setupProxy(socketURI *url.URL) error {
	cfg := registry.PodmanConfig()

	uri, err := url.Parse(cfg.URI)
	if err != nil {
		return errors.Wrapf(err, "Not a valid url: %s", uri)
	}

	if uri.Scheme != "ssh" {
		return errors.Errorf("Only ssh is supported, specify another connection: %s", uri)
	}

	bastion, err := bindings.CreateBastion(uri, "", cfg.Identity)
	defer bastion.Client.Close()
	if err != nil {
		return err
	}

	printfOrQuiet("SSH Bastion connected: %s\n", uri)

	listener, err := listenUnix(socketURI)
	if err != nil {
		return errors.Wrapf(err, "Error listening on socket: %s", socketURI.Path)
	}
	defer listener.Close()

	printfOrQuiet("Listening on: %s\n", socketURI)

	for {
		acceptConnection(listener, &bastion, socketURI)
	}
}

func printfOrQuiet(format string, a ...interface{}) (n int, err error) {
	if !upArgs.Quiet {
		return fmt.Printf(format, a...)
	}

	return 0, nil
}

func acceptConnection(listener net.Listener, bastion *bindings.Bastion, socketURI *url.URL) error {
	con, err := accept(listener)
	if err != nil {
		return errors.Wrapf(err, "Error accepting on socket: %s", socketURI.Path)
	}

	src, ok := con.(CloseWriteStream)
	if !ok {
		con.Close()
		return errors.Wrapf(err, "Underlying socket does not support half-close %s", socketURI.Path)
	}

	dest, err := connectForward(bastion)
	if err != nil {
		con.Close()
		logrus.Error(err)
		return nil // eat
	}

	go forward(src, dest)
	go forward(dest, src)

	return nil
}

func backOff(delay time.Duration) time.Duration {
	if delay == 0 {
		delay = 5 * time.Millisecond
	} else {
		delay *= 2
	}
	if delay > time.Second {
		delay = time.Second
	}
	return delay
}

func accept(listener net.Listener) (net.Conn, error) {
	con, err := listener.Accept()
	delay := time.Duration(0)
	if ne, ok := err.(net.Error); ok && ne.Temporary() {
		delay = backOff(delay)
		time.Sleep(delay)
	}
	return con, err
}

func forward(src io.Reader, dest CloseWriteStream) {
	defer dest.CloseWrite()
	io.Copy(dest, src)
}

func resolveUnixURI(_url []string) (string, error) {
	socketName := "podman-remote.sock"

	if len(_url) > 0 && _url[0] != "" {
		return _url[0], nil
	}

	xdg, err := util.GetRuntimeDir()
	if rootless.IsRootless() {
		xdg = os.TempDir()
	}

	if err != nil {
		return "", err
	}

	socketPath := filepath.Join(xdg, "podman", socketName)
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		return "", err
	}
	return "unix:" + socketPath, nil
}
