package ocipull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
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

	mannyFest := specV1.Manifest{}
	if err := json.Unmarshal(rawData, &mannyFest); err != nil {
		return nil, nil, -1, fmt.Errorf("reading manifest: %w", err)
	}

	manifestDigest := digest.FromBytes(rawData)
	return &mannyFest, &manifestDigest, int64(len(rawData)), nil
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

func GetRemoteManifest(ctx context.Context, dest string) (*specV1.Manifest, error) { //nolint:unused
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

func GetDiskArtifactReference(ctx context.Context, imgSrc types.ImageSource, opts *DiskArtifactOpts) (digest.Digest, error) {
	rawMannyFest, mannyType, err := imgSrc.GetManifest(ctx, nil)
	if err != nil {
		return "", err
	}

	if !manifest.MIMETypeIsMultiImage(mannyType) { // if not true
		return "", fmt.Errorf("wrong manifest type for disk artifact: %s", mannyType)
	}

	mannyFestList, err := manifest.ListFromBlob(rawMannyFest, mannyType)
	if err != nil {
		return "", fmt.Errorf("failed to parse manifest list from blob: %q", err)
	}

	var (
		artifactDigest digest.Digest
	)
	for _, d := range mannyFestList.Instances() {
		bar, err := mannyFestList.Instance(d)
		if err != nil {
			return "", err
		}
		val, ok := bar.ReadOnly.Annotations["disktype"]
		if !ok { // quick exit, no type match
			continue
		}
		// wrong arch
		if bar.ReadOnly.Platform.Architecture != opts.arch {
			continue
		}
		// wrong os
		if bar.ReadOnly.Platform.OS != opts.os {
			continue
		}
		// wrong disktype
		if val != opts.diskType {
			continue
		}

		// ok, we have a match
		artifactDigest = d
		logrus.Debugf("found image in digest: %q", artifactDigest.String())
		break
	}
	if artifactDigest == "" {
		return "", fmt.Errorf("no valid disk artifact found")
	}
	v1RawMannyfest, _, err := imgSrc.GetManifest(ctx, &artifactDigest)
	if err != nil {
		return "", err
	}
	v1MannyFest := specV1.Manifest{}
	if err := json.Unmarshal(v1RawMannyfest, &v1MannyFest); err != nil {
		return "", err
	}
	if layerLen := len(v1MannyFest.Layers); layerLen > 1 {
		return "", fmt.Errorf("podman-machine images should only have 1 layer: %d found", layerLen)
	}

	// podman-machine-images should have a original file name
	// stored in the annotations under org.opencontainers.image.title
	// i.e. fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2.xz
	originalFileName, ok := v1MannyFest.Layers[0].Annotations["org.opencontainers.image.title"]
	if !ok {
		return "", fmt.Errorf("unable to determine original artifact name: missing required annotation 'org.opencontainers.image.title'")
	}
	logrus.Debugf("original artifact file name: %s", originalFileName)
	return artifactDigest, err
}
