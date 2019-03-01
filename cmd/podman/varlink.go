//+build varlink,!remoteclient

package main

import (
	"bufio"
	"net"
	"os"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/varlink/go/varlink"
)

var (
	varlinkCommand     cliconfig.VarlinkValues
	varlinkDescription = `
	podman varlink

	run varlink interface
`
	_varlinkCommand = &cobra.Command{
		Use:   "varlink [flags] URI",
		Short: "Run varlink interface",
		Long:  varlinkDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			varlinkCommand.InputArgs = args
			varlinkCommand.GlobalFlags = MainGlobalOpts
			return varlinkCmd(&varlinkCommand)
		},
		Example: `podman varlink unix:/run/podman/io.podman
  podman varlink --timeout 5000 unix:/run/podman/io.podman`,
	}
)

func init() {
	varlinkCommand.Command = _varlinkCommand
	varlinkCommand.SetUsageTemplate(UsageTemplate())
	flags := varlinkCommand.Flags()
	flags.Int64VarP(&varlinkCommand.Timeout, "timeout", "t", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")
	flags.StringVarP(&varlinkCommand.Message, "message", "m", "", "Varlink Message")
}

type VarlinkDispatcher struct {
	inner         *iopodman.VarlinkInterface
	noExecMethods []string
}

func (s *VarlinkDispatcher) VarlinkGetDescription() string {
	return s.inner.VarlinkGetDescription()
}

func (s *VarlinkDispatcher) VarlinkGetName() string {
	return s.inner.VarlinkGetName()
}

func (s *VarlinkDispatcher) VarlinkDispatch(call varlink.Call, methodname string) (err error) {
	err = nil
	for _, v := range s.noExecMethods {
		if v == methodname {
			return s.inner.VarlinkDispatch(call, methodname)
		}
	}

	var file *os.File

	switch c := (*call.Conn).(type) {
	case *net.IPConn:
		file, err = c.File()
		if err != nil {
			return
		}
	case *net.UnixConn:
		file, err = c.File()
		if err != nil {
			return
		}
	default:
		logrus.Error("Can't get File() from varlink call")
		return call.ReplyError("io.podman.VarlinkRootInUserNSError", nil)
	}

	// Re-exec ourselves
	became, ret, err := rootless.VarlinkRootInUserNS([]string{"-m", string(*call.Request)}, file)

	if err != nil {
		logrus.Errorf(err.Error())
		return
	}

	if became && ret != 0 {
		return call.ReplyError("io.podman.VarlinkRootInUserNSError", nil)
	}

	return
}

func varlinkCmd(c *cliconfig.VarlinkValues) error {
	args := c.InputArgs
	if len(args) < 1 {
		return errors.Errorf("you must provide a varlink URI")
	}
	timeout := time.Duration(c.Timeout) * time.Millisecond

	// Register varlink service. The metadata can be retrieved with:
	// $ varlink info [varlink address URI]
	service, err := varlink.NewService(
		"Atomic",
		"podman",
		version.Version,
		"https://github.com/containers/libpod",
	)
	if err != nil {
		return errors.Wrapf(err, "unable to create new varlink service")
	}

	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	// Create a single runtime for varlink
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		logrus.Errorf("Error creating libpod runtime %s", err.Error())
		// FIXME: return errors.Wrapf(err, "error creating libpod runtime")
	} else {
		defer runtime.Shutdown(false)
	}
	api := varlinkapi.New(&c.PodmanCommand, runtime)

	if len(c.Message) > 0 {
		var varlinkInterfaces = []*iopodman.VarlinkInterface{api}
		for _, i := range varlinkInterfaces {
			if err := service.RegisterInterface(i); err != nil {
				return errors.Errorf("unable to register varlink interface %v", i)
			}
		}
		file := os.NewFile(uintptr(3), "varlink")
		writer := bufio.NewWriter(file)
		devnull, _ := os.Open(os.DevNull)
		reader := bufio.NewReader(devnull)
		service.HandleMessage(nil, reader, writer, []byte(c.Message))
		writer.Flush()
		return nil
	}

	err = service.Bind(args[0])
	if err != nil {
		return errors.Wrapf(err, "unable to bind varlink service")
	}

	var varlinkInterfaces = []*VarlinkDispatcher{{inner: api,
		noExecMethods: []string{
			// FIXME: add more methods
			"GetVersion",
		},
	}}
	for _, i := range varlinkInterfaces {
		if err := service.RegisterInterface(i); err != nil {
			return errors.Errorf("unable to register varlink interface %v", i)
		}
	}

	// Run the varlink server at the given address
	if err = service.DoListen(timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %d ms, 0 means never timeout)", c.Int64("timeout"))
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}
