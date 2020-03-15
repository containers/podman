package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/directory/explicitfilepath"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/oci/internal"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func init() {
	transports.Register(Transport)
}

var (
	// Transport is an ImageTransport for OCI directories.
	Transport = ociTransport{}

	// ErrMoreThanOneImage is an error returned when the manifest includes
	// more than one image and the user should choose which one to use.
	ErrMoreThanOneImage = errors.New("more than one image in oci, choose an image")
)

type ociTransport struct{}

func (t ociTransport) Name() string {
	return "oci"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t ociTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t ociTransport) ValidatePolicyConfigurationScope(scope string) error {
	return internal.ValidateScope(scope)
}

// ociReference is an ImageReference for OCI directory paths.
type ociReference struct {
	// Note that the interpretation of paths below depends on the underlying filesystem state, which may change under us at any time!
	// Either of the paths may point to a different, or no, inode over time.  resolvedDir may contain symbolic links, and so on.

	// Generally we follow the intent of the user, and use the "dir" member for filesystem operations (e.g. the user can use a relative path to avoid
	// being exposed to symlinks and renames in the parent directories to the working directory).
	// (But in general, we make no attempt to be completely safe against concurrent hostile filesystem modifications.)
	dir         string // As specified by the user. May be relative, contain symlinks, etc.
	resolvedDir string // Absolute path with no symlinks, at least at the time of its creation. Primarily used for policy namespaces.
	// If image=="", it means the "only image" in the index.json is used in the case it is a source
	// for destinations, the image name annotation "image.ref.name" is not added to the index.json
	image string
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an OCI ImageReference.
func ParseReference(reference string) (types.ImageReference, error) {
	dir, image := internal.SplitPathAndImage(reference)
	return NewReference(dir, image)
}

// NewReference returns an OCI reference for a directory and a image.
//
// We do not expose an API supplying the resolvedDir; we could, but recomputing it
// is generally cheap enough that we prefer being confident about the properties of resolvedDir.
func NewReference(dir, image string) (types.ImageReference, error) {
	resolved, err := explicitfilepath.ResolvePathToFullyExplicit(dir)
	if err != nil {
		return nil, err
	}

	if err := internal.ValidateOCIPath(dir); err != nil {
		return nil, err
	}

	if err = internal.ValidateImageName(image); err != nil {
		return nil, err
	}

	return ociReference{dir: dir, resolvedDir: resolved, image: image}, nil
}

func (ref ociReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref ociReference) StringWithinTransport() string {
	return fmt.Sprintf("%s:%s", ref.dir, ref.image)
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref ociReference) DockerReference() reference.Named {
	return nil
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
// (i.e. various references with exactly the same semantics should return the same configuration identity)
// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
// not required/guaranteed that it will be a valid input to Transport().ParseReference().
// Returns "" if configuration identities for these references are not supported.
func (ref ociReference) PolicyConfigurationIdentity() string {
	// NOTE: ref.image is not a part of the image identity, because "$dir:$someimage" and "$dir:" may mean the
	// same image and the two can’t be statically disambiguated.  Using at least the repository directory is
	// less granular but hopefully still useful.
	return ref.resolvedDir
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref ociReference) PolicyConfigurationNamespaces() []string {
	res := []string{}
	path := ref.resolvedDir
	for {
		lastSlash := strings.LastIndex(path, "/")
		// Note that we do not include "/"; it is redundant with the default "" global default,
		// and rejected by ociTransport.ValidatePolicyConfigurationScope above.
		if lastSlash == -1 || path == "/" {
			break
		}
		res = append(res, path)
		path = path[:lastSlash]
	}
	return res
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
func (ref ociReference) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	src, err := newImageSource(sys, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, sys, src)
}

// getIndex returns a pointer to the index references by this ociReference. If an error occurs opening an index nil is returned together
// with an error.
func (ref ociReference) getIndex() (*imgspecv1.Index, error) {
	indexJSON, err := os.Open(ref.indexPath())
	if err != nil {
		return nil, err
	}
	defer indexJSON.Close()

	index := &imgspecv1.Index{}
	if err := json.NewDecoder(indexJSON).Decode(index); err != nil {
		return nil, err
	}
	return index, nil
}

func (ref ociReference) getManifestDescriptor() (imgspecv1.Descriptor, error) {
	index, err := ref.getIndex()
	if err != nil {
		return imgspecv1.Descriptor{}, err
	}

	var d *imgspecv1.Descriptor
	if ref.image == "" {
		// return manifest if only one image is in the oci directory
		if len(index.Manifests) == 1 {
			d = &index.Manifests[0]
		} else {
			// ask user to choose image when more than one image in the oci directory
			return imgspecv1.Descriptor{}, ErrMoreThanOneImage
		}
	} else {
		// if image specified, look through all manifests for a match
		for _, md := range index.Manifests {
			if md.MediaType != imgspecv1.MediaTypeImageManifest && md.MediaType != imgspecv1.MediaTypeImageIndex {
				continue
			}
			refName, ok := md.Annotations[imgspecv1.AnnotationRefName]
			if !ok {
				continue
			}
			if refName == ref.image {
				d = &md
				break
			}
		}
	}
	if d == nil {
		return imgspecv1.Descriptor{}, fmt.Errorf("no descriptor found for reference %q", ref.image)
	}
	return *d, nil
}

// LoadManifestDescriptor loads the manifest descriptor to be used to retrieve the image name
// when pulling an image
func LoadManifestDescriptor(imgRef types.ImageReference) (imgspecv1.Descriptor, error) {
	ociRef, ok := imgRef.(ociReference)
	if !ok {
		return imgspecv1.Descriptor{}, errors.Errorf("error typecasting, need type ociRef")
	}
	return ociRef.getManifestDescriptor()
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref ociReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(sys, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref ociReference) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(sys, ref)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref ociReference) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	return errors.Errorf("Deleting images not implemented for oci: images")
}

// ociLayoutPath returns a path for the oci-layout within a directory using OCI conventions.
func (ref ociReference) ociLayoutPath() string {
	return filepath.Join(ref.dir, "oci-layout")
}

// indexPath returns a path for the index.json within a directory using OCI conventions.
func (ref ociReference) indexPath() string {
	return filepath.Join(ref.dir, "index.json")
}

// blobPath returns a path for a blob within a directory using OCI image-layout conventions.
func (ref ociReference) blobPath(digest digest.Digest, sharedBlobDir string) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", errors.Wrapf(err, "unexpected digest reference %s", digest)
	}
	blobDir := filepath.Join(ref.dir, "blobs")
	if sharedBlobDir != "" {
		blobDir = sharedBlobDir
	}
	return filepath.Join(blobDir, digest.Algorithm().String(), digest.Hex()), nil
}
