package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/image/directory/explicitfilepath"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func init() {
	transports.Register(Transport)
}

// Transport is an ImageTransport for OCI directories.
var Transport = ociTransport{}

type ociTransport struct{}

func (t ociTransport) Name() string {
	return "oci"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t ociTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

// annotation spex from https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys
const (
	separator = `(?:[-._:@+]|--)`
	alphanum  = `(?:[A-Za-z0-9]+)`
	component = `(?:` + alphanum + `(?:` + separator + alphanum + `)*)`
)

var refRegexp = regexp.MustCompile(`^` + component + `(?:/` + component + `)*$`)

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t ociTransport) ValidatePolicyConfigurationScope(scope string) error {
	var dir string
	sep := strings.SplitN(scope, ":", 2)
	dir = sep[0]

	if len(sep) == 2 {
		image := sep[1]
		if !refRegexp.MatchString(image) {
			return errors.Errorf("Invalid image %s", image)
		}
	}

	if !strings.HasPrefix(dir, "/") {
		return errors.Errorf("Invalid scope %s: must be an absolute path", scope)
	}
	// Refuse also "/", otherwise "/" and "" would have the same semantics,
	// and "" could be unexpectedly shadowed by the "/" entry.
	// (Note: we do allow "/:someimage", a bit ridiculous but why refuse it?)
	if scope == "/" {
		return errors.New(`Invalid scope "/": Use the generic default scope ""`)
	}
	cleaned := filepath.Clean(dir)
	if cleaned != dir {
		return errors.Errorf(`Invalid scope %s: Uses non-canonical path format, perhaps try with path %s`, scope, cleaned)
	}
	return nil
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
	image       string // If image=="", it means the only image in the index.json is used
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an OCI ImageReference.
func ParseReference(reference string) (types.ImageReference, error) {
	var dir, image string
	sep := strings.SplitN(reference, ":", 2)
	dir = sep[0]

	if len(sep) == 2 {
		image = sep[1]
	}
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
	// This is necessary to prevent directory paths returned by PolicyConfigurationNamespaces
	// from being ambiguous with values of PolicyConfigurationIdentity.
	if strings.Contains(resolved, ":") {
		return nil, errors.Errorf("Invalid OCI reference %s:%s: path %s contains a colon", dir, image, resolved)
	}
	if len(image) > 0 && !refRegexp.MatchString(image) {
		return nil, errors.Errorf("Invalid image %s", image)
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
	return fmt.Sprintf("%s", ref.resolvedDir)
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

// NewImage returns a types.Image for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned Image.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
func (ref ociReference) NewImage(ctx *types.SystemContext) (types.Image, error) {
	src, err := newImageSource(ctx, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(src)
}

func (ref ociReference) getManifestDescriptor() (imgspecv1.Descriptor, error) {
	indexJSON, err := os.Open(ref.indexPath())
	if err != nil {
		return imgspecv1.Descriptor{}, err
	}
	defer indexJSON.Close()
	index := imgspecv1.Index{}
	if err := json.NewDecoder(indexJSON).Decode(&index); err != nil {
		return imgspecv1.Descriptor{}, err
	}

	var d *imgspecv1.Descriptor
	if ref.image == "" {
		// return manifest if only one image is in the oci directory
		if len(index.Manifests) == 1 {
			d = &index.Manifests[0]
		} else {
			// ask user to choose image when more than one image in the oci directory
			return imgspecv1.Descriptor{}, errors.Wrapf(err, "more than one image in oci, choose an image")
		}
	} else {
		// if image specified, look through all manifests for a match
		for _, md := range index.Manifests {
			if md.MediaType != imgspecv1.MediaTypeImageManifest {
				continue
			}
			refName, ok := md.Annotations["org.opencontainers.image.ref.name"]
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
func (ref ociReference) NewImageSource(ctx *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(ctx, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref ociReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ref)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref ociReference) DeleteImage(ctx *types.SystemContext) error {
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
func (ref ociReference) blobPath(digest digest.Digest) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", errors.Wrapf(err, "unexpected digest reference %s", digest)
	}
	return filepath.Join(ref.dir, "blobs", digest.Algorithm().String(), digest.Hex()), nil
}
