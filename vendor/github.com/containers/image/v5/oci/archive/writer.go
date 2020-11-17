package archive

import (
	"io/ioutil"
	"os"

	"github.com/containers/image/v5/internal/tmpdir"

	"github.com/containers/image/v5/directory/explicitfilepath"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
)

// Writer keeps the TempDir for creating oci archive
type Writer struct {
	// TempDir will be tarred to oci archive
	TempDir string
}

// NewWriter creates a temp directory will be tarred to oci-archive.
// The caller should call .Close() on the returned object.
func NewWriter(sys *types.SystemContext) (*Writer, error) {
	dir, err := ioutil.TempDir(tmpdir.TemporaryDirectoryForBigFiles(sys), "oci")
	if err != nil {
		return nil, errors.Wrapf(err, "error creating temp directory")
	}
	ociWriter := &Writer{
		TempDir: dir,
	}
	return ociWriter, nil
}

// Close deletes temporary files associated with the Writer
func (w *Writer) Close() error {
	return os.RemoveAll(w.TempDir)
}

// TarDirectory converts the directory from Writer and saves it to file
func (w *Writer) TarDirectory(file string) error {
	dst, err := explicitfilepath.ResolvePathToFullyExplicit(file)
	if err != nil {
		return err
	}
	return tarDirectory(w.TempDir, dst)
}
