package image

import (
	"io"
	"strings"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

func getTags(nameInput string) (reference.NamedTagged, bool, error) {
	inputRef, err := reference.Parse(nameInput)
	if err != nil {
		return nil, false, errors.Wrapf(err, "unable to obtain tag from input name")
	}
	tagged, isTagged := inputRef.(reference.NamedTagged)

	return tagged, isTagged, nil
}

// findImageInRepotags takes an imageParts struct and searches images' repotags for
// a match on name:tag
func findImageInRepotags(search imageParts, images []*Image) (*storage.Image, error) {
	var results []*storage.Image
	for _, image := range images {
		for _, name := range image.Names() {
			d, err := decompose(name)
			// if we get an error, ignore and keep going
			if err != nil {
				continue
			}
			if d.name == search.name && d.tag == search.tag {
				results = append(results, image.image)
				continue
			}
			// account for registry:/somedir/image
			if strings.HasSuffix(d.name, search.name) && d.tag == search.tag {
				results = append(results, image.image)
				continue
			}
		}
	}
	if len(results) == 0 {
		return &storage.Image{}, errors.Errorf("unable to find a name and tag match for %s in repotags", search.name)
	} else if len(results) > 1 {
		return &storage.Image{}, errors.Errorf("found multiple name and tag matches for %s in repotags", search.name)
	}
	return results[0], nil
}

// getCopyOptions constructs a new containers/image/copy.Options{} struct from the given parameters
func getCopyOptions(reportWriter io.Writer, signaturePolicyPath string, srcDockerRegistry, destDockerRegistry *DockerRegistryOptions, signing SigningOptions, authFile, manifestType string, forceCompress bool) *cp.Options {
	if srcDockerRegistry == nil {
		srcDockerRegistry = &DockerRegistryOptions{}
	}
	if destDockerRegistry == nil {
		destDockerRegistry = &DockerRegistryOptions{}
	}
	srcContext := srcDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress)
	destContext := destDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress)
	return &cp.Options{
		RemoveSignatures:      signing.RemoveSignatures,
		SignBy:                signing.SignBy,
		ReportWriter:          reportWriter,
		SourceCtx:             srcContext,
		DestinationCtx:        destContext,
		ForceManifestMIMEType: manifestType,
	}
}

// getPolicyContext sets up, intializes and returns a new context for the specified policy
func getPolicyContext(ctx *types.SystemContext) (*signature.PolicyContext, error) {
	policy, err := signature.DefaultPolicy(ctx)
	if err != nil {
		return nil, err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	return policyContext, nil
}

// hasTransport determines if the image string contains '://', returns bool
func hasTransport(image string) bool {
	return strings.Contains(image, "://")
}
