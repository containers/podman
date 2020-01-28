package image

import (
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/pkg/errors"
)

// imageParts describes the parts of an image's name
type imageParts struct {
	unnormalizedRef reference.Named // WARNING: Did not go through docker.io[/library] normalization
	hasRegistry     bool
}

// Registries must contain a ":" or a "." or be localhost; this helper exists for users of reference.Parse.
// For inputs that should use the docker.io[/library] normalization, use reference.ParseNormalizedNamed instead.
func isRegistry(name string) bool {
	return strings.ContainsAny(name, ".:") || name == "localhost"
}

// GetImageBaseName uses decompose and string splits to obtain the base
// name of an image.  Doing this here because it beats changing the
// imageParts struct names to be exported as well.
func GetImageBaseName(input string) (string, error) {
	decomposedImage, err := decompose(input)
	if err != nil {
		return "", err
	}
	splitImageName := strings.Split(decomposedImage.unnormalizedRef.Name(), "/")
	return splitImageName[len(splitImageName)-1], nil
}

// decompose breaks an input name into an imageParts description
func decompose(input string) (imageParts, error) {
	imgRef, err := reference.Parse(input)
	if err != nil {
		return imageParts{}, err
	}
	unnormalizedNamed := imgRef.(reference.Named)
	// ip.unnormalizedRef, because it uses reference.Parse and not reference.ParseNormalizedNamed,
	// does not use the standard heuristics for domains vs. namespaces/repos, so we need to check
	// explicitly.
	hasRegistry := isRegistry(reference.Domain(unnormalizedNamed))
	return imageParts{
		unnormalizedRef: unnormalizedNamed,
		hasRegistry:     hasRegistry,
	}, nil
}

// suspiciousRefNameTagValuesForSearch returns a "tag" value used in a previous implementation.
// This exists only to preserve existing behavior in heuristic code; it’s dubious that that behavior is correct,
// gespecially for the tag value.
func (ip *imageParts) suspiciousRefNameTagValuesForSearch() (string, string, string) {
	registry := reference.Domain(ip.unnormalizedRef)
	imageName := reference.Path(ip.unnormalizedRef)
	// ip.unnormalizedRef, because it uses reference.Parse and not reference.ParseNormalizedNamed,
	// does not use the standard heuristics for domains vs. namespaces/repos.
	if registry != "" && !isRegistry(registry) {
		imageName = registry + "/" + imageName
		registry = ""
	}

	var tag string
	if tagged, isTagged := ip.unnormalizedRef.(reference.NamedTagged); isTagged {
		tag = tagged.Tag()
	} else if _, hasDigest := ip.unnormalizedRef.(reference.Digested); hasDigest {
		tag = "none"
	} else {
		tag = LatestTag
	}
	return registry, imageName, tag
}

// referenceWithRegistry returns a (normalized) reference.Named composed of ip (with !ip.hasRegistry)
// qualified with registry.
func (ip *imageParts) referenceWithRegistry(registry string) (reference.Named, error) {
	if ip.hasRegistry {
		return nil, errors.Errorf("internal error: referenceWithRegistry called on imageParts with a registry (%#v)", *ip)
	}
	// We could build a reference.WithName+WithTag/WithDigest here, but we need to round-trip via a string
	// and a ParseNormalizedNamed anyway to get the right normalization of docker.io/library, so
	// just use a string directly.
	qualified := registry + "/" + ip.unnormalizedRef.String()
	ref, err := reference.ParseNormalizedNamed(qualified)
	if err != nil {
		return nil, errors.Wrapf(err, "error normalizing registry+unqualified reference %#v", qualified)
	}
	return ref, nil
}

// normalizedReference returns a (normalized) reference for ip (with ip.hasRegistry)
func (ip *imageParts) normalizedReference() (reference.Named, error) {
	if !ip.hasRegistry {
		return nil, errors.Errorf("internal error: normalizedReference called on imageParts without a registry (%#v)", *ip)
	}
	// We need to round-trip via a string to get the right normalization of docker.io/library
	s := ip.unnormalizedRef.String()
	ref, err := reference.ParseNormalizedNamed(s)
	if err != nil { // Should never happen
		return nil, errors.Wrapf(err, "error normalizing qualified reference %#v", s)
	}
	return ref, nil
}
