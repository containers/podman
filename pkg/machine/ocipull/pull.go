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

	path, err := policyPath()
	if err != nil {
		return err
	}

	policy, err := signature.NewPolicyFromFile(path)
	if err != nil {
		return fmt.Errorf("obtaining signature policy: %w", err)
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
