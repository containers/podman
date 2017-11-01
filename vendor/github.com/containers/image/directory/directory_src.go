package directory

import (
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type dirImageSource struct {
	ref dirReference
}

// newImageSource returns an ImageSource reading from an existing directory.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ref dirReference) types.ImageSource {
	return &dirImageSource{ref}
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dirImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dirImageSource) Close() error {
	return nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *dirImageSource) GetManifest() ([]byte, string, error) {
	m, err := ioutil.ReadFile(s.ref.manifestPath())
	if err != nil {
		return nil, "", err
	}
	return m, manifest.GuessMIMEType(m), err
}

func (s *dirImageSource) GetTargetManifest(digest digest.Digest) ([]byte, string, error) {
	return nil, "", errors.Errorf(`Getting target manifest not supported by "dir:"`)
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
func (s *dirImageSource) GetBlob(info types.BlobInfo) (io.ReadCloser, int64, error) {
	r, err := os.Open(s.ref.layerPath(info.Digest))
	if err != nil {
		return nil, 0, nil
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, 0, nil
	}
	return r, fi.Size(), nil
}

func (s *dirImageSource) GetSignatures(ctx context.Context) ([][]byte, error) {
	signatures := [][]byte{}
	for i := 0; ; i++ {
		signature, err := ioutil.ReadFile(s.ref.signaturePath(i))
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		signatures = append(signatures, signature)
	}
	return signatures, nil
}

// UpdatedLayerInfos() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *dirImageSource) UpdatedLayerInfos() []types.BlobInfo {
	return nil
}
