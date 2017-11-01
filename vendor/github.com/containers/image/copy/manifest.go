package copy

import (
	"strings"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// preferredManifestMIMETypes lists manifest MIME types in order of our preference, if we can't use the original manifest and need to convert.
// Prefer v2s2 to v2s1 because v2s2 does not need to be changed when uploading to a different location.
// Include v2s1 signed but not v2s1 unsigned, because docker/distribution requires a signature even if the unsigned MIME type is used.
var preferredManifestMIMETypes = []string{manifest.DockerV2Schema2MediaType, manifest.DockerV2Schema1SignedMediaType}

// orderedSet is a list of strings (MIME types in our case), with each string appearing at most once.
type orderedSet struct {
	list     []string
	included map[string]struct{}
}

// newOrderedSet creates a correctly initialized orderedSet.
// [Sometimes it would be really nice if Golang had constructors…]
func newOrderedSet() *orderedSet {
	return &orderedSet{
		list:     []string{},
		included: map[string]struct{}{},
	}
}

// append adds s to the end of os, only if it is not included already.
func (os *orderedSet) append(s string) {
	if _, ok := os.included[s]; !ok {
		os.list = append(os.list, s)
		os.included[s] = struct{}{}
	}
}

// determineManifestConversion updates manifestUpdates to convert manifest to a supported MIME type, if necessary and canModifyManifest.
// Note that the conversion will only happen later, through src.UpdatedImage
// Returns the preferred manifest MIME type (whether we are converting to it or using it unmodified),
// and a list of other possible alternatives, in order.
func determineManifestConversion(manifestUpdates *types.ManifestUpdateOptions, src types.Image, destSupportedManifestMIMETypes []string, canModifyManifest bool) (string, []string, error) {
	_, srcType, err := src.Manifest()
	if err != nil { // This should have been cached?!
		return "", nil, errors.Wrap(err, "Error reading manifest")
	}

	if len(destSupportedManifestMIMETypes) == 0 {
		return srcType, []string{}, nil // Anything goes; just use the original as is, do not try any conversions.
	}
	supportedByDest := map[string]struct{}{}
	for _, t := range destSupportedManifestMIMETypes {
		supportedByDest[t] = struct{}{}
	}

	// destSupportedManifestMIMETypes is a static guess; a particular registry may still only support a subset of the types.
	// So, build a list of types to try in order of decreasing preference.
	// FIXME? This treats manifest.DockerV2Schema1SignedMediaType and manifest.DockerV2Schema1MediaType as distinct,
	// although we are not really making any conversion, and it is very unlikely that a destination would support one but not the other.
	// In practice, schema1 is probably the lowest common denominator, so we would expect to try the first one of the MIME types
	// and never attempt the other one.
	prioritizedTypes := newOrderedSet()

	// First of all, prefer to keep the original manifest unmodified.
	if _, ok := supportedByDest[srcType]; ok {
		prioritizedTypes.append(srcType)
	}
	if !canModifyManifest {
		// We could also drop the !canModifyManifest parameter and have the caller
		// make the choice; it is already doing that to an extent, to improve error
		// messages.  But it is nice to hide the “if !canModifyManifest, do no conversion”
		// special case in here; the caller can then worry (or not) only about a good UI.
		logrus.Debugf("We can't modify the manifest, hoping for the best...")
		return srcType, []string{}, nil // Take our chances - FIXME? Or should we fail without trying?
	}

	// Then use our list of preferred types.
	for _, t := range preferredManifestMIMETypes {
		if _, ok := supportedByDest[t]; ok {
			prioritizedTypes.append(t)
		}
	}

	// Finally, try anything else the destination supports.
	for _, t := range destSupportedManifestMIMETypes {
		prioritizedTypes.append(t)
	}

	logrus.Debugf("Manifest has MIME type %s, ordered candidate list [%s]", srcType, strings.Join(prioritizedTypes.list, ", "))
	if len(prioritizedTypes.list) == 0 { // Coverage: destSupportedManifestMIMETypes is not empty (or we would have exited in the “Anything goes” case above), so this should never happen.
		return "", nil, errors.New("Internal error: no candidate MIME types")
	}
	preferredType := prioritizedTypes.list[0]
	if preferredType != srcType {
		manifestUpdates.ManifestMIMEType = preferredType
	} else {
		logrus.Debugf("... will first try using the original manifest unmodified")
	}
	return preferredType, prioritizedTypes.list[1:], nil
}
