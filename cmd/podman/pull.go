package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pullCommand     cliconfig.PullValues
	pullDescription = `
Pulls an image from a registry and stores it locally.
An image can be pulled using its tag or digest. If a tag is not
specified, the image with the 'latest' tag (if it exists) is pulled
`
	_pullCommand = &cobra.Command{
		Use:   "pull",
		Short: "Pull an image from a registry",
		Long:  pullDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pullCommand.InputArgs = args
			pullCommand.GlobalFlags = MainGlobalOpts
			return pullCmd(&pullCommand)
		},
		Example: "",
	}
)

func init() {
	pullCommand.Command = _pullCommand
	flags := pullCommand.Flags()
	flags.StringVar(&pullCommand.Authfile, "authfile", "", "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&pullCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&pullCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&pullCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&pullCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&pullCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries (default: true)")

	rootCmd.AddCommand(pullCommand.Command)
}

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cliconfig.PullValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) == 0 {
		logrus.Errorf("an image name must be specified")
		return nil
	}
	if len(args) > 1 {
		logrus.Errorf("too many arguments. Requires exactly 1")
		return nil
	}
	image := args[0]

	var registryCreds *types.DockerAuthConfig

	if c.Flag("creds").Changed {
		creds, err := util.ParseRegistryCreds(c.Creds)
		if err != nil {
			return err
		}
		registryCreds = creds
	}

	var (
		writer io.Writer
		imgID  string
	)
	if !c.Quiet {
		writer = os.Stderr
	}

	dockerRegistryOptions := image2.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      c.CertDir,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	// Possible for docker-archive to have multiple tags, so use LoadFromArchiveReference instead
	if strings.HasPrefix(image, dockerarchive.Transport.Name()+":") {
		srcRef, err := alltransports.ParseImageName(image)
		if err != nil {
			return errors.Wrapf(err, "error parsing %q", image)
		}
		newImage, err := runtime.LoadFromArchiveReference(getContext(), srcRef, c.SignaturePolicy, writer)
		if err != nil {
			return errors.Wrapf(err, "error pulling image from %q", image)
		}
		imgID = newImage[0].ID()
	} else {
		authfile := getAuthFile(c.Authfile)
		newImage, err := runtime.New(getContext(), image, c.SignaturePolicy, authfile, writer, &dockerRegistryOptions, image2.SigningOptions{}, true, nil)
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
