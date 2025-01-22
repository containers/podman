package libartifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/opencontainers/go-digest"
)

type Artifact struct {
	Manifests []manifest.OCI1
	Name      string
}

// TotalSizeBytes returns the total bytes of the all the artifact layers
func (a *Artifact) TotalSizeBytes() int64 {
	var s int64
	for _, artifact := range a.Manifests {
		for _, layer := range artifact.Layers {
			s += layer.Size
		}
	}
	return s
}

// GetName returns the "name" or "image reference" of the artifact
func (a *Artifact) GetName() (string, error) {
	if a.Name != "" {
		return a.Name, nil
	}
	// We don't have a concept of None for artifacts yet, but if we do,
	// then we should probably not error but return `None`
	return "", errors.New("artifact is unnamed")
}

// SetName is a accessor for setting the artifact name
// Note: long term this may not be needed, and we would
// be comfortable with simply using the exported field
// called Name
func (a *Artifact) SetName(name string) {
	a.Name = name
}

func (a *Artifact) GetDigest() (*digest.Digest, error) {
	if len(a.Manifests) > 1 {
		return nil, fmt.Errorf("not supported: multiple manifests found in artifact")
	}
	if len(a.Manifests) < 1 {
		return nil, fmt.Errorf("not supported: no manifests found in artifact")
	}
	b, err := json.Marshal(a.Manifests[0])
	if err != nil {
		return nil, err
	}
	artifactDigest := digest.FromBytes(b)
	return &artifactDigest, nil
}

type ArtifactList []*Artifact

// GetByNameOrDigest returns an artifact, if present, by a given name
// Returns an error if not found
func (al ArtifactList) GetByNameOrDigest(nameOrDigest string) (*Artifact, bool, error) {
	// This is the hot route through
	for _, artifact := range al {
		if artifact.Name == nameOrDigest {
			return artifact, false, nil
		}
	}
	// Before giving up, check by digest
	for _, artifact := range al {
		// TODO Here we have to assume only a single manifest for the artifact; this will
		// need to evolve
		if len(artifact.Manifests) > 0 {
			artifactDigest, err := artifact.GetDigest()
			if err != nil {
				return nil, false, err
			}
			// If the artifact's digest matches or is a prefix of ...
			if artifactDigest.Encoded() == nameOrDigest || strings.HasPrefix(artifactDigest.Encoded(), nameOrDigest) {
				return artifact, true, nil
			}
		}
	}
	return nil, false, fmt.Errorf("no artifact found with name or digest of %s", nameOrDigest)
}
