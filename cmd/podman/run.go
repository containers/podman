package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/dnsservice"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runDescription = "Runs a command in a new container from the given image"

var runFlags []cli.Flag = append(createFlags, cli.BoolTFlag{
	Name:  "sig-proxy",
	Usage: "proxy received signals to the process (default true)",
})

var runCommand = cli.Command{
	Name:                   "run",
	Usage:                  "Run a command in a new container",
	Description:            runDescription,
	Flags:                  sortFlags(runFlags),
	Action:                 runCmd,
	ArgsUsage:              "IMAGE [COMMAND [ARG...]]",
	HideHelp:               true,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	OnUsageError:           usageErrorHandler,
}

func runCmd(c *cli.Context) error {
	var (
		serviceContainer   *libpod.Container
		extraOptions       []libpod.CtrCreateOption
		serviceContainerIP string
	)

	if err := createInit(c); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetContainerRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()

	// We need to make sure the dns service container is running prior to the new container
	// If one exists, make sure it is running; otherwise create it.
	if c.Bool("dns-service") {
		netMode := ns.NetworkMode(c.String("net"))
		if netMode.IsNone() || netMode.IsHost() || netMode.IsContainer() {
			return errors.New("you can only run a dns service on a bridged network")
		}

		network := c.String("network")
		if network == "bridge" {
			network = runtime.GetConfig().CNIDefaultNetwork
			if network == "" {
				network = "podman"
			}
		}

		serviceContainer, err = getDNSServiceContainers(runtime, network)
		if err != nil {
			return err
		}

		// If the service container does not exist, we must create one.
		if serviceContainer == nil {
			serviceContainer, err = createDNSServiceContainer(runtime, ctx, network)
			if err != nil {
				return err
			}
		}
		// Make sure dns-service container is started
		logrus.Debug("Starting DNS service container: %s", serviceContainer.ID())
		cstate, err := serviceContainer.State()
		if err != nil {
			return err
		}
		if cstate != libpod.ContainerStateRunning {
			if err := serviceContainer.Start(ctx); err != nil {
				return err
			}
		}

		serviceContainerIPs, err := serviceContainer.IPs()
		if err != nil {
			return err
		}
		serviceContainerIP = serviceContainerIPs[0].IP.String()

		// Add extra options to the containers creation that allow it to use the dns-service
		// Add in a dependency for the dns container itself; and add in the IP address of the dns-service
		extraOptions = append(extraOptions, libpod.WithDNS([]string{serviceContainerIP}))
		extraOptions = append(extraOptions, libpod.WithDependencyCtrs([]*libpod.Container{serviceContainer}))
	}

	ctr, createConfig, err := createContainer(c, runtime, extraOptions, &dnsservice.DNSIP{IPAddress: serviceContainerIP})
	if err != nil {
		return err
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CGroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}

	// Handle detached start
	if createConfig.Detach {
		if err := ctr.Start(ctx); err != nil {
			// This means the command did not exist
			exitCode = 127
			if strings.Index(err.Error(), "permission denied") > -1 {
				exitCode = 126
			}
			return err
		}

		fmt.Printf("%s\n", ctr.ID())
		exitCode = 0
		return nil
	}

	outputStream := os.Stdout
	errorStream := os.Stderr
	inputStream := os.Stdin

	// If -i is not set, clear stdin
	if !c.Bool("interactive") {
		inputStream = nil
	}

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if c.IsSet("attach") || c.IsSet("a") {
		outputStream = nil
		errorStream = nil
		if !c.Bool("interactive") {
			inputStream = nil
		}

		attachTo := c.StringSlice("attach")
		for _, stream := range attachTo {
			switch strings.ToLower(stream) {
			case "stdout":
				outputStream = os.Stdout
			case "stderr":
				errorStream = os.Stderr
			case "stdin":
				inputStream = os.Stdin
			default:
				return errors.Wrapf(libpod.ErrInvalidArg, "invalid stream %q for --attach - must be one of stdin, stdout, or stderr", stream)
			}
		}
	}
	if err := startAttachCtr(ctr, outputStream, errorStream, inputStream, c.String("detach-keys"), c.BoolT("sig-proxy"), true); err != nil {
		// This means the command did not exist
		exitCode = 127
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 126
		}
		return err
	}

	if ecode, err := ctr.Wait(); err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchCtr {
			// The container may have been removed
			// Go looking for an exit file
			ctrExitCode, err := readExitFile(runtime.GetConfig().TmpDir, ctr.ID())
			if err != nil {
				logrus.Errorf("Cannot get exit code: %v", err)
			} else {
				exitCode = ctrExitCode
			}
		}
	} else {
		exitCode = int(ecode)
	}

	if createConfig.Rm {
		return runtime.RemoveContainer(ctx, ctr, true)
	}

	if err := ctr.Cleanup(ctx); err != nil {
		// If the container has been removed already, no need to error on cleanup
		// Also, if it was restarted, don't error either
		if errors.Cause(err) == libpod.ErrNoSuchCtr ||
			errors.Cause(err) == libpod.ErrCtrRemoved ||
			errors.Cause(err) == libpod.ErrCtrStateInvalid {
			return nil
		}

		return err
	}

	return nil
}

// Read a container's exit file
func readExitFile(runtimeTmp, ctrID string) (int, error) {
	exitFile := filepath.Join(runtimeTmp, "exits", ctrID)

	logrus.Debugf("Attempting to read container %s exit code from file %s", ctrID, exitFile)

	// Check if it exists
	if _, err := os.Stat(exitFile); err != nil {
		return 0, errors.Wrapf(err, "error getting exit file for container %s", ctrID)
	}

	// File exists, read it in and convert to int
	statusStr, err := ioutil.ReadFile(exitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "error reading exit file for container %s", ctrID)
	}

	exitCode, err := strconv.Atoi(string(statusStr))
	if err != nil {
		return 0, errors.Wrapf(err, "error parsing exit code for container %s", ctrID)
	}

	return exitCode, nil
}
