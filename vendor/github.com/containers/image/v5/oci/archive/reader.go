package archive

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/archive"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Reader keeps the temp directory the oci archive will be untarred to and the manifest of the images
type Reader struct {
	manifest      *imgspecv1.Index
	tempDirectory string
	path          string // The original, user-specified path; not the maintained temporary file, if any
}

// NewReader creates the temp directory that keeps the untarred archive from src.
// The caller should call .Close() on the returned object.
func NewReader(ctx context.Context, sys *types.SystemContext, src string) (*Reader, error) {
	// TODO: This can take quite some time, and should ideally be cancellable using a context.Context.
	arch, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer arch.Close()

	dst, err := ioutil.TempDir(tmpdir.TemporaryDirectoryForBigFiles(sys), "oci")
	if err != nil {
		return nil, errors.Wrap(err, "error creating temp directory")
	}

	reader := Reader{
		tempDirectory: dst,
		path:          src,
	}

	succeeded := false
	defer func() {
		if !succeeded {
			reader.Close()
		}
	}()
	if err := archive.NewDefaultArchiver().Untar(arch, dst, &archive.TarOptions{NoLchown: true}); err != nil {
		return nil, errors.Wrapf(err, "error untarring file %q", dst)
	}

	indexJSON, err := os.Open(filepath.Join(dst, "index.json"))
	if err != nil {
		return nil, err
	}
	defer indexJSON.Close()
	reader.manifest = &imgspecv1.Index{}
	if err := json.NewDecoder(indexJSON).Decode(reader.manifest); err != nil {
		return nil, err
	}
	succeeded = true
	return &reader, nil
}

// Reference wraps the image reference and the manifest for loading
type Reference struct {
	FullImageReference types.ImageReference
	ManifestDescriptor imgspecv1.Descriptor
}

// List returns a list of Reference for images in the reader
// the ImageReferences are valid only until the Reader is closed.
func (r *Reader) List() ([]Reference, error) {
	var (
		res []Reference
		ref types.ImageReference
		err error
	)
	for i, md := range r.manifest.Manifests {
		if md.MediaType != imgspecv1.MediaTypeImageManifest && md.MediaType != imgspecv1.MediaTypeImageIndex {
			continue
		}
		refName, ok := md.Annotations[imgspecv1.AnnotationRefName]
		if !ok {
			if ref, err = layout.NewIndexReference(r.tempDirectory, i); err != nil {
				return nil, err
			}
		} else {
			if ref, err = layout.NewReference(r.tempDirectory, refName); err != nil {
				return nil, err
			}
		}
		archiveReaderRef := &tempDirOCIRef{
			tempDirectory:   r.tempDirectory,
			ociRefExtracted: ref,
		}
		archiveRef, err := newReference(r.path, "", -1, archiveReaderRef, nil)
		if err != nil {
			return nil, errors.Errorf("error creating image reference: %v", err)
		}
		reference := Reference{
			FullImageReference: archiveRef,
			ManifestDescriptor: md,
		}
		res = append(res, reference)
	}
	return res, nil
}

// Close deletes temporary files associated with the Reader, if any.
func (r *Reader) Close() error {
	return os.RemoveAll(r.tempDirectory)
}
