package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	installFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.BoolFlag{
			Name:  "display",
			Usage: "preview the command that `podman install` would execute",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "`pathname` of a directory containing TLS certificates and keys",
		},
		cli.StringFlag{
			Name:  "creds",
			Usage: "`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "Assign a name to the container",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output information when installing images",
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

	installDescription = `
Installs an image from a registry and stores it locally.
An image can be installed using its tag or digest. If a tag is not
specified, the image with the 'latest' tag (if it exists) is installed
`
	installCommand = cli.Command{
		Name:         "install",
		Usage:        "Install an image from a registry",
		Description:  installDescription,
		Flags:        installFlags,
		Action:       installCmd,
		ArgsUsage:    "",
		OnUsageError: usageErrorHandler,
	}
)

// installCmd gets the data from the command line and calls installImage
// to copy an image from a registry to a local machine
func installCmd(c *cli.Context) error {
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
	if err := validateFlags(c, installFlags); err != nil {
		return err
	}
	image := args[0]

	newImage, err := getImage(c, runtime, image)
	if err != nil {
		return errors.Wrapf(err, "error installing image %q", image)
	}

	data, err := newImage.Inspect(getContext())
	if err != nil {
		return errors.Wrapf(err, "error parsing image data %q", newImage.ID())
	}
	fmt.Println(data.Labels["INSTALL"])
	return nil
}
