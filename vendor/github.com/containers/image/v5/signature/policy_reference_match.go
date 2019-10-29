// PolicyReferenceMatch implementations.

package signature

import (
	"fmt"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
)

// parseImageAndDockerReference converts an image and a reference string into two parsed entities, failing on any error and handling unidentified images.
func parseImageAndDockerReference(image types.UnparsedImage, s2 string) (reference.Named, reference.Named, error) {
	r1 := image.Reference().DockerReference()
	if r1 == nil {
		return nil, nil, PolicyRequirementError(fmt.Sprintf("Docker reference match attempted on image %s with no known Docker reference identity",
			transports.ImageName(image.Reference())))
	}
	r2, err := reference.ParseNormalizedNamed(s2)
	if err != nil {
		return nil, nil, err
	}
	return r1, r2, nil
}

func (prm *prmMatchExact) matchesDockerReference(image types.UnparsedImage, signatureDockerReference string) bool {
	intended, signature, err := parseImageAndDockerReference(image, signatureDockerReference)
	if err != nil {
		return false
	}
	// Do not add default tags: image.Reference().DockerReference() should contain it already, and signatureDockerReference should be exact; so, verify that now.
	if reference.IsNameOnly(intended) || reference.IsNameOnly(signature) {
		return false
	}
	return signature.String() == intended.String()
}

func (prm *prmMatchRepoDigestOrExact) matchesDockerReference(image types.UnparsedImage, signatureDockerReference string) bool {
	intended, signature, err := parseImageAndDockerReference(image, signatureDockerReference)
	if err != nil {
		return false
	}

	// Do not add default tags: image.Reference().DockerReference() should contain it already, and signatureDockerReference should be exact; so, verify that now.
	if reference.IsNameOnly(signature) {
		return false
	}
	switch intended.(type) {
	case reference.NamedTagged: // Includes the case when intended has both a tag and a digest.
		return signature.String() == intended.String()
	case reference.Canonical:
		// We don’t actually compare the manifest digest against the signature here; that happens prSignedBy.in UnparsedImage.Manifest.
		// Becase UnparsedImage.Manifest verifies the intended.Digest() against the manifest, and prSignedBy verifies the signature digest against the manifest,
		// we know that signature digest matches intended.Digest() (but intended.Digest() and signature digest may use different algorithms)
		return signature.Name() == intended.Name()
	default: // !reference.IsNameOnly(intended)
		return false
	}
}

func (prm *prmMatchRepository) matchesDockerReference(image types.UnparsedImage, signatureDockerReference string) bool {
	intended, signature, err := parseImageAndDockerReference(image, signatureDockerReference)
	if err != nil {
		return false
	}
	return signature.Name() == intended.Name()
}

// parseDockerReferences converts two reference strings into parsed entities, failing on any error
func parseDockerReferences(s1, s2 string) (reference.Named, reference.Named, error) {
	r1, err := reference.ParseNormalizedNamed(s1)
	if err != nil {
		return nil, nil, err
	}
	r2, err := reference.ParseNormalizedNamed(s2)
	if err != nil {
		return nil, nil, err
	}
	return r1, r2, nil
}

func (prm *prmExactReference) matchesDockerReference(image types.UnparsedImage, signatureDockerReference string) bool {
	intended, signature, err := parseDockerReferences(prm.DockerReference, signatureDockerReference)
	if err != nil {
		return false
	}
	// prm.DockerReference and signatureDockerReference should be exact; so, verify that now.
	if reference.IsNameOnly(intended) || reference.IsNameOnly(signature) {
		return false
	}
	return signature.String() == intended.String()
}

func (prm *prmExactRepository) matchesDockerReference(image types.UnparsedImage, signatureDockerReference string) bool {
	intended, signature, err := parseDockerReferences(prm.DockerRepository, signatureDockerReference)
	if err != nil {
		return false
	}
	return signature.Name() == intended.Name()
}
