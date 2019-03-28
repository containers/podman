package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
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
)

var (
	commitCommand     cliconfig.CommitValues
	commitDescription = `Create an image from a container's changes. Optionally tag the image created, set the author with the --author flag, set the commit message with the --message flag, and make changes to the instructions with the --change flag.`

	_commitCommand = &cobra.Command{
		Use:   "commit [flags] CONTAINER IMAGE",
		Short: "Create new image based on the changed container",
		Long:  commitDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			commitCommand.InputArgs = args
			commitCommand.GlobalFlags = MainGlobalOpts
			return commitCmd(&commitCommand)
		},
		Example: `podman commit -q --message "committing container to image" reverent_golick image-commited
  podman commit -q --author "firstName lastName" reverent_golick image-commited
  podman commit -q --pause=false containerID image-commited`,
	}
)

func init() {
	commitCommand.Command = _commitCommand
	commitCommand.SetHelpTemplate(HelpTemplate())
	commitCommand.SetUsageTemplate(UsageTemplate())
	flags := commitCommand.Flags()
	flags.StringSliceVarP(&commitCommand.Change, "change", "c", []string{}, fmt.Sprintf("Apply the following possible instructions to the created image (default []): %s", strings.Join(libpod.ChangeCmds, " | ")))
	flags.StringVarP(&commitCommand.Format, "format", "f", "oci", "`Format` of the image manifest and metadata")
	flags.StringVarP(&commitCommand.Message, "message", "m", "", "Set commit message for imported image")
	flags.StringVarP(&commitCommand.Author, "author", "a", "", "Set the author for the image committed")
	flags.BoolVarP(&commitCommand.Pause, "pause", "p", false, "Pause container during commit")
	flags.BoolVarP(&commitCommand.Quiet, "quiet", "q", false, "Suppress output")

}

func commitCmd(c *cliconfig.CommitValues) error {
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var (
		writer   io.Writer
		mimeType string
	)
	args := c.InputArgs
	if len(args) != 2 {
		return errors.Errorf("you must provide a container name or ID and a target image name")
	}

	switch c.Format {
	case "oci":
		mimeType = buildah.OCIv1ImageManifest
		if c.Flag("message").Changed {
			return errors.Errorf("messages are only compatible with the docker image format (-f docker)")
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return errors.Errorf("unrecognized image format %q", c.Format)
	}
	container := args[0]
	reference := args[1]
	if c.Flag("change").Changed {
		for _, change := range c.Change {
			splitChange := strings.Split(strings.ToUpper(change), "=")
			if !util.StringInSlice(splitChange[0], libpod.ChangeCmds) {
				return errors.Errorf("invalid syntax for --change: %s", change)
			}
		}
	}

	if !c.Quiet {
		writer = os.Stderr
	}
	ctr, err := runtime.LookupContainer(container)
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", container)
	}

	rtc, err := runtime.GetConfig()
	if err != nil {
		return err
	}

	sc := image.GetSystemContext(rtc.SignaturePolicyPath, "", false)
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   rtc.SignaturePolicyPath,
		ReportWriter:          writer,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}
	options := libpod.ContainerCommitOptions{
		CommitOptions: coptions,
		Pause:         c.Pause,
		Message:       c.Message,
		Changes:       c.Change,
		Author:        c.Author,
	}
	newImage, err := ctr.Commit(getContext(), reference, options)
	if err != nil {
		return err
	}
	fmt.Println(newImage.ID())
	return nil
}
