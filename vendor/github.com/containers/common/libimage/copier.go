package libimage

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/signature"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxRetries = 3
	defaultRetryDelay = time.Second
)

// LookupReferenceFunc return an image reference based on the specified one.
// This can be used to pass custom blob caches to the copy operation.
type LookupReferenceFunc func(ref types.ImageReference) (types.ImageReference, error)

// CopyOptions allow for customizing image-copy operations.
type CopyOptions struct {
	// If set, will be used for copying the image.  Fields below may
	// override certain settings.
	SystemContext *types.SystemContext
	// Allows for customizing the source reference lookup.  This can be
	// used to use custom blob caches.
	SourceLookupReferenceFunc LookupReferenceFunc
	// Allows for customizing the destination reference lookup.  This can
	// be used to use custom blob caches.
	DestinationLookupReferenceFunc LookupReferenceFunc

	// containers-auth.json(5) file to use when authenticating against
	// container registries.
	AuthFilePath string
	// Custom path to a blob-info cache.
	BlobInfoCacheDirPath string
	// Path to the certificates directory.
	CertDirPath string
	// Force layer compression when copying to a `dir` transport destination.
	DirForceCompress bool
	// Allow contacting registries over HTTP, or HTTPS with failed TLS
	// verification. Note that this does not affect other TLS connections.
	InsecureSkipTLSVerify types.OptionalBool
	// Maximum number of retries with exponential backoff when facing
	// transient network errors.  A reasonable default is used if not set.
	// Default 3.
	MaxRetries *uint
	// RetryDelay used for the exponential back off of MaxRetries.
	// Default 1 time.Scond.
	RetryDelay *time.Duration
	// ManifestMIMEType is the desired media type the image will be
	// converted to if needed.  Note that it must contain the exact MIME
	// types.  Short forms (e.g., oci, v2s2) used by some tools are not
	// supported.
	ManifestMIMEType string
	// If OciEncryptConfig is non-nil, it indicates that an image should be
	// encrypted.  The encryption options is derived from the construction
	// of EncryptConfig object.  Note: During initial encryption process of
	// a layer, the resultant digest is not known during creation, so
	// newDigestingReader has to be set with validateDigest = false
	OciEncryptConfig *encconfig.EncryptConfig
	// OciEncryptLayers represents the list of layers to encrypt.  If nil,
	// don't encrypt any layers.  If non-nil and len==0, denotes encrypt
	// all layers.  integers in the slice represent 0-indexed layer
	// indices, with support for negative indexing. i.e. 0 is the first
	// layer, -1 is the last (top-most) layer.
	OciEncryptLayers *[]int
	// OciDecryptConfig contains the config that can be used to decrypt an
	// image if it is encrypted if non-nil. If nil, it does not attempt to
	// decrypt an image.
	OciDecryptConfig *encconfig.DecryptConfig
	// Reported to when ProgressInterval has arrived for a single
	// artifact+offset.
	Progress chan types.ProgressProperties
	// If set, allow using the storage transport even if it's disabled by
	// the specified SignaturePolicyPath.
	PolicyAllowStorage bool
	// SignaturePolicyPath to overwrite the default one.
	SignaturePolicyPath string
	// If non-empty, asks for a signature to be added during the copy, and
	// specifies a key ID.
	SignBy string
	// Remove any pre-existing signatures. SignBy will still add a new
	// signature.
	RemoveSignatures bool
	// Writer is used to display copy information including progress bars.
	Writer io.Writer

	// ----- platform -----------------------------------------------------

	// Architecture to use for choosing images.
	Architecture string
	// OS to use for choosing images.
	OS string
	// Variant to use when choosing images.
	Variant string

	// ----- credentials --------------------------------------------------

	// Username to use when authenticating at a container registry.
	Username string
	// Password to use when authenticating at a container registry.
	Password string
	// Credentials is an alternative way to specify credentials in format
	// "username[:password]".  Cannot be used in combination with
	// Username/Password.
	Credentials string
	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// ----- internal -----------------------------------------------------

	// Additional tags when creating or copying a docker-archive.
	dockerArchiveAdditionalTags []reference.NamedTagged
}

// copier is an internal helper to conveniently copy images.
type copier struct {
	imageCopyOptions copy.Options
	retryOptions     retry.RetryOptions
	systemContext    *types.SystemContext
	policyContext    *signature.PolicyContext

	sourceLookup      LookupReferenceFunc
	destinationLookup LookupReferenceFunc
}

var (
	// storageAllowedPolicyScopes overrides the policy for local storage
	// to ensure that we can read images from it.
	storageAllowedPolicyScopes = signature.PolicyTransportScopes{
		"": []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
)

// getDockerAuthConfig extracts a docker auth config from the CopyOptions.  Returns
// nil if no credentials are set.
func (options *CopyOptions) getDockerAuthConfig() (*types.DockerAuthConfig, error) {
	authConf := &types.DockerAuthConfig{IdentityToken: options.IdentityToken}

	if options.Username != "" {
		if options.Credentials != "" {
			return nil, errors.New("username/password cannot be used with credentials")
		}
		authConf.Username = options.Username
		authConf.Password = options.Password
		return authConf, nil
	}

	if options.Credentials != "" {
		split := strings.SplitN(options.Credentials, ":", 2)
		switch len(split) {
		case 1:
			authConf.Username = split[0]
		default:
			authConf.Username = split[0]
			authConf.Password = split[1]
		}
		return authConf, nil
	}

	// We should return nil unless a token was set.  That's especially
	// useful for Podman's remote API.
	if options.IdentityToken != "" {
		return authConf, nil
	}

	return nil, nil
}

// newCopier creates a copier.  Note that fields in options *may* overwrite the
// counterparts of the specified system context.  Please make sure to call
// `(*copier).close()`.
func (r *Runtime) newCopier(options *CopyOptions) (*copier, error) {
	c := copier{}
	c.systemContext = r.systemContextCopy()

	if options.SourceLookupReferenceFunc != nil {
		c.sourceLookup = options.SourceLookupReferenceFunc
	}

	if options.DestinationLookupReferenceFunc != nil {
		c.destinationLookup = options.DestinationLookupReferenceFunc
	}

	if options.InsecureSkipTLSVerify != types.OptionalBoolUndefined {
		c.systemContext.DockerInsecureSkipTLSVerify = options.InsecureSkipTLSVerify
		c.systemContext.OCIInsecureSkipTLSVerify = options.InsecureSkipTLSVerify == types.OptionalBoolTrue
		c.systemContext.DockerDaemonInsecureSkipTLSVerify = options.InsecureSkipTLSVerify == types.OptionalBoolTrue
	}

	c.systemContext.DirForceCompress = c.systemContext.DirForceCompress || options.DirForceCompress

	if options.AuthFilePath != "" {
		c.systemContext.AuthFilePath = options.AuthFilePath
	}

	c.systemContext.DockerArchiveAdditionalTags = options.dockerArchiveAdditionalTags

	if options.Architecture != "" {
		c.systemContext.ArchitectureChoice = options.Architecture
	}
	if options.OS != "" {
		c.systemContext.OSChoice = options.OS
	}
	if options.Variant != "" {
		c.systemContext.VariantChoice = options.Variant
	}

	if options.SignaturePolicyPath != "" {
		c.systemContext.SignaturePolicyPath = options.SignaturePolicyPath
	}

	dockerAuthConfig, err := options.getDockerAuthConfig()
	if err != nil {
		return nil, err
	}
	if dockerAuthConfig != nil {
		c.systemContext.DockerAuthConfig = dockerAuthConfig
	}

	if options.BlobInfoCacheDirPath != "" {
		c.systemContext.BlobInfoCacheDir = options.BlobInfoCacheDirPath
	}

	if options.CertDirPath != "" {
		c.systemContext.DockerCertPath = options.CertDirPath
	}

	policy, err := signature.DefaultPolicy(c.systemContext)
	if err != nil {
		return nil, err
	}

	// Buildah compatibility: even if the policy denies _all_ transports,
	// Buildah still wants the storage to be accessible.
	if options.PolicyAllowStorage {
		policy.Transports[storageTransport.Transport.Name()] = storageAllowedPolicyScopes
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}

	c.policyContext = policyContext

	c.retryOptions.MaxRetry = defaultMaxRetries
	if options.MaxRetries != nil {
		c.retryOptions.MaxRetry = int(*options.MaxRetries)
	}
	c.retryOptions.Delay = defaultRetryDelay
	if options.RetryDelay != nil {
		c.retryOptions.Delay = *options.RetryDelay
	}

	c.imageCopyOptions.Progress = options.Progress
	if c.imageCopyOptions.Progress != nil {
		c.imageCopyOptions.ProgressInterval = time.Second
	}

	c.imageCopyOptions.ForceManifestMIMEType = options.ManifestMIMEType
	c.imageCopyOptions.SourceCtx = c.systemContext
	c.imageCopyOptions.DestinationCtx = c.systemContext
	c.imageCopyOptions.OciEncryptConfig = options.OciEncryptConfig
	c.imageCopyOptions.OciEncryptLayers = options.OciEncryptLayers
	c.imageCopyOptions.OciDecryptConfig = options.OciDecryptConfig
	c.imageCopyOptions.RemoveSignatures = options.RemoveSignatures
	c.imageCopyOptions.SignBy = options.SignBy
	c.imageCopyOptions.ReportWriter = options.Writer

	defaultContainerConfig, err := config.Default()
	if err != nil {
		logrus.Warnf("failed to get container config for copy options: %v", err)
	} else {
		c.imageCopyOptions.MaxParallelDownloads = defaultContainerConfig.Engine.ImageParallelCopies
	}

	return &c, nil
}

// close open resources.
func (c *copier) close() error {
	return c.policyContext.Destroy()
}

// copy the source to the destination.  Returns the bytes of the copied
// manifest which may be used for digest computation.
func (c *copier) copy(ctx context.Context, source, destination types.ImageReference) ([]byte, error) {
	logrus.Debugf("Copying source image %s to destination image %s", source.StringWithinTransport(), destination.StringWithinTransport())

	var err error

	if c.sourceLookup != nil {
		source, err = c.sourceLookup(source)
		if err != nil {
			return nil, err
		}
	}

	if c.destinationLookup != nil {
		destination, err = c.destinationLookup(destination)
		if err != nil {
			return nil, err
		}
	}

	// Buildah compat: used when running in OpenShift.
	sourceInsecure, err := checkRegistrySourcesAllows(source)
	if err != nil {
		return nil, err
	}
	destinationInsecure, err := checkRegistrySourcesAllows(destination)
	if err != nil {
		return nil, err
	}

	// Sanity checks for Buildah.
	if sourceInsecure != nil && *sourceInsecure {
		if c.systemContext.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
			return nil, errors.Errorf("can't require tls verification on an insecured registry")
		}
	}
	if destinationInsecure != nil && *destinationInsecure {
		if c.systemContext.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
			return nil, errors.Errorf("can't require tls verification on an insecured registry")
		}
	}

	var copiedManifest []byte
	f := func() error {
		opts := c.imageCopyOptions
		if sourceInsecure != nil {
			value := types.NewOptionalBool(*sourceInsecure)
			opts.SourceCtx.DockerInsecureSkipTLSVerify = value
		}
		if destinationInsecure != nil {
			value := types.NewOptionalBool(*destinationInsecure)
			opts.DestinationCtx.DockerInsecureSkipTLSVerify = value
		}

		var err error
		copiedManifest, err = copy.Image(ctx, c.policyContext, destination, source, &opts)
		return err
	}
	return copiedManifest, retry.RetryIfNecessary(ctx, f, &c.retryOptions)
}

// checkRegistrySourcesAllows checks the $BUILD_REGISTRY_SOURCES environment
// variable, if it's set.  The contents are expected to be a JSON-encoded
// github.com/openshift/api/config/v1.Image, set by an OpenShift build
// controller that arranged for us to be run in a container.
//
// If set, the insecure return value indicates whether the registry is set to
// be insecure.
//
// NOTE: this functionality is required by Buildah.
func checkRegistrySourcesAllows(dest types.ImageReference) (insecure *bool, err error) {
	registrySources, ok := os.LookupEnv("BUILD_REGISTRY_SOURCES")
	if !ok || registrySources == "" {
		return nil, nil
	}

	logrus.Debugf("BUILD_REGISTRY_SOURCES set %q", registrySources)

	dref := dest.DockerReference()
	if dref == nil || reference.Domain(dref) == "" {
		return nil, nil
	}

	// Use local struct instead of github.com/openshift/api/config/v1 RegistrySources
	var sources struct {
		InsecureRegistries []string `json:"insecureRegistries,omitempty"`
		BlockedRegistries  []string `json:"blockedRegistries,omitempty"`
		AllowedRegistries  []string `json:"allowedRegistries,omitempty"`
	}
	if err := json.Unmarshal([]byte(registrySources), &sources); err != nil {
		return nil, errors.Wrapf(err, "error parsing $BUILD_REGISTRY_SOURCES (%q) as JSON", registrySources)
	}
	blocked := false
	if len(sources.BlockedRegistries) > 0 {
		for _, blockedDomain := range sources.BlockedRegistries {
			if blockedDomain == reference.Domain(dref) {
				blocked = true
			}
		}
	}
	if blocked {
		return nil, errors.Errorf("registry %q denied by policy: it is in the blocked registries list (%s)", reference.Domain(dref), registrySources)
	}
	allowed := true
	if len(sources.AllowedRegistries) > 0 {
		allowed = false
		for _, allowedDomain := range sources.AllowedRegistries {
			if allowedDomain == reference.Domain(dref) {
				allowed = true
			}
		}
	}
	if !allowed {
		return nil, errors.Errorf("registry %q denied by policy: not in allowed registries list (%s)", reference.Domain(dref), registrySources)
	}

	for _, inseureDomain := range sources.InsecureRegistries {
		if inseureDomain == reference.Domain(dref) {
			insecure := true
			return &insecure, nil
		}
	}

	return nil, nil
}
