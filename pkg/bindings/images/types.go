package images

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for image removal
type RemoveOptions struct {
	// Forces removes all containers based on the image
	Force *bool
}
