package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	pushFlags = []cli.Flag{
		cli.StringFlag{
			Name:   "signature-policy",
			Usage:  "`pathname` of signature policy file (not usually used)",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "creds",
			Usage: "`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "`pathname` of a directory containing TLS certificates and keys",
		},
		cli.BoolFlag{
			Name:  "compress",
			Usage: "compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type as source)",
		},
		cli.StringFlag{
			Name:  "format, f",
			Usage: "manifest type (oci, v2s1, or v2s2) to use when pushing an image using the 'dir:' transport (default is manifest type of source)",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when contacting registries (default: true)",
		},
		cli.BoolFlag{
			Name:  "remove-signatures",
			Usage: "discard any pre-existing signatures in the image",
		},
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "add a signature at the destination using the specified key",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "don't output progress information when pushing images",
		},
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
	}
	pushDescription = fmt.Sprintf(`
   Pushes an image to a specified location.
   The Image "DESTINATION" uses a "transport":"details" format.
   See podman-push(1) section "DESTINATION" for the expected format`)

	pushCommand = cli.Command{
		Name:         "push",
		Usage:        "Push an image to a specified destination",
		Description:  pushDescription,
		Flags:        pushFlags,
		Action:       pushCmd,
		ArgsUsage:    "IMAGE DESTINATION",
		OnUsageError: usageErrorHandler,
	}
)

func pushCmd(c *cli.Context) error {
	var (
		registryCreds *types.DockerAuthConfig
		destName      string
		forceSecure   bool
	)

	args := c.Args()
	if len(args) == 0 || len(args) > 2 {
		return errors.New("podman push requires at least one image name, and optionally a second to specify a different destination name")
	}
	srcName := args[0]
	switch len(args) {
	case 1:
		destName = args[0]
	case 2:
		destName = args[1]
	}
	if err := validateFlags(c, pushFlags); err != nil {
		return err
	}

	// --compress and --format can only be used for the "dir" transport
	splitArg := strings.SplitN(destName, ":", 2)
	if c.IsSet("compress") || c.IsSet("format") {
		if splitArg[0] != directory.Transport.Name() {
			return errors.Errorf("--compress and --format can be set only when pushing to a directory using the 'dir' transport")
		}
	}

	certPath := c.String("cert-dir")
	skipVerify := !c.BoolT("tls-verify")
	removeSignatures := c.Bool("remove-signatures")
	signBy := c.String("sign-by")

	if c.IsSet("creds") {
		creds, err := util.ParseRegistryCreds(c.String("creds"))
		if err != nil {
			return err
		}
		registryCreds = creds
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	var writer io.Writer
	if !c.Bool("quiet") {
		writer = os.Stderr
	}

	var manifestType string
	if c.IsSet("format") {
		switch c.String("format") {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s1":
			manifestType = manifest.DockerV2Schema1SignedMediaType
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return fmt.Errorf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", c.String("format"))
		}
	}

	if c.IsSet("tls-verify") {
		forceSecure = c.Bool("tls-verify")
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              certPath,
		DockerInsecureSkipTLSVerify: skipVerify,
	}

	so := image.SigningOptions{
		RemoveSignatures: removeSignatures,
		SignBy:           signBy,
	}

	newImage, err := runtime.ImageRuntime().NewFromLocal(srcName)
	if err != nil {
		return err
	}

	return newImage.PushImageToHeuristicDestination(getContext(), destName, manifestType, c.String("authfile"), c.String("signature-policy"), writer, c.Bool("compress"), so, &dockerRegistryOptions, forceSecure, nil)
}
