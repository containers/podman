package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pushCommand     cliconfig.PushValues
	pushDescription = fmt.Sprintf(`Pushes an image to a specified location.

  The Image "DESTINATION" uses a "transport":"details" format. See podman-push(1) section "DESTINATION" for the expected format.`)

	_pushCommand = &cobra.Command{
		Use:   "push [flags] IMAGE REGISTRY",
		Short: "Push an image to a specified destination",
		Long:  pushDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pushCommand.InputArgs = args
			pushCommand.GlobalFlags = MainGlobalOpts
			pushCommand.Remote = remoteclient
			return pushCmd(&pushCommand)
		},
		Example: `podman push imageID docker://registry.example.com/repository:tag
  podman push imageID oci-archive:/path/to/layout:image:tag`,
	}
)

func init() {
	if !remote {
		_pushCommand.Example = fmt.Sprintf("%s\n  podman push --authfile temp-auths/myauths.json alpine docker://docker.io/myrepo/alpine", _pushCommand.Example)

	}

	pushCommand.Command = _pushCommand
	pushCommand.SetHelpTemplate(HelpTemplate())
	pushCommand.SetUsageTemplate(UsageTemplate())
	flags := pushCommand.Flags()
	flags.StringVar(&pushCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.StringVar(&pushCommand.Digestfile, "digestfile", "", "After copying the image, write the digest of the resulting image to the file")
	flags.StringVarP(&pushCommand.Format, "format", "f", "", "Manifest type (oci, v2s1, or v2s2) to use when pushing an image using the 'dir:' transport (default is manifest type of source)")
	flags.BoolVarP(&pushCommand.Quiet, "quiet", "q", false, "Don't output progress information when pushing images")
	flags.BoolVar(&pushCommand.RemoveSignatures, "remove-signatures", false, "Discard any pre-existing signatures in the image")
	flags.StringVar(&pushCommand.SignBy, "sign-by", "", "Add a signature at the destination using the specified key")

	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&pushCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&pushCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
		flags.BoolVar(&pushCommand.Compress, "compress", false, "Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type as source)")
		flags.StringVar(&pushCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		flags.BoolVar(&pushCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
		markFlagHidden(flags, "signature-policy")
	}
}

func pushCmd(c *cliconfig.PushValues) error {
	var (
		registryCreds *types.DockerAuthConfig
		destName      string
	)

	if c.Authfile != "" {
		if _, err := os.Stat(c.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.Authfile)
		}
	}

	args := c.InputArgs
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

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	// --compress and --format can only be used for the "dir" transport
	splitArg := strings.SplitN(destName, ":", 2)
	if c.Flag("compress").Changed || c.Flag("format").Changed {
		if splitArg[0] != directory.Transport.Name() {
			return errors.Errorf("--compress and --format can be set only when pushing to a directory using the 'dir' transport")
		}
	}

	certPath := c.CertDir
	removeSignatures := c.RemoveSignatures
	signBy := c.SignBy

	if c.Flag("creds").Changed {
		creds, err := util.ParseRegistryCreds(c.Creds)
		if err != nil {
			return err
		}
		registryCreds = creds
	}

	var writer io.Writer
	if !c.Quiet {
		writer = os.Stderr
	}

	var manifestType string
	if c.Flag("format").Changed {
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

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      certPath,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	so := image.SigningOptions{
		RemoveSignatures: removeSignatures,
		SignBy:           signBy,
	}

	return runtime.Push(getContext(), srcName, destName, manifestType, c.Authfile, c.String("digestfile"), c.SignaturePolicy, writer, c.Compress, so, &dockerRegistryOptions, nil)
}
