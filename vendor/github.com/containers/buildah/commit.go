package buildah

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/containers/buildah/pkg/blobcache"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/stringid"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// BuilderIdentityAnnotation is the name of the annotation key containing
	// the name and version of the producer of the image stored as an
	// annotation on commit.
	BuilderIdentityAnnotation = "io.buildah.version"
)

// CommitOptions can be used to alter how an image is committed.
type CommitOptions struct {
	// PreferredManifestType is the preferred type of image manifest.  The
	// image configuration format will be of a compatible type.
	PreferredManifestType string
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// AdditionalTags is a list of additional names to add to the image, if
	// the transport to which we're writing the image gives us a way to add
	// them.
	AdditionalTags []string
	// ReportWriter is an io.Writer which will be used to log the writing
	// of the new image.
	ReportWriter io.Writer
	// HistoryTimestamp is the timestamp used when creating new items in the
	// image's history.  If unset, the current time will be used.
	HistoryTimestamp *time.Time
	// github.com/containers/image/types SystemContext to hold credentials
	// and other authentication/authorization information.
	SystemContext *types.SystemContext
	// IIDFile tells the builder to write the image ID to the specified file
	IIDFile string
	// Squash tells the builder to produce an image with a single layer
	// instead of with possibly more than one layer.
	Squash bool
	// BlobDirectory is the name of a directory in which we'll look for
	// prebuilt copies of layer blobs that we might otherwise need to
	// regenerate from on-disk layers.  If blobs are available, the
	// manifest of the new image will reference the blobs rather than
	// on-disk layers.
	BlobDirectory string
	// EmptyLayer tells the builder to omit the diff for the working
	// container.
	EmptyLayer bool
	// OmitTimestamp forces epoch 0 as created timestamp to allow for
	// deterministic, content-addressable builds.
	// Deprecated use HistoryTimestamp instead.
	OmitTimestamp bool
	// SignBy is the fingerprint of a GPG key to use for signing the image.
	SignBy string
	// MaxRetries is the maximum number of attempts we'll make to commit
	// the image to an external registry if the first attempt fails.
	MaxRetries int
	// RetryDelay is how long to wait before retrying a commit attempt to a
	// registry.
	RetryDelay time.Duration
	// OciEncryptConfig when non-nil indicates that an image should be encrypted.
	// The encryption options is derived from the construction of EncryptConfig object.
	OciEncryptConfig *encconfig.EncryptConfig
	// OciEncryptLayers represents the list of layers to encrypt.
	// If nil, don't encrypt any layers.
	// If non-nil and len==0, denotes encrypt all layers.
	// integers in the slice represent 0-indexed layer indices, with support for negative
	// indexing. i.e. 0 is the first layer, -1 is the last (top-most) layer.
	OciEncryptLayers *[]int
}

// PushOptions can be used to alter how an image is copied somewhere.
type PushOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to log the writing
	// of the new image.
	ReportWriter io.Writer
	// Store is the local storage store which holds the source image.
	Store storage.Store
	// github.com/containers/image/types SystemContext to hold credentials
	// and other authentication/authorization information.
	SystemContext *types.SystemContext
	// ManifestType is the format to use when saving the image using the 'dir' transport
	// possible options are oci, v2s1, and v2s2
	ManifestType string
	// BlobDirectory is the name of a directory in which we'll look for
	// prebuilt copies of layer blobs that we might otherwise need to
	// regenerate from on-disk layers, substituting them in the list of
	// blobs to copy whenever possible.
	BlobDirectory string
	// Quiet is a boolean value that determines if minimal output to
	// the user will be displayed, this is best used for logging.
	// The default is false.
	Quiet bool
	// SignBy is the fingerprint of a GPG key to use for signing the image.
	SignBy string
	// RemoveSignatures causes any existing signatures for the image to be
	// discarded for the pushed copy.
	RemoveSignatures bool
	// MaxRetries is the maximum number of attempts we'll make to push any
	// one image to the external registry if the first attempt fails.
	MaxRetries int
	// RetryDelay is how long to wait before retrying a push attempt.
	RetryDelay time.Duration
	// OciEncryptConfig when non-nil indicates that an image should be encrypted.
	// The encryption options is derived from the construction of EncryptConfig object.
	OciEncryptConfig *encconfig.EncryptConfig
	// OciEncryptLayers represents the list of layers to encrypt.
	// If nil, don't encrypt any layers.
	// If non-nil and len==0, denotes encrypt all layers.
	// integers in the slice represent 0-indexed layer indices, with support for negative
	// indexing. i.e. 0 is the first layer, -1 is the last (top-most) layer.
	OciEncryptLayers *[]int
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

// checkRegistrySourcesAllows checks the $BUILD_REGISTRY_SOURCES environment
// variable, if it's set.  The contents are expected to be a JSON-encoded
// github.com/openshift/api/config/v1.Image, set by an OpenShift build
// controller that arranged for us to be run in a container.
func checkRegistrySourcesAllows(forWhat string, dest types.ImageReference) (insecure bool, err error) {
	transport := dest.Transport()
	if transport == nil {
		return false, nil
	}
	if transport.Name() != docker.Transport.Name() {
		return false, nil
	}
	dref := dest.DockerReference()
	if dref == nil || reference.Domain(dref) == "" {
		return false, nil
	}

	if registrySources, ok := os.LookupEnv("BUILD_REGISTRY_SOURCES"); ok && len(registrySources) > 0 {
		// Use local struct instead of github.com/openshift/api/config/v1 RegistrySources
		var sources struct {
			InsecureRegistries []string `json:"insecureRegistries,omitempty"`
			BlockedRegistries  []string `json:"blockedRegistries,omitempty"`
			AllowedRegistries  []string `json:"allowedRegistries,omitempty"`
		}
		if err := json.Unmarshal([]byte(registrySources), &sources); err != nil {
			return false, errors.Wrapf(err, "error parsing $BUILD_REGISTRY_SOURCES (%q) as JSON", registrySources)
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
			return false, errors.Errorf("%s registry at %q denied by policy: it is in the blocked registries list", forWhat, reference.Domain(dref))
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
			return false, errors.Errorf("%s registry at %q denied by policy: not in allowed registries list", forWhat, reference.Domain(dref))
		}
		if len(sources.InsecureRegistries) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// Commit writes the contents of the container, along with its updated
// configuration, to a new image in the specified location, and if we know how,
// add any additional tags that were specified. Returns the ID of the new image
// if commit was successful and the image destination was local.
func (b *Builder) Commit(ctx context.Context, dest types.ImageReference, options CommitOptions) (string, reference.Canonical, digest.Digest, error) {
	var imgID string

	// If we weren't given a name, build a destination reference using a
	// temporary name that we'll remove later.  The correct thing to do
	// would be to read the manifest and configuration blob, and ask the
	// manifest for the ID that we'd give the image, but that computation
	// requires that we know the digests of the layer blobs, which we don't
	// want to compute here because we'll have to do it again when
	// cp.Image() instantiates a source image, and we don't want to do the
	// work twice.
	if options.OmitTimestamp {
		if options.HistoryTimestamp != nil {
			return imgID, nil, "", errors.Errorf("OmitTimestamp ahd HistoryTimestamp can not be used together")
		}
		timestamp := time.Unix(0, 0).UTC()
		options.HistoryTimestamp = &timestamp
	}
	nameToRemove := ""
	if dest == nil {
		nameToRemove = stringid.GenerateRandomID() + "-tmp"
		dest2, err := is.Transport.ParseStoreReference(b.store, nameToRemove)
		if err != nil {
			return imgID, nil, "", errors.Wrapf(err, "error creating temporary destination reference for image")
		}
		dest = dest2
	}

	systemContext := getSystemContext(b.store, options.SystemContext, options.SignaturePolicyPath)

	blocked, err := isReferenceBlocked(dest, systemContext)
	if err != nil {
		return "", nil, "", errors.Wrapf(err, "error checking if committing to registry for %q is blocked", transports.ImageName(dest))
	}
	if blocked {
		return "", nil, "", errors.Errorf("commit access to registry for %q is blocked by configuration", transports.ImageName(dest))
	}

	// Load the system signing policy.
	commitPolicy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return "", nil, "", errors.Wrapf(err, "error obtaining default signature policy")
	}
	// Override the settings for local storage to make sure that we can always read the source "image".
	commitPolicy.Transports[is.Transport.Name()] = storageAllowedPolicyScopes

	policyContext, err := signature.NewPolicyContext(commitPolicy)
	if err != nil {
		return imgID, nil, "", errors.Wrapf(err, "error creating new signature policy context")
	}
	defer func() {
		if err2 := policyContext.Destroy(); err2 != nil {
			logrus.Debugf("error destroying signature policy context: %v", err2)
		}
	}()

	// Check if the commit is blocked by $BUILDER_REGISTRY_SOURCES.
	insecure, err := checkRegistrySourcesAllows("commit to", dest)
	if err != nil {
		return imgID, nil, "", err
	}
	if insecure {
		if systemContext.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
			return imgID, nil, "", errors.Errorf("can't require tls verification on an insecured registry")
		}
		systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		systemContext.OCIInsecureSkipTLSVerify = true
		systemContext.DockerDaemonInsecureSkipTLSVerify = true
	}
	if len(options.AdditionalTags) > 0 {
		names, err := util.ExpandNames(options.AdditionalTags, "", systemContext, b.store)
		if err != nil {
			return imgID, nil, "", err
		}
		for _, name := range names {
			additionalDest, err := docker.Transport.ParseReference(name)
			if err != nil {
				return imgID, nil, "", errors.Wrapf(err, "error parsing image name %q as an image reference", name)
			}
			insecure, err := checkRegistrySourcesAllows("commit to", additionalDest)
			if err != nil {
				return imgID, nil, "", err
			}
			if insecure {
				if systemContext.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
					return imgID, nil, "", errors.Errorf("can't require tls verification on an insecured registry")
				}
				systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
				systemContext.OCIInsecureSkipTLSVerify = true
				systemContext.DockerDaemonInsecureSkipTLSVerify = true
			}
		}
	}
	logrus.Debugf("committing image with reference %q is allowed by policy", transports.ImageName(dest))

	// Check if the base image is already in the destination and it's some kind of local
	// storage.  If so, we can skip recompressing any layers that come from the base image.
	exportBaseLayers := true
	if transport, destIsStorage := dest.Transport().(is.StoreTransport); destIsStorage && options.OciEncryptConfig != nil {
		return imgID, nil, "", errors.New("unable to use local storage with image encryption")
	} else if destIsStorage && b.FromImageID != "" {
		if baseref, err := transport.ParseReference(b.FromImageID); baseref != nil && err == nil {
			if img, err := transport.GetImage(baseref); img != nil && err == nil {
				logrus.Debugf("base image %q is already present in local storage, no need to copy its layers", b.FromImageID)
				exportBaseLayers = false
			}
		}
	}
	// Build an image reference from which we can copy the finished image.
	src, err := b.makeImageRef(options, exportBaseLayers)
	if err != nil {
		return imgID, nil, "", errors.Wrapf(err, "error computing layer digests and building metadata for container %q", b.ContainerID)
	}
	// In case we're using caching, decide how to handle compression for a cache.
	// If we're using blob caching, set it up for the source.
	maybeCachedSrc := src
	maybeCachedDest := dest
	if options.BlobDirectory != "" {
		compress := types.PreserveOriginal
		if options.Compression != archive.Uncompressed {
			compress = types.Compress
		}
		cache, err := blobcache.NewBlobCache(src, options.BlobDirectory, compress)
		if err != nil {
			return imgID, nil, "", errors.Wrapf(err, "error wrapping image reference %q in blob cache at %q", transports.ImageName(src), options.BlobDirectory)
		}
		maybeCachedSrc = cache
		cache, err = blobcache.NewBlobCache(dest, options.BlobDirectory, compress)
		if err != nil {
			return imgID, nil, "", errors.Wrapf(err, "error wrapping image reference %q in blob cache at %q", transports.ImageName(dest), options.BlobDirectory)
		}
		maybeCachedDest = cache
	}
	// "Copy" our image to where it needs to be.
	switch options.Compression {
	case archive.Uncompressed:
		systemContext.OCIAcceptUncompressedLayers = true
	case archive.Gzip:
		systemContext.DirForceCompress = true
	}

	if systemContext.ArchitectureChoice != b.Architecture() {
		systemContext.ArchitectureChoice = b.Architecture()
	}
	if systemContext.OSChoice != b.OS() {
		systemContext.OSChoice = b.OS()
	}

	var manifestBytes []byte
	if manifestBytes, err = retryCopyImage(ctx, policyContext, maybeCachedDest, maybeCachedSrc, dest, getCopyOptions(b.store, options.ReportWriter, nil, systemContext, "", false, options.SignBy, options.OciEncryptLayers, options.OciEncryptConfig, nil), options.MaxRetries, options.RetryDelay); err != nil {
		return imgID, nil, "", errors.Wrapf(err, "error copying layers and metadata for container %q", b.ContainerID)
	}
	// If we've got more names to attach, and we know how to do that for
	// the transport that we're writing the new image to, add them now.
	if len(options.AdditionalTags) > 0 {
		switch dest.Transport().Name() {
		case is.Transport.Name():
			img, err := is.Transport.GetStoreImage(b.store, dest)
			if err != nil {
				return imgID, nil, "", errors.Wrapf(err, "error locating just-written image %q", transports.ImageName(dest))
			}
			if err = util.AddImageNames(b.store, "", systemContext, img, options.AdditionalTags); err != nil {
				return imgID, nil, "", errors.Wrapf(err, "error setting image names to %v", append(img.Names, options.AdditionalTags...))
			}
			logrus.Debugf("assigned names %v to image %q", img.Names, img.ID)
		default:
			logrus.Warnf("don't know how to add tags to images stored in %q transport", dest.Transport().Name())
		}
	}

	img, err := is.Transport.GetStoreImage(b.store, dest)
	if err != nil && errors.Cause(err) != storage.ErrImageUnknown {
		return imgID, nil, "", errors.Wrapf(err, "error locating image %q in local storage", transports.ImageName(dest))
	}
	if err == nil {
		imgID = img.ID
		prunedNames := make([]string, 0, len(img.Names))
		for _, name := range img.Names {
			if !(nameToRemove != "" && strings.Contains(name, nameToRemove)) {
				prunedNames = append(prunedNames, name)
			}
		}
		if len(prunedNames) < len(img.Names) {
			if err = b.store.SetNames(imgID, prunedNames); err != nil {
				return imgID, nil, "", errors.Wrapf(err, "failed to prune temporary name from image %q", imgID)
			}
			logrus.Debugf("reassigned names %v to image %q", prunedNames, img.ID)
			dest2, err := is.Transport.ParseStoreReference(b.store, "@"+imgID)
			if err != nil {
				return imgID, nil, "", errors.Wrapf(err, "error creating unnamed destination reference for image")
			}
			dest = dest2
		}
		if options.IIDFile != "" {
			if err = ioutil.WriteFile(options.IIDFile, []byte(img.ID), 0644); err != nil {
				return imgID, nil, "", err
			}
		}
	}

	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return imgID, nil, "", errors.Wrapf(err, "error computing digest of manifest of new image %q", transports.ImageName(dest))
	}

	var ref reference.Canonical
	if name := dest.DockerReference(); name != nil {
		ref, err = reference.WithDigest(name, manifestDigest)
		if err != nil {
			logrus.Warnf("error generating canonical reference with name %q and digest %s: %v", name, manifestDigest.String(), err)
		}
	}

	return imgID, ref, manifestDigest, nil
}

// Push copies the contents of the image to a new location.
func Push(ctx context.Context, image string, dest types.ImageReference, options PushOptions) (reference.Canonical, digest.Digest, error) {
	systemContext := getSystemContext(options.Store, options.SystemContext, options.SignaturePolicyPath)

	if options.Quiet {
		options.ReportWriter = nil // Turns off logging output
	}
	blocked, err := isReferenceBlocked(dest, systemContext)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error checking if pushing to registry for %q is blocked", transports.ImageName(dest))
	}
	if blocked {
		return nil, "", errors.Errorf("push access to registry for %q is blocked by configuration", transports.ImageName(dest))
	}

	// Load the system signing policy.
	pushPolicy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error obtaining default signature policy")
	}
	// Override the settings for local storage to make sure that we can always read the source "image".
	pushPolicy.Transports[is.Transport.Name()] = storageAllowedPolicyScopes

	policyContext, err := signature.NewPolicyContext(pushPolicy)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error creating new signature policy context")
	}
	defer func() {
		if err2 := policyContext.Destroy(); err2 != nil {
			logrus.Debugf("error destroying signature policy context: %v", err2)
		}
	}()

	// Look up the image.
	src, _, err := util.FindImage(options.Store, "", systemContext, image)
	if err != nil {
		return nil, "", err
	}
	maybeCachedSrc := src
	if options.BlobDirectory != "" {
		compress := types.PreserveOriginal
		if options.Compression != archive.Uncompressed {
			compress = types.Compress
		}
		cache, err := blobcache.NewBlobCache(src, options.BlobDirectory, compress)
		if err != nil {
			return nil, "", errors.Wrapf(err, "error wrapping image reference %q in blob cache at %q", transports.ImageName(src), options.BlobDirectory)
		}
		maybeCachedSrc = cache
	}

	// Check if the push is blocked by $BUILDER_REGISTRY_SOURCES.
	insecure, err := checkRegistrySourcesAllows("push to", dest)
	if err != nil {
		return nil, "", err
	}
	if insecure {
		if systemContext.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
			return nil, "", errors.Errorf("can't require tls verification on an insecured registry")
		}
		systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		systemContext.OCIInsecureSkipTLSVerify = true
		systemContext.DockerDaemonInsecureSkipTLSVerify = true
	}
	logrus.Debugf("pushing image to reference %q is allowed by policy", transports.ImageName(dest))

	// Copy everything.
	switch options.Compression {
	case archive.Uncompressed:
		systemContext.OCIAcceptUncompressedLayers = true
	case archive.Gzip:
		systemContext.DirForceCompress = true
	}
	var manifestBytes []byte
	if manifestBytes, err = retryCopyImage(ctx, policyContext, dest, maybeCachedSrc, dest, getCopyOptions(options.Store, options.ReportWriter, nil, systemContext, options.ManifestType, options.RemoveSignatures, options.SignBy, options.OciEncryptLayers, options.OciEncryptConfig, nil), options.MaxRetries, options.RetryDelay); err != nil {
		return nil, "", errors.Wrapf(err, "error copying layers and metadata from %q to %q", transports.ImageName(maybeCachedSrc), transports.ImageName(dest))
	}
	if options.ReportWriter != nil {
		fmt.Fprintf(options.ReportWriter, "")
	}
	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error computing digest of manifest of new image %q", transports.ImageName(dest))
	}
	var ref reference.Canonical
	if name := dest.DockerReference(); name != nil {
		ref, err = reference.WithDigest(name, manifestDigest)
		if err != nil {
			logrus.Warnf("error generating canonical reference with name %q and digest %s: %v", name, manifestDigest.String(), err)
		}
	}
	return ref, manifestDigest, nil
}
