// +build linux

// Package rootlessport provides reexec for RootlessKit-based port forwarder.
//
// init() contains reexec.Register() for ReexecKey .
//
// The reexec requires Config to be provided via stdin.
//
// The reexec writes human-readable error message on stdout on error.
//
// Debug log is printed on stderr.
package rootlessport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/storage/pkg/reexec"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	rkport "github.com/rootless-containers/rootlesskit/pkg/port"
	rkbuiltin "github.com/rootless-containers/rootlesskit/pkg/port/builtin"
	rkportutil "github.com/rootless-containers/rootlesskit/pkg/port/portutil"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// ReexecKey is the reexec key for the parent process.
	ReexecKey = "containers-rootlessport"
	// reexecChildKey is used internally for the second reexec
	reexecChildKey       = "containers-rootlessport-child"
	reexecChildEnvOpaque = "_CONTAINERS_ROOTLESSPORT_CHILD_OPAQUE"
)

// Config needs to be provided to the process via stdin as a JSON string.
// stdin needs to be closed after the message has been written.
type Config struct {
	Mappings  []ocicni.PortMapping
	NetNSPath string
	ExitFD    int
	ReadyFD   int
	TmpDir    string
}

func init() {
	reexec.Register(ReexecKey, func() {
		if err := parent(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	})
	reexec.Register(reexecChildKey, func() {
		if err := child(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	})

}

func loadConfig(r io.Reader) (*Config, io.ReadCloser, io.WriteCloser, error) {
	stdin, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, nil, err
	}
	var cfg Config
	if err := json.Unmarshal(stdin, &cfg); err != nil {
		return nil, nil, nil, err
	}
	if cfg.NetNSPath == "" {
		return nil, nil, nil, errors.New("missing NetNSPath")
	}
	if cfg.ExitFD <= 0 {
		return nil, nil, nil, errors.New("missing ExitFD")
	}
	exitFile := os.NewFile(uintptr(cfg.ExitFD), "exitfile")
	if exitFile == nil {
		return nil, nil, nil, errors.New("invalid ExitFD")
	}
	if cfg.ReadyFD <= 0 {
		return nil, nil, nil, errors.New("missing ReadyFD")
	}
	readyFile := os.NewFile(uintptr(cfg.ReadyFD), "readyfile")
	if readyFile == nil {
		return nil, nil, nil, errors.New("invalid ReadyFD")
	}
	return &cfg, exitFile, readyFile, nil
}

func parent() error {
	// load config from stdin
	cfg, exitR, readyW, err := loadConfig(os.Stdin)
	if err != nil {
		return err
	}

	exitC := make(chan os.Signal, 1)
	defer close(exitC)

	go func() {
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, unix.SIGPIPE)
		defer func() {
			signal.Stop(sigC)
			close(sigC)
		}()

		select {
		case s := <-sigC:
			if s == unix.SIGPIPE {
				if f, err := os.OpenFile("/dev/null", os.O_WRONLY, 0755); err == nil {
					unix.Dup2(int(f.Fd()), 1) // nolint:errcheck
					unix.Dup2(int(f.Fd()), 2) // nolint:errcheck
					f.Close()
				}
			}
		case <-exitC:
		}
	}()

	// create the parent driver
	stateDir, err := ioutil.TempDir(cfg.TmpDir, "rootlessport")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stateDir)
	driver, err := rkbuiltin.NewParentDriver(&logrusWriter{prefix: "parent: "}, stateDir)
	if err != nil {
		return err
	}
	initComplete := make(chan struct{})
	quit := make(chan struct{})
	errCh := make(chan error)
	// start the parent driver. initComplete will be closed when the child connected to the parent.
	logrus.Infof("starting parent driver")
	go func() {
		driverErr := driver.RunParentDriver(initComplete, quit, nil)
		if driverErr != nil {
			logrus.WithError(driverErr).Warn("parent driver exited")
		}
		errCh <- driverErr
		close(errCh)
	}()
	opaque := driver.OpaqueForChild()
	logrus.Infof("opaque=%+v", opaque)
	opaqueJSON, err := json.Marshal(opaque)
	if err != nil {
		return err
	}
	childQuitR, childQuitW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() {
		// stop the child
		logrus.Info("stopping child driver")
		if err := childQuitW.Close(); err != nil {
			logrus.WithError(err).Warn("unable to close childQuitW")
		}
	}()

	// reexec the child process in the child netns
	cmd := exec.Command("/proc/self/exe")
	cmd.Args = []string{reexecChildKey}
	cmd.Stdin = childQuitR
	cmd.Stdout = &logrusWriter{prefix: "child"}
	cmd.Stderr = cmd.Stdout
	cmd.Env = append(os.Environ(), reexecChildEnvOpaque+"="+string(opaqueJSON))
	childNS, err := ns.GetNS(cfg.NetNSPath)
	if err != nil {
		return err
	}
	if err := childNS.Do(func(_ ns.NetNS) error {
		logrus.Infof("starting child driver in child netns (%q %v)", cmd.Path, cmd.Args)
		return cmd.Start()
	}); err != nil {
		return err
	}

	childErrCh := make(chan error)
	go func() {
		err := cmd.Wait()
		childErrCh <- err
		close(childErrCh)
	}()

	defer func() {
		if err := unix.Kill(cmd.Process.Pid, unix.SIGTERM); err != nil {
			logrus.WithError(err).Warn("kill child process")
		}
	}()

	logrus.Info("waiting for initComplete")
	// wait for the child to connect to the parent
outer:
	for {
		select {
		case <-initComplete:
			logrus.Infof("initComplete is closed; parent and child established the communication channel")
			break outer
		case err := <-childErrCh:
			if err != nil {
				return err
			}
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		logrus.Info("stopping parent driver")
		quit <- struct{}{}
		if err := <-errCh; err != nil {
			logrus.WithError(err).Warn("parent driver returned error on exit")
		}
	}()

	// let parent expose ports
	logrus.Infof("exposing ports %v", cfg.Mappings)
	if err := exposePorts(driver, cfg.Mappings); err != nil {
		return err
	}

	// write and close ReadyFD (convention is same as slirp4netns --ready-fd)
	logrus.Info("ready")
	if _, err := readyW.Write([]byte("1")); err != nil {
		return err
	}
	if err := readyW.Close(); err != nil {
		return err
	}

	// wait for ExitFD to be closed
	logrus.Info("waiting for exitfd to be closed")
	if _, err := ioutil.ReadAll(exitR); err != nil {
		return err
	}
	return nil
}

func exposePorts(pm rkport.Manager, portMappings []ocicni.PortMapping) error {
	ctx := context.TODO()
	for _, i := range portMappings {
		hostIP := i.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		spec := rkport.Spec{
			Proto:      i.Protocol,
			ParentIP:   hostIP,
			ParentPort: int(i.HostPort),
			ChildPort:  int(i.ContainerPort),
		}
		if err := rkportutil.ValidatePortSpec(spec, nil); err != nil {
			return err
		}
		if _, err := pm.AddPort(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func child() error {
	// load the config from the parent
	var opaque map[string]string
	if err := json.Unmarshal([]byte(os.Getenv(reexecChildEnvOpaque)), &opaque); err != nil {
		return err
	}

	// start the child driver
	quit := make(chan struct{})
	errCh := make(chan error)
	go func() {
		d := rkbuiltin.NewChildDriver(os.Stderr)
		dErr := d.RunChildDriver(opaque, quit)
		errCh <- dErr
	}()
	defer func() {
		logrus.Info("stopping child driver")
		quit <- struct{}{}
		if err := <-errCh; err != nil {
			logrus.WithError(err).Warn("child driver returned error on exit")
		}
	}()

	// wait for stdin to be closed
	if _, err := ioutil.ReadAll(os.Stdin); err != nil {
		return err
	}
	return nil
}

type logrusWriter struct {
	prefix string
}

func (w *logrusWriter) Write(p []byte) (int, error) {
	logrus.Infof("%s%s", w.prefix, string(p))
	return len(p), nil
}
