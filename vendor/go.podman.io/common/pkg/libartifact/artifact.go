package libartifact

import (
	"encoding/json"

	"github.com/opencontainers/go-digest"
	"go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/manifest"
)

type Artifact struct {
	// Manifest is the OCI manifest for the artifact with the name.
	// In a valid artifact the Manifest is guaranteed to not be nil.
	Manifest *manifest.OCI1
	Name     string
}

// TotalSizeBytes returns the total bytes of the all the artifact layers.
func (a *Artifact) TotalSizeBytes() int64 {
	var s int64
	for _, layer := range a.Manifest.Layers {
		s += layer.Size
	}
	return s
}

// GetName returns the "name" or "image reference" of the artifact.
func (a *Artifact) GetName() (string, error) {
	if a.Name != "" {
		return a.Name, nil
	}
	// We don't have a concept of None for artifacts yet, but if we do,
	// then we should probably not error but return `None`
	return "", types.ErrArtifactUnamed
}

// SetName is a accessor for setting the artifact name
// Note: long term this may not be needed, and we would
// be comfortable with simply using the exported field
// called Name.
func (a *Artifact) SetName(name string) {
	a.Name = name
}

func (a *Artifact) GetDigest() (*digest.Digest, error) {
	b, err := json.Marshal(a.Manifest)
	if err != nil {
		return nil, err
	}
	artifactDigest := digest.FromBytes(b)
	return &artifactDigest, nil
}

type ArtifactList []*Artifact
