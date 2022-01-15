package manifests

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting manifests
type InspectOptions struct {
}

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for creating manifests
type CreateOptions struct {
	All *bool
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking
// if a manifest list exists
type ExistsOptions struct {
}

//go:generate go run ../generator/generator.go AddOptions
// AddOptions are optional options for adding manifest lists
type AddOptions struct {
	All        *bool
	Annotation map[string]string
	Arch       *string
	Features   []string
	Images     []string
	OS         *string
	OSVersion  *string
	Variant    *string
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for removing manifest lists
type RemoveOptions struct {
}

//go:generate go run ../generator/generator.go ModifyOptions
// ModifyOptions are optional options for modifying manifest lists
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
	OSFeatures  []string          // OS features for the image
	OSVersion   *string           // OSVersion overrides the operating system for the image
	Variant     *string           // Variant overrides the operating system variant for the image

}
