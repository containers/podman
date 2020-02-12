package main

import (
	"fmt"
	"io"
	"os"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/docker"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pullCommand     cliconfig.PullValues
	pullDescription = `Pulls an image from a registry and stores it locally.

  An image can be pulled using its tag or digest. If a tag is not specified, the image with the 'latest' tag (if it exists) is pulled.`
	_pullCommand = &cobra.Command{
		Use:   "pull [flags] IMAGE-PATH",
		Short: "Pull an image from a registry",
		Long:  pullDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pullCommand.InputArgs = args
			pullCommand.GlobalFlags = MainGlobalOpts
			pullCommand.Remote = remoteclient
			return pullCmd(&pullCommand)
		},
		Example: `podman pull imageName
  podman pull fedora:latest`,
	}
)

func init() {

	if !remote {
		_pullCommand.Example = fmt.Sprintf("%s\n  podman pull --cert-dir image/certs --authfile temp-auths/myauths.json docker://docker.io/myrepo/finaltest", _pullCommand.Example)

	}
	pullCommand.Command = _pullCommand
	pullCommand.SetHelpTemplate(HelpTemplate())
	pullCommand.SetUsageTemplate(UsageTemplate())
	flags := pullCommand.Flags()
	flags.BoolVar(&pullCommand.AllTags, "all-tags", false, "All tagged images in the repository will be pulled")
	flags.StringVar(&pullCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&pullCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&pullCommand.OverrideArch, "override-arch", "", "use `ARCH` instead of the architecture of the machine for choosing images")
	flags.StringVar(&pullCommand.OverrideOS, "override-os", "", "use `OS` instead of the running OS for choosing images")
	markFlagHidden(flags, "override-os")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&pullCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&pullCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
		flags.StringVar(&pullCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		flags.BoolVar(&pullCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
		markFlagHidden(flags, "signature-policy")
	}
}

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cliconfig.PullValues) (retError error) {
	defer func() {
		if retError != nil && exitCode == 0 {
			exitCode = 1
		}
	}()
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "pullCmd")
		defer span.Finish()
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)

	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments. Requires exactly 1")
	}

	if c.Authfile != "" {
		if _, err := os.Stat(c.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.Authfile)
		}
	}

	ctx := getContext()
	imageName := args[0]

	imageRef, err := alltransports.ParseImageName(imageName)
	if err != nil {
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s://%s", docker.Transport.Name(), imageName))
		if err != nil {
			return errors.Errorf("invalid image reference %q", imageName)
		}
	}

	var writer io.Writer
	if !c.Quiet {
		writer = os.Stderr
	}
	// Special-case for docker-archive which allows multiple tags.
	if imageRef.Transport().Name() == dockerarchive.Transport.Name() {
		newImage, err := runtime.LoadFromArchiveReference(getContext(), imageRef, c.SignaturePolicy, writer)
		if err != nil {
			return errors.Wrapf(err, "error pulling image %q", imageName)
		}
		fmt.Println(newImage[0].ID())
		return nil
	}

	var registryCreds *types.DockerAuthConfig
	if c.Flag("creds").Changed {
		creds, err := util.ParseRegistryCreds(c.Creds)
		if err != nil {
			return err
		}
		registryCreds = creds
	}
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      c.CertDir,
		OSChoice:            c.OverrideOS,
		ArchitectureChoice:  c.OverrideArch,
	}
	if c.IsSet("tls-verify") {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	if !c.Bool("all-tags") {
		newImage, err := runtime.New(getContext(), imageName, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			return errors.Wrapf(err, "error pulling image %q", imageName)
		}
		fmt.Println(newImage.ID())
		return nil
	}

	// --all-tags requires the docker transport
	if imageRef.Transport().Name() != docker.Transport.Name() {
		return errors.New("--all-tags requires docker transport")
	}

	// all-tags doesn't work with a tagged reference, so let's check early
	namedRef, err := reference.Parse(imageName)
	if err != nil {
		return errors.Wrapf(err, "error parsing %q", imageName)
	}
	if _, isTagged := namedRef.(reference.Tagged); isTagged {
		return errors.New("--all-tags requires a reference without a tag")

	}

	systemContext := image.GetSystemContext("", c.Authfile, false)
	tags, err := docker.GetRepositoryTags(ctx, systemContext, imageRef)
	if err != nil {
		return errors.Wrapf(err, "error getting repository tags")
	}

	var foundIDs []string
	for _, tag := range tags {
		name := imageName + ":" + tag
		newImage, err := runtime.New(getContext(), name, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			logrus.Errorf("error pulling image %q", name)
			continue
		}
		foundIDs = append(foundIDs, newImage.ID())
	}

	if len(tags) != len(foundIDs) {
		return errors.Errorf("error pulling image %q", imageName)
	}

	if len(foundIDs) > 1 {
		fmt.Println("Pulled Images:")
	}
	for _, id := range foundIDs {
		fmt.Println(id)
	}

	return nil
}
