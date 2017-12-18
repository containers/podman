package manifest

import (
	"encoding/json"
	"time"

	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// OCI1 is a manifest.Manifest implementation for OCI images.
// The underlying data from imgspecv1.Manifest is also available.
type OCI1 struct {
	imgspecv1.Manifest
}

// OCI1FromManifest creates an OCI1 manifest instance from a manifest blob.
func OCI1FromManifest(manifest []byte) (*OCI1, error) {
	oci1 := OCI1{}
	if err := json.Unmarshal(manifest, &oci1); err != nil {
		return nil, err
	}
	return &oci1, nil
}

// OCI1FromComponents creates an OCI1 manifest instance from the supplied data.
func OCI1FromComponents(config imgspecv1.Descriptor, layers []imgspecv1.Descriptor) *OCI1 {
	return &OCI1{
		imgspecv1.Manifest{
			Versioned: specs.Versioned{SchemaVersion: 2},
			Config:    config,
			Layers:    layers,
		},
	}
}

// OCI1Clone creates a copy of the supplied OCI1 manifest.
func OCI1Clone(src *OCI1) *OCI1 {
	return &OCI1{
		Manifest: src.Manifest,
	}
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
func (m *OCI1) ConfigInfo() types.BlobInfo {
	return types.BlobInfo{Digest: m.Config.Digest, Size: m.Config.Size, Annotations: m.Config.Annotations}
}

// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *OCI1) LayerInfos() []types.BlobInfo {
	blobs := []types.BlobInfo{}
	for _, layer := range m.Layers {
		blobs = append(blobs, types.BlobInfo{Digest: layer.Digest, Size: layer.Size, Annotations: layer.Annotations, URLs: layer.URLs, MediaType: layer.MediaType})
	}
	return blobs
}

// UpdateLayerInfos replaces the original layers with the specified BlobInfos (size+digest+urls), in order (the root layer first, and then successive layered layers)
func (m *OCI1) UpdateLayerInfos(layerInfos []types.BlobInfo) error {
	if len(m.Layers) != len(layerInfos) {
		return errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(m.Layers), len(layerInfos))
	}
	original := m.Layers
	m.Layers = make([]imgspecv1.Descriptor, len(layerInfos))
	for i, info := range layerInfos {
		m.Layers[i].MediaType = original[i].MediaType
		m.Layers[i].Digest = info.Digest
		m.Layers[i].Size = info.Size
		m.Layers[i].Annotations = info.Annotations
		m.Layers[i].URLs = info.URLs
	}
	return nil
}

// Serialize returns the manifest in a blob format.
// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded from one, even if no modifications were made!
func (m *OCI1) Serialize() ([]byte, error) {
	return json.Marshal(*m)
}

// Inspect returns various information for (skopeo inspect) parsed from the manifest and configuration.
func (m *OCI1) Inspect(configGetter func(types.BlobInfo) ([]byte, error)) (*types.ImageInspectInfo, error) {
	config, err := configGetter(m.ConfigInfo())
	if err != nil {
		return nil, err
	}
	v1 := &imgspecv1.Image{}
	if err := json.Unmarshal(config, v1); err != nil {
		return nil, err
	}
	d1 := &Schema2V1Image{}
	json.Unmarshal(config, d1)
	created := time.Time{}
	if v1.Created != nil {
		created = *v1.Created
	}
	i := &types.ImageInspectInfo{
		Tag:           "",
		Created:       created,
		DockerVersion: d1.DockerVersion,
		Labels:        v1.Config.Labels,
		Architecture:  v1.Architecture,
		Os:            v1.OS,
		Layers:        LayerInfosToStrings(m.LayerInfos()),
	}
	return i, nil
}

// ImageID computes an ID which can uniquely identify this image by its contents.
func (m *OCI1) ImageID([]digest.Digest) (string, error) {
	if err := m.Config.Digest.Validate(); err != nil {
		return "", err
	}
	return m.Config.Digest.Hex(), nil
}
