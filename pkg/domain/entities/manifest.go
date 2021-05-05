package entities

import "github.com/containers/image/v5/types"

// TODO: add comments to *all* types and fields.

type ManifestCreateOptions struct {
	All bool `schema:"all"`
}

// swagger:model ManifestAddOpts
type ManifestAddOptions struct {
	All           bool               `json:"all" schema:"all"`
	Annotation    []string           `json:"annotation" schema:"annotation"`
	Arch          string             `json:"arch" schema:"arch"`
	Authfile      string             `json:"-" schema:"-"`
	CertDir       string             `json:"-" schema:"-"`
	Features      []string           `json:"features" schema:"features"`
	Images        []string           `json:"images" schema:"images"`
	OS            string             `json:"os" schema:"os"`
	OSVersion     string             `json:"os_version" schema:"os_version"`
	Password      string             `json:"-" schema:"-"`
	SkipTLSVerify types.OptionalBool `json:"-" schema:"-"`
	Username      string             `json:"-" schema:"-"`
	Variant       string             `json:"variant" schema:"variant"`
}

type ManifestAnnotateOptions struct {
	Annotation []string `json:"annotation"`
	Arch       string   `json:"arch" schema:"arch"`
	Features   []string `json:"features" schema:"features"`
	OS         string   `json:"os" schema:"os"`
	OSFeatures []string `json:"os_features" schema:"os_features"`
	OSVersion  string   `json:"os_version" schema:"os_version"`
	Variant    string   `json:"variant" schema:"variant"`
}
