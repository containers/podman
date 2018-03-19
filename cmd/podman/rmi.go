package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	rmiDescription = "removes one or more locally stored images."
	rmiFlags       = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "remove all images",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force removal of the image",
		},
	}
	rmiCommand = cli.Command{
		Name:        "rmi",
		Usage:       "removes one or more images from local storage",
		Description: rmiDescription,
		Action:      rmiCmd,
		ArgsUsage:   "IMAGE-NAME-OR-ID [...]",
		Flags:       rmiFlags,
		UseShortOptionHandling: true,
	}
)

func rmiCmd(c *cli.Context) error {
	if err := validateFlags(c, rmiFlags); err != nil {
		return err
	}
	removeAll := c.Bool("all")
	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 && !removeAll {
		return errors.Errorf("image name or ID must be specified")
	}
	if len(args) > 0 && removeAll {
		return errors.Errorf("when using the --all switch, you may not pass any images names or IDs")
	}

	imagesToDelete := args[:]
	var lastError error
	if removeAll {
		localImages, err := runtime.GetImages(&libpod.ImageFilterParams{})
		if err != nil {
			return errors.Wrapf(err, "unable to query local images")
		}
		for _, image := range localImages {
			imagesToDelete = append(imagesToDelete, image.ID)
		}
	}

	for _, arg := range imagesToDelete {
		image := runtime.NewImage(arg)
		iid, err := image.Remove(c.Bool("force"))
		if err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				buildahImage, err2 := runtime.GetImage(image.LocalName)
				if err2 == nil {
					errBuildah := rmiBuildahCmd(c, buildahImage.ID)
					if errBuildah == nil {
						err = nil
					} else {
						fmt.Printf("A container created with Buildah may be associated with this image.\n")
						fmt.Printf("Remove the container with Buildah or retry using the --force option.\n")
					}
				} else {
					fmt.Printf("Unable to find image %s\n", image.LocalName)
				}
			}
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = err
		} else {
			fmt.Println(iid)
		}
	}
	return lastError
}

func rmiBuildahCmd(c *cli.Context, imageID string) error {
	rmiCmdArgs := []string{}

	// Handle Global Options
	logLevel := c.GlobalString("log-level")
	if logLevel == "debug" {
		rmiCmdArgs = append(rmiCmdArgs, "--debug")
	}

	rmiCmdArgs = append(rmiCmdArgs, "rmi")

	if c.Bool("force") {
		rmiCmdArgs = append(rmiCmdArgs, "--force")
	}

	rmiCmdArgs = append(rmiCmdArgs, imageID)
	buildah := "buildah"

	if _, err := exec.LookPath(buildah); err != nil {
		return errors.Wrapf(err, "buildah not found in PATH")
	}
	if _, err := exec.Command(buildah).Output(); err != nil {
		return errors.Wrapf(err, "buildah is not operational on this server")
	}

	cmd := exec.Command(buildah, rmiCmdArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error running the buildah rmi command")
	}

	return nil
}
