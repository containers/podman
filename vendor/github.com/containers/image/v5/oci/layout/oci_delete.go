package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/containers/image/v5/internal/set"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// DeleteImage deletes the named image from the directory, if supported.
func (ref ociReference) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	sharedBlobsDir := ""
	if sys != nil && sys.OCISharedBlobDirPath != "" {
		sharedBlobsDir = sys.OCISharedBlobDirPath
	}

	descriptor, descriptorIndex, err := ref.getManifestDescriptor()
	if err != nil {
		return err
	}

	var blobsUsedByImage map[digest.Digest]int

	switch descriptor.MediaType {
	case imgspecv1.MediaTypeImageManifest:
		blobsUsedByImage, err = ref.getBlobsUsedInSingleImage(&descriptor, sharedBlobsDir)
	case imgspecv1.MediaTypeImageIndex:
		blobsUsedByImage, err = ref.getBlobsUsedInImageIndex(&descriptor, sharedBlobsDir)
	default:
		return fmt.Errorf("unsupported mediaType in index: %q", descriptor.MediaType)
	}
	if err != nil {
		return err
	}

	blobsToDelete, err := ref.getBlobsToDelete(blobsUsedByImage, sharedBlobsDir)
	if err != nil {
		return err
	}

	err = ref.deleteBlobs(blobsToDelete)
	if err != nil {
		return err
	}

	return ref.deleteReferenceFromIndex(descriptorIndex)
}

func (ref ociReference) getBlobsUsedInSingleImage(descriptor *imgspecv1.Descriptor, sharedBlobsDir string) (map[digest.Digest]int, error) {
	manifest, err := ref.getManifest(descriptor, sharedBlobsDir)
	if err != nil {
		return nil, err
	}
	blobsUsedInManifest := ref.getBlobsUsedInManifest(manifest)
	blobsUsedInManifest[descriptor.Digest]++ // Add the current manifest to the list of blobs used by this reference

	return blobsUsedInManifest, nil
}

func (ref ociReference) getBlobsUsedInImageIndex(descriptor *imgspecv1.Descriptor, sharedBlobsDir string) (map[digest.Digest]int, error) {
	blobPath, err := ref.blobPath(descriptor.Digest, sharedBlobsDir)
	if err != nil {
		return nil, err
	}
	index, err := parseIndex(blobPath)
	if err != nil {
		return nil, err
	}

	blobsUsedInImageRefIndex := make(map[digest.Digest]int)
	err = ref.addBlobsUsedInIndex(blobsUsedInImageRefIndex, index, sharedBlobsDir)
	if err != nil {
		return nil, err
	}
	blobsUsedInImageRefIndex[descriptor.Digest]++ // Add the nested index in the list of blobs used by this reference

	return blobsUsedInImageRefIndex, nil
}

// Updates a map of digest with the usage count, so a blob that is referenced three times will have 3 in the map
func (ref ociReference) addBlobsUsedInIndex(destination map[digest.Digest]int, index *imgspecv1.Index, sharedBlobsDir string) error {
	for _, descriptor := range index.Manifests {
		destination[descriptor.Digest]++
		switch descriptor.MediaType {
		case imgspecv1.MediaTypeImageManifest:
			manifest, err := ref.getManifest(&descriptor, sharedBlobsDir)
			if err != nil {
				return err
			}
			for digest, count := range ref.getBlobsUsedInManifest(manifest) {
				destination[digest] += count
			}
		case imgspecv1.MediaTypeImageIndex:
			blobPath, err := ref.blobPath(descriptor.Digest, sharedBlobsDir)
			if err != nil {
				return err
			}
			index, err := parseIndex(blobPath)
			if err != nil {
				return err
			}
			err = ref.addBlobsUsedInIndex(destination, index, sharedBlobsDir)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported mediaType in index: %q", descriptor.MediaType)
		}
	}

	return nil
}

func (ref ociReference) getBlobsUsedInManifest(manifest *imgspecv1.Manifest) map[digest.Digest]int {
	blobsUsedInManifest := make(map[digest.Digest]int, 0)

	blobsUsedInManifest[manifest.Config.Digest]++
	for _, layer := range manifest.Layers {
		blobsUsedInManifest[layer.Digest]++
	}

	return blobsUsedInManifest
}

// This takes in a map of the digest and their usage count in the manifest to be deleted
// It will compare it to the digest usage in the root index, and return a set of the blobs that can be safely deleted
func (ref ociReference) getBlobsToDelete(blobsUsedByDescriptorToDelete map[digest.Digest]int, sharedBlobsDir string) (*set.Set[digest.Digest], error) {
	rootIndex, err := ref.getIndex()
	if err != nil {
		return nil, err
	}
	blobsUsedInRootIndex := make(map[digest.Digest]int)
	err = ref.addBlobsUsedInIndex(blobsUsedInRootIndex, rootIndex, sharedBlobsDir)
	if err != nil {
		return nil, err
	}

	blobsToDelete := set.New[digest.Digest]()

	for digest, count := range blobsUsedInRootIndex {
		if count-blobsUsedByDescriptorToDelete[digest] == 0 {
			blobsToDelete.Add(digest)
		}
	}

	return blobsToDelete, nil
}

// This transport never generates layouts where blobs for an image are both in the local blobs directory
// and the shared one; it’s either one or the other, depending on how OCISharedBlobDirPath is set.
//
// But we can’t correctly compute use counts for OCISharedBlobDirPath (because we don't know what
// the other layouts sharing that directory are, and we might not even have permission to read them),
// so we can’t really delete any blobs in that case.
// Checking the _local_ blobs directory, and deleting blobs from there, doesn't really hurt,
// in case the layout was created using some other tool or without OCISharedBlobDirPath set, so let's silently
// check for local blobs (but we should make no noise if the blobs are actually in the shared directory).
//
// So, NOTE: the blobPath() call below hard-codes "" even in calls where OCISharedBlobDirPath is set
func (ref ociReference) deleteBlobs(blobsToDelete *set.Set[digest.Digest]) error {
	for _, digest := range blobsToDelete.Values() {
		blobPath, err := ref.blobPath(digest, "") //Only delete in the local directory, see comment above
		if err != nil {
			return err
		}
		err = deleteBlob(blobPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteBlob(blobPath string) error {
	logrus.Debug(fmt.Sprintf("Deleting blob at %q", blobPath))

	err := os.Remove(blobPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else {
		return nil
	}
}

func (ref ociReference) deleteReferenceFromIndex(referenceIndex int) error {
	index, err := ref.getIndex()
	if err != nil {
		return err
	}

	index.Manifests = slices.Delete(index.Manifests, referenceIndex, referenceIndex+1)

	return saveJSON(ref.indexPath(), index)
}

func saveJSON(path string, content any) error {
	// If the file already exists, get its mode to preserve it
	var mode fs.FileMode
	existingfi, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		} else { // File does not exist, use default mode
			mode = 0644
		}
	} else {
		mode = existingfi.Mode()
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(content)
}

func (ref ociReference) getManifest(descriptor *imgspecv1.Descriptor, sharedBlobsDir string) (*imgspecv1.Manifest, error) {
	manifestPath, err := ref.blobPath(descriptor.Digest, sharedBlobsDir)
	if err != nil {
		return nil, err
	}

	manifest, err := parseJSON[imgspecv1.Manifest](manifestPath)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}
