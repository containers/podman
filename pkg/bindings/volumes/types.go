package volumes

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for creating volumes
type CreateOptions struct {
}

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting volumes
type InspectOptions struct {
}

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for listing volumes
type ListOptions struct {
	// Filters applied to the listing of volumes
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for pruning volumes
type PruneOptions struct {
	// Filters applied to the pruning of volumes
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for removing volumes
type RemoveOptions struct {
	// Force removes the volume even if it is being used
	Force *bool
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking
// if a volume exists
type ExistsOptions struct {
}
