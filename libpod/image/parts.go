package image

import (
	"fmt"
	"strings"

	"github.com/containers/image/docker/reference"
)

// imageParts describes the parts of an image's name
type imageParts struct {
	transport   string
	registry    string
	name        string
	tag         string
	isTagged    bool
	hasRegistry bool
}

// Registries must contain a ":" or a "." or be localhost
func isRegistry(name string) bool {
	return strings.ContainsAny(name, ".:") || name == "localhost"
}

// decompose breaks an input name into an imageParts description
func decompose(input string) (imageParts, error) {
	var (
		parts       imageParts
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
	// Is this a registry or a repo?
	if isRegistry(registry) {
		hasRegistry = true
	} else {
		if registry != "" {
			imageName = registry + "/" + imageName
			registry = ""
		}
	}
	return imageParts{
		registry:    registry,
		hasRegistry: hasRegistry,
		name:        imageName,
		tag:         tag,
		isTagged:    isTagged,
		transport:   DefaultTransport,
	}, nil
}

// assemble concatenates an image's parts into a string
func (ip *imageParts) assemble() string {
	spec := fmt.Sprintf("%s:%s", ip.name, ip.tag)

	if ip.registry != "" {
		spec = fmt.Sprintf("%s/%s", ip.registry, spec)
	}
	return spec
}

// assemble concatenates an image's parts with transport into a string
func (ip *imageParts) assembleWithTransport() string {
	return fmt.Sprintf("%s%s", ip.transport, ip.assemble())
}
