package main

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runDescription = "Runs a command in a new container from the given image"

var runCommand = cli.Command{
	Name:           "run",
	Usage:          "run a command in a new container",
	Description:    runDescription,
	Flags:          createFlags,
	Action:         runCmd,
	ArgsUsage:      "IMAGE [COMMAND [ARG...]]",
	SkipArgReorder: true,
}

func runCmd(c *cli.Context) error {
	var imageName string
	if err := validateFlags(c, createFlags); err != nil {
		return err
	}
	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	createConfig, err := parseCreateOpts(c, runtime)
	if err != nil {
		return err
	}

	createImage := runtime.NewImage(createConfig.image)
	createImage.LocalName, _ = createImage.GetLocalImageName()
	if createImage.LocalName == "" {
		// The image wasnt found by the user input'd name or its fqname
		// Pull the image
		fmt.Printf("Trying to pull %s...", createImage.PullName)
		createImage.Pull()
	}

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}
	logrus.Debug("spec is ", runtimeSpec)

	if createImage.LocalName != "" {
		nameIsID, err := runtime.IsImageID(createImage.LocalName)
		if err != nil {
			return err
		}
		if nameIsID {
			// If the input from the user is an ID, then we need to get the image
			// name for cstorage
			createImage.LocalName, err = createImage.GetNameByID()
			if err != nil {
				return err
			}
		}
		imageName = createImage.LocalName
	} else {
		imageName, err = createImage.GetFQName()
	}
	if err != nil {
		return err
	}
	logrus.Debug("imageName is ", imageName)

	imageID, err := createImage.GetImageID()
	if err != nil {
		return err
	}
	logrus.Debug("imageID is ", imageID)

	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(imageID, imageName, false))
	options = append(options, libpod.WithSELinuxMountLabel(createConfig.mountLabel))
	ctr, err := runtime.NewContainer(runtimeSpec, options...)
	if err != nil {
		return err
	}

	logrus.Debug("new container created ", ctr.ID())
	if err := ctr.Init(); err != nil {
		return err
	}
	logrus.Debug("container storage created for %q", ctr.ID())

	if c.String("cidfile") != "" {
		libpod.WriteFile(ctr.ID(), c.String("cidfile"))
		return nil
	}

	// Create a bool channel to track that the console socket attach
	// is successful.
	attached := make(chan bool)
	// Create a waitgroup so we can sync and wait for all goroutines
	// to finish before exiting main
	var wg sync.WaitGroup

	if !createConfig.detach {
		// We increment the wg counter because we need to do the attach
		wg.Add(1)
		// Attach to the running container
		go func() {
			logrus.Debug("trying to attach to the container %s", ctr.ID())
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
	logrus.Debug("started container ", ctr.ID())

	if createConfig.detach {
		fmt.Printf("%s\n", ctr.ID())
	}
	wg.Wait()
	if createConfig.rm {
		return runtime.RemoveContainer(ctr, true)
	}
	return nil
}
