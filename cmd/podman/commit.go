package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/manifest"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	commitFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "change, c",
			Usage: fmt.Sprintf("Apply the following possible instructions to the created image (default []): %s", strings.Join(libpod.ChangeCmds, " | ")),
		},
		cli.StringFlag{
			Name:  "format, f",
			Usage: "`format` of the image manifest and metadata",
			Value: "oci",
		},
		cli.StringFlag{
			Name:  "message, m",
			Usage: "Set commit message for imported image",
		},
		cli.StringFlag{
			Name:  "author, a",
			Usage: "Set the author for the image committed",
		},
		cli.BoolTFlag{
			Name:  "pause, p",
			Usage: "Pause container during commit",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output",
		},
	}
	commitDescription = `Create an image from a container's changes.
	 Optionally tag the image created, set the author with the --author flag,
	 set the commit message with the --message flag,
	 and make changes to the instructions with the --change flag.`
	commitCommand = cli.Command{
		Name:         "commit",
		Usage:        "Create new image based on the changed container",
		Description:  commitDescription,
		Flags:        sortFlags(commitFlags),
		Action:       commitCmd,
		ArgsUsage:    "CONTAINER [REPOSITORY[:TAG]]",
		OnUsageError: usageErrorHandler,
	}
)

func commitCmd(c *cli.Context) error {
	if err := validateFlags(c, commitFlags); err != nil {
		return err
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var (
		writer   io.Writer
		mimeType string
	)
	args := c.Args()
	if len(args) != 2 {
		return errors.Errorf("you must provide a container name or ID and a target image name")
	}

	switch c.String("format") {
	case "oci":
		mimeType = buildah.OCIv1ImageManifest
		if c.IsSet("message") || c.IsSet("m") {
			return errors.Errorf("messages are only compatible with the docker image format (-f docker)")
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return errors.Errorf("unrecognized image format %q", c.String("format"))
	}
	container := args[0]
	reference := args[1]
	if c.IsSet("change") || c.IsSet("c") {
		for _, change := range c.StringSlice("change") {
			splitChange := strings.Split(strings.ToUpper(change), "=")
			if !util.StringInSlice(splitChange[0], libpod.ChangeCmds) {
				return errors.Errorf("invalid syntax for --change ", change)
			}
		}
	}

	if !c.Bool("quiet") {
		writer = os.Stderr
	}
	ctr, err := runtime.LookupContainer(container)
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", container)
	}

	sc := image.GetSystemContext(runtime.GetConfig().SignaturePolicyPath, "", false)
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   runtime.GetConfig().SignaturePolicyPath,
		ReportWriter:          writer,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}
	options := libpod.ContainerCommitOptions{
		CommitOptions: coptions,
		Pause:         c.Bool("pause"),
		Message:       c.String("message"),
		Changes:       c.StringSlice("change"),
		Author:        c.String("author"),
	}
	newImage, err := ctr.Commit(getContext(), reference, options)
	if err != nil {
		return err
	}
	fmt.Println(newImage.ID())
	return nil
}
