package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/libpod/image"
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
	Usage:                  "run a command in a new container",
	Description:            runDescription,
	Flags:                  runFlags,
	Action:                 runCmd,
	ArgsUsage:              "IMAGE [COMMAND [ARG...]]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func runCmd(c *cli.Context) error {
	var imageName string
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

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)
	if len(c.Args()) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	rtc := runtime.GetConfig()
	newImage, err := runtime.ImageRuntime().New(c.Args()[0], rtc.SignaturePolicyPath, "", os.Stderr, nil, image.SigningOptions{}, false, false)
	if err != nil {
		return errors.Wrapf(err, "unable to find image")
	}

	data, err := newImage.Inspect()
	if err != nil {
		return err
	}
	if len(newImage.Names()) < 1 {
		imageName = newImage.ID()
	} else {
		imageName = newImage.Names()[0]
	}
	createConfig, err := parseCreateOpts(c, runtime, imageName, data)
	if err != nil {
		return err
	}
	useImageVolumes := createConfig.ImageVolumeType == "bind"

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}

	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}

	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(createConfig.ImageID, createConfig.Image, useImageVolumes))
	options = append(options, libpod.WithSELinuxLabels(createConfig.ProcessLabel, createConfig.MountLabel))
	options = append(options, libpod.WithConmonPidFile(createConfig.ConmonPidFile))
	options = append(options, libpod.WithLabels(createConfig.Labels))
	options = append(options, libpod.WithUser(createConfig.User))
	options = append(options, libpod.WithShmDir(createConfig.ShmDir))
	options = append(options, libpod.WithShmSize(createConfig.Resources.ShmSize))
	options = append(options, libpod.WithGroups(createConfig.GroupAdd))

	// Default used if not overridden on command line

	if createConfig.CgroupParent != "" {
		options = append(options, libpod.WithCgroupParent(createConfig.CgroupParent))
	}

	ctr, err := runtime.NewContainer(runtimeSpec, options...)
	if err != nil {
		return err
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		logrus.Debugf("New container created %q", ctr.ID())

		p, _ := ctr.CGroupPath()("")
		logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), p)
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
		if err := ctr.Start(); err != nil {
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
	if c.IsSet("attach") {
		outputStream = nil
		errorStream = nil
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

		// If --interactive is set, restore stdin
		if c.Bool("interactive") {
			inputStream = os.Stdin
		}
	}

	if err := startAttachCtr(ctr, outputStream, errorStream, inputStream, c.String("detach-keys"), c.BoolT("sig-proxy")); err != nil {
		// This means the command did not exist
		exitCode = 127
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 126
		}
		return err
	}

	if ecode, err := ctr.ExitCode(); err != nil {
		logrus.Errorf("unable to get exit code of container %s: %q", ctr.ID(), err)
	} else {
		exitCode = int(ecode)
	}

	if createConfig.Rm {
		return runtime.RemoveContainer(ctr, true)
	}
	return ctr.Cleanup()
}
