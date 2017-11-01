package archive

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/image/directory/explicitfilepath"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	ocilayout "github.com/containers/image/oci/layout"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
)

func init() {
	transports.Register(Transport)
}

// Transport is an ImageTransport for OCI archive
// it creates an oci-archive tar file by calling into the OCI transport
// tarring the directory created by oci and deleting the directory
var Transport = ociArchiveTransport{}

type ociArchiveTransport struct{}

// ociArchiveReference is an ImageReference for OCI Archive paths
type ociArchiveReference struct {
	file         string
	resolvedFile string
	image        string
}

func (t ociArchiveTransport) Name() string {
	return "oci-archive"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix
// into an ImageReference.
func (t ociArchiveTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
func (t ociArchiveTransport) ValidatePolicyConfigurationScope(scope string) error {
	var file string
	sep := strings.SplitN(scope, ":", 2)
	file = sep[0]

	if len(sep) == 2 {
		image := sep[1]
		if !refRegexp.MatchString(image) {
			return errors.Errorf("Invalid image %s", image)
		}
	}

	if !strings.HasPrefix(file, "/") {
		return errors.Errorf("Invalid scope %s: must be an absolute path", scope)
	}
	// Refuse also "/", otherwise "/" and "" would have the same semantics,
	// and "" could be unexpectedly shadowed by the "/" entry.
	// (Note: we do allow "/:someimage", a bit ridiculous but why refuse it?)
	if scope == "/" {
		return errors.New(`Invalid scope "/": Use the generic default scope ""`)
	}
	cleaned := filepath.Clean(file)
	if cleaned != file {
		return errors.Errorf(`Invalid scope %s: Uses non-canonical path format, perhaps try with path %s`, scope, cleaned)
	}
	return nil
}

// annotation spex from https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys
const (
	separator = `(?:[-._:@+]|--)`
	alphanum  = `(?:[A-Za-z0-9]+)`
	component = `(?:` + alphanum + `(?:` + separator + alphanum + `)*)`
)

var refRegexp = regexp.MustCompile(`^` + component + `(?:/` + component + `)*$`)

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an OCI ImageReference.
func ParseReference(reference string) (types.ImageReference, error) {
	var file, image string
	sep := strings.SplitN(reference, ":", 2)
	file = sep[0]

	if len(sep) == 2 {
		image = sep[1]
	}
	return NewReference(file, image)
}

// NewReference returns an OCI reference for a file and a image.
func NewReference(file, image string) (types.ImageReference, error) {
	resolved, err := explicitfilepath.ResolvePathToFullyExplicit(file)
	if err != nil {
		return nil, err
	}
	// This is necessary to prevent directory paths returned by PolicyConfigurationNamespaces
	// from being ambiguous with values of PolicyConfigurationIdentity.
	if strings.Contains(resolved, ":") {
		return nil, errors.Errorf("Invalid OCI reference %s:%s: path %s contains a colon", file, image, resolved)
	}
	if len(image) > 0 && !refRegexp.MatchString(image) {
		return nil, errors.Errorf("Invalid image %s", image)
	}
	return ociArchiveReference{file: file, resolvedFile: resolved, image: image}, nil
}

func (ref ociArchiveReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
func (ref ociArchiveReference) StringWithinTransport() string {
	return fmt.Sprintf("%s:%s", ref.file, ref.image)
}

// DockerReference returns a Docker reference associated with this reference
func (ref ociArchiveReference) DockerReference() reference.Named {
	return nil
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
func (ref ociArchiveReference) PolicyConfigurationIdentity() string {
	// NOTE: ref.image is not a part of the image identity, because "$dir:$someimage" and "$dir:" may mean the
	// same image and the two can’t be statically disambiguated.  Using at least the repository directory is
	// less granular but hopefully still useful.
	return fmt.Sprintf("%s", ref.resolvedFile)
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set
func (ref ociArchiveReference) PolicyConfigurationNamespaces() []string {
	res := []string{}
	path := ref.resolvedFile
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
func (ref ociArchiveReference) NewImage(ctx *types.SystemContext) (types.Image, error) {
	src, err := newImageSource(ctx, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(src)
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref ociArchiveReference) NewImageSource(ctx *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(ctx, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref ociArchiveReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ctx, ref)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref ociArchiveReference) DeleteImage(ctx *types.SystemContext) error {
	return errors.Errorf("Deleting images not implemented for oci: images")
}

// struct to store the ociReference and temporary directory returned by createOCIRef
type tempDirOCIRef struct {
	tempDirectory   string
	ociRefExtracted types.ImageReference
}

// deletes the temporary directory created
func (t *tempDirOCIRef) deleteTempDir() error {
	return os.RemoveAll(t.tempDirectory)
}

// createOCIRef creates the oci reference of the image
func createOCIRef(image string) (tempDirOCIRef, error) {
	dir, err := ioutil.TempDir("/var/tmp", "oci")
	if err != nil {
		return tempDirOCIRef{}, errors.Wrapf(err, "error creating temp directory")
	}
	ociRef, err := ocilayout.NewReference(dir, image)
	if err != nil {
		return tempDirOCIRef{}, err
	}

	tempDirRef := tempDirOCIRef{tempDirectory: dir, ociRefExtracted: ociRef}
	return tempDirRef, nil
}

// creates the temporary directory and copies the tarred content to it
func createUntarTempDir(ref ociArchiveReference) (tempDirOCIRef, error) {
	tempDirRef, err := createOCIRef(ref.image)
	if err != nil {
		return tempDirOCIRef{}, errors.Wrap(err, "error creating oci reference")
	}
	src := ref.resolvedFile
	dst := tempDirRef.tempDirectory
	if err := archive.UntarPath(src, dst); err != nil {
		if err := tempDirRef.deleteTempDir(); err != nil {
			return tempDirOCIRef{}, errors.Wrapf(err, "error deleting temp directory %q", tempDirRef.tempDirectory)
		}
		return tempDirOCIRef{}, errors.Wrapf(err, "error untarring file %q", tempDirRef.tempDirectory)
	}
	return tempDirRef, nil
}
