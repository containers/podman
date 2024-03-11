package ocipull

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/sirupsen/logrus"
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

var (
	// noSignaturePolicy is a default policy if policy.json is not found on
	// the host machine.
	noSignaturePolicy string = `{"default":[{"type":"insecureAcceptAnything"}]}`
)

// Pull `imageInput` from a container registry to `sourcePath`.
func Pull(ctx context.Context, imageInput types.ImageReference, localDestPath *define.VMFile, options *PullOptions) error {
	var (
		policy *signature.Policy
	)
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

	// Policy paths returns a slice of directories where the policy.json
	// may live.  Iterate those directories and try to see if any are
	// valid ignoring when the file does not exist
	for _, path := range policyPaths() {
		policy, err = signature.NewPolicyFromFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("reading signature policy: %w", err)
		}
	}

	// If no policy has been found yet, we use a no signature policy automatically
	if policy == nil {
		logrus.Debug("no signature policy file found: using default allow everything signature policy")
		policy, err = signature.NewPolicyFromBytes([]byte(noSignaturePolicy))
		if err != nil {
			return fmt.Errorf("obtaining signature policy: %w", err)
		}
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
