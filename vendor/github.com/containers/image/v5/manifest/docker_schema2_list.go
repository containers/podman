package manifest

import (
	"encoding/json"
	"fmt"

	platform "github.com/containers/image/v5/internal/pkg/platform"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Schema2PlatformSpec describes the platform which a particular manifest is
// specialized for.
type Schema2PlatformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"` // removed in OCI
}

// Schema2ManifestDescriptor references a platform-specific manifest.
type Schema2ManifestDescriptor struct {
	Schema2Descriptor
	Platform Schema2PlatformSpec `json:"platform"`
}

// Schema2List is a list of platform-specific manifests.
type Schema2List struct {
	SchemaVersion int                         `json:"schemaVersion"`
	MediaType     string                      `json:"mediaType"`
	Manifests     []Schema2ManifestDescriptor `json:"manifests"`
}

// MIMEType returns the MIME type of this particular manifest list.
func (list *Schema2List) MIMEType() string {
	return list.MediaType
}

// Instances returns a slice of digests of the manifests that this list knows of.
func (list *Schema2List) Instances() []digest.Digest {
	results := make([]digest.Digest, len(list.Manifests))
	for i, m := range list.Manifests {
		results[i] = m.Digest
	}
	return results
}

// Instance returns the ListUpdate of a particular instance in the list.
func (list *Schema2List) Instance(instanceDigest digest.Digest) (ListUpdate, error) {
	for _, manifest := range list.Manifests {
		if manifest.Digest == instanceDigest {
			return ListUpdate{
				Digest:    manifest.Digest,
				Size:      manifest.Size,
				MediaType: manifest.MediaType,
			}, nil
		}
	}
	return ListUpdate{}, errors.Errorf("unable to find instance %s passed to Schema2List.Instances", instanceDigest)
}

// UpdateInstances updates the sizes, digests, and media types of the manifests
// which the list catalogs.
func (list *Schema2List) UpdateInstances(updates []ListUpdate) error {
	if len(updates) != len(list.Manifests) {
		return errors.Errorf("incorrect number of update entries passed to Schema2List.UpdateInstances: expected %d, got %d", len(list.Manifests), len(updates))
	}
	for i := range updates {
		if err := updates[i].Digest.Validate(); err != nil {
			return errors.Wrapf(err, "update %d of %d passed to Schema2List.UpdateInstances contained an invalid digest", i+1, len(updates))
		}
		list.Manifests[i].Digest = updates[i].Digest
		if updates[i].Size < 0 {
			return errors.Errorf("update %d of %d passed to Schema2List.UpdateInstances had an invalid size (%d)", i+1, len(updates), updates[i].Size)
		}
		list.Manifests[i].Size = updates[i].Size
		if updates[i].MediaType == "" {
			return errors.Errorf("update %d of %d passed to Schema2List.UpdateInstances had no media type (was %q)", i+1, len(updates), list.Manifests[i].MediaType)
		}
		list.Manifests[i].MediaType = updates[i].MediaType
	}
	return nil
}

// ChooseInstance parses blob as a schema2 manifest list, and returns the digest
// of the image which is appropriate for the current environment.
func (list *Schema2List) ChooseInstance(ctx *types.SystemContext) (digest.Digest, error) {
	wantedPlatforms, err := platform.WantedPlatforms(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "getting platform information %#v", ctx)
	}
	for _, wantedPlatform := range wantedPlatforms {
		for _, d := range list.Manifests {
			imagePlatform := imgspecv1.Platform{
				Architecture: d.Platform.Architecture,
				OS:           d.Platform.OS,
				OSVersion:    d.Platform.OSVersion,
				OSFeatures:   dupStringSlice(d.Platform.OSFeatures),
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
func (list *Schema2List) Serialize() ([]byte, error) {
	buf, err := json.Marshal(list)
	if err != nil {
		return nil, errors.Wrapf(err, "marshaling Schema2List %#v", list)
	}
	return buf, nil
}

// Schema2ListFromComponents creates a Schema2 manifest list instance from the
// supplied data.
func Schema2ListFromComponents(components []Schema2ManifestDescriptor) *Schema2List {
	list := Schema2List{
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
				URLs:      dupStringSlice(component.URLs),
			},
			Schema2PlatformSpec{
				Architecture: component.Platform.Architecture,
				OS:           component.Platform.OS,
				OSVersion:    component.Platform.OSVersion,
				OSFeatures:   dupStringSlice(component.Platform.OSFeatures),
				Variant:      component.Platform.Variant,
				Features:     dupStringSlice(component.Platform.Features),
			},
		}
		list.Manifests[i] = m
	}
	return &list
}

// Schema2ListClone creates a deep copy of the passed-in list.
func Schema2ListClone(list *Schema2List) *Schema2List {
	return Schema2ListFromComponents(list.Manifests)
}

// ToOCI1Index returns the list encoded as an OCI1 index.
func (list *Schema2List) ToOCI1Index() (*OCI1Index, error) {
	components := make([]imgspecv1.Descriptor, 0, len(list.Manifests))
	for _, manifest := range list.Manifests {
		converted := imgspecv1.Descriptor{
			MediaType: manifest.MediaType,
			Size:      manifest.Size,
			Digest:    manifest.Digest,
			URLs:      dupStringSlice(manifest.URLs),
			Platform: &imgspecv1.Platform{
				OS:           manifest.Platform.OS,
				Architecture: manifest.Platform.Architecture,
				OSFeatures:   dupStringSlice(manifest.Platform.OSFeatures),
				OSVersion:    manifest.Platform.OSVersion,
				Variant:      manifest.Platform.Variant,
			},
		}
		components = append(components, converted)
	}
	oci := OCI1IndexFromComponents(components, nil)
	return oci, nil
}

// ToSchema2List returns the list encoded as a Schema2 list.
func (list *Schema2List) ToSchema2List() (*Schema2List, error) {
	return Schema2ListClone(list), nil
}

// Schema2ListFromManifest creates a Schema2 manifest list instance from marshalled
// JSON, presumably generated by encoding a Schema2 manifest list.
func Schema2ListFromManifest(manifest []byte) (*Schema2List, error) {
	list := Schema2List{
		Manifests: []Schema2ManifestDescriptor{},
	}
	if err := json.Unmarshal(manifest, &list); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling Schema2List %q", string(manifest))
	}
	if err := validateUnambiguousManifestFormat(manifest, DockerV2ListMediaType,
		allowedFieldManifests); err != nil {
		return nil, err
	}
	return &list, nil
}

// Clone returns a deep copy of this list and its contents.
func (list *Schema2List) Clone() List {
	return Schema2ListClone(list)
}

// ConvertToMIMEType converts the passed-in manifest list to a manifest
// list of the specified type.
func (list *Schema2List) ConvertToMIMEType(manifestMIMEType string) (List, error) {
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
