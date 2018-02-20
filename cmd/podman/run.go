package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runDescription = "Runs a command in a new container from the given image"

var runCommand = cli.Command{
	Name:                   "run",
	Usage:                  "run a command in a new container",
	Description:            runDescription,
	Flags:                  createFlags,
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

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}

	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}

	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(createConfig.ImageID, createConfig.Image, true))
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

	logrus.Debug("new container created ", ctr.ID())

	p, _ := ctr.CGroupPath()("")
	logrus.Debugf("createConfig.CgroupParent %v for %v", p, ctr.ID())

	if err := ctr.Init(); err != nil {
		// This means the command did not exist
		exitCode = 126
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 127
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

	logrus.Debug("new container created ", ctr.ID())

	if c.String("cidfile") != "" {
		if err := libpod.WriteFile(ctr.ID(), c.String("cidfile")); err != nil {
			logrus.Error(err)
		}
	}

	// Create a bool channel to track that the console socket attach
	// is successful.
	attached := make(chan bool)
	// Create a waitgroup so we can sync and wait for all goroutines
	// to finish before exiting main
	var wg sync.WaitGroup

	if !createConfig.Detach {
		// We increment the wg counter because we need to do the attach
		wg.Add(1)
		// Attach to the running container
		go func() {
			logrus.Debugf("trying to attach to the container %s", ctr.ID())
			defer wg.Done()
			if err := ctr.Attach(false, c.String("detach-keys"), attached); err != nil {
				logrus.Errorf("unable to attach to container %s: %q", ctr.ID(), err)
			}
		}()
		if !<-attached {
			return errors.Errorf("unable to attach to container %s", ctr.ID())
		}
	}
	// Start the container
	if err := ctr.Start(); err != nil {
		return errors.Wrapf(err, "unable to start container %q", ctr.ID())
	}
	if createConfig.Detach {
		fmt.Printf("%s\n", ctr.ID())
		exitCode = 0
		return nil
	}
	wg.Wait()
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
