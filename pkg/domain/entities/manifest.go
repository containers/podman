package entities

import (
	"github.com/containers/image/v5/types"
	entitiesTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
)

// ManifestCreateOptions provides model for creating manifest
type ManifestCreateOptions struct {
	// True when adding lists to include all images
	All bool `schema:"all"`
	// Amend an extant list if there's already one with the desired name
	Amend bool `schema:"amend"`
	// Should TLS registry certificate be verified?
	SkipTLSVerify types.OptionalBool `json:"-" schema:"-"`
}

// ManifestInspectOptions provides model for inspecting manifest
type ManifestInspectOptions struct {
	// Path to an authentication file.
	Authfile string `json:"-" schema:"-"`
	// Should TLS registry certificate be verified?
	SkipTLSVerify types.OptionalBool `json:"-" schema:"-"`
}

// ManifestAddOptions provides model for adding digests to manifest list
//
// swagger:model
type ManifestAddOptions struct {
	ManifestAnnotateOptions
	// True when operating on a list to include all images
	All bool `json:"all" schema:"all"`
	// authfile to use when pushing manifest list
	Authfile string `json:"-" schema:"-"`
	// Home directory for certificates when pushing a manifest list
	CertDir string `json:"-" schema:"-"`
	// Password to authenticate to registry when pushing manifest list
	Password string `json:"-" schema:"-"`
	// Should TLS registry certificate be verified?
	SkipTLSVerify types.OptionalBool `json:"-" schema:"-"`
	// Username to authenticate to registry when pushing manifest list
	Username string `json:"-" schema:"-"`
	// Images is an optional list of images to add to manifest list
	Images []string `json:"images" schema:"images"`
}

// ManifestAnnotateOptions provides model for annotating manifest list
type ManifestAnnotateOptions struct {
	// Annotation to add to manifest list
	Annotation []string `json:"annotation" schema:"annotation"`
	// Annotations to add to manifest list by a map which is preferred over Annotation
	Annotations map[string]string `json:"annotations" schema:"annotations"`
	// Arch overrides the architecture for the image
	Arch string `json:"arch" schema:"arch"`
	// Feature list for the image
	Features []string `json:"features" schema:"features"`
	// OS overrides the operating system for the image
	OS string `json:"os" schema:"os"`
	// OS features for the image
	OSFeatures []string `json:"os_features" schema:"os_features"`
	// OSVersion overrides the operating system for the image
	OSVersion string `json:"os_version" schema:"os_version"`
	// Variant for the image
	Variant string `json:"variant" schema:"variant"`
}

// ManifestModifyOptions provides the model for mutating a manifest
//
// swagger 2.0 does not support oneOf for schema validation.
//
// Operation "update" uses all fields.
// Operation "remove" uses fields: Operation and Images
// Operation "annotate" uses fields: Operation and Annotations
//
// swagger:model
type ManifestModifyOptions struct {
	Operation string `json:"operation" schema:"operation"` // Valid values: update, remove, annotate
	ManifestAddOptions
	ManifestRemoveOptions
}

// ManifestPushReport provides the model for the pushed manifest
//
// swagger:model
type ManifestPushReport = entitiesTypes.ManifestPushReport

// ManifestRemoveOptions provides the model for removing digests from a manifest
//
// swagger:model
type ManifestRemoveOptions struct {
}

// ManifestRemoveReport provides the model for the removed manifest
//
// swagger:model
type ManifestRemoveReport = entitiesTypes.ManifestRemoveReport

// ManifestModifyReport provides the model for removed digests and changed manifest
//
// swagger:model
type ManifestModifyReport = entitiesTypes.ManifestModifyReport
