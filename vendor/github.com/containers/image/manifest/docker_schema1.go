package manifest

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Schema1FSLayers is an entry of the "fsLayers" array in docker/distribution schema 1.
type Schema1FSLayers struct {
	BlobSum digest.Digest `json:"blobSum"`
}

// Schema1History is an entry of the "history" array in docker/distribution schema 1.
type Schema1History struct {
	V1Compatibility string `json:"v1Compatibility"`
}

// Schema1 is a manifest in docker/distribution schema 1.
type Schema1 struct {
	Name          string            `json:"name"`
	Tag           string            `json:"tag"`
	Architecture  string            `json:"architecture"`
	FSLayers      []Schema1FSLayers `json:"fsLayers"`
	History       []Schema1History  `json:"history"`
	SchemaVersion int               `json:"schemaVersion"`
}

// Schema1V1Compatibility is a v1Compatibility in docker/distribution schema 1.
type Schema1V1Compatibility struct {
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

// Schema1FromManifest creates a Schema1 manifest instance from a manifest blob.
// (NOTE: The instance is not necessary a literal representation of the original blob,
// layers with duplicate IDs are eliminated.)
func Schema1FromManifest(manifest []byte) (*Schema1, error) {
	s1 := Schema1{}
	if err := json.Unmarshal(manifest, &s1); err != nil {
		return nil, err
	}
	if s1.SchemaVersion != 1 {
		return nil, errors.Errorf("unsupported schema version %d", s1.SchemaVersion)
	}
	if len(s1.FSLayers) != len(s1.History) {
		return nil, errors.New("length of history not equal to number of layers")
	}
	if len(s1.FSLayers) == 0 {
		return nil, errors.New("no FSLayers in manifest")
	}
	if err := s1.fixManifestLayers(); err != nil {
		return nil, err
	}
	return &s1, nil
}

// Schema1FromComponents creates an Schema1 manifest instance from the supplied data.
func Schema1FromComponents(ref reference.Named, fsLayers []Schema1FSLayers, history []Schema1History, architecture string) *Schema1 {
	var name, tag string
	if ref != nil { // Well, what to do if it _is_ nil? Most consumers actually don't use these fields nowadays, so we might as well try not supplying them.
		name = reference.Path(ref)
		if tagged, ok := ref.(reference.NamedTagged); ok {
			tag = tagged.Tag()
		}
	}
	return &Schema1{
		Name:          name,
		Tag:           tag,
		Architecture:  architecture,
		FSLayers:      fsLayers,
		History:       history,
		SchemaVersion: 1,
	}
}

// Schema1Clone creates a copy of the supplied Schema1 manifest.
func Schema1Clone(src *Schema1) *Schema1 {
	copy := *src
	return &copy
}

// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
func (m *Schema1) ConfigInfo() types.BlobInfo {
	return types.BlobInfo{}
}

// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (m *Schema1) LayerInfos() []types.BlobInfo {
	layers := make([]types.BlobInfo, len(m.FSLayers))
	for i, layer := range m.FSLayers { // NOTE: This includes empty layers (where m.History.V1Compatibility->ThrowAway)
		layers[(len(m.FSLayers)-1)-i] = types.BlobInfo{Digest: layer.BlobSum, Size: -1}
	}
	return layers
}

// UpdateLayerInfos replaces the original layers with the specified BlobInfos (size+digest+urls), in order (the root layer first, and then successive layered layers)
func (m *Schema1) UpdateLayerInfos(layerInfos []types.BlobInfo) error {
	// Our LayerInfos includes empty layers (where m.History.V1Compatibility->ThrowAway), so expect them to be included here as well.
	if len(m.FSLayers) != len(layerInfos) {
		return errors.Errorf("Error preparing updated manifest: layer count changed from %d to %d", len(m.FSLayers), len(layerInfos))
	}
	for i, info := range layerInfos {
		// (docker push) sets up m.History.V1Compatibility->{Id,Parent} based on values of info.Digest,
		// but (docker pull) ignores them in favor of computing DiffIDs from uncompressed data, except verifying the child->parent links and uniqueness.
		// So, we don't bother recomputing the IDs in m.History.V1Compatibility.
		m.FSLayers[(len(layerInfos)-1)-i].BlobSum = info.Digest
	}
	return nil
}

// Serialize returns the manifest in a blob format.
// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded from one, even if no modifications were made!
func (m *Schema1) Serialize() ([]byte, error) {
	// docker/distribution requires a signature even if the incoming data uses the nominally unsigned DockerV2Schema1MediaType.
	unsigned, err := json.Marshal(*m)
	if err != nil {
		return nil, err
	}
	return AddDummyV2S1Signature(unsigned)
}

// fixManifestLayers, after validating the supplied manifest
// (to use correctly-formatted IDs, and to not have non-consecutive ID collisions in m.History),
// modifies manifest to only have one entry for each layer ID in m.History (deleting the older duplicates,
// both from m.History and m.FSLayers).
// Note that even after this succeeds, m.FSLayers may contain duplicate entries
// (for Dockerfile operations which change the configuration but not the filesystem).
func (m *Schema1) fixManifestLayers() error {
	type imageV1 struct {
		ID     string
		Parent string
	}
	// Per the specification, we can assume that len(m.FSLayers) == len(m.History)
	imgs := make([]*imageV1, len(m.FSLayers))
	for i := range m.FSLayers {
		img := &imageV1{}

		if err := json.Unmarshal([]byte(m.History[i].V1Compatibility), img); err != nil {
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
			m.FSLayers = append(m.FSLayers[:i], m.FSLayers[i+1:]...)
			m.History = append(m.History[:i], m.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return errors.Errorf("Invalid parent ID. Expected %v, got %v", imgs[i+1].ID, imgs[i].Parent)
		}
	}
	return nil
}

var validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)

func validateV1ID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return errors.Errorf("image ID %q is invalid", id)
	}
	return nil
}

// Inspect returns various information for (skopeo inspect) parsed from the manifest and configuration.
func (m *Schema1) Inspect(_ func(types.BlobInfo) ([]byte, error)) (*types.ImageInspectInfo, error) {
	s1 := &Schema2V1Image{}
	if err := json.Unmarshal([]byte(m.History[0].V1Compatibility), s1); err != nil {
		return nil, err
	}
	return &types.ImageInspectInfo{
		Tag:           m.Tag,
		Created:       s1.Created,
		DockerVersion: s1.DockerVersion,
		Labels:        make(map[string]string),
		Architecture:  s1.Architecture,
		Os:            s1.OS,
		Layers:        LayerInfosToStrings(m.LayerInfos()),
	}, nil
}

// ToSchema2 builds a schema2-style configuration blob using the supplied diffIDs.
func (m *Schema1) ToSchema2(diffIDs []digest.Digest) ([]byte, error) {
	// Convert the schema 1 compat info into a schema 2 config, constructing some of the fields
	// that aren't directly comparable using info from the manifest.
	if len(m.History) == 0 {
		return nil, errors.New("image has no layers")
	}
	s2 := struct {
		Schema2Image
		ID        string `json:"id,omitempty"`
		Parent    string `json:"parent,omitempty"`
		ParentID  string `json:"parent_id,omitempty"`
		LayerID   string `json:"layer_id,omitempty"`
		ThrowAway bool   `json:"throwaway,omitempty"`
		Size      int64  `json:",omitempty"`
	}{}
	config := []byte(m.History[0].V1Compatibility)
	err := json.Unmarshal(config, &s2)
	if err != nil {
		return nil, errors.Wrapf(err, "error decoding configuration")
	}
	// Images created with versions prior to 1.8.3 require us to re-encode the encoded object,
	// adding some fields that aren't "omitempty".
	if s2.DockerVersion != "" && versions.LessThan(s2.DockerVersion, "1.8.3") {
		config, err = json.Marshal(&s2)
		if err != nil {
			return nil, errors.Wrapf(err, "error re-encoding compat image config %#v", s2)
		}
	}
	// Build the history.
	convertedHistory := []Schema2History{}
	for _, h := range m.History {
		compat := Schema1V1Compatibility{}
		if err := json.Unmarshal([]byte(h.V1Compatibility), &compat); err != nil {
			return nil, errors.Wrapf(err, "error decoding history information")
		}
		hitem := Schema2History{
			Created:    compat.Created,
			CreatedBy:  strings.Join(compat.ContainerConfig.Cmd, " "),
			Author:     compat.Author,
			Comment:    compat.Comment,
			EmptyLayer: compat.ThrowAway,
		}
		convertedHistory = append([]Schema2History{hitem}, convertedHistory...)
	}
	// Build the rootfs information.  We need the decompressed sums that we've been
	// calculating to fill in the DiffIDs.  It's expected (but not enforced by us)
	// that the number of diffIDs corresponds to the number of non-EmptyLayer
	// entries in the history.
	rootFS := &Schema2RootFS{
		Type:    "layers",
		DiffIDs: diffIDs,
	}
	// And now for some raw manipulation.
	raw := make(map[string]*json.RawMessage)
	err = json.Unmarshal(config, &raw)
	if err != nil {
		return nil, errors.Wrapf(err, "error re-decoding compat image config %#v: %v", s2)
	}
	// Drop some fields.
	delete(raw, "id")
	delete(raw, "parent")
	delete(raw, "parent_id")
	delete(raw, "layer_id")
	delete(raw, "throwaway")
	delete(raw, "Size")
	// Add the history and rootfs information.
	rootfs, err := json.Marshal(rootFS)
	if err != nil {
		return nil, errors.Errorf("error encoding rootfs information %#v: %v", rootFS, err)
	}
	rawRootfs := json.RawMessage(rootfs)
	raw["rootfs"] = &rawRootfs
	history, err := json.Marshal(convertedHistory)
	if err != nil {
		return nil, errors.Errorf("error encoding history information %#v: %v", convertedHistory, err)
	}
	rawHistory := json.RawMessage(history)
	raw["history"] = &rawHistory
	// Encode the result.
	config, err = json.Marshal(raw)
	if err != nil {
		return nil, errors.Errorf("error re-encoding compat image config %#v: %v", s2, err)
	}
	return config, nil
}

// ImageID computes an ID which can uniquely identify this image by its contents.
func (m *Schema1) ImageID(diffIDs []digest.Digest) (string, error) {
	image, err := m.ToSchema2(diffIDs)
	if err != nil {
		return "", err
	}
	return digest.FromBytes(image).Hex(), nil
}
