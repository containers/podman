package archive

import (
	"context"
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
func newImageSource(ctx context.Context, ref archiveReference) (types.ImageSource, error) {
	if ref.destinationRef != nil {
		logrus.Warnf("docker-archive: references are not supported for sources (ignoring)")
	}
	src, err := tarfile.NewSourceFromFile(ref.path)
	if err != nil {
		return nil, err
	}
	return &archiveImageSource{
		Source: src,
		ref:    ref,
	}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *archiveImageSource) Reference() types.ImageReference {
	return s.ref
}

// LayerInfosForCopy() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *archiveImageSource) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
	return nil, nil
}
