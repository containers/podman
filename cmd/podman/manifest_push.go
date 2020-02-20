package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/buildah/manifests"
	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/buildah/util"
	cp "github.com/containers/image/v5/copy"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestPushCommand     cliconfig.ManifestPushValues
	manifestPushDescription = `pushes manifest lists and image indexes to registries`
	_manifestPushCommand    = &cobra.Command{
		Use:   "push [image] [repo]",
		Short: "manifest push",
		Long:  manifestPushDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPushCommand.InputArgs = args
			manifestPushCommand.GlobalFlags = MainGlobalOpts
			manifestPushCommand.Remote = remoteclient
			return manifestPushCmd(&manifestPushCommand)
		},
		Example: `podman manifest push mylist:v1.11 transport:imageName`,
		Args:    cobra.MinimumNArgs(2),
	}
)

func init() {
	manifestPushCommand.Command = _manifestPushCommand
	manifestPushCommand.SetUsageTemplate(HelpTemplate())
	manifestPushCommand.SetUsageTemplate(UsageTemplate())
	flags := manifestPushCommand.Flags()
	flags.BoolVar(&manifestPushCommand.Purge, "purge", false, "remove the manifest list if push succeeds")
	flags.BoolVar(&manifestPushCommand.All, "all", false, "also push the images in the list")
	flags.StringVar(&manifestPushCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&manifestPushCommand.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	flags.StringVar(&manifestPushCommand.Creds, "creds", "", "use `[username[:password]]` for accessing the registry")
	flags.StringVar(&manifestPushCommand.Digestfile, "digestfile", "", "after copying the image, write the digest of the resulting digest to the file")
	flags.StringVarP(&manifestPushCommand.Format, "format", "f", "", "manifest type (oci or v2s2) to attempt to use when pushing the manifest list (default is manifest type of source)")
	flags.BoolVarP(&manifestPushCommand.RemoveSignatures, "remove-signatures", "", false, "don't copy signatures when pushing images")
	flags.StringVar(&manifestPushCommand.SignBy, "sign-by", "", "sign the image using a GPG key with the specified `FINGERPRINT`")
	flags.StringVar(&manifestPushCommand.SignaturePolicy, "signature-policy", "", "`pathname` of signature policy file (not usually used)")
	if err := flags.MarkHidden("signature-policy"); err != nil {
		panic(fmt.Sprintf("error marking signature-policy as hidden: %v", err))
	}
	flags.BoolVar(&manifestPushCommand.TlsVerify, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.BoolVarP(&manifestPushCommand.Quiet, "quiet", "q", false, "don't output progress information when pushing lists")
}

func manifestPushCmd(c *cliconfig.ManifestPushValues) error {
	if err := buildahcli.CheckAuthFile(c.Authfile); err != nil {
		return err
	}

	listImageSpec := ""
	destSpec := ""
	args := c.InputArgs
	switch len(args) {
	case 0:
		return errors.New("At least a source list ID must be specified")
	case 1:
		return errors.New("Two arguments are necessary to push: source and destination")
	case 2:
		listImageSpec = args[0]
		destSpec = args[1]
		if listImageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, listImageSpec)
		}
		if destSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, destSpec)
		}
	default:
		return errors.New("Only two arguments are necessary to push: source and destination")
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	store := runtime.GetStore()
	systemContext := runtime.SystemContext()

	_, listImage, err := util.FindImage(store, "", systemContext, listImageSpec)
	if err != nil {
		return err
	}

	_, list, err := manifests.LoadFromImage(store, listImage.ID)
	if err != nil {
		return err
	}

	ctx := getContext()

	dest, err := alltransports.ParseImageName(destSpec)
	if err != nil {
		return err
	}

	var manifestType string
	if c.IsSet(c.Format) {
		switch c.Format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageIndex
		case "v2s2", "docker":
			manifestType = manifest.DockerV2ListMediaType
		default:
			return fmt.Errorf("unknown format %q. Choose on of the supported formats: 'oci' or 'v2s2'", c.Format)
		}
	}

	options := manifests.PushOptions{
		Store:              store,
		SystemContext:      systemContext,
		ImageListSelection: cp.CopySpecificImages,
		Instances:          nil,
		RemoveSignatures:   c.RemoveSignatures,
		SignBy:             c.SignBy,
		ManifestType:       manifestType,
	}
	if c.All {
		options.ImageListSelection = cp.CopyAllImages
	}
	if !c.Quiet {
		options.ReportWriter = os.Stderr
	}

	_, digest, err := list.Push(ctx, dest, options)

	if err == nil && c.Purge {
		_, err = store.DeleteImage(listImage.ID, true)
	}

	if c.IsSet(c.Digestfile) {
		if err = ioutil.WriteFile(c.Digestfile, []byte(digest.String()), 0644); err != nil {
			return util.GetFailureCause(err, errors.Wrapf(err, "failed to write digest to file %q", c.Digestfile))
		}
	}

	return err
}
