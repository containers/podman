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
func getCopyOptions(reportWriter io.Writer, signaturePolicyPath string, srcDockerRegistry, destDockerRegistry *DockerRegistryOptions, signing SigningOptions, authFile, manifestType string, forceCompress bool, additionalDockerArchiveTags []reference.NamedTagged) *cp.Options {
	if srcDockerRegistry == nil {
		srcDockerRegistry = &DockerRegistryOptions{}
	}
	if destDockerRegistry == nil {
		destDockerRegistry = &DockerRegistryOptions{}
	}
	srcContext := srcDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress, additionalDockerArchiveTags)
	destContext := destDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress, additionalDockerArchiveTags)
	return &cp.Options{
		RemoveSignatures:      signing.RemoveSignatures,
		SignBy:                signing.SignBy,
		ReportWriter:          reportWriter,
		SourceCtx:             srcContext,
		DestinationCtx:        destContext,
		ForceManifestMIMEType: manifestType,
	}
}

// getPolicyContext sets up, initializes and returns a new context for the specified policy
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

// ReposToMap parses the specified repotags and returns a map with repositories
// as keys and the corresponding arrays of tags as values.
func ReposToMap(repotags []string) map[string][]string {
	// map format is repo -> tag
	repos := make(map[string][]string)
	for _, repo := range repotags {
		var repository, tag string
		if len(repo) > 0 {
			li := strings.LastIndex(repo, ":")
			repository = repo[0:li]
			tag = repo[li+1:]
		}
		repos[repository] = append(repos[repository], tag)
	}
	if len(repos) == 0 {
		repos["<none>"] = []string{"<none>"}
	}
	return repos
}

// GetAdditionalTags returns a list of reference.NamedTagged for the
// additional tags given in images
func GetAdditionalTags(images []string) ([]reference.NamedTagged, error) {
	var allTags []reference.NamedTagged
	for _, img := range images {
		ref, err := reference.ParseNormalizedNamed(img)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing additional tags")
		}
		refTagged, isTagged := ref.(reference.NamedTagged)
		if isTagged {
			allTags = append(allTags, refTagged)
		}
	}
	return allTags, nil
}
