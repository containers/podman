package archive

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/docker/internal/tarfile"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/types"
)

type archiveImageDestination struct {
	*tarfile.Destination // Implements most of types.ImageDestination
	ref                  archiveReference
	writer               *Writer // Should be closed if closeWriter
	closeWriter          bool
}

func newImageDestination(sys *types.SystemContext, ref archiveReference) (private.ImageDestination, error) {
	if ref.sourceIndex != -1 {
		return nil, fmt.Errorf("Destination reference must not contain a manifest index @%d", ref.sourceIndex)
	}

	var writer *Writer
	var closeWriter bool
	if ref.writer != nil {
		writer = ref.writer
		closeWriter = false
	} else {
		w, err := NewWriter(sys, ref.path)
		if err != nil {
			return nil, err
		}
		writer = w
		closeWriter = true
	}
	tarDest := tarfile.NewDestination(sys, writer.archive, ref.Transport().Name(), ref.ref)
	if sys != nil && sys.DockerArchiveAdditionalTags != nil {
		tarDest.AddRepoTags(sys.DockerArchiveAdditionalTags)
	}
	return &archiveImageDestination{
		Destination: tarDest,
		ref:         ref,
		writer:      writer,
		closeWriter: closeWriter,
	}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *archiveImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *archiveImageDestination) Close() error {
	if d.closeWriter {
		return d.writer.Close()
	}
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *archiveImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	d.writer.imageCommitted()
	if d.closeWriter {
		// We could do this only in .Close(), but failures in .Close() are much more likely to be
		// ignored by callers that use defer. So, in single-image destinations, try to complete
		// the archive here.
		// But if Commit() is never called, let .Close() clean up.
		err := d.writer.Close()
		d.closeWriter = false
		return err
	}
	return nil
}
