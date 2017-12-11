package tarfile

import "github.com/opencontainers/go-digest"

// Various data structures.

// Based on github.com/docker/docker/image/tarexport/tarexport.go
const (
	manifestFileName = "manifest.json"
	// legacyLayerFileName        = "layer.tar"
	// legacyConfigFileName       = "json"
	// legacyVersionFileName      = "VERSION"
	// legacyRepositoriesFileName = "repositories"
)

// ManifestItem is an element of the array stored in the top-level manifest.json file.
type ManifestItem struct {
	Config       string
	RepoTags     []string
	Layers       []string
	Parent       imageID                           `json:",omitempty"`
	LayerSources map[diffID]distributionDescriptor `json:",omitempty"`
}

type imageID string
type diffID digest.Digest

// Based on github.com/docker/distribution/blobs.go
type distributionDescriptor struct {
	MediaType string        `json:"mediaType,omitempty"`
	Size      int64         `json:"size,omitempty"`
	Digest    digest.Digest `json:"digest,omitempty"`
	URLs      []string      `json:"urls,omitempty"`
}

// Based on github.com/docker/distribution/manifest/schema2/manifest.go
// FIXME: We are repeating this all over the place; make a public copy?
type schema2Manifest struct {
	SchemaVersion int                      `json:"schemaVersion"`
	MediaType     string                   `json:"mediaType,omitempty"`
	Config        distributionDescriptor   `json:"config"`
	Layers        []distributionDescriptor `json:"layers"`
}

// Based on github.com/docker/docker/image/image.go
// MOST CONTENT OMITTED AS UNNECESSARY
type image struct {
	RootFS *rootFS `json:"rootfs,omitempty"`
}

type rootFS struct {
	Type    string   `json:"type"`
	DiffIDs []diffID `json:"diff_ids,omitempty"`
}
