//go:build amd64 || arm64

package os

// Manager is the interface for operations on a Podman machine's OS
type Manager interface {
	// Apply machine OS changes from an OCI image.
	Apply(image string, opts ApplyOptions) error
}

// ApplyOptions are the options for applying an image into a Podman machine VM
type ApplyOptions struct {
	Image string
}
