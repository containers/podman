package daemon

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/internal/tmpdir"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type daemonImageSource struct {
	ref             daemonReference
	*tarfile.Source // Implements most of types.ImageSource
	tarCopyPath     string
}

type layerInfo struct {
	path string
	size int64
}

// newImageSource returns a types.ImageSource for the specified image reference.
// The caller must call .Close() on the returned ImageSource.
//
// It would be great if we were able to stream the input tar as it is being
// sent; but Docker sends the top-level manifest, which determines which paths
// to look for, at the end, so in we will need to seek back and re-read, several times.
// (We could, perhaps, expect an exact sequence, assume that the first plaintext file
// is the config, and that the following len(RootFS) files are the layers, but that feels
// way too brittle.)
func newImageSource(ctx *types.SystemContext, ref daemonReference) (types.ImageSource, error) {
	c, err := newDockerClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Error initializing docker engine client")
	}
	// Per NewReference(), ref.StringWithinTransport() is either an image ID (config digest), or a !reference.NameOnly() reference.
	// Either way ImageSave should create a tarball with exactly one image.
	inputStream, err := c.ImageSave(context.TODO(), []string{ref.StringWithinTransport()})
	if err != nil {
		return nil, errors.Wrap(err, "Error loading image from docker engine")
	}
	defer inputStream.Close()

	// FIXME: use SystemContext here.
	tarCopyFile, err := ioutil.TempFile(tmpdir.TemporaryDirectoryForBigFiles(), "docker-daemon-tar")
	if err != nil {
		return nil, err
	}
	defer tarCopyFile.Close()

	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(tarCopyFile.Name())
		}
	}()

	if _, err := io.Copy(tarCopyFile, inputStream); err != nil {
		return nil, err
	}

	succeeded = true
	return &daemonImageSource{
		ref:         ref,
		Source:      tarfile.NewSource(tarCopyFile.Name()),
		tarCopyPath: tarCopyFile.Name(),
	}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *daemonImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *daemonImageSource) Close() error {
	return os.Remove(s.tarCopyPath)
}

// LayerInfosForCopy() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *daemonImageSource) LayerInfosForCopy() []types.BlobInfo {
	return nil
}
