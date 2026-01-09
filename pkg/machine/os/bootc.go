package os

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/transports/alltransports"
)

// BootcHost represents the top-level bootc status structure
type BootcHost struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Metadata   BootcMetadata `json:"metadata"`
	Spec       BootcSpec     `json:"spec"`
	Status     BootcStatus   `json:"status"`
}

// BootcMetadata contains metadata about the host
type BootcMetadata struct {
	Name string `json:"name"`
}

// BootcSpec contains the specification for the bootc host
type BootcSpec struct {
	BootOrder string   `json:"bootOrder"`
	Image     ImageRef `json:"image"`
}

// ImageRef represents a container image reference
type ImageRef struct {
	Image     string `json:"image"`
	Transport string `json:"transport"`
}

// BootcStatus contains the current status of the bootc host
type BootcStatus struct {
	Booted         *BootEntry `json:"booted"`
	Rollback       *BootEntry `json:"rollback"`
	Staged         *BootEntry `json:"staged"`
	RollbackQueued bool       `json:"rollbackQueued"`
	Type           string     `json:"type"`
}

// BootEntry represents a boot entry (booted, rollback, or staged)
type BootEntry struct {
	CachedUpdate      *ImageStatus `json:"cachedUpdate"`
	Image             ImageStatus  `json:"image"`
	Incompatible      bool         `json:"incompatible"`
	Ostree            OstreeInfo   `json:"ostree"`
	Pinned            bool         `json:"pinned"`
	SoftRebootCapable bool         `json:"softRebootCapable"`
	Store             string       `json:"store"`
}

// ImageStatus contains detailed information about a container image
type ImageStatus struct {
	Architecture string          `json:"architecture"`
	Image        ImageRefWithSig `json:"image"`
	ImageDigest  string          `json:"imageDigest"`
	Timestamp    time.Time       `json:"timestamp"`
	Version      string          `json:"version"`
}

// ImageRefWithSig is an image reference that may include a signature
type ImageRefWithSig struct {
	Image     string     `json:"image"`
	Transport string     `json:"transport"`
	Signature *Signature `json:"signature,omitempty"`
}

// Signature contains signature information for an image
type Signature struct {
	OstreeRemote string `json:"ostreeRemote"`
}

// OstreeInfo contains OSTree-specific information
type OstreeInfo struct {
	Checksum     string `json:"checksum"`
	DeploySerial int    `json:"deploySerial"`
	Stateroot    string `json:"stateroot"`
}

// GetBootedImageRef returns the booted image reference (status -> booted -> image -> image)
// Returns an empty string if booted is nil
func (b *BootcHost) GetBootedImageRef() string {
	if b.Status.Booted == nil {
		return ""
	}
	return b.Status.Booted.Image.Image.Image
}

// GetBootedOSTreeChecksum returns the booted OSTree checksum
// Returns an empty string if booted is nil
func (b *BootcHost) GetBootedOSTreeChecksum() string {
	if b.Status.Booted == nil {
		return ""
	}
	return b.Status.Booted.Ostree.Checksum
}

func (b *BootcHost) getLocalOSOCIInfo() (reference.Named, *semver.Version, error) {
	ir, err := alltransports.ParseImageName("docker://" + b.GetBootedImageRef())
	if err != nil {
		return nil, nil, err
	}
	ref, err := reference.Parse(ir.DockerReference().String())
	if err != nil {
		return nil, nil, err
	}
	named, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, nil, errors.New("failed to parse ostree origin")
	}
	tagAsVersion, err := semver.ParseTolerant(named.Tag())
	return named, &tagAsVersion, err
}

func (b *BootcHost) getLocalOsImageDigest() (digest.Digest, error) {
	args := []string{"container", "image", "metadata", "--repo", "/ostree/repo", "docker://" + b.GetBootedImageRef()}
	logrus.Debugf("executing : ostree %s", strings.Join(args, " "))
	cmd := exec.Command("ostree", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	localOs, err := manifest.FromBlob(out, v1.MediaTypeImageManifest)
	if err != nil {
		return "", err
	}
	return localOs.ConfigInfo().Digest, nil
}

// newBootcHost creates a new BootcHost by executing 'sudo bootc status --format json'
// and unmarshaling the output
func newBootcHost() (*BootcHost, error) {
	cmd := exec.Command("sudo", "bootc", "status", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var host BootcHost
	if err := json.Unmarshal(output, &host); err != nil {
		return nil, err
	}

	return &host, nil
}
