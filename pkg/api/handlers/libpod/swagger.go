package libpod

import "github.com/containers/image/v5/manifest"

// List Containers
// swagger:response ListContainers
type swagInspectPodResponse struct {
	// in:body
	Body []ListContainer
}

// Inspect Manifest
// swagger:response InspectManifest
type swagInspectManifestResponse struct {
	// in:body
	Body manifest.List
}
