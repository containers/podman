package types

import (
	"io"

	"github.com/containers/podman/v5/pkg/libartifact"
	"github.com/opencontainers/go-digest"
)

type ArtifactInspectReport struct {
	*libartifact.Artifact
	Digest string
}

type ArtifactBlob struct {
	BlobReader   io.Reader
	BlobFilePath string
	FileName     string
}

type ArtifactAddReport struct {
	ArtifactDigest *digest.Digest
}

type ArtifactRemoveReport struct {
	ArtifactDigests []*digest.Digest
}

type ArtifactListReport struct {
	*libartifact.Artifact
}

type ArtifactPushReport struct {
	ArtifactDigest *digest.Digest
}

type ArtifactPullReport struct {
	ArtifactDigest *digest.Digest
}
