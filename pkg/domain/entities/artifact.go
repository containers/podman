package entities

import (
	"io"

	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	entityTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/libartifact"
	"github.com/opencontainers/go-digest"
)

type ArtifactAddOptions struct {
	Annotations  map[string]string
	ArtifactType string
	Append       bool
	FileType     string
}

type ArtifactExtractOptions struct {
	// Title annotation value to extract only a single blob matching that name.
	// Conflicts with Digest. Optional.
	Title string
	// Digest of the blob to extract.
	// Conflicts with Title. Optional.
	Digest string
}

type ArtifactBlob struct {
	BlobReader   io.Reader
	BlobFilePath string
	FileName     string
}

type ArtifactInspectOptions struct {
	Remote bool
}

type ArtifactListOptions struct {
	ImagePushOptions
}

type ArtifactPullOptions struct {
	// containers-auth.json(5) file to use when authenticating against
	// container registries.
	AuthFilePath string
	// Path to the certificates directory.
	CertDirPath string
	// Allow contacting registries over HTTP, or HTTPS with failed TLS
	// verification. Note that this does not affect other TLS connections.
	InsecureSkipTLSVerify types.OptionalBool
	// Maximum number of retries with exponential backoff when facing
	// transient network errors.
	// Default 3.
	MaxRetries *uint
	// RetryDelay used for the exponential back off of MaxRetries.
	// Default 1 time.Second.
	RetryDelay string
	// OciDecryptConfig contains the config that can be used to decrypt an image if it is
	// encrypted if non-nil. If nil, it does not attempt to decrypt an image.
	OciDecryptConfig *encconfig.DecryptConfig
	// Quiet can be specified to suppress pull progress when pulling.  Ignored
	// for remote calls. //TODO: Verify that claim
	Quiet bool
	// SignaturePolicyPath to overwrite the default one.
	SignaturePolicyPath string
	// Writer is used to display copy information including progress bars.
	Writer io.Writer

	// ----- credentials --------------------------------------------------

	// Username to use when authenticating at a container registry.
	Username string
	// Password to use when authenticating at a container registry.
	Password string
	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`
}

type ArtifactPushOptions struct {
	ImagePushOptions
	CredentialsCLI             string
	DigestFile                 string
	EncryptLayers              []int
	EncryptionKeys             []string
	SignBySigstoreParamFileCLI string
	SignPassphraseFileCLI      string
	TLSVerifyCLI               bool // CLI only
}

type ArtifactRemoveOptions struct {
	// Remove all artifacts
	All bool
}

type ArtifactPullReport struct {
	ArtifactDigest *digest.Digest
}

type ArtifactPushReport struct {
	ArtifactDigest *digest.Digest
}

type ArtifactInspectReport = entityTypes.ArtifactInspectReport

type ArtifactListReport struct {
	*libartifact.Artifact
}

type ArtifactAddReport struct {
	ArtifactDigest *digest.Digest
}

type ArtifactRemoveReport struct {
	ArtifactDigests []*digest.Digest
}
