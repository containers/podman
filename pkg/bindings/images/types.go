package images

import (
	buildahDefine "github.com/containers/buildah/define"
)

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for image removal
type RemoveOptions struct {
	// All removes all images
	All *bool
	// Forces removes all containers based on the image
	Force *bool
}

//go:generate go run ../generator/generator.go DiffOptions
// DiffOptions are optional options image diffs
type DiffOptions struct {
}

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for listing images
type ListOptions struct {
	// All lists all image in the image store including dangling images
	All *bool
	// filters that can be used to get a more specific list of images
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go GetOptions
// GetOptions are optional options for inspecting an image
type GetOptions struct {
	// Size computes the amount of storage the image consumes
	Size *bool
}

//go:generate go run ../generator/generator.go TreeOptions
// TreeOptions are optional options for a tree-based representation
// of the image
type TreeOptions struct {
	// WhatRequires ...
	WhatRequires *bool
}

//go:generate go run ../generator/generator.go HistoryOptions
// HistoryOptions are optional options image history
type HistoryOptions struct {
}

//go:generate go run ../generator/generator.go LoadOptions
// LoadOptions are optional options for loading an image
type LoadOptions struct {
	// Reference is the name of the loaded image
	Reference *string
}

//go:generate go run ../generator/generator.go ExportOptions
// ExportOptions are optional options for exporting images
type ExportOptions struct {
	// Compress the image
	Compress *bool
	// Format of the output
	Format *string
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for pruning images
type PruneOptions struct {
	// Prune all images
	All *bool
	// Filters to apply when pruning images
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go TagOptions
// TagOptions are optional options for tagging images
type TagOptions struct {
}

//go:generate go run ../generator/generator.go UntagOptions
// UntagOptions are optional options for untagging images
type UntagOptions struct {
}

//go:generate go run ../generator/generator.go ImportOptions
// ImportOptions are optional options for importing images
type ImportOptions struct {
	// Changes to be applied to the image
	Changes *[]string
	// Message to be applied to the image
	Message *string
	// Reference is a tag to be applied to the image
	Reference *string
	// Url to option image to import. Cannot be used with the reader
	URL *string
}

//go:generate go run ../generator/generator.go PushOptions
// PushOptions are optional options for importing images
type PushOptions struct {
	// All indicates whether to push all images related to the image list
	All *bool
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile *string
	// Compress tarball image layers when pushing to a directory using the 'dir' transport.
	Compress *bool
	// Manifest type of the pushed image
	Format *string
	// Password for authenticating against the registry.
	Password *string
	// SkipTLSVerify to skip HTTPS and certificate verification.
	SkipTLSVerify *bool
	// Username for authenticating against the registry.
	Username *string
}

//go:generate go run ../generator/generator.go SearchOptions
// SearchOptions are optional options for searching images on registries
type SearchOptions struct {
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile *string
	// Filters for the search results.
	Filters map[string][]string
	// Limit the number of results.
	Limit *int
	// NoTrunc will not truncate the output.
	NoTrunc *bool
	// SkipTLSVerify to skip  HTTPS and certificate verification.
	SkipTLSVerify *bool
	// ListTags search the available tags of the repository
	ListTags *bool
}

//go:generate go run ../generator/generator.go PullOptions
// PullOptions are optional options for pulling images
type PullOptions struct {
	// AllTags can be specified to pull all tags of an image. Note
	// that this only works if the image does not include a tag.
	AllTags *bool
	// Arch will overwrite the local architecture for image pulls.
	Arch *string
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile *string
	// OS will overwrite the local operating system (OS) for image
	// pulls.
	OS *string
	// Password for authenticating against the registry.
	Password *string
	// Quiet can be specified to suppress pull progress when pulling.  Ignored
	// for remote calls.
	Quiet *bool
	// SkipTLSVerify to skip HTTPS and certificate verification.
	SkipTLSVerify *bool
	// Username for authenticating against the registry.
	Username *string
	// Variant will overwrite the local variant for image pulls.
	Variant *string
}

//BuildOptions are optional options for building images
type BuildOptions struct {
	buildahDefine.BuildOptions
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking if an image exists
type ExistsOptions struct {
}
