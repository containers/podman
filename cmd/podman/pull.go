package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/docker"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
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
	markFlagHidden(flags, "override-arch")
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

	arr := strings.SplitN(args[0], ":", 2)
	if len(arr) == 2 {
		if c.Bool("all-tags") {
			return errors.Errorf("tag can't be used with --all-tags")
		}
	}

	ctx := getContext()
	imgArg := args[0]

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
	)
	if !c.Quiet {
		writer = os.Stderr
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

	// Special-case for docker-archive which allows multiple tags.
	if strings.HasPrefix(imgArg, dockerarchive.Transport.Name()+":") {
		srcRef, err := alltransports.ParseImageName(imgArg)
		if err != nil {
			return errors.Wrapf(err, "error parsing %q", imgArg)
		}
		newImage, err := runtime.LoadFromArchiveReference(getContext(), srcRef, c.SignaturePolicy, writer)
		if err != nil {
			return errors.Wrapf(err, "error pulling image from %q", imgArg)
		}
		fmt.Println(newImage[0].ID())

		return nil
	}

	// FIXME: the default pull consults the registries.conf's search registries
	// while the all-tags pull does not. This behavior must be fixed in the
	// future and span across c/buildah, c/image and c/libpod to avoid redundant
	// and error prone code.
	//
	// See https://bugzilla.redhat.com/show_bug.cgi?id=1701922 for background
	// information.
	if !c.Bool("all-tags") {
		newImage, err := runtime.New(getContext(), imgArg, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			return errors.Wrapf(err, "error pulling image %q", imgArg)
		}
		fmt.Println(newImage.ID())
		return nil
	}

	// FIXME: all-tags should use the libpod backend instead of baking its own bread.
	spec := imgArg
	systemContext := image.GetSystemContext("", c.Authfile, false)
	srcRef, err := alltransports.ParseImageName(spec)
	if err != nil {
		dockerTransport := "docker://"
		logrus.Debugf("error parsing image name %q, trying with transport %q: %v", spec, dockerTransport, err)
		spec = dockerTransport + spec
		srcRef2, err2 := alltransports.ParseImageName(spec)
		if err2 != nil {
			return errors.Wrapf(err2, "error parsing image name %q", imgArg)
		}
		srcRef = srcRef2
	}
	var names []string
	if srcRef.DockerReference() == nil {
		return errors.New("Non-docker transport is currently not supported")
	}
	tags, err := docker.GetRepositoryTags(ctx, systemContext, srcRef)
	if err != nil {
		return errors.Wrapf(err, "error getting repository tags")
	}
	for _, tag := range tags {
		name := spec + ":" + tag
		names = append(names, name)
	}

	var foundIDs []string
	foundImage := true
	for _, name := range names {
		newImage, err := runtime.New(getContext(), name, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			logrus.Errorf("error pulling image %q", name)
			foundImage = false
			continue
		}
		foundIDs = append(foundIDs, newImage.ID())
	}
	if len(names) == 1 && !foundImage {
		return errors.Wrapf(err, "error pulling image %q", imgArg)
	}
	if len(names) > 1 {
		fmt.Println("Pulled Images:")
	}
	for _, id := range foundIDs {
		fmt.Println(id)
	}

	return nil
}
