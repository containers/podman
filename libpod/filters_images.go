package libpod

import (
	"github.com/containers/libpod/libpod/adapter"
)

// type (
// 	// Filterable defines items to be filtered for CLI and Varlink
// 	FilterableImage interface {
// 		Created() time.Time
// 		ID() string
// 		Labels() map[string]string
// 		Mounts() []specs.Mount
// 		Name() string
// 		State() (libpod.ContainerStatus, error)
// 		Dangling() bool
// 	}
// )

// DanglingFilter allows you to filterable images for dangling images
func DanglingFilter() (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{Dangling() bool}); ok {
			return m.Dangling()
		}
		return false
	}, nil
}

// FilterImages filters images using a set of predefined filterable funcs
func FilterImages(images []*adapter.ContainerImage, filters []Filter) []*adapter.ContainerImage {
	var filteredImages []*adapter.ContainerImage
	for _, image := range images {
		include := true
		for _, filter := range filters {
			include = include && filter(image)
		}
		if include {
			filteredImages = append(filteredImages, image)
		}
	}
	return filteredImages
}
