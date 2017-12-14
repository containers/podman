package main

import (
	"io"
	"os"
	"strings"

	"github.com/containers/image/manifest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
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
		Name:        "save",
		Usage:       "Save image to an archive",
		Description: saveDescription,
		Flags:       saveFlags,
		Action:      saveCmd,
		ArgsUsage:   "",
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

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	if c.IsSet("compress") && (c.String("format") != ociManifestDir && c.String("format") != v2s2ManifestDir && c.String("format") == "") {
		return errors.Errorf("--compress can only be set when --format is either 'oci-dir' or 'docker-dir'")
	}

	var writer io.Writer
	if !c.Bool("quiet") {
		writer = os.Stdout
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		fi := os.Stdout
		if logrus.IsTerminal(fi) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
	}

	var dst, manifestType string
	switch c.String("format") {
	case libpod.OCIArchive:
		dst = libpod.OCIArchive + ":" + output
	case "oci-dir":
		dst = libpod.DirTransport + ":" + output
		manifestType = imgspecv1.MediaTypeImageManifest
	case "docker-dir":
		dst = libpod.DirTransport + ":" + output
		manifestType = manifest.DockerV2Schema2MediaType
	case libpod.DockerArchive:
		fallthrough
	case "":
		dst = libpod.DockerArchive + ":" + output
	default:
		return errors.Errorf("unknown format option %q", c.String("format"))
	}

	saveOpts := libpod.CopyOptions{
		SignaturePolicyPath: "",
		Writer:              writer,
		ManifestMIMEType:    manifestType,
		ForceCompress:       c.Bool("compress"),
	}

	// only one image is supported for now
	// future pull requests will fix this
	for _, image := range args {
		dest := dst
		// need dest to be in the format transport:path:reference for the following transports
		if strings.Contains(dst, libpod.OCIArchive) || strings.Contains(dst, libpod.DockerArchive) {
			dest = dst + ":" + image
		}
		if err := runtime.PushImage(image, dest, saveOpts); err != nil {
			if err2 := os.Remove(output); err2 != nil {
				logrus.Errorf("error deleting %q: %v", output, err)
			}
			return errors.Wrapf(err, "unable to save %q", image)
		}
	}
	return nil
}
