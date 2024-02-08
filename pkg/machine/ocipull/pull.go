package ocipull

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/machine/define"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// PullOptions includes data to alter certain knobs when pulling a source
// image.
type PullOptions struct {
	// Require HTTPS and verify certificates when accessing the registry.
	TLSVerify bool
	// [username[:password] to use when connecting to the registry.
	Credentials string
	// Quiet the progress bars when pushing.
	Quiet bool
}

// Pull `imageInput` from a container registry to `sourcePath`.
func Pull(ctx context.Context, imageInput types.ImageReference, localDestPath *define.VMFile, options *PullOptions) error {
	if _, err := os.Stat(localDestPath.GetPath()); err == nil {
		return fmt.Errorf("%q already exists", localDestPath.GetPath())
	}

	destRef, err := layout.ParseReference(localDestPath.GetPath())
	if err != nil {
		return err
	}

	sysCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!options.TLSVerify),
	}
	if options.Credentials != "" {
		authConf, err := parse.AuthConfig(options.Credentials)
		if err != nil {
			return err
		}
		sysCtx.DockerAuthConfig = authConf
	}

	if err := validateSourceImageReference(ctx, imageInput, sysCtx); err != nil {
		return err
	}

	policy, err := signature.DefaultPolicy(sysCtx)
	if err != nil {
		return fmt.Errorf("obtaining default signature policy: %w", err)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("creating new signature policy context: %w", err)
	}

	copyOpts := copy.Options{
		SourceCtx: sysCtx,
	}
	if !options.Quiet {
		copyOpts.ReportWriter = os.Stderr
	}
	if _, err := copy.Image(ctx, policyContext, destRef, imageInput, &copyOpts); err != nil {
		return fmt.Errorf("pulling source image: %w", err)
	}

	return nil
}

func stringToImageReference(imageInput string) (types.ImageReference, error) { //nolint:unused
	if shortnames.IsShortName(imageInput) {
		return nil, fmt.Errorf("pulling source images by short name (%q) is not supported, please use a fully-qualified name", imageInput)
	}

	ref, err := alltransports.ParseImageName("docker://" + imageInput)
	if err != nil {
		return nil, fmt.Errorf("parsing image name: %w", err)
	}

	return ref, nil
}

func validateSourceImageReference(ctx context.Context, ref types.ImageReference, sysCtx *types.SystemContext) error {
	src, err := ref.NewImageSource(ctx, sysCtx)
	if err != nil {
		return fmt.Errorf("creating image source from reference: %w", err)
	}
	defer src.Close()

	ociManifest, _, _, err := readManifestFromImageSource(ctx, src)
	if err != nil {
		return err
	}
	if ociManifest.Config.MediaType != specV1.MediaTypeImageConfig {
		return fmt.Errorf("invalid media type of image config %q (expected: %q)", ociManifest.Config.MediaType, specV1.MediaTypeImageConfig)
	}
	return nil
}
