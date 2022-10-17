package manifests

// InspectOptions are optional options for inspecting manifests
//
//go:generate go run ../generator/generator.go InspectOptions
type InspectOptions struct {
}

// CreateOptions are optional options for creating manifests
//
//go:generate go run ../generator/generator.go CreateOptions
type CreateOptions struct {
	All   *bool
	Amend *bool
}

// ExistsOptions are optional options for checking
// if a manifest list exists
//
//go:generate go run ../generator/generator.go ExistsOptions
type ExistsOptions struct {
}

// AddOptions are optional options for adding manifest lists
//
//go:generate go run ../generator/generator.go AddOptions
type AddOptions struct {
	All           *bool
	Annotation    map[string]string
	Arch          *string
	Features      []string
	Images        []string
	OS            *string
	OSVersion     *string
	Variant       *string
	Authfile      *string
	Password      *string
	Username      *string
	SkipTLSVerify *bool `schema:"-"`
}

// RemoveOptions are optional options for removing manifest lists
//
//go:generate go run ../generator/generator.go RemoveOptions
type RemoveOptions struct {
}

// ModifyOptions are optional options for modifying manifest lists
//
//go:generate go run ../generator/generator.go ModifyOptions
type ModifyOptions struct {
	// Operation values are "update", "remove" and "annotate". This allows the service to
	//   efficiently perform each update on a manifest list.
	Operation   *string
	All         *bool             // All when true, operate on all images in a manifest list that may be included in Images
	Annotations map[string]string // Annotations to add to manifest list
	Arch        *string           // Arch overrides the architecture for the image
	Features    []string          // Feature list for the image
	Images      []string          // Images is an optional list of images to add/remove to/from manifest list depending on operation
	OS          *string           // OS overrides the operating system for the image
	// OS features for the image
	OSFeatures []string `json:"os_features" schema:"os_features"`
	// OSVersion overrides the operating system for the image
	OSVersion     *string `json:"os_version" schema:"os_version"`
	Variant       *string // Variant overrides the operating system variant for the image
	Authfile      *string
	Password      *string
	Username      *string
	SkipTLSVerify *bool `schema:"-"`
}
