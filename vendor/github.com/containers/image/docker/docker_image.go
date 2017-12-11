package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

// Image is a Docker-specific implementation of types.ImageCloser with a few extra methods
// which are specific to Docker.
type Image struct {
	types.ImageCloser
	src *dockerImageSource
}

// newImage returns a new Image interface type after setting up
// a client to the registry hosting the given image.
// The caller must call .Close() on the returned Image.
func newImage(ctx *types.SystemContext, ref dockerReference) (types.ImageCloser, error) {
	s, err := newImageSource(ctx, ref)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(ctx, s)
	if err != nil {
		return nil, err
	}
	return &Image{ImageCloser: img, src: s}, nil
}

// SourceRefFullName returns a fully expanded name for the repository this image is in.
func (i *Image) SourceRefFullName() string {
	return i.src.ref.ref.Name()
}

// GetRepositoryTags list all tags available in the repository. Note that this has no connection with the tag(s) used for this specific image, if any.
func (i *Image) GetRepositoryTags() ([]string, error) {
	path := fmt.Sprintf(tagsPath, reference.Path(i.src.ref.ref))
	// FIXME: Pass the context.Context
	res, err := i.src.c.makeRequest(context.TODO(), "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, errors.Errorf("Invalid status code returned when fetching tags list %d", res.StatusCode)
	}
	type tagsRes struct {
		Tags []string
	}
	tags := &tagsRes{}
	if err := json.NewDecoder(res.Body).Decode(tags); err != nil {
		return nil, err
	}
	return tags.Tags, nil
}
