package image

import (
	"fmt"
	"strings"

	"github.com/containers/image/docker/reference"
)

// Parts describes the parts of an image's name
type Parts struct {
	transport   string
	Registry    string
	name        string
	Tag         string
	isTagged    bool
	hasRegistry bool
}

// Registries must contain a ":" or a "." or be localhost
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
	splitImageName := strings.Split(decomposedImage.name, "/")
	return splitImageName[len(splitImageName)-1], nil
}

// DecomposeString decomposes a string name into imageParts description. This
// is a wrapper for decompose
func DecomposeString(input string) (Parts, error) {
	return decompose(input)
}

// decompose breaks an input name into an imageParts description
func decompose(input string) (Parts, error) {
	var (
		parts       Parts
		hasRegistry bool
		tag         string
	)
	imgRef, err := reference.Parse(input)
	if err != nil {
		return parts, err
	}
	ntag, isTagged := imgRef.(reference.NamedTagged)
	if !isTagged {
		tag = "latest"
		if _, hasDigest := imgRef.(reference.Digested); hasDigest {
			tag = "none"
		}
	} else {
		tag = ntag.Tag()
	}
	registry := reference.Domain(imgRef.(reference.Named))
	imageName := reference.Path(imgRef.(reference.Named))
	// Is this a Registry or a repo?
	if isRegistry(registry) {
		hasRegistry = true
	} else {
		if registry != "" {
			imageName = registry + "/" + imageName
			registry = ""
		}
	}
	return Parts{
		Registry:    registry,
		hasRegistry: hasRegistry,
		name:        imageName,
		Tag:         tag,
		isTagged:    isTagged,
		transport:   DefaultTransport,
	}, nil
}

// assemble concatenates an image's parts into a string
func (ip *Parts) assemble() string {
	spec := fmt.Sprintf("%s:%s", ip.name, ip.Tag)

	if ip.Registry != "" {
		spec = fmt.Sprintf("%s/%s", ip.Registry, spec)
	}
	return spec
}

// assemble concatenates an image's parts with transport into a string
func (ip *Parts) assembleWithTransport() string {
	return fmt.Sprintf("%s%s", ip.transport, ip.assemble())
}
