package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	image2 "github.com/projectatomic/libpod/libpod/image"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	pullFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "`pathname` of a directory containing TLS certificates and keys",
		},
		cli.StringFlag{
			Name:  "creds",
			Usage: "`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output information when pulling images",
		},
		cli.StringFlag{
			Name:  "signature-policy",
			Usage: "`pathname` of signature policy file (not usually used)",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when contacting registries (default: true)",
		},
	}

	pullDescription = `
Pulls an image from a registry and stores it locally.
An image can be pulled using its tag or digest. If a tag is not
specified, the image with the 'latest' tag (if it exists) is pulled
`
	pullCommand = cli.Command{
		Name:        "pull",
		Usage:       "Pull an image from a registry",
		Description: pullDescription,
		Flags:       pullFlags,
		Action:      pullCmd,
		ArgsUsage:   "",
	}
)

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cli.Context) error {
	forceSecure := false
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		logrus.Errorf("an image name must be specified")
		return nil
	}
	if len(args) > 1 {
		logrus.Errorf("too many arguments. Requires exactly 1")
		return nil
	}
	if err := validateFlags(c, pullFlags); err != nil {
		return err
	}
	image := args[0]

	var registryCreds *types.DockerAuthConfig

	if c.IsSet("creds") {
		creds, err := util.ParseRegistryCreds(c.String("creds"))
		if err != nil {
			return err
		}
		registryCreds = creds
	}

	var (
		writer io.Writer
		imgID  string
	)
	if !c.Bool("quiet") {
		writer = os.Stderr
	}

	dockerRegistryOptions := image2.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              c.String("cert-dir"),
		DockerInsecureSkipTLSVerify: !c.BoolT("tls-verify"),
	}
	if c.IsSet("tls-verify") {
		forceSecure = c.Bool("tls-verify")
	}

	// Possible for docker-archive to have multiple tags, so use LoadFromArchiveReference instead
	if strings.HasPrefix(image, dockerarchive.Transport.Name()+":") {
		srcRef, err := alltransports.ParseImageName(image)
		if err != nil {
			return errors.Wrapf(err, "error parsing %q", image)
		}
		newImage, err := runtime.ImageRuntime().LoadFromArchiveReference(getContext(), srcRef, c.String("signature-policy"), writer)
		if err != nil {
			return errors.Wrapf(err, "error pulling image from %q", image)
		}
		imgID = newImage[0].ID()
	} else {
		newImage, err := runtime.ImageRuntime().New(getContext(), image, c.String("signature-policy"), c.String("authfile"), writer, &dockerRegistryOptions, image2.SigningOptions{}, true, forceSecure)
		if err != nil {
			return errors.Wrapf(err, "error pulling image %q", image)
		}
		imgID = newImage.ID()
	}

	// Intentionally choosing to ignore if there is an error because
	// outputting the image ID is a NTH and not integral to the pull
	fmt.Println(imgID)
	return nil
}
