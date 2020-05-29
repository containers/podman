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
	"github.com/pkg/errors"
)

// OCI1Index is just an alias for the OCI index type, but one which we can
// provide methods for.
type OCI1Index struct {
	imgspecv1.Index
}

// MIMEType returns the MIME type of this particular manifest index.
func (index *OCI1Index) MIMEType() string {
	return imgspecv1.MediaTypeImageIndex
}

// Instances returns a slice of digests of the manifests that this index knows of.
func (index *OCI1Index) Instances() []digest.Digest {
	results := make([]digest.Digest, len(index.Manifests))
	for i, m := range index.Manifests {
		results[i] = m.Digest
	}
	return results
}

// Instance returns the ListUpdate of a particular instance in the index.
func (index *OCI1Index) Instance(instanceDigest digest.Digest) (ListUpdate, error) {
	for _, manifest := range index.Manifests {
		if manifest.Digest == instanceDigest {
			return ListUpdate{
				Digest:    manifest.Digest,
				Size:      manifest.Size,
				MediaType: manifest.MediaType,
			}, nil
		}
	}
	return ListUpdate{}, errors.Errorf("unable to find instance %s in OCI1Index", instanceDigest)
}

// UpdateInstances updates the sizes, digests, and media types of the manifests
// which the list catalogs.
func (index *OCI1Index) UpdateInstances(updates []ListUpdate) error {
	if len(updates) != len(index.Manifests) {
		return errors.Errorf("incorrect number of update entries passed to OCI1Index.UpdateInstances: expected %d, got %d", len(index.Manifests), len(updates))
	}
	for i := range updates {
		if err := updates[i].Digest.Validate(); err != nil {
			return errors.Wrapf(err, "update %d of %d passed to OCI1Index.UpdateInstances contained an invalid digest", i+1, len(updates))
		}
		index.Manifests[i].Digest = updates[i].Digest
		if updates[i].Size < 0 {
			return errors.Errorf("update %d of %d passed to OCI1Index.UpdateInstances had an invalid size (%d)", i+1, len(updates), updates[i].Size)
		}
		index.Manifests[i].Size = updates[i].Size
		if updates[i].MediaType == "" {
			return errors.Errorf("update %d of %d passed to OCI1Index.UpdateInstances had no media type (was %q)", i+1, len(updates), index.Manifests[i].MediaType)
		}
		index.Manifests[i].MediaType = updates[i].MediaType
	}
	return nil
}

// ChooseInstance parses blob as an oci v1 manifest index, and returns the digest
// of the image which is appropriate for the current environment.
func (index *OCI1Index) ChooseInstance(ctx *types.SystemContext) (digest.Digest, error) {
	wantedPlatforms, err := platform.WantedPlatforms(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "error getting platform information %#v", ctx)
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
				OSFeatures:   dupStringSlice(d.Platform.OSFeatures),
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
func (index *OCI1Index) Serialize() ([]byte, error) {
	buf, err := json.Marshal(index)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshaling OCI1Index %#v", index)
	}
	return buf, nil
}

// OCI1IndexFromComponents creates an OCI1 image index instance from the
// supplied data.
func OCI1IndexFromComponents(components []imgspecv1.Descriptor, annotations map[string]string) *OCI1Index {
	index := OCI1Index{
		imgspecv1.Index{
			Versioned:   imgspec.Versioned{SchemaVersion: 2},
			Manifests:   make([]imgspecv1.Descriptor, len(components)),
			Annotations: dupStringStringMap(annotations),
		},
	}
	for i, component := range components {
		var platform *imgspecv1.Platform
		if component.Platform != nil {
			platform = &imgspecv1.Platform{
				Architecture: component.Platform.Architecture,
				OS:           component.Platform.OS,
				OSVersion:    component.Platform.OSVersion,
				OSFeatures:   dupStringSlice(component.Platform.OSFeatures),
				Variant:      component.Platform.Variant,
			}
		}
		m := imgspecv1.Descriptor{
			MediaType:   component.MediaType,
			Size:        component.Size,
			Digest:      component.Digest,
			URLs:        dupStringSlice(component.URLs),
			Annotations: dupStringStringMap(component.Annotations),
			Platform:    platform,
		}
		index.Manifests[i] = m
	}
	return &index
}

// OCI1IndexClone creates a deep copy of the passed-in index.
func OCI1IndexClone(index *OCI1Index) *OCI1Index {
	return OCI1IndexFromComponents(index.Manifests, index.Annotations)
}

// ToOCI1Index returns the index encoded as an OCI1 index.
func (index *OCI1Index) ToOCI1Index() (*OCI1Index, error) {
	return OCI1IndexClone(index), nil
}

// ToSchema2List returns the index encoded as a Schema2 list.
func (index *OCI1Index) ToSchema2List() (*Schema2List, error) {
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
				URLs:      dupStringSlice(manifest.URLs),
			},
			Schema2PlatformSpec{
				OS:           platform.OS,
				Architecture: platform.Architecture,
				OSFeatures:   dupStringSlice(platform.OSFeatures),
				OSVersion:    platform.OSVersion,
				Variant:      platform.Variant,
			},
		}
		components = append(components, converted)
	}
	s2 := Schema2ListFromComponents(components)
	return s2, nil
}

// OCI1IndexFromManifest creates an OCI1 manifest index instance from marshalled
// JSON, presumably generated by encoding a OCI1 manifest index.
func OCI1IndexFromManifest(manifest []byte) (*OCI1Index, error) {
	index := OCI1Index{
		Index: imgspecv1.Index{
			Versioned:   imgspec.Versioned{SchemaVersion: 2},
			Manifests:   []imgspecv1.Descriptor{},
			Annotations: make(map[string]string),
		},
	}
	if err := json.Unmarshal(manifest, &index); err != nil {
		return nil, errors.Wrapf(err, "error unmarshaling OCI1Index %q", string(manifest))
	}
	return &index, nil
}

// Clone returns a deep copy of this list and its contents.
func (index *OCI1Index) Clone() List {
	return OCI1IndexClone(index)
}

// ConvertToMIMEType converts the passed-in image index to a manifest list of
// the specified type.
func (index *OCI1Index) ConvertToMIMEType(manifestMIMEType string) (List, error) {
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
