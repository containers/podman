// Package release contains APIs for interacting with a
// particular "release".  Avoid this unless you are sure
// you need it.  It's expected that CoreOS users interact
// with streams instead.
package release

import (
	relrhcos "github.com/coreos/stream-metadata-go/release/rhcos"
)

// Index models the release index:
// https://github.com/coreos/fedora-coreos-tracker/tree/master/metadata/release-index
type Index struct {
	Note     string         `json:"note"` // used to note to users not to consume the release metadata index
	Releases []IndexRelease `json:"releases"`
	Metadata Metadata       `json:"metadata"`
	Stream   string         `json:"stream"`
}

// IndexRelease is a "release pointer" from a release index
type IndexRelease struct {
	Commits     []IndexReleaseCommit `json:"commits"`
	Version     string               `json:"version"`
	MetadataURL string               `json:"metadata"`
}

// IndexReleaseCommit describes an ostree commit plus architecture
type IndexReleaseCommit struct {
	Architecture string `json:"architecture"`
	Checksum     string `json:"checksum"`
}

// Release contains details from release.json
type Release struct {
	Release       string          `json:"release"`
	Stream        string          `json:"stream"`
	Metadata      Metadata        `json:"metadata"`
	Architectures map[string]Arch `json:"architectures"`
}

// Metadata is common metadata that contains last-modified
type Metadata struct {
	LastModified string `json:"last-modified"`
}

// Arch release details
type Arch struct {
	Commit               string               `json:"commit"`
	Media                Media                `json:"media"`
	RHELCoreOSExtensions *relrhcos.Extensions `json:"rhel-coreos-extensions,omitempty"`
}

// Media contains release details for various platforms
type Media struct {
	Aliyun       *PlatformBase `json:"aliyun"`
	Aws          *PlatformAws  `json:"aws"`
	Azure        *PlatformBase `json:"azure"`
	Digitalocean *PlatformBase `json:"digitalocean"`
	Exoscale     *PlatformBase `json:"exoscale"`
	Gcp          *PlatformGcp  `json:"gcp"`
	Ibmcloud     *PlatformBase `json:"ibmcloud"`
	Metal        *PlatformBase `json:"metal"`
	Openstack    *PlatformBase `json:"openstack"`
	Qemu         *PlatformBase `json:"qemu"`
	Vmware       *PlatformBase `json:"vmware"`
	Vultr        *PlatformBase `json:"vultr"`
}

// PlatformBase with no cloud images
type PlatformBase struct {
	Artifacts map[string]ImageFormat `json:"artifacts"`
}

// PlatformAws contains AWS image information
type PlatformAws struct {
	PlatformBase
	Images map[string]CloudImage `json:"images"`
}

// PlatformGcp GCP image detail
type PlatformGcp struct {
	PlatformBase
	Image *GcpImage `json:"image"`
}

// ImageFormat contains all artifacts for a single OS image
type ImageFormat struct {
	Disk      *Artifact `json:"disk,omitempty"`
	Kernel    *Artifact `json:"kernel,omitempty"`
	Initramfs *Artifact `json:"initramfs,omitempty"`
	Rootfs    *Artifact `json:"rootfs,omitempty"`
}

// Artifact represents one image file, plus its metadata
type Artifact struct {
	Location           string `json:"location"`
	Signature          string `json:"signature"`
	Sha256             string `json:"sha256"`
	UncompressedSha256 string `json:"uncompressed-sha256,omitempty"`
}

// CloudImage generic image detail
type CloudImage struct {
	Image string `json:"image"`
}

// GcpImage represents a GCP cloud image
type GcpImage struct {
	Project string `json:"project,omitempty"`
	Family  string `json:"family,omitempty"`
	Name    string `json:"name,omitempty"`
}
