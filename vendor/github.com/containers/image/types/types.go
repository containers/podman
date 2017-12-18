package types

import (
	"context"
	"io"
	"time"

	"github.com/containers/image/docker/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageTransport is a top-level namespace for ways to to store/load an image.
// It should generally correspond to ImageSource/ImageDestination implementations.
//
// Note that ImageTransport is based on "ways the users refer to image storage", not necessarily on the underlying physical transport.
// For example, all Docker References would be used within a single "docker" transport, regardless of whether the images are pulled over HTTP or HTTPS
// (or, even, IPv4 or IPv6).
//
// OTOH all images using the same transport should (apart from versions of the image format), be interoperable.
// For example, several different ImageTransport implementations may be based on local filesystem paths,
// but using completely different formats for the contents of that path (a single tar file, a directory containing tarballs, a fully expanded container filesystem, ...)
//
// See also transports.KnownTransports.
type ImageTransport interface {
	// Name returns the name of the transport, which must be unique among other transports.
	Name() string
	// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
	ParseReference(reference string) (ImageReference, error)
	// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
	// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
	// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
	// scope passed to this function will not be "", that value is always allowed.
	ValidatePolicyConfigurationScope(scope string) error
}

// ImageReference is an abstracted way to refer to an image location, namespaced within an ImageTransport.
//
// The object should preferably be immutable after creation, with any parsing/state-dependent resolving happening
// within an ImageTransport.ParseReference() or equivalent API creating the reference object.
// That's also why the various identification/formatting methods of this type do not support returning errors.
//
// WARNING: While this design freezes the content of the reference within this process, it can not freeze the outside
// world: paths may be replaced by symlinks elsewhere, HTTP APIs may start returning different results, and so on.
type ImageReference interface {
	Transport() ImageTransport
	// StringWithinTransport returns a string representation of the reference, which MUST be such that
	// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
	// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
	// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
	// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix;
	// instead, see transports.ImageName().
	StringWithinTransport() string

	// DockerReference returns a Docker reference associated with this reference
	// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
	// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
	DockerReference() reference.Named

	// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
	// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
	// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
	// (i.e. various references with exactly the same semantics should return the same configuration identity)
	// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
	// not required/guaranteed that it will be a valid input to Transport().ParseReference().
	// Returns "" if configuration identities for these references are not supported.
	PolicyConfigurationIdentity() string

	// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
	// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
	// in order, terminating on first match, and an implicit "" is always checked at the end.
	// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
	// and each following element to be a prefix of the element preceding it.
	PolicyConfigurationNamespaces() []string

	// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
	// The caller must call .Close() on the returned ImageCloser.
	// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
	// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
	// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
	NewImage(ctx *SystemContext) (ImageCloser, error)
	// NewImageSource returns a types.ImageSource for this reference.
	// The caller must call .Close() on the returned ImageSource.
	NewImageSource(ctx *SystemContext) (ImageSource, error)
	// NewImageDestination returns a types.ImageDestination for this reference.
	// The caller must call .Close() on the returned ImageDestination.
	NewImageDestination(ctx *SystemContext) (ImageDestination, error)

	// DeleteImage deletes the named image from the registry, if supported.
	DeleteImage(ctx *SystemContext) error
}

// BlobInfo collects known information about a blob (layer/config).
// In some situations, some fields may be unknown, in others they may be mandatory; documenting an “unknown” value here does not override that.
type BlobInfo struct {
	Digest      digest.Digest // "" if unknown.
	Size        int64         // -1 if unknown
	URLs        []string
	Annotations map[string]string
	MediaType   string
}

// ImageSource is a service, possibly remote (= slow), to download components of a single image or a named image set (manifest list).
// This is primarily useful for copying images around; for examining their properties, Image (below)
// is usually more useful.
// Each ImageSource should eventually be closed by calling Close().
//
// WARNING: Various methods which return an object identified by digest generally do not
// validate that the returned data actually matches that digest; this is the caller’s responsibility.
type ImageSource interface {
	// Reference returns the reference used to set up this source, _as specified by the user_
	// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
	Reference() ImageReference
	// Close removes resources associated with an initialized ImageSource, if any.
	Close() error
	// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
	// It may use a remote (= slow) service.
	// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
	// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
	GetManifest(instanceDigest *digest.Digest) ([]byte, string, error)
	// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
	// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
	GetBlob(BlobInfo) (io.ReadCloser, int64, error)
	// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
	// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
	// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
	// (e.g. if the source never returns manifest lists).
	GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error)
	// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer blobsums that are listed in the image's manifest.
	// The Digest field is guaranteed to be provided; Size may be -1.
	// WARNING: The list may contain duplicates, and they are semantically relevant.
	LayerInfosForCopy() []BlobInfo
}

// ImageDestination is a service, possibly remote (= slow), to store components of a single image.
//
// There is a specific required order for some of the calls:
// PutBlob on the various blobs, if any, MUST be called before PutManifest (manifest references blobs, which may be created or compressed only at push time)
// ReapplyBlob, if used, MUST only be called if HasBlob returned true for the same blob digest
// PutSignatures, if called, MUST be called after PutManifest (signatures reference manifest contents)
// Finally, Commit MUST be called if the caller wants the image, as formed by the components saved above, to persist.
//
// Each ImageDestination should eventually be closed by calling Close().
type ImageDestination interface {
	// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
	// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
	Reference() ImageReference
	// Close removes resources associated with an initialized ImageDestination, if any.
	Close() error

	// SupportedManifestMIMETypes tells which manifest mime types the destination supports
	// If an empty slice or nil it's returned, then any mime type can be tried to upload
	SupportedManifestMIMETypes() []string
	// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
	// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
	SupportsSignatures() error
	// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
	ShouldCompressLayers() bool
	// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
	// uploaded to the image destination, true otherwise.
	AcceptsForeignLayerURLs() bool
	// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
	MustMatchRuntimeOS() bool
	// PutBlob writes contents of stream and returns data representing the result.
	// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
	// inputInfo.Size is the expected length of stream, if known.
	// inputInfo.MediaType describes the blob format, if known.
	// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
	// to any other readers for download using the supplied digest.
	// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
	PutBlob(stream io.Reader, inputInfo BlobInfo) (BlobInfo, error)
	// HasBlob returns true iff the image destination already contains a blob with the matching digest which can be reapplied using ReapplyBlob.
	// Unlike PutBlob, the digest can not be empty.  If HasBlob returns true, the size of the blob must also be returned.
	// If the destination does not contain the blob, or it is unknown, HasBlob ordinarily returns (false, -1, nil);
	// it returns a non-nil error only on an unexpected failure.
	HasBlob(info BlobInfo) (bool, int64, error)
	// ReapplyBlob informs the image destination that a blob for which HasBlob previously returned true would have been passed to PutBlob if it had returned false.  Like HasBlob and unlike PutBlob, the digest can not be empty.  If the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree.
	ReapplyBlob(info BlobInfo) (BlobInfo, error)
	// PutManifest writes manifest to the destination.
	// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
	// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
	// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
	PutManifest(manifest []byte) error
	PutSignatures(signatures [][]byte) error
	// Commit marks the process of storing the image as successful and asks for the image to be persisted.
	// WARNING: This does not have any transactional semantics:
	// - Uploaded data MAY be visible to others before Commit() is called
	// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
	Commit() error
}

// ManifestTypeRejectedError is returned by ImageDestination.PutManifest if the destination is in principle available,
// refuses specifically this manifest type, but may accept a different manifest type.
type ManifestTypeRejectedError struct { // We only use a struct to allow a type assertion, without limiting the contents of the error otherwise.
	Err error
}

func (e ManifestTypeRejectedError) Error() string {
	return e.Err.Error()
}

// UnparsedImage is an Image-to-be; until it is verified and accepted, it only caries its identity and caches manifest and signature blobs.
// Thus, an UnparsedImage can be created from an ImageSource simply by fetching blobs without interpreting them,
// allowing cryptographic signature verification to happen first, before even fetching the manifest, or parsing anything else.
// This also makes the UnparsedImage→Image conversion an explicitly visible step.
//
// An UnparsedImage is a pair of (ImageSource, instance digest); it can represent either a manifest list or a single image instance.
//
// The UnparsedImage must not be used after the underlying ImageSource is Close()d.
type UnparsedImage interface {
	// Reference returns the reference used to set up this source, _as specified by the user_
	// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
	Reference() ImageReference
	// Manifest is like ImageSource.GetManifest, but the result is cached; it is OK to call this however often you need.
	Manifest() ([]byte, string, error)
	// Signatures is like ImageSource.GetSignatures, but the result is cached; it is OK to call this however often you need.
	Signatures(ctx context.Context) ([][]byte, error)
	// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer blobsums that are listed in the image's manifest.
	// The Digest field is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
	// WARNING: The list may contain duplicates, and they are semantically relevant.
	LayerInfosForCopy() []BlobInfo
}

// Image is the primary API for inspecting properties of images.
// An Image is based on a pair of (ImageSource, instance digest); it can represent either a manifest list or a single image instance.
//
// The Image must not be used after the underlying ImageSource is Close()d.
type Image interface {
	// Note that Reference may return nil in the return value of UpdatedImage!
	UnparsedImage
	// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
	// Note that the config object may not exist in the underlying storage in the return value of UpdatedImage! Use ConfigBlob() below.
	ConfigInfo() BlobInfo
	// ConfigBlob returns the blob described by ConfigInfo, if ConfigInfo().Digest != ""; nil otherwise.
	// The result is cached; it is OK to call this however often you need.
	ConfigBlob() ([]byte, error)
	// OCIConfig returns the image configuration as per OCI v1 image-spec. Information about
	// layers in the resulting configuration isn't guaranteed to be returned to due how
	// old image manifests work (docker v2s1 especially).
	OCIConfig() (*v1.Image, error)
	// LayerInfos returns a list of BlobInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
	// The Digest field is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
	// WARNING: The list may contain duplicates, and they are semantically relevant.
	LayerInfos() []BlobInfo
	// EmbeddedDockerReferenceConflicts whether a Docker reference embedded in the manifest, if any, conflicts with destination ref.
	// It returns false if the manifest does not embed a Docker reference.
	// (This embedding unfortunately happens for Docker schema1, please do not add support for this in any new formats.)
	EmbeddedDockerReferenceConflicts(ref reference.Named) bool
	// Inspect returns various information for (skopeo inspect) parsed from the manifest and configuration.
	Inspect() (*ImageInspectInfo, error)
	// UpdatedImageNeedsLayerDiffIDs returns true iff UpdatedImage(options) needs InformationOnly.LayerDiffIDs.
	// This is a horribly specific interface, but computing InformationOnly.LayerDiffIDs can be very expensive to compute
	// (most importantly it forces us to download the full layers even if they are already present at the destination).
	UpdatedImageNeedsLayerDiffIDs(options ManifestUpdateOptions) bool
	// UpdatedImage returns a types.Image modified according to options.
	// Everything in options.InformationOnly should be provided, other fields should be set only if a modification is desired.
	// This does not change the state of the original Image object.
	UpdatedImage(options ManifestUpdateOptions) (Image, error)
	// Size returns an approximation of the amount of disk space which is consumed by the image in its current
	// location.  If the size is not known, -1 will be returned.
	Size() (int64, error)
}

// ImageCloser is an Image with a Close() method which must be called by the user.
// This is returned by ImageReference.NewImage, which transparently instantiates a types.ImageSource,
// to ensure that the ImageSource is closed.
type ImageCloser interface {
	Image
	// Close removes resources associated with an initialized ImageCloser.
	Close() error
}

// ManifestUpdateOptions is a way to pass named optional arguments to Image.UpdatedManifest
type ManifestUpdateOptions struct {
	LayerInfos              []BlobInfo // Complete BlobInfos (size+digest+urls+annotations) which should replace the originals, in order (the root layer first, and then successive layered layers). BlobInfos' MediaType fields are ignored.
	EmbeddedDockerReference reference.Named
	ManifestMIMEType        string
	// The values below are NOT requests to modify the image; they provide optional context which may or may not be used.
	InformationOnly ManifestUpdateInformation
}

// ManifestUpdateInformation is a component of ManifestUpdateOptions, named here
// only to make writing struct literals possible.
type ManifestUpdateInformation struct {
	Destination  ImageDestination // and yes, UpdatedManifest may write to Destination (see the schema2 → schema1 conversion logic in image/docker_schema2.go)
	LayerInfos   []BlobInfo       // Complete BlobInfos (size+digest) which have been uploaded, in order (the root layer first, and then successive layered layers)
	LayerDiffIDs []digest.Digest  // Digest values for the _uncompressed_ contents of the blobs which have been uploaded, in the same order.
}

// ImageInspectInfo is a set of metadata describing Docker images, primarily their manifest and configuration.
// The Tag field is a legacy field which is here just for the Docker v2s1 manifest. It won't be supported
// for other manifest types.
type ImageInspectInfo struct {
	Tag           string
	Created       time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
}

// DockerAuthConfig contains authorization information for connecting to a registry.
type DockerAuthConfig struct {
	Username string
	Password string
}

// SystemContext allows parameterizing access to implicitly-accessed resources,
// like configuration files in /etc and users' login state in their home directory.
// Various components can share the same field only if their semantics is exactly
// the same; if in doubt, add a new field.
// It is always OK to pass nil instead of a SystemContext.
type SystemContext struct {
	// If not "", prefixed to any absolute paths used by default by the library (e.g. in /etc/).
	// Not used for any of the more specific path overrides available in this struct.
	// Not used for any paths specified by users in config files (even if the location of the config file _was_ affected by it).
	// NOTE: If this is set, environment-variable overrides of paths are ignored (to keep the semantics simple: to create an /etc replacement, just set RootForImplicitAbsolutePaths .
	// and there is no need to worry about the environment.)
	// NOTE: This does NOT affect paths starting by $HOME.
	RootForImplicitAbsolutePaths string

	// === Global configuration overrides ===
	// If not "", overrides the system's default path for signature.Policy configuration.
	SignaturePolicyPath string
	// If not "", overrides the system's default path for registries.d (Docker signature storage configuration)
	RegistriesDirPath string
	// Path to the system-wide registries configuration file
	SystemRegistriesConfPath string
	// If not "", overrides the default path for the authentication file
	AuthFilePath string
	// If not "", overrides the use of platform.GOARCH when choosing an image or verifying architecture match.
	ArchitectureChoice string
	// If not "", overrides the use of platform.GOOS when choosing an image or verifying OS match.
	OSChoice string

	// === OCI.Transport overrides ===
	// If not "", a directory containing a CA certificate (ending with ".crt"),
	// a client certificate (ending with ".cert") and a client ceritificate key
	// (ending with ".key") used when downloading OCI image layers.
	OCICertPath string
	// Allow downloading OCI image layers over HTTP, or HTTPS with failed TLS verification. Note that this does not affect other TLS connections.
	OCIInsecureSkipTLSVerify bool
	// If not "", use a shared directory for storing blobs rather than within OCI layouts
	OCISharedBlobDirPath string

	// === docker.Transport overrides ===
	// If not "", a directory containing a CA certificate (ending with ".crt"),
	// a client certificate (ending with ".cert") and a client ceritificate key
	// (ending with ".key") used when talking to a Docker Registry.
	DockerCertPath string
	// If not "", overrides the system’s default path for a directory containing host[:port] subdirectories with the same structure as DockerCertPath above.
	// Ignored if DockerCertPath is non-empty.
	DockerPerHostCertDirPath string
	// Allow contacting docker registries over HTTP, or HTTPS with failed TLS verification. Note that this does not affect other TLS connections.
	DockerInsecureSkipTLSVerify bool
	// if nil, the library tries to parse ~/.docker/config.json to retrieve credentials
	DockerAuthConfig *DockerAuthConfig
	// if not "", an User-Agent header is added to each request when contacting a registry.
	DockerRegistryUserAgent string
	// if true, a V1 ping attempt isn't done to give users a better error. Default is false.
	// Note that this field is used mainly to integrate containers/image into projectatomic/docker
	// in order to not break any existing docker's integration tests.
	DockerDisableV1Ping bool
	// Directory to use for OSTree temporary files
	OSTreeTmpDirPath string

	// === docker/daemon.Transport overrides ===
	// A directory containing a CA certificate (ending with ".crt"),
	// a client certificate (ending with ".cert") and a client certificate key
	// (ending with ".key") used when talking to a Docker daemon.
	DockerDaemonCertPath string
	// The hostname or IP to the Docker daemon. If not set (aka ""), client.DefaultDockerHost is assumed.
	DockerDaemonHost string
	// Used to skip TLS verification, off by default. To take effect DockerDaemonCertPath needs to be specified as well.
	DockerDaemonInsecureSkipTLSVerify bool

	// === dir.Transport overrides ===
	// DirForceCompress compresses the image layers if set to true
	DirForceCompress bool
}

// ProgressProperties is used to pass information from the copy code to a monitor which
// can use the real-time information to produce output or react to changes.
type ProgressProperties struct {
	Artifact BlobInfo
	Offset   uint64
}
