package image

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// findImageInRepotags takes an imageParts struct and searches images' repotags for
// a match on name:tag
func findImageInRepotags(search imageParts, images []*Image) (*storage.Image, error) {
	_, searchName, searchSuspiciousTagValueForSearch := search.suspiciousRefNameTagValuesForSearch()
	var results []*storage.Image
	for _, image := range images {
		for _, name := range image.Names() {
			d, err := decompose(name)
			// if we get an error, ignore and keep going
			if err != nil {
				continue
			}
			_, dName, dSuspiciousTagValueForSearch := d.suspiciousRefNameTagValuesForSearch()
			if dName == searchName && dSuspiciousTagValueForSearch == searchSuspiciousTagValueForSearch {
				results = append(results, image.image)
				continue
			}
			// account for registry:/somedir/image
			if strings.HasSuffix(dName, "/"+searchName) && dSuspiciousTagValueForSearch == searchSuspiciousTagValueForSearch {
				results = append(results, image.image)
				continue
			}
		}
	}
	if len(results) == 0 {
		return &storage.Image{}, errors.Errorf("unable to find a name and tag match for %s in repotags", searchName)
	} else if len(results) > 1 {
		return &storage.Image{}, errors.Wrapf(define.ErrMultipleImages, searchName)
	}
	return results[0], nil
}

// getCopyOptions constructs a new containers/image/copy.Options{} struct from the given parameters, inheriting some from sc.
func getCopyOptions(sc *types.SystemContext, reportWriter io.Writer, srcDockerRegistry, destDockerRegistry *DockerRegistryOptions, signing SigningOptions, manifestType string, additionalDockerArchiveTags []reference.NamedTagged) *cp.Options {
	if srcDockerRegistry == nil {
		srcDockerRegistry = &DockerRegistryOptions{}
	}
	if destDockerRegistry == nil {
		destDockerRegistry = &DockerRegistryOptions{}
	}
	srcContext := srcDockerRegistry.GetSystemContext(sc, additionalDockerArchiveTags)
	destContext := destDockerRegistry.GetSystemContext(sc, additionalDockerArchiveTags)
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

// IsValidImageURI checks if image name has valid format
func IsValidImageURI(imguri string) (bool, error) {
	uri := "http://" + imguri
	u, err := url.Parse(uri)
	if err != nil {
		return false, errors.Wrapf(err, "invalid image uri: %s", imguri)
	}
	reg := regexp.MustCompile(`^[a-zA-Z0-9-_\.]+\/?:?[0-9]*[a-z0-9-\/:]*$`)
	ret := reg.FindAllString(u.Host, -1)
	if len(ret) == 0 {
		return false, errors.Wrapf(err, "invalid image uri: %s", imguri)
	}
	reg = regexp.MustCompile(`^[a-z0-9-:\./]*$`)
	ret = reg.FindAllString(u.Fragment, -1)
	if len(ret) == 0 {
		return false, errors.Wrapf(err, "invalid image uri: %s", imguri)
	}
	return true, nil
}

// imageNameForSaveDestination returns a Docker-like reference appropriate for saving img,
// which the user referred to as imgUserInput; or an empty string, if there is no appropriate
// reference.
func imageNameForSaveDestination(img *Image, imgUserInput string) string {
	if strings.Contains(img.ID(), imgUserInput) {
		return ""
	}

	prepend := ""
	localRegistryPrefix := fmt.Sprintf("%s/", DefaultLocalRegistry)
	if !strings.HasPrefix(imgUserInput, localRegistryPrefix) {
		// we need to check if localhost was added to the image name in NewFromLocal
		for _, name := range img.Names() {
			// If the user is saving an image in the localhost registry,  getLocalImage need
			// a name that matches the format localhost/<tag1>:<tag2> or localhost/<tag>:latest to correctly
			// set up the manifest and save.
			if strings.HasPrefix(name, localRegistryPrefix) && (strings.HasSuffix(name, imgUserInput) || strings.HasSuffix(name, fmt.Sprintf("%s:latest", imgUserInput))) {
				prepend = localRegistryPrefix
				break
			}
		}
	}
	return fmt.Sprintf("%s%s", prepend, imgUserInput)
}
