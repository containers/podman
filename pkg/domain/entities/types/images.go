package types

import (
	"time"

	"github.com/containers/podman/v6/pkg/inspect"
	"github.com/containers/podman/v6/pkg/trust"
)

// swagger:model LibpodImageSummary
type ImageSummary struct {
	ID          string `json:"Id"`
	ParentId    string
	RepoTags    []string
	RepoDigests []string
	Created     int64
	Size        int64
	SharedSize  int
	VirtualSize int64 `json:",omitempty"`
	Labels      map[string]string
	Containers  int
	ReadOnly    bool `json:",omitempty"`
	Dangling    bool `json:",omitempty"`

	// Podman extensions
	Arch    string   `json:",omitempty"`
	Digest  string   `json:",omitempty"`
	History []string `json:",omitempty"`
	// IsManifestList is a ptr so we can distinguish between a true
	// json empty response and false.  the docker compat side needs to return
	// empty; where as the libpod side needs a value of true or false
	IsManifestList *bool    `json:",omitempty"`
	Names          []string `json:",omitempty"`
	Os             string   `json:",omitempty"`
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

type ImageInspectReport struct {
	*inspect.ImageData
}

type ImageTreeReport struct {
	Tree string // TODO: Refactor move presentation work out of server
}

type ImageLoadReport struct {
	Names []string
}

type ImageImportReport struct {
	Id string
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

// ShowTrustReport describes the results of show trust
type ShowTrustReport struct {
	Raw                     []byte
	SystemRegistriesDirPath string
	JSONOutput              []byte
	Policies                []*trust.Policy
}

// ImageMountReport describes the response from image mount
type ImageMountReport struct {
	Id           string
	Name         string
	Repositories []string
	Path         string
}

// ImageUnmountReport describes the response from umounting an image
type ImageUnmountReport struct {
	Err error
	Id  string
}

// FarmInspectReport describes the response from farm inspect
type FarmInspectReport struct {
	NativePlatforms   []string
	EmulatedPlatforms []string
	OS                string
	Arch              string
	Variant           string
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

type ImageHistoryLayer struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created"`
	CreatedBy string    `json:",omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size"`
	Comment   string    `json:"comment,omitempty"`
}

type ImageHistoryReport struct {
	Layers []ImageHistoryLayer
}

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

// ImagePullReportV2 provides a detailed status of a container image pull operation.
type ImagePullReportV2 struct {
	// Status indicates the current state of the image pull operation.
	// Possible values are:
	// - "error":   An error occurred during the pull.
	// - "pulling": The image pull is in progress.
	// - "success": The image pull completed successfully.
	Status string `json:"status,omitempty"`

	// Images contains the unique identifiers of the successfully pulled images.
	// This field is only populated when the Status is "success".
	Images []string `json:"images,omitempty"`

	// Stream provides a human-readable stream of events from the containers/image
	// library. This data is intended for display purposes and should not be parsed
	// by software.
	Stream string `json:"stream,omitempty"`

	// Progress provides detailed information about the download progress of an
	// image layer. This field is only populated when the Status is "pulling".
	Progress *ImagePullProgress `json:"progress,omitempty"`
}

// ImagePullProgress details the download progress of a single image layer.
type ImagePullProgress struct {
	// Current is the number of bytes of the current artifact that have been
	// downloaded so far.
	Current uint64 `json:"current,omitempty"`

	// Total is the total size of the artifact in bytes. A value of -1
	// indicates that the total size is unknown.
	Total int64 `json:"total,omitempty"`

	// Completed indicates whether the pull of the associated layer has finished.
	// This is particularly useful for tracking the status of "partial pulls"
	// where only a portion of the layers may be downloaded.
	Completed bool `json:"completed,omitempty"`

	// ProgressComponentID is the unique identifier for the artifact being downloaded.
	// When the status is `PullComplete`, this field will contain the digest of the blobs at the source.
	// If the status is `Success`, indicating that the entire PullOperation is complete,
	// this field will contain the digest or ID of the artifact in local storage.
	// Note: The values in this field may change in future updates of podman.
	ProgressComponentID string `json:"progressComponentID,omitempty"`

	// ProgressText is a human-readable string that represents the current
	// progress, often as a progress bar or status message. This text is for
	// display purposes only and should not be parsed.
	ProgressText string `json:"progressText,omitempty"`
}

type ImagePushStream struct {
	// ManifestDigest is the digest of the manifest of the pushed image.
	ManifestDigest string `json:"manifestdigest,omitempty"`
	// Stream used to provide push progress
	Stream string `json:"stream,omitempty"`
	// Error contains text of errors from pushing
	Error string `json:"error,omitempty"`
}
