package manifests

import (
	"encoding/json"
	"os"

	"github.com/containers/image/v5/manifest"
	digest "github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// List is a generic interface for manipulating a manifest list or an image
// index.
type List interface {
	AddInstance(manifestDigest digest.Digest, manifestSize int64, manifestType, os, architecture, osVersion string, osFeatures []string, variant string, features []string, annotations []string) error
	Remove(instanceDigest digest.Digest) error

	SetURLs(instanceDigest digest.Digest, urls []string) error
	URLs(instanceDigest digest.Digest) ([]string, error)

	SetAnnotations(instanceDigest *digest.Digest, annotations map[string]string) error
	Annotations(instanceDigest *digest.Digest) (map[string]string, error)

	SetOS(instanceDigest digest.Digest, os string) error
	OS(instanceDigest digest.Digest) (string, error)

	SetArchitecture(instanceDigest digest.Digest, arch string) error
	Architecture(instanceDigest digest.Digest) (string, error)

	SetOSVersion(instanceDigest digest.Digest, osVersion string) error
	OSVersion(instanceDigest digest.Digest) (string, error)

	SetVariant(instanceDigest digest.Digest, variant string) error
	Variant(instanceDigest digest.Digest) (string, error)

	SetFeatures(instanceDigest digest.Digest, features []string) error
	Features(instanceDigest digest.Digest) ([]string, error)

	SetOSFeatures(instanceDigest digest.Digest, osFeatures []string) error
	OSFeatures(instanceDigest digest.Digest) ([]string, error)

	Serialize(mimeType string) ([]byte, error)
	Instances() []digest.Digest
	OCIv1() *v1.Index
	Docker() *manifest.Schema2List

	findDocker(instanceDigest digest.Digest) (*manifest.Schema2ManifestDescriptor, error)
	findOCIv1(instanceDigest digest.Digest) (*v1.Descriptor, error)
}

type list struct {
	docker manifest.Schema2List
	oci    v1.Index
}

// OCIv1 returns the list as a Docker schema 2 list.  The returned structure should NOT be modified.
func (l *list) Docker() *manifest.Schema2List {
	return &l.docker
}

// OCIv1 returns the list as an OCI image index.  The returned structure should NOT be modified.
func (l *list) OCIv1() *v1.Index {
	return &l.oci
}

// Create creates a new list.
func Create() List {
	return &list{
		docker: manifest.Schema2List{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2ListMediaType,
		},
		oci: v1.Index{
			Versioned: imgspec.Versioned{SchemaVersion: 2},
		},
	}
}

// AddInstance adds an entry for the specified manifest digest, with assorted
// additional information specified in parameters, to the list or index.
func (l *list) AddInstance(manifestDigest digest.Digest, manifestSize int64, manifestType, osName, architecture, osVersion string, osFeatures []string, variant string, features []string, annotations []string) error {
	if err := l.Remove(manifestDigest); err != nil && !os.IsNotExist(errors.Cause(err)) {
		return err
	}

	schema2platform := manifest.Schema2PlatformSpec{
		Architecture: architecture,
		OS:           osName,
		OSVersion:    osVersion,
		OSFeatures:   osFeatures,
		Variant:      variant,
		Features:     features,
	}
	l.docker.Manifests = append(l.docker.Manifests, manifest.Schema2ManifestDescriptor{
		Schema2Descriptor: manifest.Schema2Descriptor{
			MediaType: manifestType,
			Size:      manifestSize,
			Digest:    manifestDigest,
		},
		Platform: schema2platform,
	})

	ociv1platform := v1.Platform{
		Architecture: architecture,
		OS:           osName,
		OSVersion:    osVersion,
		OSFeatures:   osFeatures,
		Variant:      variant,
	}
	l.oci.Manifests = append(l.oci.Manifests, v1.Descriptor{
		MediaType: manifestType,
		Size:      manifestSize,
		Digest:    manifestDigest,
		Platform:  &ociv1platform,
	})

	return nil
}

// Remove filters out any instances in the list which match the specified digest.
func (l *list) Remove(instanceDigest digest.Digest) error {
	err := errors.Wrapf(os.ErrNotExist, "no instance matching digest %q found in manifest list", instanceDigest)
	newDockerManifests := make([]manifest.Schema2ManifestDescriptor, 0, len(l.docker.Manifests))
	for i := range l.docker.Manifests {
		if l.docker.Manifests[i].Digest != instanceDigest {
			newDockerManifests = append(newDockerManifests, l.docker.Manifests[i])
		} else {
			err = nil
		}
	}
	l.docker.Manifests = newDockerManifests
	newOCIv1Manifests := make([]v1.Descriptor, 0, len(l.oci.Manifests))
	for i := range l.oci.Manifests {
		if l.oci.Manifests[i].Digest != instanceDigest {
			newOCIv1Manifests = append(newOCIv1Manifests, l.oci.Manifests[i])
		} else {
			err = nil
		}
	}
	l.oci.Manifests = newOCIv1Manifests
	return err
}

func (l *list) findDocker(instanceDigest digest.Digest) (*manifest.Schema2ManifestDescriptor, error) {
	for i := range l.docker.Manifests {
		if l.docker.Manifests[i].Digest == instanceDigest {
			return &l.docker.Manifests[i], nil
		}
	}
	return nil, errors.Wrapf(ErrDigestNotFound, "no Docker manifest matching digest %q was found in list", instanceDigest.String())
}

func (l *list) findOCIv1(instanceDigest digest.Digest) (*v1.Descriptor, error) {
	for i := range l.oci.Manifests {
		if l.oci.Manifests[i].Digest == instanceDigest {
			return &l.oci.Manifests[i], nil
		}
	}
	return nil, errors.Wrapf(ErrDigestNotFound, "no OCI manifest matching digest %q was found in list", instanceDigest.String())
}

// SetURLs sets the URLs where the manifest might also be found.
func (l *list) SetURLs(instanceDigest digest.Digest, urls []string) error {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci.URLs = append([]string{}, urls...)
	docker.URLs = append([]string{}, urls...)
	return nil
}

// URLs retrieves the locations from which this object might possibly be downloaded.
func (l *list) URLs(instanceDigest digest.Digest) ([]string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return nil, err
	}
	return append([]string{}, oci.URLs...), nil
}

// SetAnnotations sets annotations on the image index, or on a specific manifest.
// The field is specific to the OCI image index format, and is not present in Docker manifest lists.
func (l *list) SetAnnotations(instanceDigest *digest.Digest, annotations map[string]string) error {
	a := &l.oci.Annotations
	if instanceDigest != nil {
		oci, err := l.findOCIv1(*instanceDigest)
		if err != nil {
			return err
		}
		a = &oci.Annotations
	}
	(*a) = make(map[string]string)
	for k, v := range annotations {
		(*a)[k] = v
	}
	return nil
}

// Annotations retrieves the annotations which have been set on the image index, or on one instance.
// The field is specific to the OCI image index format, and is not present in Docker manifest lists.
func (l *list) Annotations(instanceDigest *digest.Digest) (map[string]string, error) {
	a := l.oci.Annotations
	if instanceDigest != nil {
		oci, err := l.findOCIv1(*instanceDigest)
		if err != nil {
			return nil, err
		}
		a = oci.Annotations
	}
	annotations := make(map[string]string)
	for k, v := range a {
		annotations[k] = v
	}
	return annotations, nil
}

// SetOS sets the OS field in the platform information associated with the instance with the specified digest.
func (l *list) SetOS(instanceDigest digest.Digest, os string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.OS = os
	oci.Platform.OS = os
	return nil
}

// OS retrieves the OS field in the platform information associated with the instance with the specified digest.
func (l *list) OS(instanceDigest digest.Digest) (string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return "", err
	}
	return oci.Platform.OS, nil
}

// SetArchitecture sets the Architecture field in the platform information associated with the instance with the specified digest.
func (l *list) SetArchitecture(instanceDigest digest.Digest, arch string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.Architecture = arch
	oci.Platform.Architecture = arch
	return nil
}

// Architecture retrieves the Architecture field in the platform information associated with the instance with the specified digest.
func (l *list) Architecture(instanceDigest digest.Digest) (string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return "", err
	}
	return oci.Platform.Architecture, nil
}

// SetOSVersion sets the OSVersion field in the platform information associated with the instance with the specified digest.
func (l *list) SetOSVersion(instanceDigest digest.Digest, osVersion string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.OSVersion = osVersion
	oci.Platform.OSVersion = osVersion
	return nil
}

// OSVersion retrieves the OSVersion field in the platform information associated with the instance with the specified digest.
func (l *list) OSVersion(instanceDigest digest.Digest) (string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return "", err
	}
	return oci.Platform.OSVersion, nil
}

// SetVariant sets the Variant field in the platform information associated with the instance with the specified digest.
func (l *list) SetVariant(instanceDigest digest.Digest, variant string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.Variant = variant
	oci.Platform.Variant = variant
	return nil
}

// Variant retrieves the Variant field in the platform information associated with the instance with the specified digest.
func (l *list) Variant(instanceDigest digest.Digest) (string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return "", err
	}
	return oci.Platform.Variant, nil
}

// SetFeatures sets the features list in the platform information associated with the instance with the specified digest.
// The field is specific to the Docker manifest list format, and is not present in OCI's image indexes.
func (l *list) SetFeatures(instanceDigest digest.Digest, features []string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.Features = append([]string{}, features...)
	// no OCI equivalent
	return nil
}

// Features retrieves the features list from the platform information associated with the instance with the specified digest.
// The field is specific to the Docker manifest list format, and is not present in OCI's image indexes.
func (l *list) Features(instanceDigest digest.Digest) ([]string, error) {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return nil, err
	}
	return append([]string{}, docker.Platform.Features...), nil
}

// SetOSFeatures sets the OS features list in the platform information associated with the instance with the specified digest.
func (l *list) SetOSFeatures(instanceDigest digest.Digest, osFeatures []string) error {
	docker, err := l.findDocker(instanceDigest)
	if err != nil {
		return err
	}
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return err
	}
	docker.Platform.OSFeatures = append([]string{}, osFeatures...)
	oci.Platform.OSFeatures = append([]string{}, osFeatures...)
	return nil
}

// OSFeatures retrieves the OS features list from the platform information associated with the instance with the specified digest.
func (l *list) OSFeatures(instanceDigest digest.Digest) ([]string, error) {
	oci, err := l.findOCIv1(instanceDigest)
	if err != nil {
		return nil, err
	}
	return append([]string{}, oci.Platform.OSFeatures...), nil
}

// FromBlob builds a list from an encoded manifest list or image index.
func FromBlob(manifestBytes []byte) (List, error) {
	manifestType := manifest.GuessMIMEType(manifestBytes)
	list := &list{
		docker: manifest.Schema2List{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2ListMediaType,
		},
		oci: v1.Index{
			Versioned: imgspec.Versioned{SchemaVersion: 2},
		},
	}
	switch manifestType {
	default:
		return nil, errors.Wrapf(ErrManifestTypeNotSupported, "unable to load manifest list: unsupported format %q", manifestType)
	case manifest.DockerV2ListMediaType:
		if err := json.Unmarshal(manifestBytes, &list.docker); err != nil {
			return nil, errors.Wrapf(err, "unable to parse Docker manifest list from image")
		}
		for _, m := range list.docker.Manifests {
			list.oci.Manifests = append(list.oci.Manifests, v1.Descriptor{
				MediaType: m.Schema2Descriptor.MediaType,
				Size:      m.Schema2Descriptor.Size,
				Digest:    m.Schema2Descriptor.Digest,
				Platform: &v1.Platform{
					Architecture: m.Platform.Architecture,
					OS:           m.Platform.OS,
					OSVersion:    m.Platform.OSVersion,
					OSFeatures:   m.Platform.OSFeatures,
					Variant:      m.Platform.Variant,
				},
			})
		}
	case v1.MediaTypeImageIndex:
		if err := json.Unmarshal(manifestBytes, &list.oci); err != nil {
			return nil, errors.Wrapf(err, "unable to parse OCIv1 manifest list")
		}
		for _, m := range list.oci.Manifests {
			platform := m.Platform
			if platform == nil {
				platform = &v1.Platform{}
			}
			list.docker.Manifests = append(list.docker.Manifests, manifest.Schema2ManifestDescriptor{
				Schema2Descriptor: manifest.Schema2Descriptor{
					MediaType: m.MediaType,
					Size:      m.Size,
					Digest:    m.Digest,
				},
				Platform: manifest.Schema2PlatformSpec{
					Architecture: platform.Architecture,
					OS:           platform.OS,
					OSVersion:    platform.OSVersion,
					OSFeatures:   platform.OSFeatures,
					Variant:      platform.Variant,
				},
			})
		}
	}
	return list, nil
}

func (l *list) preferOCI() bool {
	// If we have any data that's only in the OCI format, use that.
	for _, m := range l.oci.Manifests {
		if len(m.URLs) > 0 {
			return true
		}
		if len(m.Annotations) > 0 {
			return true
		}
	}
	// If we have any data that's only in the Docker format, use that.
	for _, m := range l.docker.Manifests {
		if len(m.Platform.Features) > 0 {
			return false
		}
	}
	// If we have no manifests, remember that the Docker format is
	// explicitly typed, so use that.  Otherwise, default to using the OCI
	// format.
	return len(l.docker.Manifests) != 0
}

// Serialize encodes the list using the specified format, or by selecting one
// which it thinks is appropriate.
func (l *list) Serialize(mimeType string) ([]byte, error) {
	var manifestBytes []byte
	switch mimeType {
	case "":
		if l.preferOCI() {
			manifest, err := json.Marshal(&l.oci)
			if err != nil {
				return nil, errors.Wrapf(err, "error marshalling OCI image index")
			}
			manifestBytes = manifest
		} else {
			manifest, err := json.Marshal(&l.docker)
			if err != nil {
				return nil, errors.Wrapf(err, "error marshalling Docker manifest list")
			}
			manifestBytes = manifest
		}
	case v1.MediaTypeImageIndex:
		manifest, err := json.Marshal(&l.oci)
		if err != nil {
			return nil, errors.Wrapf(err, "error marshalling OCI image index")
		}
		manifestBytes = manifest
	case manifest.DockerV2ListMediaType:
		manifest, err := json.Marshal(&l.docker)
		if err != nil {
			return nil, errors.Wrapf(err, "error marshalling Docker manifest list")
		}
		manifestBytes = manifest
	default:
		return nil, errors.Wrapf(ErrManifestTypeNotSupported, "serializing list to type %q not implemented", mimeType)
	}
	return manifestBytes, nil
}

// Instances returns the list of image instances mentioned in this list.
func (l *list) Instances() []digest.Digest {
	instances := make([]digest.Digest, 0, len(l.oci.Manifests))
	for _, instance := range l.oci.Manifests {
		instances = append(instances, instance.Digest)
	}
	return instances
}
