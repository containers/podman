package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/directory"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	libpodImage "github.com/projectatomic/libpod/libpod/image"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	ociManifestDir  = "oci-dir"
	v2s2ManifestDir = "docker-dir"
)

var (
	saveFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "compress",
			Usage: "compress tarball image layers when saving to a directory using the 'dir' transport. (default is same compression type as source)",
		},
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress the output",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Save image to oci-archive, oci-dir (directory with oci manifest type), docker-dir (directory with v2s2 manifest type)",
		},
	}
	saveDescription = `
	Save an image to docker-archive or oci-archive on the local machine.
	Default is docker-archive`

	saveCommand = cli.Command{
		Name:           "save",
		Usage:          "Save image to an archive",
		Description:    saveDescription,
		Flags:          saveFlags,
		Action:         saveCmd,
		ArgsUsage:      "",
		SkipArgReorder: true,
	}
)

// saveCmd saves the image to either docker-archive or oci
func saveCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("need at least 1 argument")
	}
	if err := validateFlags(c, saveFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	if c.IsSet("compress") && (c.String("format") != ociManifestDir && c.String("format") != v2s2ManifestDir && c.String("format") == "") {
		return errors.Errorf("--compress can only be set when --format is either 'oci-dir' or 'docker-dir'")
	}

	var writer io.Writer
	if !c.Bool("quiet") {
		writer = os.Stderr
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		fi := os.Stdout
		if logrus.IsTerminal(fi) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
	}
	if err := validateFileName(output); err != nil {
		return err
	}

	source := args[0]
	newImage, err := runtime.ImageRuntime().NewFromLocal(source)
	if err != nil {
		return err
	}

	var destRef types.ImageReference
	var manifestType string
	switch c.String("format") {
	case libpod.OCIArchive:
		destImageName := imageNameForSaveDestination(newImage, source)
		destRef, err = ociarchive.NewReference(output, destImageName) // destImageName may be ""
		if err != nil {
			return errors.Wrapf(err, "error getting OCI archive ImageReference for (%q, %q)", output, destImageName)
		}
	case "oci-dir":
		destRef, err = directory.NewReference(output)
		if err != nil {
			return errors.Wrapf(err, "error getting directory ImageReference for %q", output)
		}
		manifestType = imgspecv1.MediaTypeImageManifest
	case "docker-dir":
		destRef, err = directory.NewReference(output)
		if err != nil {
			return errors.Wrapf(err, "error getting directory ImageReference for %q", output)
		}
		manifestType = manifest.DockerV2Schema2MediaType
	case libpod.DockerArchive:
		fallthrough
	case "":
		dst := output
		destImageName := imageNameForSaveDestination(newImage, source)
		if destImageName != "" {
			dst = fmt.Sprintf("%s:%s", dst, destImageName)
		}
		destRef, err = dockerarchive.ParseReference(dst) // FIXME? Add dockerarchive.NewReference
		if err != nil {
			return errors.Wrapf(err, "error getting Docker archive ImageReference for %q", dst)
		}
	default:
		return errors.Errorf("unknown format option %q", c.String("format"))
	}

	// supports saving multiple tags to the same tar archive
	var additionaltags []reference.NamedTagged
	if len(args) > 1 {
		additionaltags, err = libpodImage.GetAdditionalTags(args[1:])
		if err != nil {
			return err
		}
	}

	if err := newImage.PushImageToReference(getContext(), destRef, manifestType, "", "", writer, c.Bool("compress"), libpodImage.SigningOptions{}, &libpodImage.DockerRegistryOptions{}, false, additionaltags); err != nil {
		if err2 := os.Remove(output); err2 != nil {
			logrus.Errorf("error deleting %q: %v", output, err)
		}
		return errors.Wrapf(err, "unable to save %q", args)
	}

	return nil
}

// imageNameForSaveDestination returns a Docker-like reference appropriate for saving img,
// which the user referred to as imgUserInput; or an empty string, if there is no appropriate
// reference.
func imageNameForSaveDestination(img *libpodImage.Image, imgUserInput string) string {
	if strings.Contains(img.ID(), imgUserInput) {
		return ""
	}

	prepend := ""
	if !strings.Contains(imgUserInput, libpodImage.DefaultLocalRepo) {
		// we need to check if localhost was added to the image name in NewFromLocal
		for _, name := range img.Names() {
			// if the user searched for the image whose tag was prepended with localhost, we'll need to prepend localhost to successfully search
			if strings.Contains(name, libpodImage.DefaultLocalRepo) && strings.Contains(name, imgUserInput) {
				prepend = fmt.Sprintf("%s/", libpodImage.DefaultLocalRepo)
				break
			}
		}
	}
	return fmt.Sprintf("%s%s", prepend, imgUserInput)
}
