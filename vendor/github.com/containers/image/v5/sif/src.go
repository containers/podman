package sif

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecs "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/sylabs/sif/v2/pkg/sif"
)

type sifImageSource struct {
	ref          sifReference
	workDir      string
	layerDigest  digest.Digest
	layerSize    int64
	layerFile    string
	config       []byte
	configDigest digest.Digest
	manifest     []byte
}

// getBlobInfo returns the digest,  and size of the provided file.
func getBlobInfo(path string) (digest.Digest, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", -1, fmt.Errorf("opening %q for reading: %w", path, err)
	}
	defer f.Close()

	// TODO: Instead of writing the tar file to disk, and reading
	// it here again, stream the tar file to a pipe and
	// compute the digest while writing it to disk.
	logrus.Debugf("Computing a digest of the SIF conversion output...")
	digester := digest.Canonical.Digester()
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	size, err := io.Copy(digester.Hash(), f)
	if err != nil {
		return "", -1, fmt.Errorf("reading %q: %w", path, err)
	}
	digest := digester.Digest()
	logrus.Debugf("... finished computing the digest of the SIF conversion output")

	return digest, size, nil
}

// newImageSource returns an ImageSource for reading from an existing directory.
// newImageSource extracts SIF objects and saves them in a temp directory.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref sifReference) (types.ImageSource, error) {
	sifImg, err := sif.LoadContainerFromPath(ref.file, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return nil, fmt.Errorf("loading SIF file: %w", err)
	}
	defer func() {
		_ = sifImg.UnloadContainer()
	}()

	workDir, err := os.MkdirTemp(tmpdir.TemporaryDirectoryForBigFiles(sys), "sif")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			os.RemoveAll(workDir)
		}
	}()

	layerPath, commandLine, err := convertSIFToElements(ctx, sifImg, workDir)
	if err != nil {
		return nil, fmt.Errorf("converting rootfs from SquashFS to Tarball: %w", err)
	}

	layerDigest, layerSize, err := getBlobInfo(layerPath)
	if err != nil {
		return nil, fmt.Errorf("gathering blob information: %w", err)
	}

	created := sifImg.ModifiedAt()
	config := imgspecv1.Image{
		Created:      &created,
		Architecture: sifImg.PrimaryArch(),
		OS:           "linux",
		Config: imgspecv1.ImageConfig{
			Cmd: commandLine,
		},
		RootFS: imgspecv1.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layerDigest},
		},
		History: []imgspecv1.History{
			{
				Created:   &created,
				CreatedBy: fmt.Sprintf("/bin/sh -c #(nop) ADD file:%s in %c", layerDigest.Hex(), os.PathSeparator),
				Comment:   "imported from SIF, uuid: " + sifImg.ID(),
			},
			{
				Created:    &created,
				CreatedBy:  "/bin/sh -c #(nop) CMD [\"bash\"]",
				EmptyLayer: true,
			},
		},
	}
	configBytes, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("generating configuration blob for %q: %w", ref.resolvedFile, err)
	}
	configDigest := digest.Canonical.FromBytes(configBytes)

	manifest := imgspecv1.Manifest{
		Versioned: imgspecs.Versioned{SchemaVersion: 2},
		MediaType: imgspecv1.MediaTypeImageManifest,
		Config: imgspecv1.Descriptor{
			Digest:    configDigest,
			Size:      int64(len(configBytes)),
			MediaType: imgspecv1.MediaTypeImageConfig,
		},
		Layers: []imgspecv1.Descriptor{{
			Digest:    layerDigest,
			Size:      layerSize,
			MediaType: imgspecv1.MediaTypeImageLayer,
		}},
	}
	manifestBytes, err := json.Marshal(&manifest)
	if err != nil {
		return nil, fmt.Errorf("generating manifest for %q: %w", ref.resolvedFile, err)
	}

	succeeded = true
	return &sifImageSource{
		ref:          ref,
		workDir:      workDir,
		layerDigest:  layerDigest,
		layerSize:    layerSize,
		layerFile:    layerPath,
		config:       configBytes,
		configDigest: configDigest,
		manifest:     manifestBytes,
	}, nil
}

// Reference returns the reference used to set up this source.
func (s *sifImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *sifImageSource) Close() error {
	return os.RemoveAll(s.workDir)
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *sifImageSource) HasThreadSafeGetBlob() bool {
	return true
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *sifImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	switch info.Digest {
	case s.configDigest:
		return io.NopCloser(bytes.NewBuffer(s.config)), int64(len(s.config)), nil
	case s.layerDigest:
		reader, err := os.Open(s.layerFile)
		if err != nil {
			return nil, -1, fmt.Errorf("opening %q: %w", s.layerFile, err)
		}
		return reader, s.layerSize, nil
	default:
		return nil, -1, fmt.Errorf("no blob with digest %q found", info.Digest.String())
	}
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *sifImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return nil, "", errors.New("manifest lists are not supported by the sif transport")
	}
	return s.manifest, imgspecv1.MediaTypeImageManifest, nil
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *sifImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, errors.New("manifest lists are not supported by the sif transport")
	}
	return nil, nil
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve BlobInfos for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *sifImageSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	return nil, nil
}
