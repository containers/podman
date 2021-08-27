//go:build containers_image_ostree
// +build containers_image_ostree

package ostree

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/image/v5/directory/explicitfilepath"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
)

const defaultOSTreeRepo = "/ostree/repo"

// Transport is an ImageTransport for ostree paths.
var Transport = ostreeTransport{}

type ostreeTransport struct{}

func (t ostreeTransport) Name() string {
	return "ostree"
}

func init() {
	transports.Register(Transport)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t ostreeTransport) ValidatePolicyConfigurationScope(scope string) error {
	sep := strings.Index(scope, ":")
	if sep < 0 {
		return errors.Errorf("Invalid ostree: scope %s: Must include a repo", scope)
	}
	repo := scope[:sep]

	if !strings.HasPrefix(repo, "/") {
		return errors.Errorf("Invalid ostree: scope %s: repository must be an absolute path", scope)
	}
	cleaned := filepath.Clean(repo)
	if cleaned != repo {
		return errors.Errorf(`Invalid ostree: scope %s: Uses non-canonical path format, perhaps try with path %s`, scope, cleaned)
	}

	// FIXME? In the namespaces within a repo,
	// we could be verifying the various character set and length restrictions
	// from docker/distribution/reference.regexp.go, but other than that there
	// are few semantically invalid strings.
	return nil
}

// ostreeReference is an ImageReference for ostree paths.
type ostreeReference struct {
	image      string
	branchName string
	repo       string
}

type ostreeImageCloser struct {
	types.ImageCloser
	size int64
}

func (t ostreeTransport) ParseReference(ref string) (types.ImageReference, error) {
	var repo = ""
	var image = ""
	s := strings.SplitN(ref, "@/", 2)
	if len(s) == 1 {
		image, repo = s[0], defaultOSTreeRepo
	} else {
		image, repo = s[0], "/"+s[1]
	}

	return NewReference(image, repo)
}

// NewReference returns an OSTree reference for a specified repo and image.
func NewReference(image string, repo string) (types.ImageReference, error) {
	// image is not _really_ in a containers/image/docker/reference format;
	// as far as the libOSTree ociimage/* namespace is concerned, it is more or
	// less an arbitrary string with an implied tag.
	// Parse the image using reference.ParseNormalizedNamed so that we can
	// check whether the images has a tag specified and we can add ":latest" if needed
	ostreeImage, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}

	if reference.IsNameOnly(ostreeImage) {
		image = image + ":latest"
	}

	resolved, err := explicitfilepath.ResolvePathToFullyExplicit(repo)
	if err != nil {
		// With os.IsNotExist(err), the parent directory of repo is also not existent;
		// that should ordinarily not happen, but it would be a bit weird to reject
		// references which do not specify a repo just because the implicit defaultOSTreeRepo
		// does not exist.
		if os.IsNotExist(err) && repo == defaultOSTreeRepo {
			resolved = repo
		} else {
			return nil, err
		}
	}
	// This is necessary to prevent directory paths returned by PolicyConfigurationNamespaces
	// from being ambiguous with values of PolicyConfigurationIdentity.
	if strings.Contains(resolved, ":") {
		return nil, errors.Errorf("Invalid OSTree reference %s@%s: path %s contains a colon", image, repo, resolved)
	}

	return ostreeReference{
		image:      image,
		branchName: encodeOStreeRef(image),
		repo:       resolved,
	}, nil
}

func (ref ostreeReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref ostreeReference) StringWithinTransport() string {
	return fmt.Sprintf("%s@%s", ref.image, ref.repo)
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref ostreeReference) DockerReference() reference.Named {
	return nil
}

func (ref ostreeReference) PolicyConfigurationIdentity() string {
	return fmt.Sprintf("%s:%s", ref.repo, ref.image)
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref ostreeReference) PolicyConfigurationNamespaces() []string {
	s := strings.SplitN(ref.image, ":", 2)
	if len(s) != 2 { // Coverage: Should never happen, NewReference above ensures ref.image has a :tag.
		panic(fmt.Sprintf("Internal inconsistency: ref.image value %q does not have a :tag", ref.image))
	}
	name := s[0]
	res := []string{}
	for {
		res = append(res, fmt.Sprintf("%s:%s", ref.repo, name))

		lastSlash := strings.LastIndex(name, "/")
		if lastSlash == -1 {
			break
		}
		name = name[:lastSlash]
	}
	return res
}

func (s *ostreeImageCloser) Size() (int64, error) {
	return s.size, nil
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
func (ref ostreeReference) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	var tmpDir string
	if sys == nil || sys.OSTreeTmpDirPath == "" {
		tmpDir = os.TempDir()
	} else {
		tmpDir = sys.OSTreeTmpDirPath
	}
	src, err := newImageSource(tmpDir, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, sys, src)
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref ostreeReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	var tmpDir string
	if sys == nil || sys.OSTreeTmpDirPath == "" {
		tmpDir = os.TempDir()
	} else {
		tmpDir = sys.OSTreeTmpDirPath
	}
	return newImageSource(tmpDir, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref ostreeReference) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	var tmpDir string
	if sys == nil || sys.OSTreeTmpDirPath == "" {
		tmpDir = os.TempDir()
	} else {
		tmpDir = sys.OSTreeTmpDirPath
	}
	return newImageDestination(ref, tmpDir)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref ostreeReference) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	return errors.Errorf("Deleting images not implemented for ostree: images")
}

var ostreeRefRegexp = regexp.MustCompile(`^[A-Za-z0-9.-]$`)

func encodeOStreeRef(in string) string {
	var buffer bytes.Buffer
	for i := range in {
		sub := in[i : i+1]
		if ostreeRefRegexp.MatchString(sub) {
			buffer.WriteString(sub)
		} else {
			buffer.WriteString(fmt.Sprintf("_%02X", sub[0]))
		}

	}
	return buffer.String()
}

// manifestPath returns a path for the manifest within a ostree using our conventions.
func (ref ostreeReference) manifestPath() string {
	return filepath.Join("manifest", "manifest.json")
}

// signaturePath returns a path for a signature within a ostree using our conventions.
func (ref ostreeReference) signaturePath(index int) string {
	return filepath.Join("manifest", fmt.Sprintf("signature-%d", index+1))
}
