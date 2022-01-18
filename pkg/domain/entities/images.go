package entities

import (
	"net/url"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/inspect"
	"github.com/containers/podman/v4/pkg/trust"
	"github.com/docker/docker/api/types/container"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Image struct {
	ID              string            `json:"Id"`
	RepoTags        []string          `json:",omitempty"`
	RepoDigests     []string          `json:",omitempty"`
	Parent          string            `json:",omitempty"`
	Comment         string            `json:",omitempty"`
	Created         string            `json:",omitempty"`
	Container       string            `json:",omitempty"`
	ContainerConfig *container.Config `json:",omitempty"`
	DockerVersion   string            `json:",omitempty"`
	Author          string            `json:",omitempty"`
	Config          *container.Config `json:",omitempty"`
	Architecture    string            `json:",omitempty"`
	Variant         string            `json:",omitempty"`
	Os              string            `json:",omitempty"`
	OsVersion       string            `json:",omitempty"`
	Size            int64             `json:",omitempty"`
	VirtualSize     int64             `json:",omitempty"`
	GraphDriver     string            `json:",omitempty"`
	RootFS          string            `json:",omitempty"`
	Metadata        string            `json:",omitempty"`

	// Podman extensions
	Digest        digest.Digest                 `json:",omitempty"`
	PodmanVersion string                        `json:",omitempty"`
	ManifestType  string                        `json:",omitempty"`
	User          string                        `json:",omitempty"`
	History       []v1.History                  `json:",omitempty"`
	NamesHistory  []string                      `json:",omitempty"`
	HealthCheck   *manifest.Schema2HealthConfig `json:",omitempty"`
}

func (i *Image) Id() string { // nolint
	return i.ID
}

// swagger:model LibpodImageSummary
type ImageSummary struct {
	ID          string `json:"Id"`
	ParentId    string // nolint
	RepoTags    []string
	RepoDigests []string
	Created     int64
	Size        int64
	SharedSize  int
	VirtualSize int64
	Labels      map[string]string
	Containers  int
	ReadOnly    bool `json:",omitempty"`
	Dangling    bool `json:",omitempty"`

	// Podman extensions
	Names        []string `json:",omitempty"`
	Digest       string   `json:",omitempty"`
	ConfigDigest string   `json:",omitempty"`
	History      []string `json:",omitempty"`
}

func (i *ImageSummary) Id() string { // nolint
	return i.ID
}

func (i *ImageSummary) IsReadOnly() bool {
	return i.ReadOnly
}

func (i *ImageSummary) IsDangling() bool {
	return i.Dangling
}

// ImageRemoveOptions can be used to alter image removal.
type ImageRemoveOptions struct {
	// All will remove all images.
	All bool
	// Foce will force image removal including containers using the images.
	Force bool
	// Confirms if given name is a manifest list and removes it, otherwise returns error.
	LookupManifest bool
}

// ImageRemoveReport is the response for removing one or more image(s) from storage
// and images what was untagged vs actually removed.
type ImageRemoveReport struct {
	// Deleted images.
	Deleted []string `json:",omitempty"`
	// Untagged images. Can be longer than Deleted.
	Untagged []string `json:",omitempty"`
	// ExitCode describes the exit codes as described in the `podman rmi`
	// man page.
	ExitCode int
}

type ImageHistoryOptions struct{}

type ImageHistoryLayer struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created,omitempty"`
	CreatedBy string    `json:",omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size"`
	Comment   string    `json:"comment,omitempty"`
}

type ImageHistoryReport struct {
	Layers []ImageHistoryLayer
}

// ImagePullOptions are the arguments for pulling images.
type ImagePullOptions struct {
	// AllTags can be specified to pull all tags of an image. Note
	// that this only works if the image does not include a tag.
	AllTags bool
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile string
	// CertDir is the path to certificate directories.  Ignored for remote
	// calls.
	CertDir string
	// Username for authenticating against the registry.
	Username string
	// Password for authenticating against the registry.
	Password string
	// Arch will overwrite the local architecture for image pulls.
	Arch string
	// OS will overwrite the local operating system (OS) for image
	// pulls.
	OS string
	// Variant will overwrite the local variant for image pulls.
	Variant string
	// Quiet can be specified to suppress pull progress when pulling.  Ignored
	// for remote calls.
	Quiet bool
	// SignaturePolicy to use when pulling.  Ignored for remote calls.
	SignaturePolicy string
	// SkipTLSVerify to skip HTTPS and certificate verification.
	SkipTLSVerify types.OptionalBool
	// PullPolicy whether to pull new image
	PullPolicy config.PullPolicy
}

// ImagePullReport is the response from pulling one or more images.
type ImagePullReport struct {
	// Stream used to provide output from c/image
	Stream string `json:"stream,omitempty"`
	// Error contains text of errors from c/image
	Error string `json:"error,omitempty"`
	// Images contains the ID's of the images pulled
	Images []string `json:"images,omitempty"`
	// ID contains image id (retained for backwards compatibility)
	ID string `json:"id,omitempty"`
}

// ImagePushOptions are the arguments for pushing images.
type ImagePushOptions struct {
	// All indicates that all images referenced in an manifest list should be pushed
	All bool
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile string
	// CertDir is the path to certificate directories.  Ignored for remote
	// calls.
	CertDir string
	// Compress tarball image layers when pushing to a directory using the 'dir'
	// transport. Default is same compression type as source. Ignored for remote
	// calls.
	Compress bool
	// Username for authenticating against the registry.
	Username string
	// Password for authenticating against the registry.
	Password string
	// DigestFile, after copying the image, write the digest of the resulting
	// image to the file.  Ignored for remote calls.
	DigestFile string
	// Format is the Manifest type (oci, v2s1, or v2s2) to use when pushing an
	// image. Default is manifest type of source, with fallbacks.
	// Ignored for remote calls.
	Format string
	// Quiet can be specified to suppress pull progress when pulling.  Ignored
	// for remote calls.
	Quiet bool
	// Rm indicates whether to remove the manifest list if push succeeds
	Rm bool
	// RemoveSignatures, discard any pre-existing signatures in the image.
	// Ignored for remote calls.
	RemoveSignatures bool
	// SignaturePolicy to use when pulling.  Ignored for remote calls.
	SignaturePolicy string
	// SignBy adds a signature at the destination using the specified key.
	// Ignored for remote calls.
	SignBy string
	// SkipTLSVerify to skip HTTPS and certificate verification.
	SkipTLSVerify types.OptionalBool
	// Progress to get progress notifications
	Progress chan types.ProgressProperties
	// CompressionFormat is the format to use for the compression of the blobs
	CompressionFormat string
}

// ImageSearchOptions are the arguments for searching images.
type ImageSearchOptions struct {
	// Authfile is the path to the authentication file. Ignored for remote
	// calls.
	Authfile string
	// Filters for the search results.
	Filters []string
	// Limit the number of results.
	Limit int
	// SkipTLSVerify to skip  HTTPS and certificate verification.
	SkipTLSVerify types.OptionalBool
	// ListTags search the available tags of the repository
	ListTags bool
}

// ImageSearchReport is the response from searching images.
type ImageSearchReport struct {
	// Index is the image index (e.g., "docker.io" or "quay.io")
	Index string
	// Name is the canonical name of the image (e.g., "docker.io/library/alpine").
	Name string
	// Description of the image.
	Description string
	// Stars is the number of stars of the image.
	Stars int
	// Official indicates if it's an official image.
	Official string
	// Automated indicates if the image was created by an automated build.
	Automated string
	// Tag is the repository tag
	Tag string
}

// Image List Options
type ImageListOptions struct {
	All    bool     `json:"all" schema:"all"`
	Filter []string `json:"Filter,omitempty"`
}

type ImagePruneOptions struct {
	All      bool     `json:"all" schema:"all"`
	External bool     `json:"external" schema:"external"`
	Filter   []string `json:"filter" schema:"filter"`
}

type ImageTagOptions struct{}
type ImageUntagOptions struct{}

// ImageInspectReport is the data when inspecting an image.
type ImageInspectReport struct {
	*inspect.ImageData
}

type ImageLoadOptions struct {
	Input           string
	Quiet           bool
	SignaturePolicy string
}

type ImageLoadReport struct {
	Names []string
}

type ImageImportOptions struct {
	Architecture    string
	Changes         []string
	Message         string
	OS              string
	Quiet           bool
	Reference       string
	SignaturePolicy string
	Source          string
	SourceIsURL     bool
}

type ImageImportReport struct {
	Id string // nolint
}

// ImageSaveOptions provide options for saving images.
type ImageSaveOptions struct {
	// Compress layers when saving to a directory.
	Compress bool
	// Format of saving the image: oci-archive, oci-dir (directory with oci
	// manifest type), docker-archive, docker-dir (directory with v2s2
	// manifest type).
	Format string
	// MultiImageArchive denotes if the created archive shall include more
	// than one image.  Additional tags will be interpreted as references
	// to images which are added to the archive.
	MultiImageArchive bool
	// Accept uncompressed layers when copying OCI images.
	OciAcceptUncompressedLayers bool
	// Output - write image to the specified path.
	Output string
	// Quiet - suppress output when copying images
	Quiet bool
}

// ImageScpOptions provide options for securely copying images to and from a remote host
type ImageScpOptions struct {
	// Remote determines if this entity is operating on a remote machine
	Remote bool `json:"remote,omitempty"`
	// File is the input/output file for the save and load Operation
	File string `json:"file,omitempty"`
	// Quiet Determines if the save and load operation will be done quietly
	Quiet bool `json:"quiet,omitempty"`
	// Image is the image the user is providing to save and load
	Image string `json:"image,omitempty"`
	// User is used in conjunction with Transfer to determine if a valid user was given to save from/load into
	User string `json:"user,omitempty"`
}

// ImageScpConnections provides the ssh related information used in remote image transfer
type ImageScpConnections struct {
	// Connections holds the raw string values for connections (ssh or unix)
	Connections []string
	// URI contains the ssh connection URLs to be used by the client
	URI []*url.URL
	// Identities contains ssh identity keys to be used by the client
	Identities []string
}

// ImageTreeOptions provides options for ImageEngine.Tree()
type ImageTreeOptions struct {
	WhatRequires bool // Show all child images and layers of the specified image
}

// ImageTreeReport provides results from ImageEngine.Tree()
type ImageTreeReport struct {
	Tree string // TODO: Refactor move presentation work out of server
}

// ShowTrustOptions are the cli options for showing trust
type ShowTrustOptions struct {
	JSON         bool
	PolicyPath   string
	Raw          bool
	RegistryPath string
}

// ShowTrustReport describes the results of show trust
type ShowTrustReport struct {
	Raw                     []byte
	SystemRegistriesDirPath string
	JSONOutput              []byte
	Policies                []*trust.Policy
}

// SetTrustOptions describes the CLI options for setting trust
type SetTrustOptions struct {
	PolicyPath  string
	PubKeysFile []string
	Type        string
}

// SignOptions describes input options for the CLI signing
type SignOptions struct {
	Directory string
	SignBy    string
	CertDir   string
	Authfile  string
	All       bool
}

// SignReport describes the result of signing
type SignReport struct{}

// ImageMountOptions describes the input values for mounting images
// in the CLI
type ImageMountOptions struct {
	All    bool
	Format string
}

// ImageUnmountOptions are the options from the cli for unmounting
type ImageUnmountOptions struct {
	All   bool
	Force bool
}

// ImageMountReport describes the response from image mount
type ImageMountReport struct {
	Err          error
	Id           string // nolint
	Name         string
	Repositories []string
	Path         string
}

// ImageUnmountReport describes the response from umounting an image
type ImageUnmountReport struct {
	Err error
	Id  string // nolint
}
