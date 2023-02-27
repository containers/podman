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
	Generator    string `json:"generator,omitempty"`
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
	Signature          string `json:"signature,omitempty"`
	Sha256             string `json:"sha256"`
	UncompressedSha256 string `json:"uncompressed-sha256,omitempty"`
}

// Images contains images available in cloud providers
type Images struct {
	Aliyun   *ReplicatedImage  `json:"aliyun,omitempty"`
	Aws      *AwsImage         `json:"aws,omitempty"`
	Gcp      *GcpImage         `json:"gcp,omitempty"`
	Ibmcloud *ReplicatedObject `json:"ibmcloud,omitempty"`
	KubeVirt *ContainerImage   `json:"kubevirt,omitempty"`
	PowerVS  *ReplicatedObject `json:"powervs,omitempty"`
}

// ReplicatedImage represents an image in all regions of an AWS-like cloud
type ReplicatedImage struct {
	Regions map[string]SingleImage `json:"regions,omitempty"`
}

// SingleImage represents a globally-accessible image or an image in a
// single region of an AWS-like cloud
type SingleImage struct {
	Release string `json:"release"`
	Image   string `json:"image"`
}

// ContainerImage represents a tagged container image
type ContainerImage struct {
	Release string `json:"release"`
	// Preferred way to reference the image, which might be by tag or digest
	Image     string `json:"image"`
	DigestRef string `json:"digest-ref"`
}

// AwsImage is a typedef for backwards compatibility.
type AwsImage = ReplicatedImage

// AwsRegionImage is a typedef for backwards compatibility.
type AwsRegionImage = SingleImage

// RegionImage is a typedef for backwards compatibility.
type RegionImage = SingleImage

// GcpImage represents a GCP cloud image
type GcpImage struct {
	Release string `json:"release"`
	Project string `json:"project"`
	Family  string `json:"family,omitempty"`
	Name    string `json:"name"`
}

// ReplicatedObject represents an object in all regions of an IBMCloud-like
// cloud
type ReplicatedObject struct {
	Regions map[string]SingleObject `json:"regions,omitempty"`
}

// SingleObject represents a globally-accessible cloud storage object, or
// an object in a single region of an IBMCloud-like cloud
type SingleObject struct {
	Release string `json:"release"`
	Object  string `json:"object"`
	Bucket  string `json:"bucket"`
	Url     string `json:"url"`
}

// RegionObject is a typedef for backwards compatibility.
type RegionObject = SingleObject
