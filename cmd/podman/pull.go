package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/docker"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/common"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	opentracing "github.com/opentracing/opentracing-go"
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
			return pullCmd(&pullCommand)
		},
		Example: `podman pull imageName
  podman pull --cert-dir image/certs --authfile temp-auths/myauths.json docker://docker.io/myrepo/finaltest
  podman pull fedora:latest`,
	}
)

func init() {
	pullCommand.Command = _pullCommand
	pullCommand.SetHelpTemplate(HelpTemplate())
	pullCommand.SetUsageTemplate(UsageTemplate())
	flags := pullCommand.Flags()
	flags.BoolVar(&pullCommand.AllTags, "all-tags", false, "All tagged images inthe repository will be pulled")
	flags.StringVar(&pullCommand.Authfile, "authfile", "", "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&pullCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&pullCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&pullCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&pullCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&pullCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

}

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cliconfig.PullValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "pullCmd")
		defer span.Finish()
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)

	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments. Requires exactly 1")
	}

	arr := strings.SplitN(args[0], ":", 2)
	if len(arr) == 2 {
		if c.Bool("all-tags") {
			return errors.Errorf("tag can't be used with --all-tags")
		}
	}
	ctx := getContext()
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
		fmt.Println(newImage[0].ID())
	} else {
		authfile := getAuthFile(c.String("authfile"))
		spec := image
		systemContext := common.GetSystemContext("", authfile, false)
		srcRef, err := alltransports.ParseImageName(spec)
		if err != nil {
			dockerTransport := "docker://"
			logrus.Debugf("error parsing image name %q, trying with transport %q: %v", spec, dockerTransport, err)
			spec = dockerTransport + spec
			srcRef2, err2 := alltransports.ParseImageName(spec)
			if err2 != nil {
				return errors.Wrapf(err2, "error parsing image name %q", image)
			}
			srcRef = srcRef2
		}
		var names []string
		if c.Bool("all-tags") {
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
		} else {
			names = append(names, spec)
		}
		var foundIDs []string
		foundImage := true
		for _, name := range names {
			newImage, err := runtime.New(getContext(), name, c.String("signature-policy"), authfile, writer, &dockerRegistryOptions, image2.SigningOptions{}, true, nil)
			if err != nil {
				println(errors.Wrapf(err, "error pulling image %q", name))
				foundImage = false
				continue
			}
			foundIDs = append(foundIDs, newImage.ID())
		}
		if len(names) == 1 && !foundImage {
			return errors.Wrapf(err, "error pulling image %q", image)
		}
		if len(names) > 1 {
			fmt.Println("Pulled Images:")
		}
		for _, id := range foundIDs {
			fmt.Println(id)
		}
	} // end else if strings.HasPrefix(image, dockerarchive.Transport.Name()+":")
	return nil
}
