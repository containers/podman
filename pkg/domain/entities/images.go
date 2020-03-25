package entities

import (
	"net/url"

	"github.com/containers/image/v5/manifest"
	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Image struct {
	IdOrNamed
	ID              string                 `json:"Id"`
	RepoTags        []string               `json:",omitempty"`
	RepoDigests     []string               `json:",omitempty"`
	Parent          string                 `json:",omitempty"`
	Comment         string                 `json:",omitempty"`
	Created         string                 `json:",omitempty"`
	Container       string                 `json:",omitempty"`
	ContainerConfig *container.Config      `json:",omitempty"`
	DockerVersion   string                 `json:",omitempty"`
	Author          string                 `json:",omitempty"`
	Config          *container.Config      `json:",omitempty"`
	Architecture    string                 `json:",omitempty"`
	Variant         string                 `json:",omitempty"`
	Os              string                 `json:",omitempty"`
	OsVersion       string                 `json:",omitempty"`
	Size            int64                  `json:",omitempty"`
	VirtualSize     int64                  `json:",omitempty"`
	GraphDriver     docker.GraphDriverData `json:",omitempty"`
	RootFS          docker.RootFS          `json:",omitempty"`
	Metadata        docker.ImageMetadata   `json:",omitempty"`

	// Podman extensions
	Digest        digest.Digest                 `json:",omitempty"`
	PodmanVersion string                        `json:",omitempty"`
	ManifestType  string                        `json:",omitempty"`
	User          string                        `json:",omitempty"`
	History       []v1.History                  `json:",omitempty"`
	NamesHistory  []string                      `json:",omitempty"`
	HealthCheck   *manifest.Schema2HealthConfig `json:",omitempty"`
}

func (i *Image) Id() string {
	return i.ID
}

type ImageSummary struct {
	ID          string            `json:"Id"`
	ParentId    string            `json:",omitempty"`
	RepoTags    []string          `json:",omitempty"`
	Created     int64             `json:",omitempty"`
	Size        int64             `json:",omitempty"`
	SharedSize  int               `json:",omitempty"`
	VirtualSize int64             `json:",omitempty"`
	Labels      map[string]string `json:",omitempty"`
	Containers  int               `json:",omitempty"`
	ReadOnly    bool              `json:",omitempty"`
	Dangling    bool              `json:",omitempty"`

	// Podman extensions
	Names        []string `json:",omitempty"`
	Digest       string   `json:",omitempty"`
	Digests      []string `json:",omitempty"`
	ConfigDigest string   `json:",omitempty"`
	History      []string `json:",omitempty"`
}

func (i *ImageSummary) Id() string {
	return i.ID
}

func (i *ImageSummary) IsReadOnly() bool {
	return i.ReadOnly
}

func (i *ImageSummary) IsDangling() bool {
	return i.Dangling
}

type ImageDeleteOptions struct {
	All   bool
	Force bool
}

// ImageDeleteResponse is the response for removing one or more image(s) from storage
// and containers what was untagged vs actually removed
type ImageDeleteReport struct {
	Untagged      []string `json:",omitempty"`
	Deleted       []string `json:",omitempty"`
	Errors        []error
	ImageNotFound error
	ImageInUse    error
}

type ImageHistoryOptions struct{}

type ImageHistoryLayer struct {
	ID        string   `json:"Id"`
	Created   int64    `json:",omitempty"`
	CreatedBy string   `json:",omitempty"`
	Tags      []string `json:",omitempty"`
	Size      int64    `json:",omitempty"`
	Comment   string   `json:",omitempty"`
}

type ImageHistoryReport struct {
	Layers []ImageHistoryLayer
}

type ImageInspectOptions struct {
	TypeObject string `json:",omitempty"`
	Format     string `json:",omitempty"`
	Size       bool   `json:",omitempty"`
	Latest     bool   `json:",omitempty"`
}

type ImageListOptions struct {
	All     bool       `json:"all" schema:"all"`
	Filter  []string   `json:",omitempty"`
	Filters url.Values `json:"filters" schema:"filters"`
}

// type ImageListReport struct {
// 	Images []ImageSummary
// }

type ImagePruneOptions struct {
	All    bool
	Filter ImageFilter
}

type ImagePruneReport struct {
	Report Report
	Size   int64
}
