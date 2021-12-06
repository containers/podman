package manifest

import (
	"encoding/json"
	"fmt"
	"strings"

	compressiontypes "github.com/containers/image/v5/pkg/compression/types"
	"github.com/containers/image/v5/types"
	ociencspec "github.com/containers/ocicrypt/spec"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// BlobInfoFromOCI1Descriptor returns a types.BlobInfo based on the input OCI1 descriptor.
func BlobInfoFromOCI1Descriptor(desc imgspecv1.Descriptor) types.BlobInfo {
	return types.BlobInfo{
		Digest:      desc.Digest,
		Size:        desc.Size,
		URLs:        desc.URLs,
		Annotations: desc.Annotations,
		MediaType:   desc.MediaType,
	}
}

// OCI1 is a manifest.Manifest implementation for OCI images.
// The underlying data from imgspecv1.Manifest is also available.
type OCI1 struct {
	imgspecv1.Manifest
}

// SupportedOCI1MediaType checks if the specified string is a supported OCI1
// media type.
//
// Deprecated: blindly rejecting unknown MIME types when the consumer does not
// need to process the input just reduces interoperability (and violates the
// standard) with no benefit, and that this function does not check that the
// media type is appropriate for any specific purpose, so it’s not all that
// useful for validation anyway.
func SupportedOCI1MediaType(m string) error {
	switch m {
	case imgspecv1.MediaTypeDescriptor, imgspecv1.MediaTypeImageConfig, imgspecv1.MediaTypeImageLayer, imgspecv1.MediaTypeImageLayerGzip, imgspecv1.MediaTypeImageLayerNonDistributable, imgspecv1.MediaTypeImageLayerNonDistributableGzip, imgspecv1.MediaTypeImageLayerNonDistributableZstd, imgspecv1.MediaTypeImageLayerZstd, imgspecv1.MediaTypeImageManifest, imgspecv1.MediaTypeLayoutHeader, ociencspec.MediaTypeLayerEnc, ociencspec.MediaTypeLayerGzipEnc:
		return nil
	default:
		return fmt.Errorf("unsupported OCIv1 media type: %q", m)
	}
}

// OCI1FromManifest creates an OCI1 manifest instance from a manifest blob.
func OCI1FromManifest(manifest []byte) (*OCI1, error) {
	oci1 := OCI1{}
	if err := json.Unmarshal(manifest, &oci1); err != nil {
		return nil, err
	}
	if err := validateUnambiguousManifestFormat(manifest, imgspecv1.MediaTypeImageIndex,
		allowedFieldConfig|allowedFieldLayers); err != nil {
		return nil, err
	}
	return &oci1, nil
}

// OCI1FromComponents creates an OCI1 manifest instance from the supplied data.
func OCI1FromComponents(config imgspecv1.Descriptor, layers []imgspecv1.Descriptor) *OCI1 {
	return &OCI1{
		imgspecv1.Manifest{
			Versioned: specs.Versioned{SchemaVersion: 2},
			MediaType: imgspecv1.MediaTypeImageManifest,
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
	return BlobInfoFromOCI1Descriptor(m.Config)
}

// LayerInfos returns a list of LayerInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *OCI1) LayerInfos() []LayerInfo {
	blobs := []LayerInfo{}
	for _, layer := range m.Layers {
		blobs = append(blobs, LayerInfo{
			BlobInfo:   BlobInfoFromOCI1Descriptor(layer),
			EmptyLayer: false,
		})
	}
	return blobs
}

var oci1CompressionMIMETypeSets = []compressionMIMETypeSet{
	{
		mtsUncompressed:                    imgspecv1.MediaTypeImageLayerNonDistributable,
		compressiontypes.GzipAlgorithmName: imgspecv1.MediaTypeImageLayerNonDistributableGzip,
		compressiontypes.ZstdAlgorithmName: imgspecv1.MediaTypeImageLayerNonDistributableZstd,
	},
	{
		mtsUncompressed:                    imgspecv1.MediaTypeImageLayer,
		compressiontypes.GzipAlgorithmName: imgspecv1.MediaTypeImageLayerGzip,
		compressiontypes.ZstdAlgorithmName: imgspecv1.MediaTypeImageLayerZstd,
	},
}

// UpdateLayerInfos replaces the original layers with the specified BlobInfos (size+digest+urls+mediatype), in order (the root layer first, and then successive layered layers)
// The returned error will be a manifest.ManifestLayerCompressionIncompatibilityError if any of the layerInfos includes a combination of CompressionOperation and
// CompressionAlgorithm that isn't supported by OCI.
func (m *OCI1) UpdateLayerInfos(layerInfos []types.BlobInfo) error {
	if len(m.Layers) != len(layerInfos) {
		return errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(m.Layers), len(layerInfos))
	}
	original := m.Layers
	m.Layers = make([]imgspecv1.Descriptor, len(layerInfos))
	for i, info := range layerInfos {
		mimeType := original[i].MediaType
		if info.CryptoOperation == types.Decrypt {
			decMimeType, err := getDecryptedMediaType(mimeType)
			if err != nil {
				return fmt.Errorf("error preparing updated manifest: decryption specified but original mediatype is not encrypted: %q", mimeType)
			}
			mimeType = decMimeType
		}
		mimeType, err := updatedMIMEType(oci1CompressionMIMETypeSets, mimeType, info)
		if err != nil {
			return errors.Wrapf(err, "preparing updated manifest, layer %q", info.Digest)
		}
		if info.CryptoOperation == types.Encrypt {
			encMediaType, err := getEncryptedMediaType(mimeType)
			if err != nil {
				return fmt.Errorf("error preparing updated manifest: encryption specified but no counterpart for mediatype: %q", mimeType)
			}
			mimeType = encMediaType
		}

		m.Layers[i].MediaType = mimeType
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
	if err := json.Unmarshal(config, d1); err != nil {
		return nil, err
	}
	i := &types.ImageInspectInfo{
		Tag:           "",
		Created:       v1.Created,
		DockerVersion: d1.DockerVersion,
		Labels:        v1.Config.Labels,
		Architecture:  v1.Architecture,
		Os:            v1.OS,
		Layers:        layerInfosToStrings(m.LayerInfos()),
		Env:           v1.Config.Env,
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

// getEncryptedMediaType will return the mediatype to its encrypted counterpart and return
// an error if the mediatype does not support encryption
func getEncryptedMediaType(mediatype string) (string, error) {
	for _, s := range strings.Split(mediatype, "+")[1:] {
		if s == "encrypted" {
			return "", errors.Errorf("unsupportedmediatype: %v already encrypted", mediatype)
		}
	}
	unsuffixedMediatype := strings.Split(mediatype, "+")[0]
	switch unsuffixedMediatype {
	case DockerV2Schema2LayerMediaType, imgspecv1.MediaTypeImageLayer, imgspecv1.MediaTypeImageLayerNonDistributable:
		return mediatype + "+encrypted", nil
	}

	return "", errors.Errorf("unsupported mediatype to encrypt: %v", mediatype)
}

// getEncryptedMediaType will return the mediatype to its encrypted counterpart and return
// an error if the mediatype does not support decryption
func getDecryptedMediaType(mediatype string) (string, error) {
	if !strings.HasSuffix(mediatype, "+encrypted") {
		return "", errors.Errorf("unsupported mediatype to decrypt %v:", mediatype)
	}

	return strings.TrimSuffix(mediatype, "+encrypted"), nil
}
