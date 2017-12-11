package image

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)
)

type fsLayersSchema1 struct {
	BlobSum digest.Digest `json:"blobSum"`
}

type historySchema1 struct {
	V1Compatibility string `json:"v1Compatibility"`
}

// historySchema1 is a string containing this.  It is similar to v1Image but not the same, in particular note the ThrowAway field.
type v1Compatibility struct {
	ID              string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	ContainerConfig struct {
		Cmd []string
	} `json:"container_config,omitempty"`
	Author    string `json:"author,omitempty"`
	ThrowAway bool   `json:"throwaway,omitempty"`
}

type manifestSchema1 struct {
	Name          string            `json:"name"`
	Tag           string            `json:"tag"`
	Architecture  string            `json:"architecture"`
	FSLayers      []fsLayersSchema1 `json:"fsLayers"`
	History       []historySchema1  `json:"history"`
	SchemaVersion int               `json:"schemaVersion"`
}

func manifestSchema1FromManifest(manifest []byte) (genericManifest, error) {
	mschema1 := &manifestSchema1{}
	if err := json.Unmarshal(manifest, mschema1); err != nil {
		return nil, err
	}
	if mschema1.SchemaVersion != 1 {
		return nil, errors.Errorf("unsupported schema version %d", mschema1.SchemaVersion)
	}
	if len(mschema1.FSLayers) != len(mschema1.History) {
		return nil, errors.New("length of history not equal to number of layers")
	}
	if len(mschema1.FSLayers) == 0 {
		return nil, errors.New("no FSLayers in manifest")
	}

	if err := fixManifestLayers(mschema1); err != nil {
		return nil, err
	}
	return mschema1, nil
}

// manifestSchema1FromComponents builds a new manifestSchema1 from the supplied data.
func manifestSchema1FromComponents(ref reference.Named, fsLayers []fsLayersSchema1, history []historySchema1, architecture string) genericManifest {
	var name, tag string
	if ref != nil { // Well, what to do if it _is_ nil? Most consumers actually don't use these fields nowadays, so we might as well try not supplying them.
		name = reference.Path(ref)
		if tagged, ok := ref.(reference.NamedTagged); ok {
			tag = tagged.Tag()
		}
	}
	return &manifestSchema1{
		Name:          name,
		Tag:           tag,
		Architecture:  architecture,
		FSLayers:      fsLayers,
		History:       history,
		SchemaVersion: 1,
	}
}

func (m *manifestSchema1) serialize() ([]byte, error) {
	// docker/distribution requires a signature even if the incoming data uses the nominally unsigned DockerV2Schema1MediaType.
	unsigned, err := json.Marshal(*m)
	if err != nil {
		return nil, err
	}
	return manifest.AddDummyV2S1Signature(unsigned)
}

func (m *manifestSchema1) manifestMIMEType() string {
	return manifest.DockerV2Schema1SignedMediaType
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
// Note that the config object may not exist in the underlying storage in the return value of UpdatedImage! Use ConfigBlob() below.
func (m *manifestSchema1) ConfigInfo() types.BlobInfo {
	return types.BlobInfo{}
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
	layers := make([]types.BlobInfo, len(m.FSLayers))
	for i, layer := range m.FSLayers { // NOTE: This includes empty layers (where m.History.V1Compatibility->ThrowAway)
		layers[(len(m.FSLayers)-1)-i] = types.BlobInfo{Digest: layer.BlobSum, Size: -1}
	}
	return layers
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
	return m.Name != name || m.Tag != tag
}

func (m *manifestSchema1) imageInspectInfo() (*types.ImageInspectInfo, error) {
	v1 := &v1Image{}
	if err := json.Unmarshal([]byte(m.History[0].V1Compatibility), v1); err != nil {
		return nil, err
	}
	i := &types.ImageInspectInfo{
		Tag:           m.Tag,
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
	copy := *m
	if options.LayerInfos != nil {
		// Our LayerInfos includes empty layers (where m.History.V1Compatibility->ThrowAway), so expect them to be included here as well.
		if len(copy.FSLayers) != len(options.LayerInfos) {
			return nil, errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(copy.FSLayers), len(options.LayerInfos))
		}
		for i, info := range options.LayerInfos {
			// (docker push) sets up m.History.V1Compatibility->{Id,Parent} based on values of info.Digest,
			// but (docker pull) ignores them in favor of computing DiffIDs from uncompressed data, except verifying the child->parent links and uniqueness.
			// So, we don't bother recomputing the IDs in m.History.V1Compatibility.
			copy.FSLayers[(len(options.LayerInfos)-1)-i].BlobSum = info.Digest
		}
	}
	if options.EmbeddedDockerReference != nil {
		copy.Name = reference.Path(options.EmbeddedDockerReference)
		if tagged, isTagged := options.EmbeddedDockerReference.(reference.NamedTagged); isTagged {
			copy.Tag = tagged.Tag()
		} else {
			copy.Tag = ""
		}
	}

	switch options.ManifestMIMEType {
	case "": // No conversion, OK
	case manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType:
	// We have 2 MIME types for schema 1, which are basically equivalent (even the un-"Signed" MIME type will be rejected if there isn’t a signature; so,
	// handle conversions between them by doing nothing.
	case manifest.DockerV2Schema2MediaType:
		m2, err := copy.convertToManifestSchema2(options.InformationOnly.LayerInfos, options.InformationOnly.LayerDiffIDs)
		if err != nil {
			return nil, err
		}
		return memoryImageFromManifest(m2), nil
	case imgspecv1.MediaTypeImageManifest:
		// We can't directly convert to OCI, but we can transitively convert via a Docker V2.2 Distribution manifest
		m2, err := copy.convertToManifestSchema2(options.InformationOnly.LayerInfos, options.InformationOnly.LayerDiffIDs)
		if err != nil {
			return nil, err
		}
		return m2.UpdatedImage(types.ManifestUpdateOptions{
			ManifestMIMEType: imgspecv1.MediaTypeImageManifest,
			InformationOnly:  options.InformationOnly,
		})
	default:
		return nil, errors.Errorf("Conversion of image manifest from %s to %s is not implemented", manifest.DockerV2Schema1SignedMediaType, options.ManifestMIMEType)
	}

	return memoryImageFromManifest(&copy), nil
}

// fixManifestLayers, after validating the supplied manifest
// (to use correctly-formatted IDs, and to not have non-consecutive ID collisions in manifest.History),
// modifies manifest to only have one entry for each layer ID in manifest.History (deleting the older duplicates,
// both from manifest.History and manifest.FSLayers).
// Note that even after this succeeds, manifest.FSLayers may contain duplicate entries
// (for Dockerfile operations which change the configuration but not the filesystem).
func fixManifestLayers(manifest *manifestSchema1) error {
	type imageV1 struct {
		ID     string
		Parent string
	}
	// Per the specification, we can assume that len(manifest.FSLayers) == len(manifest.History)
	imgs := make([]*imageV1, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		img := &imageV1{}

		if err := json.Unmarshal([]byte(manifest.History[i].V1Compatibility), img); err != nil {
			return err
		}

		imgs[i] = img
		if err := validateV1ID(img.ID); err != nil {
			return err
		}
	}
	if imgs[len(imgs)-1].Parent != "" {
		return errors.New("Invalid parent ID in the base layer of the image")
	}
	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})
	var lastID string
	for _, img := range imgs {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return errors.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}
	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(imgs) - 2; i >= 0; i-- {
		if imgs[i].ID == imgs[i+1].ID { // repeated ID. remove and continue
			manifest.FSLayers = append(manifest.FSLayers[:i], manifest.FSLayers[i+1:]...)
			manifest.History = append(manifest.History[:i], manifest.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return errors.Errorf("Invalid parent ID. Expected %v, got %v", imgs[i+1].ID, imgs[i].Parent)
		}
	}
	return nil
}

func validateV1ID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return errors.Errorf("image ID %q is invalid", id)
	}
	return nil
}

// Based on github.com/docker/docker/distribution/pull_v2.go
func (m *manifestSchema1) convertToManifestSchema2(uploadedLayerInfos []types.BlobInfo, layerDiffIDs []digest.Digest) (genericManifest, error) {
	if len(m.History) == 0 {
		// What would this even mean?! Anyhow, the rest of the code depends on fsLayers[0] and history[0] existing.
		return nil, errors.Errorf("Cannot convert an image with 0 history entries to %s", manifest.DockerV2Schema2MediaType)
	}
	if len(m.History) != len(m.FSLayers) {
		return nil, errors.Errorf("Inconsistent schema 1 manifest: %d history entries, %d fsLayers entries", len(m.History), len(m.FSLayers))
	}
	if uploadedLayerInfos != nil && len(uploadedLayerInfos) != len(m.FSLayers) {
		return nil, errors.Errorf("Internal error: uploaded %d blobs, but schema1 manifest has %d fsLayers", len(uploadedLayerInfos), len(m.FSLayers))
	}
	if layerDiffIDs != nil && len(layerDiffIDs) != len(m.FSLayers) {
		return nil, errors.Errorf("Internal error: collected %d DiffID values, but schema1 manifest has %d fsLayers", len(layerDiffIDs), len(m.FSLayers))
	}

	rootFS := rootFS{
		Type:      "layers",
		DiffIDs:   []digest.Digest{},
		BaseLayer: "",
	}
	var layers []descriptor
	history := make([]imageHistory, len(m.History))
	for v1Index := len(m.History) - 1; v1Index >= 0; v1Index-- {
		v2Index := (len(m.History) - 1) - v1Index

		var v1compat v1Compatibility
		if err := json.Unmarshal([]byte(m.History[v1Index].V1Compatibility), &v1compat); err != nil {
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
			layers = append(layers, descriptor{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      size,
				Digest:    m.FSLayers[v1Index].BlobSum,
			})
			rootFS.DiffIDs = append(rootFS.DiffIDs, d)
		}
	}
	configJSON, err := configJSONFromV1Config([]byte(m.History[0].V1Compatibility), rootFS, history)
	if err != nil {
		return nil, err
	}
	configDescriptor := descriptor{
		MediaType: "application/vnd.docker.container.image.v1+json",
		Size:      int64(len(configJSON)),
		Digest:    digest.FromBytes(configJSON),
	}

	return manifestSchema2FromComponents(configDescriptor, nil, configJSON, layers), nil
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
