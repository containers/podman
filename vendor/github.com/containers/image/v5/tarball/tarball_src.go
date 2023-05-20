package tarball

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/containers/image/v5/internal/imagesource/impl"
	"github.com/containers/image/v5/internal/imagesource/stubs"
	"github.com/containers/image/v5/types"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	imgspecs "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/maps"
)

type tarballImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	impl.NoSignatures
	impl.DoesNotAffectLayerInfosForCopy
	stubs.NoGetBlobAtInitialize

	reference tarballReference
	blobs     map[digest.Digest]tarballBlob
	manifest  []byte
}

// tarballBlob is a blob that tarballImagSource can return by GetBlob.
type tarballBlob struct {
	contents []byte // or nil to read from filename below
	filename string // valid if contents == nil
	size     int64
}

func (r *tarballReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	// Pick up the layer comment from the configuration's history list, if one is set.
	comment := "imported from tarball"
	if len(r.config.History) > 0 && r.config.History[0].Comment != "" {
		comment = r.config.History[0].Comment
	}

	// Gather up the digests, sizes, and history information for all of the files.
	blobs := map[digest.Digest]tarballBlob{}
	diffIDs := []digest.Digest{}
	created := time.Time{}
	history := []imgspecv1.History{}
	layerDescriptors := []imgspecv1.Descriptor{}
	for _, filename := range r.filenames {
		var reader io.Reader
		var blobTime time.Time
		var blob tarballBlob
		if filename == "-" {
			reader = bytes.NewReader(r.stdin)
			blobTime = time.Now()
			blob = tarballBlob{
				contents: r.stdin,
				size:     int64(len(r.stdin)),
			}
		} else {
			file, err := os.Open(filename)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			reader = file
			fileinfo, err := file.Stat()
			if err != nil {
				return nil, fmt.Errorf("error reading size of %q: %w", filename, err)
			}
			blobTime = fileinfo.ModTime()
			blob = tarballBlob{
				filename: filename,
				size:     fileinfo.Size(),
			}
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
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return nil, fmt.Errorf("error reading %q: %v", filename, err)
		}
		if uncompressed != nil {
			uncompressed.Close()
		}

		// Grab our uncompressed and possibly-compressed digests and sizes.
		diffID := diffIDdigester.Digest()
		blobID := blobIDdigester.Digest()
		diffIDs = append(diffIDs, diffID)
		blobs[blobID] = blob

		history = append(history, imgspecv1.History{
			Created:   &blobTime,
			CreatedBy: fmt.Sprintf("/bin/sh -c #(nop) ADD file:%s in %c", diffID.Hex(), os.PathSeparator),
			Comment:   comment,
		})
		// Use the mtime of the most recently modified file as the image's creation time.
		if created.Before(blobTime) {
			created = blobTime
		}

		layerDescriptors = append(layerDescriptors, imgspecv1.Descriptor{
			Digest:    blobID,
			Size:      blob.size,
			MediaType: layerType,
		})
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
	config.RootFS = imgspecv1.RootFS{
		Type:    "layers",
		DiffIDs: diffIDs,
	}
	config.History = history

	// Encode and digest the image configuration blob.
	configBytes, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("error generating configuration blob for %q: %v", strings.Join(r.filenames, separator), err)
	}
	configID := digest.Canonical.FromBytes(configBytes)
	blobs[configID] = tarballBlob{
		contents: configBytes,
		size:     int64(len(configBytes)),
	}

	// Populate a manifest with the configuration blob and the layers.
	manifest := imgspecv1.Manifest{
		Versioned: imgspecs.Versioned{
			SchemaVersion: 2,
		},
		Config: imgspecv1.Descriptor{
			Digest:    configID,
			Size:      int64(len(configBytes)),
			MediaType: imgspecv1.MediaTypeImageConfig,
		},
		Layers:      layerDescriptors,
		Annotations: maps.Clone(r.annotations),
	}

	// Encode the manifest.
	manifestBytes, err := json.Marshal(&manifest)
	if err != nil {
		return nil, fmt.Errorf("error generating manifest for %q: %v", strings.Join(r.filenames, separator), err)
	}

	// Return the image.
	src := &tarballImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: false,
		}),
		NoGetBlobAtInitialize: stubs.NoGetBlobAt(r),

		reference: *r,
		blobs:     blobs,
		manifest:  manifestBytes,
	}
	src.Compat = impl.AddCompat(src)

	return src, nil
}

func (is *tarballImageSource) Close() error {
	return nil
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (is *tarballImageSource) GetBlob(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	blob, ok := is.blobs[blobinfo.Digest]
	if !ok {
		return nil, -1, fmt.Errorf("no blob with digest %q found", blobinfo.Digest.String())
	}
	if blob.contents != nil {
		return io.NopCloser(bytes.NewReader(blob.contents)), int64(len(blob.contents)), nil
	}
	reader, err := os.Open(blob.filename)
	if err != nil {
		return nil, -1, err
	}
	return reader, blob.size, nil
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

func (is *tarballImageSource) Reference() types.ImageReference {
	return &is.reference
}
