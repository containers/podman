package libpod

import (
	"encoding/json"
	"strings"

	digest "github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/image"
	"github.com/projectatomic/libpod/pkg/inspect"
)

// GetImageData returns an image's inspect data
func GetImageData(img *image.Image) (*inspect.ImageData, error) {
	imgRef, err := img.ToImageRef()
	if err != nil {
		return nil, err
	}
	size, err := imgRef.Size()
	if err != nil {
		return nil, err
	}
	manifest, manifestType, err := imgRef.Manifest()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading manifest for image %q", img.ID)
	}
	imgDigest := digest.Digest("")
	if len(manifest) > 0 {
		imgDigest = digest.Canonical.FromBytes(manifest)
	}
	annotations := annotations(manifest, manifestType)

	ociv1Img, err := imgRef.OCIConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting oci image info %q", img.ID)
	}
	info, err := imgRef.Inspect()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting image info %q", img.ID)
	}

	var repoDigests []string
	for _, name := range img.Names() {
		repoDigests = append(repoDigests, strings.SplitN(name, ":", 2)[0]+"@"+imgDigest.String())
	}

	driver, err := img.DriverData()
	if err != nil {
		return nil, err
	}
	data := &inspect.ImageData{
		ID:              img.ID(),
		RepoTags:        img.Names(),
		RepoDigests:     repoDigests,
		Comment:         ociv1Img.History[0].Comment,
		Created:         ociv1Img.Created,
		Author:          ociv1Img.Author,
		Architecture:    ociv1Img.Architecture,
		Os:              ociv1Img.OS,
		ContainerConfig: &ociv1Img.Config,
		Version:         info.DockerVersion,
		Size:            size,
		VirtualSize:     size,
		Annotations:     annotations,
		Digest:          imgDigest,
		Labels:          info.Labels,
		RootFS: &inspect.RootFS{
			Type:   ociv1Img.RootFS.Type,
			Layers: ociv1Img.RootFS.DiffIDs,
		},
		GraphDriver: driver,
	}
	return data, nil
}

func annotations(manifest []byte, manifestType string) map[string]string {
	annotations := make(map[string]string)
	switch manifestType {
	case ociv1.MediaTypeImageManifest:
		var m ociv1.Manifest
		if err := json.Unmarshal(manifest, &m); err == nil {
			for k, v := range m.Annotations {
				annotations[k] = v
			}
		}
	}
	return annotations
}
