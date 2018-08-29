package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/libpod/pkg/rootless"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
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
	Flags:                  runFlags,
	Action:                 runCmd,
	ArgsUsage:              "IMAGE [COMMAND [ARG...]]",
	HideHelp:               true,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	OnUsageError:           usageErrorHandler,
}

func runCmd(c *cli.Context) error {
	var imageName string

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/libpod/issues/1367).
	if c.Bool("help") {
		cli.ShowCommandHelpAndExit(c, "run", 0)
	}

	if err := validateFlags(c, createFlags); err != nil {
		return err
	}

	if c.String("cidfile") != "" {
		if _, err := os.Stat(c.String("cidfile")); err == nil {
			return errors.Errorf("container id file exists. ensure another container is not using it or delete %s", c.String("cidfile"))
		}
		if err := libpod.WriteFile("", c.String("cidfile")); err != nil {
			return errors.Wrapf(err, "unable to write cidfile %s", c.String("cidfile"))
		}
	}

	storageOpts, err := libpodruntime.GetDefaultStoreOptions()
	if err != nil {
		return err
	}
	mappings, err := util.ParseIDMapping(c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidmap"), c.String("subgidmap"))
	if err != nil {
		return err
	}
	storageOpts.UIDMap = mappings.UIDMap
	storageOpts.GIDMap = mappings.GIDMap

	if os.Getuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetRuntimeWithStorageOpts(c, &storageOpts)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.Args()) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.Args()[0]
	}

	ctx := getContext()
	rtc := runtime.GetConfig()

	var newImage *image.Image = nil
	var data *inspect.ImageData = nil
	if rootfs == "" && !rootless.SkipStorageSetup() {
		newImage, err = runtime.ImageRuntime().New(ctx, c.Args()[0], rtc.SignaturePolicyPath, "", os.Stderr, nil, image.SigningOptions{}, false, false)
		if err != nil {
			return errors.Wrapf(err, "unable to find image")
		}

		data, err = newImage.Inspect(ctx)
		if err != nil {
			return err
		}
		if len(newImage.Names()) < 1 {
			imageName = newImage.ID()
		} else {
			imageName = newImage.Names()[0]
		}
	}
	createConfig, err := parseCreateOpts(ctx, c, runtime, imageName, data)
	if err != nil {
		return err
	}

	runtimeSpec, err := cc.CreateConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}

	options, err := createConfig.GetContainerCreateOptions(runtime)
	if err != nil {
		return err
	}

	became, ret, err := joinOrCreateRootlessUserNamespace(createConfig, runtime)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}

	ctr, err := runtime.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return err
	}

	logrus.Debugf("New container created %q", ctr.ID())

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CGroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}

	createConfigJSON, err := json.Marshal(createConfig)
	if err != nil {
		return err
	}
	if err := ctr.AddArtifact("create-config", createConfigJSON); err != nil {
		return err
	}

	if c.String("cidfile") != "" {
		if err := libpod.WriteFile(ctr.ID(), c.String("cidfile")); err != nil {
			logrus.Error(err)
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

		inputStream = nil

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

	if err := ctr.Cleanup(); err != nil {
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
