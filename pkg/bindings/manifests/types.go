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
// AddOptions are optional options for adding manifests
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
// RemoveOptions are optional options for removing manifests
type RemoveOptions struct {
}
