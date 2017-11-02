package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/libpod/common"
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

	pullDescription = "Pulls an image from a registry and stores it locally.\n" +
		"An image can be pulled using its tag or digest. If a tag is not\n" +
		"specified, the image with the 'latest' tag (if it exists) is pulled."
	pullCommand = cli.Command{
		Name:        "pull",
		Usage:       "pull an image from a registry",
		Description: pullDescription,
		Flags:       pullFlags,
		Action:      pullCmd,
		ArgsUsage:   "",
	}
)

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cli.Context) error {
	runtime, err := getRuntime(c)
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
	if c.String("creds") != "" {
		creds, err := common.ParseRegistryCreds(c.String("creds"))
		if err != nil {
			if err == common.ErrNoPassword {
				fmt.Print("Password: ")
				password, err := terminal.ReadPassword(0)
				if err != nil {
					return errors.Wrapf(err, "could not read password from terminal")
				}
				creds.Password = string(password)
			} else {
				return err
			}
		}
		registryCreds = creds
	}

	var writer io.Writer
	if !c.Bool("quiet") {
		writer = os.Stdout
	}

	options := libpod.CopyOptions{
		SignaturePolicyPath: c.String("signature-policy"),
		AuthFile:            c.String("authfile"),
		DockerRegistryOptions: common.DockerRegistryOptions{
			DockerRegistryCreds:         registryCreds,
			DockerCertPath:              c.String("cert-dir"),
			DockerInsecureSkipTLSVerify: !c.BoolT("tls-verify"),
		},
		Writer: writer,
	}

	return runtime.PullImage(image, options)

}
