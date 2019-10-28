package tarball

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/containers/image/v5/types"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	imgspecs "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type tarballImageSource struct {
	reference  tarballReference
	filenames  []string
	diffIDs    []digest.Digest
	diffSizes  []int64
	blobIDs    []digest.Digest
	blobSizes  []int64
	blobTypes  []string
	config     []byte
	configID   digest.Digest
	configSize int64
	manifest   []byte
}

func (r *tarballReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	// Gather up the digests, sizes, and date information for all of the files.
	filenames := []string{}
	diffIDs := []digest.Digest{}
	diffSizes := []int64{}
	blobIDs := []digest.Digest{}
	blobSizes := []int64{}
	blobTimes := []time.Time{}
	blobTypes := []string{}
	for _, filename := range r.filenames {
		var file *os.File
		var err error
		var blobSize int64
		var blobTime time.Time
		var reader io.Reader
		if filename == "-" {
			blobSize = int64(len(r.stdin))
			blobTime = time.Now()
			reader = bytes.NewReader(r.stdin)
		} else {
			file, err = os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("error opening %q for reading: %v", filename, err)
			}
			defer file.Close()
			reader = file
			fileinfo, err := file.Stat()
			if err != nil {
				return nil, fmt.Errorf("error reading size of %q: %v", filename, err)
			}
			blobSize = fileinfo.Size()
			blobTime = fileinfo.ModTime()
		}

		// Default to assuming the layer is compressed.
		layerType := imgspecv1.MediaTypeImageLayerGzip

		// Set up to digest the file as it is.
		blobIDdigester := digest.Canonical.Digester()
		reader = io.TeeReader(reader, blobIDdigester.Hash())

		// Set up to digest the file after we maybe decompress it.
		diffIDdigester := digest.Canonical.Digester()
		uncompressed, err := pgzip.NewReader(reader)
		if err == nil {
			// It is compressed, so the diffID is the digest of the uncompressed version
			reader = io.TeeReader(uncompressed, diffIDdigester.Hash())
		} else {
			// It is not compressed, so the diffID and the blobID are going to be the same
			diffIDdigester = blobIDdigester
			layerType = imgspecv1.MediaTypeImageLayer
			uncompressed = nil
		}
		// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
		n, err := io.Copy(ioutil.Discard, reader)
		if err != nil {
			return nil, fmt.Errorf("error reading %q: %v", filename, err)
		}
		if uncompressed != nil {
			uncompressed.Close()
		}

		// Grab our uncompressed and possibly-compressed digests and sizes.
		filenames = append(filenames, filename)
		diffIDs = append(diffIDs, diffIDdigester.Digest())
		diffSizes = append(diffSizes, n)
		blobIDs = append(blobIDs, blobIDdigester.Digest())
		blobSizes = append(blobSizes, blobSize)
		blobTimes = append(blobTimes, blobTime)
		blobTypes = append(blobTypes, layerType)
	}

	// Build the rootfs and history for the configuration blob.
	rootfs := imgspecv1.RootFS{
		Type:    "layers",
		DiffIDs: diffIDs,
	}
	created := time.Time{}
	history := []imgspecv1.History{}
	// Pick up the layer comment from the configuration's history list, if one is set.
	comment := "imported from tarball"
	if len(r.config.History) > 0 && r.config.History[0].Comment != "" {
		comment = r.config.History[0].Comment
	}
	for i := range diffIDs {
		createdBy := fmt.Sprintf("/bin/sh -c #(nop) ADD file:%s in %c", diffIDs[i].Hex(), os.PathSeparator)
		history = append(history, imgspecv1.History{
			Created:   &blobTimes[i],
			CreatedBy: createdBy,
			Comment:   comment,
		})
		// Use the mtime of the most recently modified file as the image's creation time.
		if created.Before(blobTimes[i]) {
			created = blobTimes[i]
		}
	}

	// Pick up other defaults from the config in the reference.
	config := r.config
	if config.Created == nil {
		config.Created = &created
	}
	if config.Architecture == "" {
		config.Architecture = runtime.GOARCH
	}
	if config.OS == "" {
		config.OS = runtime.GOOS
	}
	config.RootFS = rootfs
	config.History = history

	// Encode and digest the image configuration blob.
	configBytes, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("error generating configuration blob for %q: %v", strings.Join(r.filenames, separator), err)
	}
	configID := digest.Canonical.FromBytes(configBytes)
	configSize := int64(len(configBytes))

	// Populate a manifest with the configuration blob and the file as the single layer.
	layerDescriptors := []imgspecv1.Descriptor{}
	for i := range blobIDs {
		layerDescriptors = append(layerDescriptors, imgspecv1.Descriptor{
			Digest:    blobIDs[i],
			Size:      blobSizes[i],
			MediaType: blobTypes[i],
		})
	}
	annotations := make(map[string]string)
	for k, v := range r.annotations {
		annotations[k] = v
	}
	manifest := imgspecv1.Manifest{
		Versioned: imgspecs.Versioned{
			SchemaVersion: 2,
		},
		Config: imgspecv1.Descriptor{
			Digest:    configID,
			Size:      configSize,
			MediaType: imgspecv1.MediaTypeImageConfig,
		},
		Layers:      layerDescriptors,
		Annotations: annotations,
	}

	// Encode the manifest.
	manifestBytes, err := json.Marshal(&manifest)
	if err != nil {
		return nil, fmt.Errorf("error generating manifest for %q: %v", strings.Join(r.filenames, separator), err)
	}

	// Return the image.
	src := &tarballImageSource{
		reference:  *r,
		filenames:  filenames,
		diffIDs:    diffIDs,
		diffSizes:  diffSizes,
		blobIDs:    blobIDs,
		blobSizes:  blobSizes,
		blobTypes:  blobTypes,
		config:     configBytes,
		configID:   configID,
		configSize: configSize,
		manifest:   manifestBytes,
	}

	return src, nil
}

func (is *tarballImageSource) Close() error {
	return nil
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (is *tarballImageSource) HasThreadSafeGetBlob() bool {
	return false
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (is *tarballImageSource) GetBlob(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	// We should only be asked about things in the manifest.  Maybe the configuration blob.
	if blobinfo.Digest == is.configID {
		return ioutil.NopCloser(bytes.NewBuffer(is.config)), is.configSize, nil
	}
	// Maybe one of the layer blobs.
	for i := range is.blobIDs {
		if blobinfo.Digest == is.blobIDs[i] {
			// We want to read that layer: open the file or memory block and hand it back.
			if is.filenames[i] == "-" {
				return ioutil.NopCloser(bytes.NewBuffer(is.reference.stdin)), int64(len(is.reference.stdin)), nil
			}
			reader, err := os.Open(is.filenames[i])
			if err != nil {
				return nil, -1, fmt.Errorf("error opening %q: %v", is.filenames[i], err)
			}
			return reader, is.blobSizes[i], nil
		}
	}
	return nil, -1, fmt.Errorf("no blob with digest %q found", blobinfo.Digest.String())
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (is *tarballImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return nil, "", fmt.Errorf("manifest lists are not supported by the %q transport", transportName)
	}
	return is.manifest, imgspecv1.MediaTypeImageManifest, nil
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as there can be no secondary manifests.
func (*tarballImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, fmt.Errorf("manifest lists are not supported by the %q transport", transportName)
	}
	return nil, nil
}

func (is *tarballImageSource) Reference() types.ImageReference {
	return &is.reference
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve BlobInfos for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (*tarballImageSource) LayerInfosForCopy(context.Context, *digest.Digest) ([]types.BlobInfo, error) {
	return nil, nil
}
