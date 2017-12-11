package daemon

import (
	"github.com/pkg/errors"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
)

func init() {
	transports.Register(Transport)
}

// Transport is an ImageTransport for images managed by a local Docker daemon.
var Transport = daemonTransport{}

type daemonTransport struct{}

// Name returns the name of the transport, which must be unique among other transports.
func (t daemonTransport) Name() string {
	return "docker-daemon"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t daemonTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t daemonTransport) ValidatePolicyConfigurationScope(scope string) error {
	// See the explanation in daemonReference.PolicyConfigurationIdentity.
	return errors.New(`docker-daemon: does not support any scopes except the default "" one`)
}

// daemonReference is an ImageReference for images managed by a local Docker daemon
// Exactly one of id and ref can be set.
// For daemonImageSource, both id and ref are acceptable, ref must not be a NameOnly (interpreted as all tags in that repository by the daemon)
// For daemonImageDestination, it must be a ref, which is NamedTagged.
// (We could, in principle, also allow storing images without tagging them, and the user would have to refer to them using the docker image ID = config digest.
//  Using the config digest requires the caller to parse the manifest themselves, which is very cumbersome; so, for now, we don’t bother.)
type daemonReference struct {
	id  digest.Digest
	ref reference.Named // !reference.IsNameOnly
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func ParseReference(refString string) (types.ImageReference, error) {
	// This is intended to be compatible with reference.ParseAnyReference, but more strict about refusing some of the ambiguous cases.
	// In particular, this rejects unprefixed digest values (64 hex chars), and sha256 digest prefixes (sha256:fewer-than-64-hex-chars).

	// digest:hexstring is structurally the same as a reponame:tag (meaning docker.io/library/reponame:tag).
	// reference.ParseAnyReference interprets such strings as digests.
	if dgst, err := digest.Parse(refString); err == nil {
		// The daemon explicitly refuses to tag images with a reponame equal to digest.Canonical - but _only_ this digest name.
		// Other digest references are ambiguous, so refuse them.
		if dgst.Algorithm() != digest.Canonical {
			return nil, errors.Errorf("Invalid docker-daemon: reference %s: only digest algorithm %s accepted", refString, digest.Canonical)
		}
		return NewReference(dgst, nil)
	}

	ref, err := reference.ParseNormalizedNamed(refString) // This also rejects unprefixed digest values
	if err != nil {
		return nil, err
	}
	if reference.FamiliarName(ref) == digest.Canonical.String() {
		return nil, errors.Errorf("Invalid docker-daemon: reference %s: The %s repository name is reserved for (non-shortened) digest references", refString, digest.Canonical)
	}
	return NewReference("", ref)
}

// NewReference returns a docker-daemon reference for either the supplied image ID (config digest) or the supplied reference (which must satisfy !reference.IsNameOnly)
func NewReference(id digest.Digest, ref reference.Named) (types.ImageReference, error) {
	if id != "" && ref != nil {
		return nil, errors.New("docker-daemon: reference must not have an image ID and a reference string specified at the same time")
	}
	if ref != nil {
		if reference.IsNameOnly(ref) {
			return nil, errors.Errorf("docker-daemon: reference %s has neither a tag nor a digest", reference.FamiliarString(ref))
		}
		// A github.com/distribution/reference value can have a tag and a digest at the same time!
		// Most versions of docker/reference do not handle that (ignoring the tag), so reject such input.
		// This MAY be accepted in the future.
		_, isTagged := ref.(reference.NamedTagged)
		_, isDigested := ref.(reference.Canonical)
		if isTagged && isDigested {
			return nil, errors.Errorf("docker-daemon: references with both a tag and digest are currently not supported")
		}
	}
	return daemonReference{
		id:  id,
		ref: ref,
	}, nil
}

func (ref daemonReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix;
// instead, see transports.ImageName().
func (ref daemonReference) StringWithinTransport() string {
	switch {
	case ref.id != "":
		return ref.id.String()
	case ref.ref != nil:
		return reference.FamiliarString(ref.ref)
	default: // Coverage: Should never happen, NewReference above should refuse such values.
		panic("Internal inconsistency: daemonReference has empty id and nil ref")
	}
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref daemonReference) DockerReference() reference.Named {
	return ref.ref // May be nil
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
// (i.e. various references with exactly the same semantics should return the same configuration identity)
// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
// not required/guaranteed that it will be a valid input to Transport().ParseReference().
// Returns "" if configuration identities for these references are not supported.
func (ref daemonReference) PolicyConfigurationIdentity() string {
	// We must allow referring to images in the daemon by image ID, otherwise untagged images would not be accessible.
	// But the existence of image IDs means that we can’t truly well namespace the input; the untagged images would have to fall into the default policy,
	// which can be unexpected.  So, punt.
	return "" // This still allows using the default "" scope to define a policy for this transport.
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref daemonReference) PolicyConfigurationNamespaces() []string {
	// See the explanation in daemonReference.PolicyConfigurationIdentity.
	return []string{}
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
func (ref daemonReference) NewImage(ctx *types.SystemContext) (types.ImageCloser, error) {
	src, err := newImageSource(ctx, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, src)
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref daemonReference) NewImageSource(ctx *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(ctx, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref daemonReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ctx, ref)
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref daemonReference) DeleteImage(ctx *types.SystemContext) error {
	// Should this just untag the image? Should this stop running containers?
	// The semantics is not quite as clear as for remote repositories.
	// The user can run (docker rmi) directly anyway, so, for now(?), punt instead of trying to guess what the user meant.
	return errors.Errorf("Deleting images not implemented for docker-daemon: images")
}
