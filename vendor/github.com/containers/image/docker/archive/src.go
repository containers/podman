package archive

import (
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
)

type archiveImageSource struct {
	*tarfile.Source // Implements most of types.ImageSource
	ref             archiveReference
}

// newImageSource returns a types.ImageSource for the specified image reference.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ctx *types.SystemContext, ref archiveReference) types.ImageSource {
	if ref.destinationRef != nil {
		logrus.Warnf("docker-archive: references are not supported for sources (ignoring)")
	}
	src := tarfile.NewSource(ref.path)
	return &archiveImageSource{
		Source: src,
		ref:    ref,
	}
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *archiveImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *archiveImageSource) Close() error {
	return nil
}
