// Package stream models a CoreOS "stream", which is
// a description of the recommended set of binary images for CoreOS.   Use
// this API to find cloud images, bare metal disk images, etc.
package stream

import (
	"github.com/coreos/stream-metadata-go/stream/rhcos"
)

// Stream contains artifacts available in a stream
type Stream struct {
	Stream        string          `json:"stream"`
	Metadata      Metadata        `json:"metadata"`
	Architectures map[string]Arch `json:"architectures"`
}

// Metadata for a release or stream
type Metadata struct {
	LastModified string `json:"last-modified"`
}

// Arch contains release details for a particular hardware architecture
type Arch struct {
	Artifacts map[string]PlatformArtifacts `json:"artifacts"`
	Images    Images                       `json:"images,omitempty"`
	// RHELCoreOSExtensions is data specific to Red Hat Enterprise Linux CoreOS
	RHELCoreOSExtensions *rhcos.Extensions `json:"rhel-coreos-extensions,omitempty"`
}

// PlatformArtifacts contains images for a platform
type PlatformArtifacts struct {
	Release string                 `json:"release"`
	Formats map[string]ImageFormat `json:"formats"`
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

// Images contains images available in cloud providers
type Images struct {
	Aws *AwsImage `json:"aws,omitempty"`
	Gcp *GcpImage `json:"gcp,omitempty"`
}

// AwsImage represents an image across all AWS regions
type AwsImage struct {
	Regions map[string]AwsRegionImage `json:"regions,omitempty"`
}

// AwsRegionImage represents an image in one AWS region
type AwsRegionImage struct {
	Release string `json:"release"`
	Image   string `json:"image"`
}

// GcpImage represents a GCP cloud image
type GcpImage struct {
	Project string `json:"project,omitempty"`
	Family  string `json:"family,omitempty"`
	Name    string `json:"name,omitempty"`
}
