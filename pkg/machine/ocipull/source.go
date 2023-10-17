package ocipull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// readManifestFromImageSource reads the manifest from the specified image
// source.  Note that the manifest is expected to be an OCI v1 manifest.
func readManifestFromImageSource(ctx context.Context, src types.ImageSource) (*specV1.Manifest, *digest.Digest, int64, error) {
	rawData, mimeType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return nil, nil, -1, err
	}
	if mimeType != specV1.MediaTypeImageManifest {
		return nil, nil, -1, fmt.Errorf("image %q is of type %q (expected: %q)", strings.TrimPrefix(src.Reference().StringWithinTransport(), "//"), mimeType, specV1.MediaTypeImageManifest)
	}

	manifest := specV1.Manifest{}
	if err := json.Unmarshal(rawData, &manifest); err != nil {
		return nil, nil, -1, fmt.Errorf("reading manifest: %w", err)
	}

	manifestDigest := digest.FromBytes(rawData)
	return &manifest, &manifestDigest, int64(len(rawData)), nil
}

// readManifestFromOCIPath returns the manifest of the specified source image
// at `sourcePath` along with its digest.  The digest can later on be used to
// locate the manifest on the file system.
func readManifestFromOCIPath(ctx context.Context, sourcePath string) (*specV1.Manifest, *digest.Digest, int64, error) {
	ociRef, err := layout.ParseReference(sourcePath)
	if err != nil {
		return nil, nil, -1, err
	}

	ociSource, err := ociRef.NewImageSource(ctx, &types.SystemContext{})
	if err != nil {
		return nil, nil, -1, err
	}
	defer ociSource.Close()

	return readManifestFromImageSource(ctx, ociSource)
}

func GetLocalBlob(ctx context.Context, path string) (*types.BlobInfo, error) {
	ociRef, err := layout.ParseReference(path)
	if err != nil {
		return nil, err
	}
	img, err := ociRef.NewImage(ctx, &types.SystemContext{})
	if err != nil {
		return nil, err
	}

	b, _, err := img.Manifest(ctx)
	if err != nil {
		return nil, err
	}

	localManifest := specV1.Manifest{}
	if err := json.Unmarshal(b, &localManifest); err != nil {
		return nil, err
	}
	blobs := img.LayerInfos()
	if err != nil {
		return nil, err
	}
	if len(blobs) != 1 {
		return nil, errors.New("invalid disk image")
	}
	fmt.Println(blobs[0].Digest.Hex())
	return &blobs[0], nil
}

func GetRemoteManifest(ctx context.Context, dest string) (*specV1.Manifest, error) {
	ref, err := docker.ParseReference(fmt.Sprintf("//%s", dest))
	if err != nil {
		return nil, err
	}

	imgSrc, err := ref.NewImage(ctx, &types.SystemContext{})
	if err != nil {
		return nil, err
	}

	b, _, err := imgSrc.Manifest(ctx)
	if err != nil {
		return nil, err
	}

	remoteManifest := specV1.Manifest{}
	err = json.Unmarshal(b, &remoteManifest)
	return &remoteManifest, err
}

func GetRemoteDescriptor(ctx context.Context, dest string) (*specV1.Descriptor, error) {
	remoteManifest, err := GetRemoteManifest(ctx, dest)
	if err != nil {
		return nil, err
	}
	if len(remoteManifest.Layers) != 1 {
		return nil, errors.New("invalid remote disk image")
	}
	return &remoteManifest.Layers[0], nil
}

func ReadImageManifestFromOCIPath(ctx context.Context, ociImagePath string) (*specV1.Manifest, error) {
	imageManifest, _, _, err := readManifestFromOCIPath(ctx, ociImagePath)
	return imageManifest, err
}
