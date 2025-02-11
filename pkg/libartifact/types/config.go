package types

// GetArtifactOptions is a struct containing options that for obtaining artifacts.
// It is meant for future growth or changes required without wacking the API
type GetArtifactOptions struct{}

// AddOptions are additional descriptors of an artifact file
type AddOptions struct {
	Annotations  map[string]string `json:"annotations,omitempty"`
	ArtifactType string            `json:",omitempty"`
}

type ExtractOptions struct {
	// Title annotation value to extract only a single blob matching that name. Optional.
	Title string
	// Digest of the blob to extract. Optional.
	Digest string
}
