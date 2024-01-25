// copied from github.com/docker/docker/api/types
package types

// ComponentVersion describes the version information for a specific component.
type ComponentVersion struct {
	Name    string
	Version string
	Details map[string]string `json:",omitempty"`
}

// Version contains response of Engine API:
// GET "/version"
type Version struct {
	Platform   struct{ Name string } `json:",omitempty"`
	Components []ComponentVersion    `json:",omitempty"`

	// The following fields are deprecated, they relate to the Engine component and are kept for backwards compatibility

	Version       string
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// SystemComponentVersion is the type used by pkg/domain/entities
type SystemComponentVersion struct {
	Version
}

// ContainerCreateResponse is the response struct for creating a container
type ContainerCreateResponse struct {
	// ID of the container created
	// required: true
	ID string `json:"Id"`
	// Warnings during container creation
	// required: true
	Warnings []string `json:"Warnings"`
}
