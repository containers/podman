package secrets

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for inspecting secrets
type ListOptions struct {
}

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting secrets
type InspectOptions struct {
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for removing secrets
type RemoveOptions struct {
}

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for Creating secrets
type CreateOptions struct {
	Driver *string
	Name   *string
}
