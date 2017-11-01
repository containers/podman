// Package image consolidates knowledge about various container image formats
// (as opposed to image storage mechanisms, which are handled by types.ImageSource)
// and exposes all of them using an unified interface.
package image

import (
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
)

// FromSource returns a types.Image implementation for source.
// The caller must call .Close() on the returned Image.
//
// FromSource “takes ownership” of the input ImageSource and will call src.Close()
// when the image is closed.  (This does not prevent callers from using both the
// Image and ImageSource objects simultaneously, but it means that they only need to
// the Image.)
//
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage instead of calling this function.
func FromSource(src types.ImageSource) (types.Image, error) {
	return FromUnparsedImage(UnparsedFromSource(src))
}

// sourcedImage is a general set of utilities for working with container images,
// whatever is their underlying location (i.e. dockerImageSource-independent).
// Note the existence of skopeo/docker.Image: some instances of a `types.Image`
// may not be a `sourcedImage` directly. However, most users of `types.Image`
// do not care, and those who care about `skopeo/docker.Image` know they do.
type sourcedImage struct {
	*UnparsedImage
	manifestBlob     []byte
	manifestMIMEType string
	// genericManifest contains data corresponding to manifestBlob.
	// NOTE: The manifest may have been modified in the process; DO NOT reserialize and store genericManifest
	// if you want to preserve the original manifest; use manifestBlob directly.
	genericManifest
}

// FromUnparsedImage returns a types.Image implementation for unparsed.
// The caller must call .Close() on the returned Image.
//
// FromSource “takes ownership” of the input UnparsedImage and will call uparsed.Close()
// when the image is closed.  (This does not prevent callers from using both the
// UnparsedImage and ImageSource objects simultaneously, but it means that they only need to
// keep a reference to the Image.)
func FromUnparsedImage(unparsed *UnparsedImage) (types.Image, error) {
	// Note that the input parameter above is specifically *image.UnparsedImage, not types.UnparsedImage:
	// we want to be able to use unparsed.src.  We could make that an explicit interface, but, well,
	// this is the only UnparsedImage implementation around, anyway.

	// Also, we do not explicitly implement types.Image.Close; we let the implementation fall through to
	// unparsed.Close.

	// NOTE: It is essential for signature verification that all parsing done in this object happens on the same manifest which is returned by unparsed.Manifest().
	manifestBlob, manifestMIMEType, err := unparsed.Manifest()
	if err != nil {
		return nil, err
	}

	parsedManifest, err := manifestInstanceFromBlob(unparsed.src, manifestBlob, manifestMIMEType)
	if err != nil {
		return nil, err
	}

	return &sourcedImage{
		UnparsedImage:    unparsed,
		manifestBlob:     manifestBlob,
		manifestMIMEType: manifestMIMEType,
		genericManifest:  parsedManifest,
	}, nil
}

// Size returns the size of the image as stored, if it's known, or -1 if it isn't.
func (i *sourcedImage) Size() (int64, error) {
	return -1, nil
}

// Manifest overrides the UnparsedImage.Manifest to always use the fields which we have already fetched.
func (i *sourcedImage) Manifest() ([]byte, string, error) {
	return i.manifestBlob, i.manifestMIMEType, nil
}

func (i *sourcedImage) Inspect() (*types.ImageInspectInfo, error) {
	return inspectManifest(i.genericManifest)
}

func (i *sourcedImage) IsMultiImage() bool {
	return i.manifestMIMEType == manifest.DockerV2ListMediaType
}

func (i *sourcedImage) UpdatedLayerInfos() []types.BlobInfo {
	return i.UnparsedImage.UpdatedLayerInfos()
}
