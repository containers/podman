package archive

import (
	"io"
	"os"

	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

type archiveImageDestination struct {
	*tarfile.Destination // Implements most of types.ImageDestination
	ref                  archiveReference
	writer               io.Closer
}

func newImageDestination(ctx *types.SystemContext, ref archiveReference) (types.ImageDestination, error) {
	if ref.destinationRef == nil {
		return nil, errors.Errorf("docker-archive: destination reference not supplied (must be of form <path>:<reference:tag>)")
	}

	// ref.path can be either a pipe or a regular file
	// in the case of a pipe, we require that we can open it for write
	// in the case of a regular file, we don't want to overwrite any pre-existing file
	// so we check for Size() == 0 below (This is racy, but using O_EXCL would also be racy,
	// only in a different way. Either way, it’s up to the user to not have two writers to the same path.)
	fh, err := os.OpenFile(ref.path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening file %q", ref.path)
	}

	fhStat, err := fh.Stat()
	if err != nil {
		return nil, errors.Wrapf(err, "error statting file %q", ref.path)
	}

	if fhStat.Mode().IsRegular() && fhStat.Size() != 0 {
		return nil, errors.New("docker-archive doesn't support modifying existing images")
	}

	return &archiveImageDestination{
		Destination: tarfile.NewDestination(fh, ref.destinationRef),
		ref:         ref,
		writer:      fh,
	}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *archiveImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *archiveImageDestination) Close() error {
	return d.writer.Close()
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *archiveImageDestination) Commit() error {
	return d.Destination.Commit()
}
