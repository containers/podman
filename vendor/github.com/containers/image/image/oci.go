package image

import (
	"encoding/json"
	"io/ioutil"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type descriptorOCI1 struct {
	descriptor
	Annotations map[string]string `json:"annotations,omitempty"`
}

type manifestOCI1 struct {
	src               types.ImageSource // May be nil if configBlob is not nil
	configBlob        []byte            // If set, corresponds to contents of ConfigDescriptor.
	SchemaVersion     int               `json:"schemaVersion"`
	ConfigDescriptor  descriptorOCI1    `json:"config"`
	LayersDescriptors []descriptorOCI1  `json:"layers"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

func manifestOCI1FromManifest(src types.ImageSource, manifest []byte) (genericManifest, error) {
	oci := manifestOCI1{src: src}
	if err := json.Unmarshal(manifest, &oci); err != nil {
		return nil, err
	}
	return &oci, nil
}

// manifestOCI1FromComponents builds a new manifestOCI1 from the supplied data:
func manifestOCI1FromComponents(config descriptorOCI1, src types.ImageSource, configBlob []byte, layers []descriptorOCI1) genericManifest {
	return &manifestOCI1{
		src:               src,
		configBlob:        configBlob,
		SchemaVersion:     2,
		ConfigDescriptor:  config,
		LayersDescriptors: layers,
	}
}

func (m *manifestOCI1) serialize() ([]byte, error) {
	return json.Marshal(*m)
}

func (m *manifestOCI1) manifestMIMEType() string {
	return imgspecv1.MediaTypeImageManifest
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
// Note that the config object may not exist in the underlying storage in the return value of UpdatedImage! Use ConfigBlob() below.
func (m *manifestOCI1) ConfigInfo() types.BlobInfo {
	return types.BlobInfo{Digest: m.ConfigDescriptor.Digest, Size: m.ConfigDescriptor.Size, Annotations: m.ConfigDescriptor.Annotations}
}

// ConfigBlob returns the blob described by ConfigInfo, iff ConfigInfo().Digest != ""; nil otherwise.
// The result is cached; it is OK to call this however often you need.
func (m *manifestOCI1) ConfigBlob() ([]byte, error) {
	if m.configBlob == nil {
		if m.src == nil {
			return nil, errors.Errorf("Internal error: neither src nor configBlob set in manifestOCI1")
		}
		stream, _, err := m.src.GetBlob(types.BlobInfo{
			Digest: m.ConfigDescriptor.Digest,
			Size:   m.ConfigDescriptor.Size,
			URLs:   m.ConfigDescriptor.URLs,
		})
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		blob, err := ioutil.ReadAll(stream)
		if err != nil {
			return nil, err
		}
		computedDigest := digest.FromBytes(blob)
		if computedDigest != m.ConfigDescriptor.Digest {
			return nil, errors.Errorf("Download config.json digest %s does not match expected %s", computedDigest, m.ConfigDescriptor.Digest)
		}
		m.configBlob = blob
	}
	return m.configBlob, nil
}

// OCIConfig returns the image configuration as per OCI v1 image-spec. Information about
// layers in the resulting configuration isn't guaranteed to be returned to due how
// old image manifests work (docker v2s1 especially).
func (m *manifestOCI1) OCIConfig() (*imgspecv1.Image, error) {
	cb, err := m.ConfigBlob()
	if err != nil {
		return nil, err
	}
	configOCI := &imgspecv1.Image{}
	if err := json.Unmarshal(cb, configOCI); err != nil {
		return nil, err
	}
	return configOCI, nil
}

// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *manifestOCI1) LayerInfos() []types.BlobInfo {
	blobs := []types.BlobInfo{}
	for _, layer := range m.LayersDescriptors {
		blobs = append(blobs, types.BlobInfo{Digest: layer.Digest, Size: layer.Size, Annotations: layer.Annotations, URLs: layer.URLs, MediaType: layer.MediaType})
	}
	return blobs
}

// EmbeddedDockerReferenceConflicts whether a Docker reference embedded in the manifest, if any, conflicts with destination ref.
// It returns false if the manifest does not embed a Docker reference.
// (This embedding unfortunately happens for Docker schema1, please do not add support for this in any new formats.)
func (m *manifestOCI1) EmbeddedDockerReferenceConflicts(ref reference.Named) bool {
	return false
}

func (m *manifestOCI1) imageInspectInfo() (*types.ImageInspectInfo, error) {
	config, err := m.ConfigBlob()
	if err != nil {
		return nil, err
	}
	v1 := &v1Image{}
	if err := json.Unmarshal(config, v1); err != nil {
		return nil, err
	}
	i := &types.ImageInspectInfo{
		DockerVersion: v1.DockerVersion,
		Created:       v1.Created,
		Architecture:  v1.Architecture,
		Os:            v1.OS,
	}
	if v1.Config != nil {
		i.Labels = v1.Config.Labels
	}
	return i, nil
}

// UpdatedImageNeedsLayerDiffIDs returns true iff UpdatedImage(options) needs InformationOnly.LayerDiffIDs.
// This is a horribly specific interface, but computing InformationOnly.LayerDiffIDs can be very expensive to compute
// (most importantly it forces us to download the full layers even if they are already present at the destination).
func (m *manifestOCI1) UpdatedImageNeedsLayerDiffIDs(options types.ManifestUpdateOptions) bool {
	return false
}

// UpdatedImage returns a types.Image modified according to options.
// This does not change the state of the original Image object.
func (m *manifestOCI1) UpdatedImage(options types.ManifestUpdateOptions) (types.Image, error) {
	copy := *m // NOTE: This is not a deep copy, it still shares slices etc.
	if options.LayerInfos != nil {
		if len(copy.LayersDescriptors) != len(options.LayerInfos) {
			return nil, errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(copy.LayersDescriptors), len(options.LayerInfos))
		}
		copy.LayersDescriptors = make([]descriptorOCI1, len(options.LayerInfos))
		for i, info := range options.LayerInfos {
			copy.LayersDescriptors[i].MediaType = m.LayersDescriptors[i].MediaType
			copy.LayersDescriptors[i].Digest = info.Digest
			copy.LayersDescriptors[i].Size = info.Size
			copy.LayersDescriptors[i].Annotations = info.Annotations
			copy.LayersDescriptors[i].URLs = info.URLs
		}
	}
	// Ignore options.EmbeddedDockerReference: it may be set when converting from schema1, but we really don't care.

	switch options.ManifestMIMEType {
	case "": // No conversion, OK
	case manifest.DockerV2Schema2MediaType:
		return copy.convertToManifestSchema2()
	default:
		return nil, errors.Errorf("Conversion of image manifest from %s to %s is not implemented", imgspecv1.MediaTypeImageManifest, options.ManifestMIMEType)
	}

	return memoryImageFromManifest(&copy), nil
}

func (m *manifestOCI1) convertToManifestSchema2() (types.Image, error) {
	// Create a copy of the descriptor.
	config := m.ConfigDescriptor.descriptor

	// The only difference between OCI and DockerSchema2 is the mediatypes. The
	// media type of the manifest is handled by manifestSchema2FromComponents.
	config.MediaType = manifest.DockerV2Schema2ConfigMediaType

	layers := make([]descriptor, len(m.LayersDescriptors))
	for idx := range layers {
		layers[idx] = m.LayersDescriptors[idx].descriptor
		layers[idx].MediaType = manifest.DockerV2Schema2LayerMediaType
	}

	// Rather than copying the ConfigBlob now, we just pass m.src to the
	// translated manifest, since the only difference is the mediatype of
	// descriptors there is no change to any blob stored in m.src.
	m1 := manifestSchema2FromComponents(config, m.src, nil, layers)
	return memoryImageFromManifest(m1), nil
}
