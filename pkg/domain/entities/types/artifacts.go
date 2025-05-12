package types

import "github.com/containers/podman/v5/pkg/libartifact"

type ArtifactInspectReport struct {
	*libartifact.Artifact
	Digest string
}
