package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
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
	if err := validateFlags(c, createFlags); err != nil {
		return err
	}

	if c.String("cidfile") != "" {
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

	imageName, _, data, err := imageData(c, runtime, c.Args()[0])
	if err != nil {
		return err
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
	options = append(options, libpod.WithLabels(createConfig.Labels))
	options = append(options, libpod.WithUser(createConfig.User))
	options = append(options, libpod.WithShmDir(createConfig.ShmDir))
	options = append(options, libpod.WithShmSize(createConfig.Resources.ShmSize))

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

	if err := ctr.Init(); err != nil {
		// This means the command did not exist
		exitCode = 127
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 126
		}
		return err
	}
	logrus.Debugf("container storage created for %q", ctr.ID())

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
			return errors.Wrapf(err, "unable to start container %q", ctr.ID())
		}

		fmt.Printf("%s\n", ctr.ID())
		exitCode = 0
		return nil
	}

	// TODO: that "false" should probably be linked to -i
	// Handle this when we split streams to allow attaching just stdin/out/err
	attachChan, err := ctr.StartAndAttach(false, c.String("detach-keys"))
	if err != nil {
		return errors.Wrapf(err, "unable to start container %q", ctr.ID())
	}

	if c.BoolT("sig-proxy") {
		ProxySignals(ctr)
	}

	// Wait for attach to complete
	err = <-attachChan
	if err != nil {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
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
