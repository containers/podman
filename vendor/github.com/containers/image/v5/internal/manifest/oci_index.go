package manifest

import (
	"encoding/json"
	"fmt"
	"runtime"

	platform "github.com/containers/image/v5/internal/pkg/platform"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// OCI1IndexPublic is just an alias for the OCI index type, but one which we can
// provide methods for.
// This is publicly visible as c/image/manifest.OCI1Index
// Internal users should usually use OCI1Index instead.
type OCI1IndexPublic struct {
	imgspecv1.Index
}

// MIMEType returns the MIME type of this particular manifest index.
func (index *OCI1IndexPublic) MIMEType() string {
	return imgspecv1.MediaTypeImageIndex
}

// Instances returns a slice of digests of the manifests that this index knows of.
func (index *OCI1IndexPublic) Instances() []digest.Digest {
	results := make([]digest.Digest, len(index.Manifests))
	for i, m := range index.Manifests {
		results[i] = m.Digest
	}
	return results
}

// Instance returns the ListUpdate of a particular instance in the index.
func (index *OCI1IndexPublic) Instance(instanceDigest digest.Digest) (ListUpdate, error) {
	for _, manifest := range index.Manifests {
		if manifest.Digest == instanceDigest {
			return ListUpdate{
				Digest:    manifest.Digest,
				Size:      manifest.Size,
				MediaType: manifest.MediaType,
			}, nil
		}
	}
	return ListUpdate{}, fmt.Errorf("unable to find instance %s in OCI1Index", instanceDigest)
}

// UpdateInstances updates the sizes, digests, and media types of the manifests
// which the list catalogs.
func (index *OCI1IndexPublic) UpdateInstances(updates []ListUpdate) error {
	if len(updates) != len(index.Manifests) {
		return fmt.Errorf("incorrect number of update entries passed to OCI1Index.UpdateInstances: expected %d, got %d", len(index.Manifests), len(updates))
	}
	for i := range updates {
		if err := updates[i].Digest.Validate(); err != nil {
			return fmt.Errorf("update %d of %d passed to OCI1Index.UpdateInstances contained an invalid digest: %w", i+1, len(updates), err)
		}
		index.Manifests[i].Digest = updates[i].Digest
		if updates[i].Size < 0 {
			return fmt.Errorf("update %d of %d passed to OCI1Index.UpdateInstances had an invalid size (%d)", i+1, len(updates), updates[i].Size)
		}
		index.Manifests[i].Size = updates[i].Size
		if updates[i].MediaType == "" {
			return fmt.Errorf("update %d of %d passed to OCI1Index.UpdateInstances had no media type (was %q)", i+1, len(updates), index.Manifests[i].MediaType)
		}
		index.Manifests[i].MediaType = updates[i].MediaType
	}
	return nil
}

// ChooseInstance parses blob as an oci v1 manifest index, and returns the digest
// of the image which is appropriate for the current environment.
func (index *OCI1IndexPublic) ChooseInstance(ctx *types.SystemContext) (digest.Digest, error) {
	wantedPlatforms, err := platform.WantedPlatforms(ctx)
	if err != nil {
		return "", fmt.Errorf("getting platform information %#v: %w", ctx, err)
	}
	for _, wantedPlatform := range wantedPlatforms {
		for _, d := range index.Manifests {
			if d.Platform == nil {
				continue
			}
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

	for _, d := range index.Manifests {
		if d.Platform == nil {
			return d.Digest, nil
		}
	}
	return "", fmt.Errorf("no image found in image index for architecture %s, variant %q, OS %s", wantedPlatforms[0].Architecture, wantedPlatforms[0].Variant, wantedPlatforms[0].OS)
}

// Serialize returns the index in a blob format.
// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded from one, even if no modifications were made!
func (index *OCI1IndexPublic) Serialize() ([]byte, error) {
	buf, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("marshaling OCI1Index %#v: %w", index, err)
	}
	return buf, nil
}

// OCI1IndexPublicFromComponents creates an OCI1 image index instance from the
// supplied data.
// This is publicly visible as c/image/manifest.OCI1IndexFromComponents.
func OCI1IndexPublicFromComponents(components []imgspecv1.Descriptor, annotations map[string]string) *OCI1IndexPublic {
	index := OCI1IndexPublic{
		imgspecv1.Index{
			Versioned:   imgspec.Versioned{SchemaVersion: 2},
			MediaType:   imgspecv1.MediaTypeImageIndex,
			Manifests:   make([]imgspecv1.Descriptor, len(components)),
			Annotations: maps.Clone(annotations),
		},
	}
	for i, component := range components {
		var platform *imgspecv1.Platform
		if component.Platform != nil {
			platform = &imgspecv1.Platform{
				Architecture: component.Platform.Architecture,
				OS:           component.Platform.OS,
				OSVersion:    component.Platform.OSVersion,
				OSFeatures:   slices.Clone(component.Platform.OSFeatures),
				Variant:      component.Platform.Variant,
			}
		}
		m := imgspecv1.Descriptor{
			MediaType:   component.MediaType,
			Size:        component.Size,
			Digest:      component.Digest,
			URLs:        slices.Clone(component.URLs),
			Annotations: maps.Clone(component.Annotations),
			Platform:    platform,
		}
		index.Manifests[i] = m
	}
	return &index
}

// OCI1IndexPublicClone creates a deep copy of the passed-in index.
// This is publicly visible as c/image/manifest.OCI1IndexClone.
func OCI1IndexPublicClone(index *OCI1IndexPublic) *OCI1IndexPublic {
	return OCI1IndexPublicFromComponents(index.Manifests, index.Annotations)
}

// ToOCI1Index returns the index encoded as an OCI1 index.
func (index *OCI1IndexPublic) ToOCI1Index() (*OCI1IndexPublic, error) {
	return OCI1IndexPublicClone(index), nil
}

// ToSchema2List returns the index encoded as a Schema2 list.
func (index *OCI1IndexPublic) ToSchema2List() (*Schema2ListPublic, error) {
	components := make([]Schema2ManifestDescriptor, 0, len(index.Manifests))
	for _, manifest := range index.Manifests {
		platform := manifest.Platform
		if platform == nil {
			platform = &imgspecv1.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}
		}
		converted := Schema2ManifestDescriptor{
			Schema2Descriptor{
				MediaType: manifest.MediaType,
				Size:      manifest.Size,
				Digest:    manifest.Digest,
				URLs:      slices.Clone(manifest.URLs),
			},
			Schema2PlatformSpec{
				OS:           platform.OS,
				Architecture: platform.Architecture,
				OSFeatures:   slices.Clone(platform.OSFeatures),
				OSVersion:    platform.OSVersion,
				Variant:      platform.Variant,
			},
		}
		components = append(components, converted)
	}
	s2 := Schema2ListPublicFromComponents(components)
	return s2, nil
}

// OCI1IndexPublicFromManifest creates an OCI1 manifest index instance from marshalled
// JSON, presumably generated by encoding a OCI1 manifest index.
// This is publicly visible as c/image/manifest.OCI1IndexFromManifest.
func OCI1IndexPublicFromManifest(manifest []byte) (*OCI1IndexPublic, error) {
	index := OCI1IndexPublic{
		Index: imgspecv1.Index{
			Versioned:   imgspec.Versioned{SchemaVersion: 2},
			MediaType:   imgspecv1.MediaTypeImageIndex,
			Manifests:   []imgspecv1.Descriptor{},
			Annotations: make(map[string]string),
		},
	}
	if err := json.Unmarshal(manifest, &index); err != nil {
		return nil, fmt.Errorf("unmarshaling OCI1Index %q: %w", string(manifest), err)
	}
	if err := ValidateUnambiguousManifestFormat(manifest, imgspecv1.MediaTypeImageIndex,
		AllowedFieldManifests); err != nil {
		return nil, err
	}
	return &index, nil
}

// Clone returns a deep copy of this list and its contents.
func (index *OCI1IndexPublic) Clone() ListPublic {
	return OCI1IndexPublicClone(index)
}

// ConvertToMIMEType converts the passed-in image index to a manifest list of
// the specified type.
func (index *OCI1IndexPublic) ConvertToMIMEType(manifestMIMEType string) (ListPublic, error) {
	switch normalized := NormalizedMIMEType(manifestMIMEType); normalized {
	case DockerV2ListMediaType:
		return index.ToSchema2List()
	case imgspecv1.MediaTypeImageIndex:
		return index.Clone(), nil
	case DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType, imgspecv1.MediaTypeImageManifest, DockerV2Schema2MediaType:
		return nil, fmt.Errorf("Can not convert image index to MIME type %q, which is not a list type", manifestMIMEType)
	default:
		// Note that this may not be reachable, NormalizedMIMEType has a default for unknown values.
		return nil, fmt.Errorf("Unimplemented manifest MIME type %s", manifestMIMEType)
	}
}

type OCI1Index struct {
	OCI1IndexPublic
}

func oci1IndexFromPublic(public *OCI1IndexPublic) *OCI1Index {
	return &OCI1Index{*public}
}

func (index *OCI1Index) CloneInternal() List {
	return oci1IndexFromPublic(OCI1IndexPublicClone(&index.OCI1IndexPublic))
}

func (index *OCI1Index) Clone() ListPublic {
	return index.CloneInternal()
}

// OCI1IndexFromManifest creates a OCI1 manifest list instance from marshalled
// JSON, presumably generated by encoding a OCI1 manifest list.
func OCI1IndexFromManifest(manifest []byte) (*OCI1Index, error) {
	public, err := OCI1IndexPublicFromManifest(manifest)
	if err != nil {
		return nil, err
	}
	return oci1IndexFromPublic(public), nil
}
