package archive

import (
	"fmt"
	"strings"

	"github.com/containers/image/docker/reference"
	ctrImage "github.com/containers/image/image"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

func init() {
	transports.Register(Transport)
}

// Transport is an ImageTransport for local Docker archives.
var Transport = archiveTransport{}

type archiveTransport struct{}

func (t archiveTransport) Name() string {
	return "docker-archive"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t archiveTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t archiveTransport) ValidatePolicyConfigurationScope(scope string) error {
	// See the explanation in archiveReference.PolicyConfigurationIdentity.
	return errors.New(`docker-archive: does not support any scopes except the default "" one`)
}

// archiveReference is an ImageReference for Docker images.
type archiveReference struct {
	destinationRef reference.NamedTagged // only used for destinations
	path           string
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an Docker ImageReference.
func ParseReference(refString string) (types.ImageReference, error) {
	if refString == "" {
		return nil, errors.Errorf("docker-archive reference %s isn't of the form <path>[:<reference>]", refString)
	}

	parts := strings.SplitN(refString, ":", 2)
	path := parts[0]
	var destinationRef reference.NamedTagged

	// A :tag was specified, which is only necessary for destinations.
	if len(parts) == 2 {
		ref, err := reference.ParseNormalizedNamed(parts[1])
		if err != nil {
			return nil, errors.Wrapf(err, "docker-archive parsing reference")
		}
		ref = reference.TagNameOnly(ref)

		if _, isDigest := ref.(reference.Canonical); isDigest {
			return nil, errors.Errorf("docker-archive doesn't support digest references: %s", refString)
		}

		refTagged, isTagged := ref.(reference.NamedTagged)
		if !isTagged {
			// Really shouldn't be hit...
			return nil, errors.Errorf("internal error: reference is not tagged even after reference.TagNameOnly: %s", refString)
		}
		destinationRef = refTagged
	}

	return archiveReference{
		destinationRef: destinationRef,
		path:           path,
	}, nil
}

func (ref archiveReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref archiveReference) StringWithinTransport() string {
	if ref.destinationRef == nil {
		return ref.path
	}
	return fmt.Sprintf("%s:%s", ref.path, ref.destinationRef.String())
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref archiveReference) DockerReference() reference.Named {
	return ref.destinationRef
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
// (i.e. various references with exactly the same semantics should return the same configuration identity)
// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
// not required/guaranteed that it will be a valid input to Transport().ParseReference().
// Returns "" if configuration identities for these references are not supported.
func (ref archiveReference) PolicyConfigurationIdentity() string {
	// Punt, the justification is similar to dockerReference.PolicyConfigurationIdentity.
	return ""
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref archiveReference) PolicyConfigurationNamespaces() []string {
	// TODO
	return []string{}
}

// NewImage returns a types.Image for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned Image.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
func (ref archiveReference) NewImage(ctx *types.SystemContext) (types.Image, error) {
	src := newImageSource(ctx, ref)
	return ctrImage.FromSource(src)
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref archiveReference) NewImageSource(ctx *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(ctx, ref), nil
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref archiveReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ctx, ref)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref archiveReference) DeleteImage(ctx *types.SystemContext) error {
	// Not really supported, for safety reasons.
	return errors.New("Deleting images not implemented for docker-archive: images")
}
