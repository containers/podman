package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
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
func newImage(ctx context.Context, sys *types.SystemContext, ref dockerReference) (types.ImageCloser, error) {
	s, err := newImageSource(ctx, sys, ref)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(ctx, sys, s)
	if err != nil {
		return nil, err
	}
	return &Image{ImageCloser: img, src: s}, nil
}

// SourceRefFullName returns a fully expanded name for the repository this image is in.
func (i *Image) SourceRefFullName() string {
	return i.src.logicalRef.ref.Name()
}

// GetRepositoryTags list all tags available in the repository. The tag
// provided inside the ImageReference will be ignored. (This is a
// backward-compatible shim method which calls the module-level
// GetRepositoryTags)
func (i *Image) GetRepositoryTags(ctx context.Context) ([]string, error) {
	return GetRepositoryTags(ctx, i.src.c.sys, i.src.logicalRef)
}

// GetRepositoryTags list all tags available in the repository. The tag
// provided inside the ImageReference will be ignored.
func GetRepositoryTags(ctx context.Context, sys *types.SystemContext, ref types.ImageReference) ([]string, error) {
	dr, ok := ref.(dockerReference)
	if !ok {
		return nil, errors.Errorf("ref must be a dockerReference")
	}

	path := fmt.Sprintf(tagsPath, reference.Path(dr.ref))
	client, err := newDockerClientFromRef(sys, dr, false, "pull")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}

	tags := make([]string, 0)

	for {
		res, err := client.makeRequest(ctx, http.MethodGet, path, nil, nil, v2Auth, nil)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if err := httpResponseToError(res, "fetching tags list"); err != nil {
			return nil, err
		}

		var tagsHolder struct {
			Tags []string
		}
		if err = json.NewDecoder(res.Body).Decode(&tagsHolder); err != nil {
			return nil, err
		}
		tags = append(tags, tagsHolder.Tags...)

		link := res.Header.Get("Link")
		if link == "" {
			break
		}

		linkURLStr := strings.Trim(strings.Split(link, ";")[0], "<>")
		linkURL, err := url.Parse(linkURLStr)
		if err != nil {
			return tags, err
		}

		// can be relative or absolute, but we only want the path (and I
		// guess we're in trouble if it forwards to a new place...)
		path = linkURL.Path
		if linkURL.RawQuery != "" {
			path += "?"
			path += linkURL.RawQuery
		}
	}
	return tags, nil
}

// GetDigest returns the image's digest
// Use this to optimize and avoid use of an ImageSource based on the returned digest;
// if you are going to use an ImageSource anyway, it’s more efficient to create it first
// and compute the digest from the value returned by GetManifest.
// NOTE: Implemented to avoid Docker Hub API limits, and mirror configuration may be
// ignored (but may be implemented in the future)
func GetDigest(ctx context.Context, sys *types.SystemContext, ref types.ImageReference) (digest.Digest, error) {
	dr, ok := ref.(dockerReference)
	if !ok {
		return "", errors.Errorf("ref must be a dockerReference")
	}

	tagOrDigest, err := dr.tagOrDigest()
	if err != nil {
		return "", err
	}

	client, err := newDockerClientFromRef(sys, dr, false, "pull")
	if err != nil {
		return "", errors.Wrap(err, "failed to create client")
	}

	path := fmt.Sprintf(manifestPath, reference.Path(dr.ref), tagOrDigest)
	headers := map[string][]string{
		"Accept": manifest.DefaultRequestedManifestMIMETypes,
	}

	res, err := client.makeRequest(ctx, http.MethodHead, path, headers, nil, v2Auth, nil)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errors.Wrapf(registryHTTPResponseToError(res), "reading digest %s in %s", tagOrDigest, dr.ref.Name())
	}

	dig, err := digest.Parse(res.Header.Get("Docker-Content-Digest"))
	if err != nil {
		return "", err
	}

	return dig, nil
}
