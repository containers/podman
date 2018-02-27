package image

import (
	"fmt"

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
	ntag, isTagged, err := getTags(input)
	if err != nil {
		return parts, err
	}
	if !isTagged {
		tag = "latest"
	} else {
		tag = ntag.Tag()
	}
	registry := reference.Domain(imgRef.(reference.Named))
	if registry != "" {
		hasRegistry = true
	}
	imageName := reference.Path(imgRef.(reference.Named))
	return imageParts{
		registry:    registry,
		hasRegistry: hasRegistry,
		name:        imageName,
		tag:         tag,
		isTagged:    isTagged,
	}, nil
}

// assemble concatenates an image's parts into a string
func (ip *imageParts) assemble() string {
	return fmt.Sprintf("%s/%s:%s", ip.registry, ip.name, ip.tag)
}
