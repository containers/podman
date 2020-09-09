package archive

import (
	"io"
	"os"

	"github.com/containers/image/v5/docker/internal/tarfile"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
)

// Writer manages a single in-progress Docker archive and allows adding images to it.
type Writer struct {
	path    string // The original, user-specified path; not the maintained temporary file, if any
	archive *tarfile.Writer
	writer  io.Closer
}

// NewWriter returns a Writer for path.
// The caller should call .Close() on the returned object.
func NewWriter(sys *types.SystemContext, path string) (*Writer, error) {
	fh, err := openArchiveForWriting(path)
	if err != nil {
		return nil, err
	}
	archive := tarfile.NewWriter(fh)

	return &Writer{
		path:    path,
		archive: archive,
		writer:  fh,
	}, nil
}

// Close writes all outstanding data about images to the archive, and
// releases state associated with the Writer, if any.
// No more images can be added after this is called.
func (w *Writer) Close() error {
	err := w.archive.Close()
	if err2 := w.writer.Close(); err2 != nil && err == nil {
		err = err2
	}
	return err
}

// NewReference returns an ImageReference that allows adding an image to Writer,
// with an optional reference.
func (w *Writer) NewReference(destinationRef reference.NamedTagged) (types.ImageReference, error) {
	return newReference(w.path, destinationRef, -1, nil, w.archive)
}

// openArchiveForWriting opens path for writing a tar archive,
// making a few sanity checks.
func openArchiveForWriting(path string) (*os.File, error) {
	// path can be either a pipe or a regular file
	// in the case of a pipe, we require that we can open it for write
	// in the case of a regular file, we don't want to overwrite any pre-existing file
	// so we check for Size() == 0 below (This is racy, but using O_EXCL would also be racy,
	// only in a different way. Either way, it’s up to the user to not have two writers to the same path.)
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening file %q", path)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			fh.Close()
		}
	}()
	fhStat, err := fh.Stat()
	if err != nil {
		return nil, errors.Wrapf(err, "error statting file %q", path)
	}

	if fhStat.Mode().IsRegular() && fhStat.Size() != 0 {
		return nil, errors.New("docker-archive doesn't support modifying existing images")
	}

	succeeded = true
	return fh, nil
}
