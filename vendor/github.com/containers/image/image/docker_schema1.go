package image

import (
	"encoding/json"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type manifestSchema1 struct {
	m *manifest.Schema1
}

func manifestSchema1FromManifest(manifestBlob []byte) (genericManifest, error) {
	m, err := manifest.Schema1FromManifest(manifestBlob)
	if err != nil {
		return nil, err
	}
	return &manifestSchema1{m: m}, nil
}

// manifestSchema1FromComponents builds a new manifestSchema1 from the supplied data.
func manifestSchema1FromComponents(ref reference.Named, fsLayers []manifest.Schema1FSLayers, history []manifest.Schema1History, architecture string) genericManifest {
	return &manifestSchema1{m: manifest.Schema1FromComponents(ref, fsLayers, history, architecture)}
}

func (m *manifestSchema1) serialize() ([]byte, error) {
	return m.m.Serialize()
}

func (m *manifestSchema1) manifestMIMEType() string {
	return manifest.DockerV2Schema1SignedMediaType
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
// Note that the config object may not exist in the underlying storage in the return value of UpdatedImage! Use ConfigBlob() below.
func (m *manifestSchema1) ConfigInfo() types.BlobInfo {
	return m.m.ConfigInfo()
}

// ConfigBlob returns the blob described by ConfigInfo, iff ConfigInfo().Digest != ""; nil otherwise.
// The result is cached; it is OK to call this however often you need.
func (m *manifestSchema1) ConfigBlob() ([]byte, error) {
	return nil, nil
}

// OCIConfig returns the image configuration as per OCI v1 image-spec. Information about
// layers in the resulting configuration isn't guaranteed to be returned to due how
// old image manifests work (docker v2s1 especially).
func (m *manifestSchema1) OCIConfig() (*imgspecv1.Image, error) {
	v2s2, err := m.convertToManifestSchema2(nil, nil)
	if err != nil {
		return nil, err
	}
	return v2s2.OCIConfig()
}

// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *manifestSchema1) LayerInfos() []types.BlobInfo {
	return m.m.LayerInfos()
}

// EmbeddedDockerReferenceConflicts whether a Docker reference embedded in the manifest, if any, conflicts with destination ref.
// It returns false if the manifest does not embed a Docker reference.
// (This embedding unfortunately happens for Docker schema1, please do not add support for this in any new formats.)
func (m *manifestSchema1) EmbeddedDockerReferenceConflicts(ref reference.Named) bool {
	// This is a bit convoluted: We can’t just have a "get embedded docker reference" method
	// and have the “does it conflict” logic in the generic copy code, because the manifest does not actually
	// embed a full docker/distribution reference, but only the repo name and tag (without the host name).
	// So we would have to provide a “return repo without host name, and tag” getter for the generic code,
	// which would be very awkward.  Instead, we do the matching here in schema1-specific code, and all the
	// generic copy code needs to know about is reference.Named and that a manifest may need updating
	// for some destinations.
	name := reference.Path(ref)
	var tag string
	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		tag = tagged.Tag()
	} else {
		tag = ""
	}
	return m.m.Name != name || m.m.Tag != tag
}

func (m *manifestSchema1) imageInspectInfo() (*types.ImageInspectInfo, error) {
	v1 := &v1Image{}
	if err := json.Unmarshal([]byte(m.m.History[0].V1Compatibility), v1); err != nil {
		return nil, err
	}
	i := &types.ImageInspectInfo{
		Tag:           m.m.Tag,
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
func (m *manifestSchema1) UpdatedImageNeedsLayerDiffIDs(options types.ManifestUpdateOptions) bool {
	return options.ManifestMIMEType == manifest.DockerV2Schema2MediaType
}

// UpdatedImage returns a types.Image modified according to options.
// This does not change the state of the original Image object.
func (m *manifestSchema1) UpdatedImage(options types.ManifestUpdateOptions) (types.Image, error) {
	copy := manifestSchema1{m: manifest.Schema1Clone(m.m)}
	if options.LayerInfos != nil {
		if err := copy.m.UpdateLayerInfos(options.LayerInfos); err != nil {
			return nil, err
		}
	}
	if options.EmbeddedDockerReference != nil {
		copy.m.Name = reference.Path(options.EmbeddedDockerReference)
		if tagged, isTagged := options.EmbeddedDockerReference.(reference.NamedTagged); isTagged {
			copy.m.Tag = tagged.Tag()
		} else {
			copy.m.Tag = ""
		}
	}

	switch options.ManifestMIMEType {
	case "": // No conversion, OK
	case manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType:
	// We have 2 MIME types for schema 1, which are basically equivalent (even the un-"Signed" MIME type will be rejected if there isn’t a signature; so,
	// handle conversions between them by doing nothing.
	case manifest.DockerV2Schema2MediaType:
		return copy.convertToManifestSchema2(options.InformationOnly.LayerInfos, options.InformationOnly.LayerDiffIDs)
	default:
		return nil, errors.Errorf("Conversion of image manifest from %s to %s is not implemented", manifest.DockerV2Schema1SignedMediaType, options.ManifestMIMEType)
	}

	return memoryImageFromManifest(&copy), nil
}

// Based on github.com/docker/docker/distribution/pull_v2.go
func (m *manifestSchema1) convertToManifestSchema2(uploadedLayerInfos []types.BlobInfo, layerDiffIDs []digest.Digest) (types.Image, error) {
	if len(m.m.History) == 0 {
		// What would this even mean?! Anyhow, the rest of the code depends on fsLayers[0] and history[0] existing.
		return nil, errors.Errorf("Cannot convert an image with 0 history entries to %s", manifest.DockerV2Schema2MediaType)
	}
	if len(m.m.History) != len(m.m.FSLayers) {
		return nil, errors.Errorf("Inconsistent schema 1 manifest: %d history entries, %d fsLayers entries", len(m.m.History), len(m.m.FSLayers))
	}
	if uploadedLayerInfos != nil && len(uploadedLayerInfos) != len(m.m.FSLayers) {
		return nil, errors.Errorf("Internal error: uploaded %d blobs, but schema1 manifest has %d fsLayers", len(uploadedLayerInfos), len(m.m.FSLayers))
	}
	if layerDiffIDs != nil && len(layerDiffIDs) != len(m.m.FSLayers) {
		return nil, errors.Errorf("Internal error: collected %d DiffID values, but schema1 manifest has %d fsLayers", len(layerDiffIDs), len(m.m.FSLayers))
	}

	rootFS := rootFS{
		Type:      "layers",
		DiffIDs:   []digest.Digest{},
		BaseLayer: "",
	}
	var layers []manifest.Schema2Descriptor
	history := make([]imageHistory, len(m.m.History))
	for v1Index := len(m.m.History) - 1; v1Index >= 0; v1Index-- {
		v2Index := (len(m.m.History) - 1) - v1Index

		var v1compat manifest.Schema1V1Compatibility
		if err := json.Unmarshal([]byte(m.m.History[v1Index].V1Compatibility), &v1compat); err != nil {
			return nil, errors.Wrapf(err, "Error decoding history entry %d", v1Index)
		}
		history[v2Index] = imageHistory{
			Created:    v1compat.Created,
			Author:     v1compat.Author,
			CreatedBy:  strings.Join(v1compat.ContainerConfig.Cmd, " "),
			Comment:    v1compat.Comment,
			EmptyLayer: v1compat.ThrowAway,
		}

		if !v1compat.ThrowAway {
			var size int64
			if uploadedLayerInfos != nil {
				size = uploadedLayerInfos[v2Index].Size
			}
			var d digest.Digest
			if layerDiffIDs != nil {
				d = layerDiffIDs[v2Index]
			}
			layers = append(layers, manifest.Schema2Descriptor{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      size,
				Digest:    m.m.FSLayers[v1Index].BlobSum,
			})
			rootFS.DiffIDs = append(rootFS.DiffIDs, d)
		}
	}
	configJSON, err := configJSONFromV1Config([]byte(m.m.History[0].V1Compatibility), rootFS, history)
	if err != nil {
		return nil, err
	}
	configDescriptor := manifest.Schema2Descriptor{
		MediaType: "application/vnd.docker.container.image.v1+json",
		Size:      int64(len(configJSON)),
		Digest:    digest.FromBytes(configJSON),
	}

	m2 := manifestSchema2FromComponents(configDescriptor, nil, configJSON, layers)
	return memoryImageFromManifest(m2), nil
}

func configJSONFromV1Config(v1ConfigJSON []byte, rootFS rootFS, history []imageHistory) ([]byte, error) {
	// github.com/docker/docker/image/v1/imagev1.go:MakeConfigFromV1Config unmarshals and re-marshals the input if docker_version is < 1.8.3 to remove blank fields;
	// we don't do that here. FIXME? Should we? AFAICT it would only affect the digest value of the schema2 manifest, and we don't particularly need that to be
	// a consistently reproducible value.

	// Preserve everything we don't specifically know about.
	// (This must be a *json.RawMessage, even though *[]byte is fairly redundant, because only *RawMessage implements json.Marshaler.)
	rawContents := map[string]*json.RawMessage{}
	if err := json.Unmarshal(v1ConfigJSON, &rawContents); err != nil { // We have already unmarshaled it before, using a more detailed schema?!
		return nil, err
	}

	delete(rawContents, "id")
	delete(rawContents, "parent")
	delete(rawContents, "Size")
	delete(rawContents, "parent_id")
	delete(rawContents, "layer_id")
	delete(rawContents, "throwaway")

	updates := map[string]interface{}{"rootfs": rootFS, "history": history}
	for field, value := range updates {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		rawContents[field] = (*json.RawMessage)(&encoded)
	}
	return json.Marshal(rawContents)
}
