package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// gzippedEmptyLayer is a gzip-compressed version of an empty tar file (1024 NULL bytes)
// This comes from github.com/docker/distribution/manifest/schema1/config_builder.go; there is
// a non-zero embedded timestamp; we could zero that, but that would just waste storage space
// in registries, so let’s use the same values.
var gzippedEmptyLayer = []byte{
	31, 139, 8, 0, 0, 9, 110, 136, 0, 255, 98, 24, 5, 163, 96, 20, 140, 88,
	0, 8, 0, 0, 255, 255, 46, 175, 181, 239, 0, 4, 0, 0,
}

// gzippedEmptyLayerDigest is a digest of gzippedEmptyLayer
const gzippedEmptyLayerDigest = digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")

type descriptor struct {
	MediaType string        `json:"mediaType"`
	Size      int64         `json:"size"`
	Digest    digest.Digest `json:"digest"`
	URLs      []string      `json:"urls,omitempty"`
}

type manifestSchema2 struct {
	src               types.ImageSource // May be nil if configBlob is not nil
	configBlob        []byte            // If set, corresponds to contents of ConfigDescriptor.
	SchemaVersion     int               `json:"schemaVersion"`
	MediaType         string            `json:"mediaType"`
	ConfigDescriptor  descriptor        `json:"config"`
	LayersDescriptors []descriptor      `json:"layers"`
}

func manifestSchema2FromManifest(src types.ImageSource, manifest []byte) (genericManifest, error) {
	v2s2 := manifestSchema2{src: src}
	if err := json.Unmarshal(manifest, &v2s2); err != nil {
		return nil, err
	}
	return &v2s2, nil
}

// manifestSchema2FromComponents builds a new manifestSchema2 from the supplied data:
func manifestSchema2FromComponents(config descriptor, src types.ImageSource, configBlob []byte, layers []descriptor) genericManifest {
	return &manifestSchema2{
		src:               src,
		configBlob:        configBlob,
		SchemaVersion:     2,
		MediaType:         manifest.DockerV2Schema2MediaType,
		ConfigDescriptor:  config,
		LayersDescriptors: layers,
	}
}

func (m *manifestSchema2) serialize() ([]byte, error) {
	return json.Marshal(*m)
}

func (m *manifestSchema2) manifestMIMEType() string {
	return m.MediaType
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
// Note that the config object may not exist in the underlying storage in the return value of UpdatedImage! Use ConfigBlob() below.
func (m *manifestSchema2) ConfigInfo() types.BlobInfo {
	return types.BlobInfo{Digest: m.ConfigDescriptor.Digest, Size: m.ConfigDescriptor.Size}
}

// OCIConfig returns the image configuration as per OCI v1 image-spec. Information about
// layers in the resulting configuration isn't guaranteed to be returned to due how
// old image manifests work (docker v2s1 especially).
func (m *manifestSchema2) OCIConfig() (*imgspecv1.Image, error) {
	configBlob, err := m.ConfigBlob()
	if err != nil {
		return nil, err
	}
	// docker v2s2 and OCI v1 are mostly compatible but v2s2 contains more fields
	// than OCI v1. This unmarshal makes sure we drop docker v2s2
	// fields that aren't needed in OCI v1.
	configOCI := &imgspecv1.Image{}
	if err := json.Unmarshal(configBlob, configOCI); err != nil {
		return nil, err
	}
	return configOCI, nil
}

// ConfigBlob returns the blob described by ConfigInfo, iff ConfigInfo().Digest != ""; nil otherwise.
// The result is cached; it is OK to call this however often you need.
func (m *manifestSchema2) ConfigBlob() ([]byte, error) {
	if m.configBlob == nil {
		if m.src == nil {
			return nil, errors.Errorf("Internal error: neither src nor configBlob set in manifestSchema2")
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

// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *manifestSchema2) LayerInfos() []types.BlobInfo {
	blobs := []types.BlobInfo{}
	for _, layer := range m.LayersDescriptors {
		blobs = append(blobs, types.BlobInfo{
			Digest: layer.Digest,
			Size:   layer.Size,
			URLs:   layer.URLs,
		})
	}
	return blobs
}

// EmbeddedDockerReferenceConflicts whether a Docker reference embedded in the manifest, if any, conflicts with destination ref.
// It returns false if the manifest does not embed a Docker reference.
// (This embedding unfortunately happens for Docker schema1, please do not add support for this in any new formats.)
func (m *manifestSchema2) EmbeddedDockerReferenceConflicts(ref reference.Named) bool {
	return false
}

func (m *manifestSchema2) imageInspectInfo() (*types.ImageInspectInfo, error) {
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
func (m *manifestSchema2) UpdatedImageNeedsLayerDiffIDs(options types.ManifestUpdateOptions) bool {
	return false
}

// UpdatedImage returns a types.Image modified according to options.
// This does not change the state of the original Image object.
func (m *manifestSchema2) UpdatedImage(options types.ManifestUpdateOptions) (types.Image, error) {
	copy := *m // NOTE: This is not a deep copy, it still shares slices etc.
	if options.LayerInfos != nil {
		if len(copy.LayersDescriptors) != len(options.LayerInfos) {
			return nil, errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(copy.LayersDescriptors), len(options.LayerInfos))
		}
		copy.LayersDescriptors = make([]descriptor, len(options.LayerInfos))
		for i, info := range options.LayerInfos {
			copy.LayersDescriptors[i].MediaType = m.LayersDescriptors[i].MediaType
			copy.LayersDescriptors[i].Digest = info.Digest
			copy.LayersDescriptors[i].Size = info.Size
			copy.LayersDescriptors[i].URLs = info.URLs
		}
	}
	// Ignore options.EmbeddedDockerReference: it may be set when converting from schema1 to schema2, but we really don't care.

	switch options.ManifestMIMEType {
	case "": // No conversion, OK
	case manifest.DockerV2Schema1SignedMediaType, manifest.DockerV2Schema1MediaType:
		return copy.convertToManifestSchema1(options.InformationOnly.Destination)
	case imgspecv1.MediaTypeImageManifest:
		return copy.convertToManifestOCI1()
	default:
		return nil, errors.Errorf("Conversion of image manifest from %s to %s is not implemented", manifest.DockerV2Schema2MediaType, options.ManifestMIMEType)
	}

	return memoryImageFromManifest(&copy), nil
}

func (m *manifestSchema2) convertToManifestOCI1() (types.Image, error) {
	configOCI, err := m.OCIConfig()
	if err != nil {
		return nil, err
	}
	configOCIBytes, err := json.Marshal(configOCI)
	if err != nil {
		return nil, err
	}

	config := descriptorOCI1{
		descriptor: descriptor{
			MediaType: imgspecv1.MediaTypeImageConfig,
			Size:      int64(len(configOCIBytes)),
			Digest:    digest.FromBytes(configOCIBytes),
		},
	}

	layers := make([]descriptorOCI1, len(m.LayersDescriptors))
	for idx := range layers {
		layers[idx] = descriptorOCI1{descriptor: m.LayersDescriptors[idx]}
		if m.LayersDescriptors[idx].MediaType == manifest.DockerV2Schema2ForeignLayerMediaType {
			layers[idx].MediaType = imgspecv1.MediaTypeImageLayerNonDistributable
		} else {
			// we assume layers are gzip'ed because docker v2s2 only deals with
			// gzip'ed layers. However, OCI has non-gzip'ed layers as well.
			layers[idx].MediaType = imgspecv1.MediaTypeImageLayerGzip
		}
	}

	m1 := manifestOCI1FromComponents(config, m.src, configOCIBytes, layers)
	return memoryImageFromManifest(m1), nil
}

// Based on docker/distribution/manifest/schema1/config_builder.go
func (m *manifestSchema2) convertToManifestSchema1(dest types.ImageDestination) (types.Image, error) {
	configBytes, err := m.ConfigBlob()
	if err != nil {
		return nil, err
	}
	imageConfig := &image{}
	if err := json.Unmarshal(configBytes, imageConfig); err != nil {
		return nil, err
	}

	// Build fsLayers and History, discarding all configs. We will patch the top-level config in later.
	fsLayers := make([]fsLayersSchema1, len(imageConfig.History))
	history := make([]historySchema1, len(imageConfig.History))
	nonemptyLayerIndex := 0
	var parentV1ID string // Set in the loop
	v1ID := ""
	haveGzippedEmptyLayer := false
	if len(imageConfig.History) == 0 {
		// What would this even mean?! Anyhow, the rest of the code depends on fsLayers[0] and history[0] existing.
		return nil, errors.Errorf("Cannot convert an image with 0 history entries to %s", manifest.DockerV2Schema1SignedMediaType)
	}
	for v2Index, historyEntry := range imageConfig.History {
		parentV1ID = v1ID
		v1Index := len(imageConfig.History) - 1 - v2Index

		var blobDigest digest.Digest
		if historyEntry.EmptyLayer {
			if !haveGzippedEmptyLayer {
				logrus.Debugf("Uploading empty layer during conversion to schema 1")
				info, err := dest.PutBlob(bytes.NewReader(gzippedEmptyLayer), types.BlobInfo{Digest: gzippedEmptyLayerDigest, Size: int64(len(gzippedEmptyLayer))})
				if err != nil {
					return nil, errors.Wrap(err, "Error uploading empty layer")
				}
				if info.Digest != gzippedEmptyLayerDigest {
					return nil, errors.Errorf("Internal error: Uploaded empty layer has digest %#v instead of %s", info.Digest, gzippedEmptyLayerDigest)
				}
				haveGzippedEmptyLayer = true
			}
			blobDigest = gzippedEmptyLayerDigest
		} else {
			if nonemptyLayerIndex >= len(m.LayersDescriptors) {
				return nil, errors.Errorf("Invalid image configuration, needs more than the %d distributed layers", len(m.LayersDescriptors))
			}
			blobDigest = m.LayersDescriptors[nonemptyLayerIndex].Digest
			nonemptyLayerIndex++
		}

		// AFAICT pull ignores these ID values, at least nowadays, so we could use anything unique, including a simple counter. Use what Docker uses for cargo-cult consistency.
		v, err := v1IDFromBlobDigestAndComponents(blobDigest, parentV1ID)
		if err != nil {
			return nil, err
		}
		v1ID = v

		fakeImage := v1Compatibility{
			ID:        v1ID,
			Parent:    parentV1ID,
			Comment:   historyEntry.Comment,
			Created:   historyEntry.Created,
			Author:    historyEntry.Author,
			ThrowAway: historyEntry.EmptyLayer,
		}
		fakeImage.ContainerConfig.Cmd = []string{historyEntry.CreatedBy}
		v1CompatibilityBytes, err := json.Marshal(&fakeImage)
		if err != nil {
			return nil, errors.Errorf("Internal error: Error creating v1compatibility for %#v", fakeImage)
		}

		fsLayers[v1Index] = fsLayersSchema1{BlobSum: blobDigest}
		history[v1Index] = historySchema1{V1Compatibility: string(v1CompatibilityBytes)}
		// Note that parentV1ID of the top layer is preserved when exiting this loop
	}

	// Now patch in real configuration for the top layer (v1Index == 0)
	v1ID, err = v1IDFromBlobDigestAndComponents(fsLayers[0].BlobSum, parentV1ID, string(configBytes)) // See above WRT v1ID value generation and cargo-cult consistency.
	if err != nil {
		return nil, err
	}
	v1Config, err := v1ConfigFromConfigJSON(configBytes, v1ID, parentV1ID, imageConfig.History[len(imageConfig.History)-1].EmptyLayer)
	if err != nil {
		return nil, err
	}
	history[0].V1Compatibility = string(v1Config)

	m1 := manifestSchema1FromComponents(dest.Reference().DockerReference(), fsLayers, history, imageConfig.Architecture)
	return memoryImageFromManifest(m1), nil
}

func v1IDFromBlobDigestAndComponents(blobDigest digest.Digest, others ...string) (string, error) {
	if err := blobDigest.Validate(); err != nil {
		return "", err
	}
	parts := append([]string{blobDigest.Hex()}, others...)
	v1IDHash := sha256.Sum256([]byte(strings.Join(parts, " ")))
	return hex.EncodeToString(v1IDHash[:]), nil
}

func v1ConfigFromConfigJSON(configJSON []byte, v1ID, parentV1ID string, throwaway bool) ([]byte, error) {
	// Preserve everything we don't specifically know about.
	// (This must be a *json.RawMessage, even though *[]byte is fairly redundant, because only *RawMessage implements json.Marshaler.)
	rawContents := map[string]*json.RawMessage{}
	if err := json.Unmarshal(configJSON, &rawContents); err != nil { // We have already unmarshaled it before, using a more detailed schema?!
		return nil, err
	}
	delete(rawContents, "rootfs")
	delete(rawContents, "history")

	updates := map[string]interface{}{"id": v1ID}
	if parentV1ID != "" {
		updates["parent"] = parentV1ID
	}
	if throwaway {
		updates["throwaway"] = throwaway
	}
	for field, value := range updates {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		rawContents[field] = (*json.RawMessage)(&encoded)
	}
	return json.Marshal(rawContents)
}
