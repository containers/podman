package manifest

import (
	"encoding/json"
	"fmt"

	platform "github.com/containers/image/v5/internal/pkg/platform"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
)

// Schema2PlatformSpec describes the platform which a particular manifest is
// specialized for.
// This is publicly visible as c/image/manifest.Schema2PlatformSpec.
type Schema2PlatformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"` // removed in OCI
}

// Schema2ManifestDescriptor references a platform-specific manifest.
// This is publicly visible as c/image/manifest.Schema2ManifestDescriptor.
type Schema2ManifestDescriptor struct {
	Schema2Descriptor
	Platform Schema2PlatformSpec `json:"platform"`
}

// Schema2ListPublic is a list of platform-specific manifests.
// This is publicly visible as c/image/manifest.Schema2List.
// Internal users should usually use Schema2List instead.
type Schema2ListPublic struct {
	SchemaVersion int                         `json:"schemaVersion"`
	MediaType     string                      `json:"mediaType"`
	Manifests     []Schema2ManifestDescriptor `json:"manifests"`
}

// MIMEType returns the MIME type of this particular manifest list.
func (list *Schema2ListPublic) MIMEType() string {
	return list.MediaType
}

// Instances returns a slice of digests of the manifests that this list knows of.
func (list *Schema2ListPublic) Instances() []digest.Digest {
	results := make([]digest.Digest, len(list.Manifests))
	for i, m := range list.Manifests {
		results[i] = m.Digest
	}
	return results
}

// Instance returns the ListUpdate of a particular instance in the list.
func (list *Schema2ListPublic) Instance(instanceDigest digest.Digest) (ListUpdate, error) {
	for _, manifest := range list.Manifests {
		if manifest.Digest == instanceDigest {
			return ListUpdate{
				Digest:    manifest.Digest,
				Size:      manifest.Size,
				MediaType: manifest.MediaType,
			}, nil
		}
	}
	return ListUpdate{}, fmt.Errorf("unable to find instance %s passed to Schema2List.Instances", instanceDigest)
}

// UpdateInstances updates the sizes, digests, and media types of the manifests
// which the list catalogs.
func (index *Schema2ListPublic) UpdateInstances(updates []ListUpdate) error {
	editInstances := []ListEdit{}
	for i, instance := range updates {
		editInstances = append(editInstances, ListEdit{
			UpdateOldDigest: index.Manifests[i].Digest,
			UpdateDigest:    instance.Digest,
			UpdateSize:      instance.Size,
			UpdateMediaType: instance.MediaType,
			ListOperation:   ListOpUpdate})
	}
	return index.editInstances(editInstances)
}

func (index *Schema2ListPublic) editInstances(editInstances []ListEdit) error {
	addedEntries := []Schema2ManifestDescriptor{}
	for i, editInstance := range editInstances {
		switch editInstance.ListOperation {
		case ListOpUpdate:
			if err := editInstance.UpdateOldDigest.Validate(); err != nil {
				return fmt.Errorf("Schema2List.EditInstances: Attempting to update %s which is an invalid digest: %w", editInstance.UpdateOldDigest, err)
			}
			if err := editInstance.UpdateDigest.Validate(); err != nil {
				return fmt.Errorf("Schema2List.EditInstances: Modified digest %s is an invalid digest: %w", editInstance.UpdateDigest, err)
			}
			targetIndex := slices.IndexFunc(index.Manifests, func(m Schema2ManifestDescriptor) bool {
				return m.Digest == editInstance.UpdateOldDigest
			})
			if targetIndex == -1 {
				return fmt.Errorf("Schema2List.EditInstances: digest %s not found", editInstance.UpdateOldDigest)
			}
			index.Manifests[targetIndex].Digest = editInstance.UpdateDigest
			if editInstance.UpdateSize < 0 {
				return fmt.Errorf("update %d of %d passed to Schema2List.UpdateInstances had an invalid size (%d)", i+1, len(editInstances), editInstance.UpdateSize)
			}
			index.Manifests[targetIndex].Size = editInstance.UpdateSize
			if editInstance.UpdateMediaType == "" {
				return fmt.Errorf("update %d of %d passed to Schema2List.UpdateInstances had no media type (was %q)", i+1, len(editInstances), index.Manifests[i].MediaType)
			}
			index.Manifests[targetIndex].MediaType = editInstance.UpdateMediaType
		case ListOpAdd:
			addInstance := Schema2ManifestDescriptor{
				Schema2Descriptor{Digest: editInstance.AddDigest, Size: editInstance.AddSize, MediaType: editInstance.AddMediaType},
				Schema2PlatformSpec{
					OS:           editInstance.AddPlatform.OS,
					Architecture: editInstance.AddPlatform.Architecture,
					OSVersion:    editInstance.AddPlatform.OSVersion,
					OSFeatures:   editInstance.AddPlatform.OSFeatures,
					Variant:      editInstance.AddPlatform.Variant,
				},
			}
			addedEntries = append(addedEntries, addInstance)
		default:
			return fmt.Errorf("internal error: invalid operation: %d", editInstance.ListOperation)
		}
	}
	if len(addedEntries) != 0 {
		index.Manifests = append(index.Manifests, addedEntries...)
	}
	return nil
}

func (index *Schema2List) EditInstances(editInstances []ListEdit) error {
	return index.editInstances(editInstances)
}

func (list *Schema2ListPublic) ChooseInstanceByCompression(ctx *types.SystemContext, preferGzip types.OptionalBool) (digest.Digest, error) {
	// ChooseInstanceByCompression is same as ChooseInstance for schema2 manifest list.
	return list.ChooseInstance(ctx)
}

// ChooseInstance parses blob as a schema2 manifest list, and returns the digest
// of the image which is appropriate for the current environment.
func (list *Schema2ListPublic) ChooseInstance(ctx *types.SystemContext) (digest.Digest, error) {
	wantedPlatforms, err := platform.WantedPlatforms(ctx)
	if err != nil {
		return "", fmt.Errorf("getting platform information %#v: %w", ctx, err)
	}
	for _, wantedPlatform := range wantedPlatforms {
		for _, d := range list.Manifests {
			imagePlatform := imgspecv1.Platform{
				Architecture: d.Platform.Architecture,
				OS:           d.Platform.OS,
				OSVersion:    d.Platform.OSVersion,
				OSFeatures:   slices.Clone(d.Platform.OSFeatures),
				Variant:      d.Platform.Variant,
			}
			if platform.MatchesPlatform(imagePlatform, wantedPlatform) {
				return d.Digest, nil
			}
		}
	}
	return "", fmt.Errorf("no image found in manifest list for architecture %s, variant %q, OS %s", wantedPlatforms[0].Architecture, wantedPlatforms[0].Variant, wantedPlatforms[0].OS)
}

// Serialize returns the list in a blob format.
// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded from one, even if no modifications were made!
func (list *Schema2ListPublic) Serialize() ([]byte, error) {
	buf, err := json.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("marshaling Schema2List %#v: %w", list, err)
	}
	return buf, nil
}

// Schema2ListPublicFromComponents creates a Schema2 manifest list instance from the
// supplied data.
// This is publicly visible as c/image/manifest.Schema2ListFromComponents.
func Schema2ListPublicFromComponents(components []Schema2ManifestDescriptor) *Schema2ListPublic {
	list := Schema2ListPublic{
		SchemaVersion: 2,
		MediaType:     DockerV2ListMediaType,
		Manifests:     make([]Schema2ManifestDescriptor, len(components)),
	}
	for i, component := range components {
		m := Schema2ManifestDescriptor{
			Schema2Descriptor{
				MediaType: component.MediaType,
				Size:      component.Size,
				Digest:    component.Digest,
				URLs:      slices.Clone(component.URLs),
			},
			Schema2PlatformSpec{
				Architecture: component.Platform.Architecture,
				OS:           component.Platform.OS,
				OSVersion:    component.Platform.OSVersion,
				OSFeatures:   slices.Clone(component.Platform.OSFeatures),
				Variant:      component.Platform.Variant,
				Features:     slices.Clone(component.Platform.Features),
			},
		}
		list.Manifests[i] = m
	}
	return &list
}

// Schema2ListPublicClone creates a deep copy of the passed-in list.
// This is publicly visible as c/image/manifest.Schema2ListClone.
func Schema2ListPublicClone(list *Schema2ListPublic) *Schema2ListPublic {
	return Schema2ListPublicFromComponents(list.Manifests)
}

// ToOCI1Index returns the list encoded as an OCI1 index.
func (list *Schema2ListPublic) ToOCI1Index() (*OCI1IndexPublic, error) {
	components := make([]imgspecv1.Descriptor, 0, len(list.Manifests))
	for _, manifest := range list.Manifests {
		converted := imgspecv1.Descriptor{
			MediaType: manifest.MediaType,
			Size:      manifest.Size,
			Digest:    manifest.Digest,
			URLs:      slices.Clone(manifest.URLs),
			Platform: &imgspecv1.Platform{
				OS:           manifest.Platform.OS,
				Architecture: manifest.Platform.Architecture,
				OSFeatures:   slices.Clone(manifest.Platform.OSFeatures),
				OSVersion:    manifest.Platform.OSVersion,
				Variant:      manifest.Platform.Variant,
			},
		}
		components = append(components, converted)
	}
	oci := OCI1IndexPublicFromComponents(components, nil)
	return oci, nil
}

// ToSchema2List returns the list encoded as a Schema2 list.
func (list *Schema2ListPublic) ToSchema2List() (*Schema2ListPublic, error) {
	return Schema2ListPublicClone(list), nil
}

// Schema2ListPublicFromManifest creates a Schema2 manifest list instance from marshalled
// JSON, presumably generated by encoding a Schema2 manifest list.
// This is publicly visible as c/image/manifest.Schema2ListFromManifest.
func Schema2ListPublicFromManifest(manifest []byte) (*Schema2ListPublic, error) {
	list := Schema2ListPublic{
		Manifests: []Schema2ManifestDescriptor{},
	}
	if err := json.Unmarshal(manifest, &list); err != nil {
		return nil, fmt.Errorf("unmarshaling Schema2List %q: %w", string(manifest), err)
	}
	if err := ValidateUnambiguousManifestFormat(manifest, DockerV2ListMediaType,
		AllowedFieldManifests); err != nil {
		return nil, err
	}
	return &list, nil
}

// Clone returns a deep copy of this list and its contents.
func (list *Schema2ListPublic) Clone() ListPublic {
	return Schema2ListPublicClone(list)
}

// ConvertToMIMEType converts the passed-in manifest list to a manifest
// list of the specified type.
func (list *Schema2ListPublic) ConvertToMIMEType(manifestMIMEType string) (ListPublic, error) {
	switch normalized := NormalizedMIMEType(manifestMIMEType); normalized {
	case DockerV2ListMediaType:
		return list.Clone(), nil
	case imgspecv1.MediaTypeImageIndex:
		return list.ToOCI1Index()
	case DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType, imgspecv1.MediaTypeImageManifest, DockerV2Schema2MediaType:
		return nil, fmt.Errorf("Can not convert manifest list to MIME type %q, which is not a list type", manifestMIMEType)
	default:
		// Note that this may not be reachable, NormalizedMIMEType has a default for unknown values.
		return nil, fmt.Errorf("Unimplemented manifest list MIME type %s", manifestMIMEType)
	}
}

// Schema2List is a list of platform-specific manifests.
type Schema2List struct {
	Schema2ListPublic
}

func schema2ListFromPublic(public *Schema2ListPublic) *Schema2List {
	return &Schema2List{*public}
}

func (index *Schema2List) CloneInternal() List {
	return schema2ListFromPublic(Schema2ListPublicClone(&index.Schema2ListPublic))
}

func (index *Schema2List) Clone() ListPublic {
	return index.CloneInternal()
}

// Schema2ListFromManifest creates a Schema2 manifest list instance from marshalled
// JSON, presumably generated by encoding a Schema2 manifest list.
func Schema2ListFromManifest(manifest []byte) (*Schema2List, error) {
	public, err := Schema2ListPublicFromManifest(manifest)
	if err != nil {
		return nil, err
	}
	return schema2ListFromPublic(public), nil
}
