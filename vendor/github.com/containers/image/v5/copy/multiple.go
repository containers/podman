package copy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/image"
	internalManifest "github.com/containers/image/v5/internal/manifest"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type instanceCopyKind int

const (
	instanceCopyCopy instanceCopyKind = iota
	instanceCopyClone
)

type instanceCopy struct {
	op           instanceCopyKind
	sourceDigest digest.Digest
}

// prepareInstanceCopies prepares a list of instances which needs to copied to the manifest list.
func prepareInstanceCopies(instanceDigests []digest.Digest, options *Options) []instanceCopy {
	res := []instanceCopy{}
	for i, instanceDigest := range instanceDigests {
		if options.ImageListSelection == CopySpecificImages &&
			!slices.Contains(options.Instances, instanceDigest) {
			logrus.Debugf("Skipping instance %s (%d/%d)", instanceDigest, i+1, len(instanceDigests))
			continue
		}
		res = append(res, instanceCopy{
			op:           instanceCopyCopy,
			sourceDigest: instanceDigest,
		})
	}
	return res
}

// copyMultipleImages copies some or all of an image list's instances, using
// policyContext to validate source image admissibility.
func (c *copier) copyMultipleImages(ctx context.Context, policyContext *signature.PolicyContext, options *Options, unparsedToplevel *image.UnparsedImage) (copiedManifest []byte, retErr error) {
	// Parse the list and get a copy of the original value after it's re-encoded.
	manifestList, manifestType, err := unparsedToplevel.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading manifest list: %w", err)
	}
	originalList, err := internalManifest.ListFromBlob(manifestList, manifestType)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest list %q: %w", string(manifestList), err)
	}
	updatedList := originalList.CloneInternal()

	sigs, err := c.sourceSignatures(ctx, unparsedToplevel, options,
		"Getting image list signatures",
		"Checking if image list destination supports signatures")
	if err != nil {
		return nil, err
	}

	// If the destination is a digested reference, make a note of that, determine what digest value we're
	// expecting, and check that the source manifest matches it.
	destIsDigestedReference := false
	if named := c.dest.Reference().DockerReference(); named != nil {
		if digested, ok := named.(reference.Digested); ok {
			destIsDigestedReference = true
			matches, err := manifest.MatchesDigest(manifestList, digested.Digest())
			if err != nil {
				return nil, fmt.Errorf("computing digest of source image's manifest: %w", err)
			}
			if !matches {
				return nil, errors.New("Digest of source image's manifest would not match destination reference")
			}
		}
	}

	// Determine if we're allowed to modify the manifest list.
	// If we can, set to the empty string. If we can't, set to the reason why.
	// Compare, and perhaps keep in sync with, the version in copySingleImage.
	cannotModifyManifestListReason := ""
	if len(sigs) > 0 {
		cannotModifyManifestListReason = "Would invalidate signatures"
	}
	if destIsDigestedReference {
		cannotModifyManifestListReason = "Destination specifies a digest"
	}
	if options.PreserveDigests {
		cannotModifyManifestListReason = "Instructed to preserve digests"
	}

	// Determine if we'll need to convert the manifest list to a different format.
	forceListMIMEType := options.ForceManifestMIMEType
	switch forceListMIMEType {
	case manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType, manifest.DockerV2Schema2MediaType:
		forceListMIMEType = manifest.DockerV2ListMediaType
	case imgspecv1.MediaTypeImageManifest:
		forceListMIMEType = imgspecv1.MediaTypeImageIndex
	}
	selectedListType, otherManifestMIMETypeCandidates, err := c.determineListConversion(manifestType, c.dest.SupportedManifestMIMETypes(), forceListMIMEType)
	if err != nil {
		return nil, fmt.Errorf("determining manifest list type to write to destination: %w", err)
	}
	if selectedListType != originalList.MIMEType() {
		if cannotModifyManifestListReason != "" {
			return nil, fmt.Errorf("Manifest list must be converted to type %q to be written to destination, but we cannot modify it: %q", selectedListType, cannotModifyManifestListReason)
		}
	}

	// Copy each image, or just the ones we want to copy, in turn.
	instanceDigests := updatedList.Instances()
	instanceEdits := []internalManifest.ListEdit{}
	instanceCopyList := prepareInstanceCopies(instanceDigests, options)
	c.Printf("Copying %d of %d images in list\n", len(instanceCopyList), len(instanceDigests))
	for i, instance := range instanceCopyList {
		// Update instances to be edited by their `ListOperation` and
		// populate necessary fields.
		switch instance.op {
		case instanceCopyCopy:
			logrus.Debugf("Copying instance %s (%d/%d)", instance.sourceDigest, i+1, len(instanceCopyList))
			c.Printf("Copying image %s (%d/%d)\n", instance.sourceDigest, i+1, len(instanceCopyList))
			unparsedInstance := image.UnparsedInstance(c.rawSource, &instanceCopyList[i].sourceDigest)
			updatedManifest, updatedManifestType, updatedManifestDigest, err := c.copySingleImage(ctx, policyContext, options, unparsedToplevel, unparsedInstance, &instanceCopyList[i].sourceDigest)
			if err != nil {
				return nil, fmt.Errorf("copying image %d/%d from manifest list: %w", i+1, len(instanceCopyList), err)
			}
			// Record the result of a possible conversion here.
			instanceEdits = append(instanceEdits, internalManifest.ListEdit{
				ListOperation:   internalManifest.ListOpUpdate,
				UpdateOldDigest: instance.sourceDigest,
				UpdateDigest:    updatedManifestDigest,
				UpdateSize:      int64(len(updatedManifest)),
				UpdateMediaType: updatedManifestType})
		default:
			return nil, fmt.Errorf("copying image: invalid copy operation %d", instance.op)
		}
	}

	// Now reset the digest/size/types of the manifests in the list to account for any conversions that we made.
	if err = updatedList.EditInstances(instanceEdits); err != nil {
		return nil, fmt.Errorf("updating manifest list: %w", err)
	}

	// Iterate through supported list types, preferred format first.
	c.Printf("Writing manifest list to image destination\n")
	var errs []string
	for _, thisListType := range append([]string{selectedListType}, otherManifestMIMETypeCandidates...) {
		var attemptedList internalManifest.ListPublic = updatedList

		logrus.Debugf("Trying to use manifest list type %s…", thisListType)

		// Perform the list conversion, if we need one.
		if thisListType != updatedList.MIMEType() {
			attemptedList, err = updatedList.ConvertToMIMEType(thisListType)
			if err != nil {
				return nil, fmt.Errorf("converting manifest list to list with MIME type %q: %w", thisListType, err)
			}
		}

		// Check if the updates or a type conversion meaningfully changed the list of images
		// by serializing them both so that we can compare them.
		attemptedManifestList, err := attemptedList.Serialize()
		if err != nil {
			return nil, fmt.Errorf("encoding updated manifest list (%q: %#v): %w", updatedList.MIMEType(), updatedList.Instances(), err)
		}
		originalManifestList, err := originalList.Serialize()
		if err != nil {
			return nil, fmt.Errorf("encoding original manifest list for comparison (%q: %#v): %w", originalList.MIMEType(), originalList.Instances(), err)
		}

		// If we can't just use the original value, but we have to change it, flag an error.
		if !bytes.Equal(attemptedManifestList, originalManifestList) {
			if cannotModifyManifestListReason != "" {
				return nil, fmt.Errorf("Manifest list must be converted to type %q to be written to destination, but we cannot modify it: %q", thisListType, cannotModifyManifestListReason)
			}
			logrus.Debugf("Manifest list has been updated")
		} else {
			// We can just use the original value, so use it instead of the one we just rebuilt, so that we don't change the digest.
			attemptedManifestList = manifestList
		}

		// Save the manifest list.
		err = c.dest.PutManifest(ctx, attemptedManifestList, nil)
		if err != nil {
			logrus.Debugf("Upload of manifest list type %s failed: %v", thisListType, err)
			errs = append(errs, fmt.Sprintf("%s(%v)", thisListType, err))
			continue
		}
		errs = nil
		manifestList = attemptedManifestList
		break
	}
	if errs != nil {
		return nil, fmt.Errorf("Uploading manifest list failed, attempted the following formats: %s", strings.Join(errs, ", "))
	}

	// Sign the manifest list.
	newSigs, err := c.createSignatures(ctx, manifestList, options.SignIdentity)
	if err != nil {
		return nil, err
	}
	sigs = append(sigs, newSigs...)

	c.Printf("Storing list signatures\n")
	if err := c.dest.PutSignaturesWithFormat(ctx, sigs, nil); err != nil {
		return nil, fmt.Errorf("writing signatures: %w", err)
	}

	return manifestList, nil
}
