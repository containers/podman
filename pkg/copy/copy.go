package copy

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/storage/pkg/archive"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/pkg/errors"
)

// ********************************* NOTE *************************************
//
// Most security bugs are caused by attackers playing around with symlinks
// trying to escape from the container onto the host and/or trick into data
// corruption on the host.  Hence, file operations on containers (including
// *stat) should always be handled by `github.com/containers/buildah/copier`
// which makes sure to evaluate files in a chroot'ed environment.
//
// Please make sure to add verbose comments when changing code to make the
// lives of future readers easier.
//
// ****************************************************************************

// Copier copies data from a source to a destination CopyItem.
type Copier struct {
	copyFunc     func() error
	cleanUpFuncs []deferFunc
}

// cleanUp releases resources the Copier may hold open.
func (c *Copier) cleanUp() {
	for _, f := range c.cleanUpFuncs {
		f()
	}
}

// Copy data from a source to a destination CopyItem.
func (c *Copier) Copy() error {
	defer c.cleanUp()
	return c.copyFunc()
}

// GetCopiers returns a Copier to copy the source item to destination.  Use
// extract to untar the source if it's a tar archive.
func GetCopier(source *CopyItem, destination *CopyItem, extract bool) (*Copier, error) {
	copier := &Copier{}

	// First, do the man-page dance.  See podman-cp(1) for details.
	if err := enforceCopyRules(source, destination); err != nil {
		return nil, err
	}

	// Destination is a stream (e.g., stdout or an http body).
	if destination.info.IsStream {
		// Source is a stream (e.g., stdin or an http body).
		if source.info.IsStream {
			copier.copyFunc = func() error {
				_, err := io.Copy(destination.writer, source.reader)
				return err
			}
			return copier, nil
		}
		root, glob, err := source.buildahGlobs()
		if err != nil {
			return nil, err
		}
		copier.copyFunc = func() error {
			return buildahCopiah.Get(root, "", source.getOptions(), []string{glob}, destination.writer)
		}
		return copier, nil
	}

	// Destination is either a file or a directory.
	if source.info.IsStream {
		copier.copyFunc = func() error {
			return buildahCopiah.Put(destination.root, destination.resolved, source.putOptions(), source.reader)
		}
		return copier, nil
	}

	tarOptions := &archive.TarOptions{
		Compression: archive.Uncompressed,
		CopyPass:    true,
	}

	root := destination.root
	dir := destination.resolved
	if !source.info.IsDir {
		// When copying a file, make sure to rename the
		// destination base path.
		nameMap := make(map[string]string)
		nameMap[filepath.Base(source.resolved)] = filepath.Base(destination.resolved)
		tarOptions.RebaseNames = nameMap
		dir = filepath.Dir(dir)
	}

	var tarReader io.ReadCloser
	if extract && archive.IsArchivePath(source.resolved) {
		if !destination.info.IsDir {
			return nil, errors.Errorf("cannot extract archive %q to file %q", source.original, destination.original)
		}

		reader, err := os.Open(source.resolved)
		if err != nil {
			return nil, err
		}
		copier.cleanUpFuncs = append(copier.cleanUpFuncs, func() { reader.Close() })

		// The stream from stdin may be compressed (e.g., via gzip).
		decompressedStream, err := archive.DecompressStream(reader)
		if err != nil {
			return nil, err
		}

		copier.cleanUpFuncs = append(copier.cleanUpFuncs, func() { decompressedStream.Close() })
		tarReader = decompressedStream
	} else {
		reader, err := archive.TarWithOptions(source.resolved, tarOptions)
		if err != nil {
			return nil, err
		}
		copier.cleanUpFuncs = append(copier.cleanUpFuncs, func() { reader.Close() })
		tarReader = reader
	}

	copier.copyFunc = func() error {
		return buildahCopiah.Put(root, dir, source.putOptions(), tarReader)
	}
	return copier, nil
}

// enforceCopyRules enforces the rules for copying from a source to a
// destination as mentioned in the podman-cp(1) man page.  Please refer to the
// man page and/or the inline comments for further details.  Note that source
// and destination are passed by reference and the their data may be changed.
func enforceCopyRules(source, destination *CopyItem) error {
	if source.statError != nil {
		return source.statError
	}

	// We can copy everything to a stream.
	if destination.info.IsStream {
		return nil
	}

	if source.info.IsStream {
		if !(destination.info.IsDir || destination.info.IsStream) {
			return errors.New("destination must be a directory or stream when copying from a stream")
		}
		return nil
	}

	// Source is a *directory*.
	if source.info.IsDir {
		if destination.statError != nil {
			// It's okay if the destination does not exist.  We
			// made sure before that it's parent exists, so it
			// would be created while copying.
			if os.IsNotExist(destination.statError) {
				return nil
			}
			// Could be a permission error.
			return destination.statError
		}

		// If the destination exists and is not a directory, we have a
		// problem.
		if !destination.info.IsDir {
			return errors.Errorf("cannot copy directory %q to file %q", source.original, destination.original)
		}

		// If the destination exists and is a directory, we need to
		// append the source base directory to it.  This makes sure
		// that copying "/foo/bar" "/tmp" will copy to "/tmp/bar" (and
		// not "/tmp").
		newDestination, err := securejoin.SecureJoin(destination.resolved, filepath.Base(source.resolved))
		if err != nil {
			return err
		}
		destination.resolved = newDestination
		return nil
	}

	// Source is a *file*.
	if destination.statError != nil {
		// It's okay if the destination does not exist, unless it ends
		// with "/".
		if !os.IsNotExist(destination.statError) {
			return destination.statError
		} else if strings.HasSuffix(destination.resolved, "/") {
			// Note: this is practically unreachable code as the
			// existence of parent directories is enforced early
			// on.  It's left here as an extra security net.
			return errors.Errorf("destination directory %q must exist (trailing %q)", destination.original, "/")
		}
		// Does not exist and does not end with "/".
		return nil
	}

	// If the destination is a file, we're good.  We will overwrite the
	// contents while copying.
	if !destination.info.IsDir {
		return nil
	}

	// If the destination exists and is a directory, we need to append the
	// source base directory to it.  This makes sure that copying
	// "/foo/bar" "/tmp" will copy to "/tmp/bar" (and not "/tmp").
	newDestination, err := securejoin.SecureJoin(destination.resolved, filepath.Base(source.resolved))
	if err != nil {
		return err
	}

	destination.resolved = newDestination
	return nil
}
