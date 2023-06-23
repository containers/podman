package copy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/image"
	"github.com/containers/image/v5/internal/pkg/platform"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/set"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/compression"
	compressiontypes "github.com/containers/image/v5/pkg/compression/types"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"
	"golang.org/x/exp/slices"
)

// imageCopier tracks state specific to a single image (possibly an item of a manifest list)
type imageCopier struct {
	c                          *copier
	manifestUpdates            *types.ManifestUpdateOptions
	src                        *image.SourcedImage
	diffIDsAreNeeded           bool
	cannotModifyManifestReason string // The reason the manifest cannot be modified, or an empty string if it can
	canSubstituteBlobs         bool
	compressionFormat          *compressiontypes.Algorithm // Compression algorithm to use, if the user explicitly requested one, or nil.
	compressionLevel           *int
	ociEncryptLayers           *[]int
}

// copySingleImage copies a single (non-manifest-list) image unparsedImage, using policyContext to validate
// source image admissibility.
func (c *copier) copySingleImage(ctx context.Context, policyContext *signature.PolicyContext, options *Options, unparsedToplevel, unparsedImage *image.UnparsedImage, targetInstance *digest.Digest) (retManifest []byte, retManifestType string, retManifestDigest digest.Digest, retErr error) {
	// The caller is handling manifest lists; this could happen only if a manifest list contains a manifest list.
	// Make sure we fail cleanly in such cases.
	multiImage, err := isMultiImage(ctx, unparsedImage)
	if err != nil {
		// FIXME FIXME: How to name a reference for the sub-image?
		return nil, "", "", fmt.Errorf("determining manifest MIME type for %s: %w", transports.ImageName(unparsedImage.Reference()), err)
	}
	if multiImage {
		return nil, "", "", fmt.Errorf("Unexpectedly received a manifest list instead of a manifest for a single image")
	}

	// Please keep this policy check BEFORE reading any other information about the image.
	// (The multiImage check above only matches the MIME type, which we have received anyway.
	// Actual parsing of anything should be deferred.)
	if allowed, err := policyContext.IsRunningImageAllowed(ctx, unparsedImage); !allowed || err != nil { // Be paranoid and fail if either return value indicates so.
		return nil, "", "", fmt.Errorf("Source image rejected: %w", err)
	}
	src, err := image.FromUnparsedImage(ctx, options.SourceCtx, unparsedImage)
	if err != nil {
		return nil, "", "", fmt.Errorf("initializing image from source %s: %w", transports.ImageName(c.rawSource.Reference()), err)
	}

	// If the destination is a digested reference, make a note of that, determine what digest value we're
	// expecting, and check that the source manifest matches it.  If the source manifest doesn't, but it's
	// one item from a manifest list that matches it, accept that as a match.
	destIsDigestedReference := false
	if named := c.dest.Reference().DockerReference(); named != nil {
		if digested, ok := named.(reference.Digested); ok {
			destIsDigestedReference = true
			matches, err := manifest.MatchesDigest(src.ManifestBlob, digested.Digest())
			if err != nil {
				return nil, "", "", fmt.Errorf("computing digest of source image's manifest: %w", err)
			}
			if !matches {
				manifestList, _, err := unparsedToplevel.Manifest(ctx)
				if err != nil {
					return nil, "", "", fmt.Errorf("reading manifest from source image: %w", err)
				}
				matches, err = manifest.MatchesDigest(manifestList, digested.Digest())
				if err != nil {
					return nil, "", "", fmt.Errorf("computing digest of source image's manifest: %w", err)
				}
				if !matches {
					return nil, "", "", errors.New("Digest of source image's manifest would not match destination reference")
				}
			}
		}
	}

	if err := checkImageDestinationForCurrentRuntime(ctx, options.DestinationCtx, src, c.dest); err != nil {
		return nil, "", "", err
	}

	sigs, err := c.sourceSignatures(ctx, src, options,
		"Getting image source signatures",
		"Checking if image destination supports signatures")
	if err != nil {
		return nil, "", "", err
	}

	// Determine if we're allowed to modify the manifest.
	// If we can, set to the empty string. If we can't, set to the reason why.
	// Compare, and perhaps keep in sync with, the version in copyMultipleImages.
	cannotModifyManifestReason := ""
	if len(sigs) > 0 {
		cannotModifyManifestReason = "Would invalidate signatures"
	}
	if destIsDigestedReference {
		cannotModifyManifestReason = "Destination specifies a digest"
	}
	if options.PreserveDigests {
		cannotModifyManifestReason = "Instructed to preserve digests"
	}

	ic := imageCopier{
		c:               c,
		manifestUpdates: &types.ManifestUpdateOptions{InformationOnly: types.ManifestUpdateInformation{Destination: c.dest}},
		src:             src,
		// diffIDsAreNeeded is computed later
		cannotModifyManifestReason: cannotModifyManifestReason,
		ociEncryptLayers:           options.OciEncryptLayers,
	}
	if options.DestinationCtx != nil {
		// Note that compressionFormat and compressionLevel can be nil.
		ic.compressionFormat = options.DestinationCtx.CompressionFormat
		ic.compressionLevel = options.DestinationCtx.CompressionLevel
	}
	// Decide whether we can substitute blobs with semantic equivalents:
	// - Don’t do that if we can’t modify the manifest at all
	// - Ensure _this_ copy sees exactly the intended data when either processing a signed image or signing it.
	//   This may be too conservative, but for now, better safe than sorry, _especially_ on the len(c.signers) != 0 path:
	//   The signature makes the content non-repudiable, so it very much matters that the signature is made over exactly what the user intended.
	//   We do intend the RecordDigestUncompressedPair calls to only work with reliable data, but at least there’s a risk
	//   that the compressed version coming from a third party may be designed to attack some other decompressor implementation,
	//   and we would reuse and sign it.
	ic.canSubstituteBlobs = ic.cannotModifyManifestReason == "" && len(c.signers) == 0

	if err := ic.updateEmbeddedDockerReference(); err != nil {
		return nil, "", "", err
	}

	destRequiresOciEncryption := (isEncrypted(src) && ic.c.ociDecryptConfig != nil) || options.OciEncryptLayers != nil

	manifestConversionPlan, err := determineManifestConversion(determineManifestConversionInputs{
		srcMIMEType:                    ic.src.ManifestMIMEType,
		destSupportedManifestMIMETypes: ic.c.dest.SupportedManifestMIMETypes(),
		forceManifestMIMEType:          options.ForceManifestMIMEType,
		requiresOCIEncryption:          destRequiresOciEncryption,
		cannotModifyManifestReason:     ic.cannotModifyManifestReason,
	})
	if err != nil {
		return nil, "", "", err
	}
	// We set up this part of ic.manifestUpdates quite early, not just around the
	// code that calls copyUpdatedConfigAndManifest, so that other parts of the copy code
	// (e.g. the UpdatedImageNeedsLayerDiffIDs check just below) can make decisions based
	// on the expected destination format.
	if manifestConversionPlan.preferredMIMETypeNeedsConversion {
		ic.manifestUpdates.ManifestMIMEType = manifestConversionPlan.preferredMIMEType
	}

	// If src.UpdatedImageNeedsLayerDiffIDs(ic.manifestUpdates) will be true, it needs to be true by the time we get here.
	ic.diffIDsAreNeeded = src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates)

	// If enabled, fetch and compare the destination's manifest. And as an optimization skip updating the destination iff equal
	if options.OptimizeDestinationImageAlreadyExists {
		shouldUpdateSigs := len(sigs) > 0 || len(c.signers) != 0 // TODO: Consider allowing signatures updates only and skipping the image's layers/manifest copy if possible
		noPendingManifestUpdates := ic.noPendingManifestUpdates()

		logrus.Debugf("Checking if we can skip copying: has signatures=%t, OCI encryption=%t, no manifest updates=%t", shouldUpdateSigs, destRequiresOciEncryption, noPendingManifestUpdates)
		if !shouldUpdateSigs && !destRequiresOciEncryption && noPendingManifestUpdates {
			isSrcDestManifestEqual, retManifest, retManifestType, retManifestDigest, err := compareImageDestinationManifestEqual(ctx, options, src, targetInstance, c.dest)
			if err != nil {
				logrus.Warnf("Failed to compare destination image manifest: %v", err)
				return nil, "", "", err
			}

			if isSrcDestManifestEqual {
				c.Printf("Skipping: image already present at destination\n")
				return retManifest, retManifestType, retManifestDigest, nil
			}
		}
	}

	if err := ic.copyLayers(ctx); err != nil {
		return nil, "", "", err
	}

	// With docker/distribution registries we do not know whether the registry accepts schema2 or schema1 only;
	// and at least with the OpenShift registry "acceptschema2" option, there is no way to detect the support
	// without actually trying to upload something and getting a types.ManifestTypeRejectedError.
	// So, try the preferred manifest MIME type with possibly-updated blob digests, media types, and sizes if
	// we're altering how they're compressed.  If the process succeeds, fine…
	manifestBytes, retManifestDigest, err := ic.copyUpdatedConfigAndManifest(ctx, targetInstance)
	retManifestType = manifestConversionPlan.preferredMIMEType
	if err != nil {
		logrus.Debugf("Writing manifest using preferred type %s failed: %v", manifestConversionPlan.preferredMIMEType, err)
		// … if it fails, and the failure is either because the manifest is rejected by the registry, or
		// because we failed to create a manifest of the specified type because the specific manifest type
		// doesn't support the type of compression we're trying to use (e.g. docker v2s2 and zstd), we may
		// have other options available that could still succeed.
		var manifestTypeRejectedError types.ManifestTypeRejectedError
		var manifestLayerCompressionIncompatibilityError manifest.ManifestLayerCompressionIncompatibilityError
		isManifestRejected := errors.As(err, &manifestTypeRejectedError)
		isCompressionIncompatible := errors.As(err, &manifestLayerCompressionIncompatibilityError)
		if (!isManifestRejected && !isCompressionIncompatible) || len(manifestConversionPlan.otherMIMETypeCandidates) == 0 {
			// We don’t have other options.
			// In principle the code below would handle this as well, but the resulting  error message is fairly ugly.
			// Don’t bother the user with MIME types if we have no choice.
			return nil, "", "", err
		}
		// If the original MIME type is acceptable, determineManifestConversion always uses it as manifestConversionPlan.preferredMIMEType.
		// So if we are here, we will definitely be trying to convert the manifest.
		// With ic.cannotModifyManifestReason != "", that would just be a string of repeated failures for the same reason,
		// so let’s bail out early and with a better error message.
		if ic.cannotModifyManifestReason != "" {
			return nil, "", "", fmt.Errorf("writing manifest failed and we cannot try conversions: %q: %w", cannotModifyManifestReason, err)
		}

		// errs is a list of errors when trying various manifest types. Also serves as an "upload succeeded" flag when set to nil.
		errs := []string{fmt.Sprintf("%s(%v)", manifestConversionPlan.preferredMIMEType, err)}
		for _, manifestMIMEType := range manifestConversionPlan.otherMIMETypeCandidates {
			logrus.Debugf("Trying to use manifest type %s…", manifestMIMEType)
			ic.manifestUpdates.ManifestMIMEType = manifestMIMEType
			attemptedManifest, attemptedManifestDigest, err := ic.copyUpdatedConfigAndManifest(ctx, targetInstance)
			if err != nil {
				logrus.Debugf("Upload of manifest type %s failed: %v", manifestMIMEType, err)
				errs = append(errs, fmt.Sprintf("%s(%v)", manifestMIMEType, err))
				continue
			}

			// We have successfully uploaded a manifest.
			manifestBytes = attemptedManifest
			retManifestDigest = attemptedManifestDigest
			retManifestType = manifestMIMEType
			errs = nil // Mark this as a success so that we don't abort below.
			break
		}
		if errs != nil {
			return nil, "", "", fmt.Errorf("Uploading manifest failed, attempted the following formats: %s", strings.Join(errs, ", "))
		}
	}
	if targetInstance != nil {
		targetInstance = &retManifestDigest
	}

	newSigs, err := c.createSignatures(ctx, manifestBytes, options.SignIdentity)
	if err != nil {
		return nil, "", "", err
	}
	sigs = append(sigs, newSigs...)

	if len(sigs) > 0 {
		c.Printf("Storing signatures\n")
		if err := c.dest.PutSignaturesWithFormat(ctx, sigs, targetInstance); err != nil {
			return nil, "", "", fmt.Errorf("writing signatures: %w", err)
		}
	}

	return manifestBytes, retManifestType, retManifestDigest, nil
}

// checkImageDestinationForCurrentRuntime enforces dest.MustMatchRuntimeOS, if necessary.
func checkImageDestinationForCurrentRuntime(ctx context.Context, sys *types.SystemContext, src types.Image, dest types.ImageDestination) error {
	if dest.MustMatchRuntimeOS() {
		c, err := src.OCIConfig(ctx)
		if err != nil {
			return fmt.Errorf("parsing image configuration: %w", err)
		}
		wantedPlatforms, err := platform.WantedPlatforms(sys)
		if err != nil {
			return fmt.Errorf("getting current platform information %#v: %w", sys, err)
		}

		options := newOrderedSet()
		match := false
		for _, wantedPlatform := range wantedPlatforms {
			// Waiting for https://github.com/opencontainers/image-spec/pull/777 :
			// This currently can’t use image.MatchesPlatform because we don’t know what to use
			// for image.Variant.
			if wantedPlatform.OS == c.OS && wantedPlatform.Architecture == c.Architecture {
				match = true
				break
			}
			options.append(fmt.Sprintf("%s+%s", wantedPlatform.OS, wantedPlatform.Architecture))
		}
		if !match {
			logrus.Infof("Image operating system mismatch: image uses OS %q+architecture %q, expecting one of %q",
				c.OS, c.Architecture, strings.Join(options.list, ", "))
		}
	}
	return nil
}

// updateEmbeddedDockerReference handles the Docker reference embedded in Docker schema1 manifests.
func (ic *imageCopier) updateEmbeddedDockerReference() error {
	if ic.c.dest.IgnoresEmbeddedDockerReference() {
		return nil // Destination would prefer us not to update the embedded reference.
	}
	destRef := ic.c.dest.Reference().DockerReference()
	if destRef == nil {
		return nil // Destination does not care about Docker references
	}
	if !ic.src.EmbeddedDockerReferenceConflicts(destRef) {
		return nil // No reference embedded in the manifest, or it matches destRef already.
	}

	if ic.cannotModifyManifestReason != "" {
		return fmt.Errorf("Copying a schema1 image with an embedded Docker reference to %s (Docker reference %s) would change the manifest, which we cannot do: %q",
			transports.ImageName(ic.c.dest.Reference()), destRef.String(), ic.cannotModifyManifestReason)
	}
	ic.manifestUpdates.EmbeddedDockerReference = destRef
	return nil
}

func (ic *imageCopier) noPendingManifestUpdates() bool {
	return reflect.DeepEqual(*ic.manifestUpdates, types.ManifestUpdateOptions{InformationOnly: ic.manifestUpdates.InformationOnly})
}

// compareImageDestinationManifestEqual compares the `src` and `dest` image manifests (reading the manifest from the
// (possibly remote) destination). Returning true and the destination's manifest, type and digest if they compare equal.
func compareImageDestinationManifestEqual(ctx context.Context, options *Options, src *image.SourcedImage, targetInstance *digest.Digest, dest types.ImageDestination) (bool, []byte, string, digest.Digest, error) {
	srcManifestDigest, err := manifest.Digest(src.ManifestBlob)
	if err != nil {
		return false, nil, "", "", fmt.Errorf("calculating manifest digest: %w", err)
	}

	destImageSource, err := dest.Reference().NewImageSource(ctx, options.DestinationCtx)
	if err != nil {
		logrus.Debugf("Unable to create destination image %s source: %v", dest.Reference(), err)
		return false, nil, "", "", nil
	}

	destManifest, destManifestType, err := destImageSource.GetManifest(ctx, targetInstance)
	if err != nil {
		logrus.Debugf("Unable to get destination image %s/%s manifest: %v", destImageSource, targetInstance, err)
		return false, nil, "", "", nil
	}

	destManifestDigest, err := manifest.Digest(destManifest)
	if err != nil {
		return false, nil, "", "", fmt.Errorf("calculating manifest digest: %w", err)
	}

	logrus.Debugf("Comparing source and destination manifest digests: %v vs. %v", srcManifestDigest, destManifestDigest)
	if srcManifestDigest != destManifestDigest {
		return false, nil, "", "", nil
	}

	// Destination and source manifests, types and digests should all be equivalent
	return true, destManifest, destManifestType, destManifestDigest, nil
}

// copyLayers copies layers from ic.src/ic.c.rawSource to dest, using and updating ic.manifestUpdates if necessary and ic.cannotModifyManifestReason == "".
func (ic *imageCopier) copyLayers(ctx context.Context) error {
	srcInfos := ic.src.LayerInfos()
	numLayers := len(srcInfos)
	updatedSrcInfos, err := ic.src.LayerInfosForCopy(ctx)
	if err != nil {
		return err
	}
	srcInfosUpdated := false
	if updatedSrcInfos != nil && !reflect.DeepEqual(srcInfos, updatedSrcInfos) {
		if ic.cannotModifyManifestReason != "" {
			return fmt.Errorf("Copying this image would require changing layer representation, which we cannot do: %q", ic.cannotModifyManifestReason)
		}
		srcInfos = updatedSrcInfos
		srcInfosUpdated = true
	}

	type copyLayerData struct {
		destInfo types.BlobInfo
		diffID   digest.Digest
		err      error
	}

	// The manifest is used to extract the information whether a given
	// layer is empty.
	man, err := manifest.FromBlob(ic.src.ManifestBlob, ic.src.ManifestMIMEType)
	if err != nil {
		return err
	}
	manifestLayerInfos := man.LayerInfos()

	// copyGroup is used to determine if all layers are copied
	copyGroup := sync.WaitGroup{}

	data := make([]copyLayerData, numLayers)
	copyLayerHelper := func(index int, srcLayer types.BlobInfo, toEncrypt bool, pool *mpb.Progress, srcRef reference.Named) {
		defer ic.c.concurrentBlobCopiesSemaphore.Release(1)
		defer copyGroup.Done()
		cld := copyLayerData{}
		if !ic.c.downloadForeignLayers && ic.c.dest.AcceptsForeignLayerURLs() && len(srcLayer.URLs) != 0 {
			// DiffIDs are, currently, needed only when converting from schema1.
			// In which case src.LayerInfos will not have URLs because schema1
			// does not support them.
			if ic.diffIDsAreNeeded {
				cld.err = errors.New("getting DiffID for foreign layers is unimplemented")
			} else {
				cld.destInfo = srcLayer
				logrus.Debugf("Skipping foreign layer %q copy to %s", cld.destInfo.Digest, ic.c.dest.Reference().Transport().Name())
			}
		} else {
			cld.destInfo, cld.diffID, cld.err = ic.copyLayer(ctx, srcLayer, toEncrypt, pool, index, srcRef, manifestLayerInfos[index].EmptyLayer)
		}
		data[index] = cld
	}

	// Decide which layers to encrypt
	layersToEncrypt := set.New[int]()
	var encryptAll bool
	if ic.ociEncryptLayers != nil {
		encryptAll = len(*ic.ociEncryptLayers) == 0
		totalLayers := len(srcInfos)
		for _, l := range *ic.ociEncryptLayers {
			// if layer is negative, it is reverse indexed.
			layersToEncrypt.Add((totalLayers + l) % totalLayers)
		}

		if encryptAll {
			for i := 0; i < len(srcInfos); i++ {
				layersToEncrypt.Add(i)
			}
		}
	}

	if err := func() error { // A scope for defer
		progressPool := ic.c.newProgressPool()
		defer progressPool.Wait()

		// Ensure we wait for all layers to be copied. progressPool.Wait() must not be called while any of the copyLayerHelpers interact with the progressPool.
		defer copyGroup.Wait()

		for i, srcLayer := range srcInfos {
			err = ic.c.concurrentBlobCopiesSemaphore.Acquire(ctx, 1)
			if err != nil {
				// This can only fail with ctx.Err(), so no need to blame acquiring the semaphore.
				return fmt.Errorf("copying layer: %w", err)
			}
			copyGroup.Add(1)
			go copyLayerHelper(i, srcLayer, layersToEncrypt.Contains(i), progressPool, ic.c.rawSource.Reference().DockerReference())
		}

		// A call to copyGroup.Wait() is done at this point by the defer above.
		return nil
	}(); err != nil {
		return err
	}

	destInfos := make([]types.BlobInfo, numLayers)
	diffIDs := make([]digest.Digest, numLayers)
	for i, cld := range data {
		if cld.err != nil {
			return cld.err
		}
		destInfos[i] = cld.destInfo
		diffIDs[i] = cld.diffID
	}

	// WARNING: If you are adding new reasons to change ic.manifestUpdates, also update the
	// OptimizeDestinationImageAlreadyExists short-circuit conditions
	ic.manifestUpdates.InformationOnly.LayerInfos = destInfos
	if ic.diffIDsAreNeeded {
		ic.manifestUpdates.InformationOnly.LayerDiffIDs = diffIDs
	}
	if srcInfosUpdated || layerDigestsDiffer(srcInfos, destInfos) {
		ic.manifestUpdates.LayerInfos = destInfos
	}
	return nil
}

// layerDigestsDiffer returns true iff the digests in a and b differ (ignoring sizes and possible other fields)
func layerDigestsDiffer(a, b []types.BlobInfo) bool {
	return !slices.EqualFunc(a, b, func(a, b types.BlobInfo) bool {
		return a.Digest == b.Digest
	})
}

// copyUpdatedConfigAndManifest updates the image per ic.manifestUpdates, if necessary,
// stores the resulting config and manifest to the destination, and returns the stored manifest
// and its digest.
func (ic *imageCopier) copyUpdatedConfigAndManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, digest.Digest, error) {
	var pendingImage types.Image = ic.src
	if !ic.noPendingManifestUpdates() {
		if ic.cannotModifyManifestReason != "" {
			return nil, "", fmt.Errorf("Internal error: copy needs an updated manifest but that was known to be forbidden: %q", ic.cannotModifyManifestReason)
		}
		if !ic.diffIDsAreNeeded && ic.src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates) {
			// We have set ic.diffIDsAreNeeded based on the preferred MIME type returned by determineManifestConversion.
			// So, this can only happen if we are trying to upload using one of the other MIME type candidates.
			// Because UpdatedImageNeedsLayerDiffIDs is true only when converting from s1 to s2, this case should only arise
			// when ic.c.dest.SupportedManifestMIMETypes() includes both s1 and s2, the upload using s1 failed, and we are now trying s2.
			// Supposedly s2-only registries do not exist or are extremely rare, so failing with this error message is good enough for now.
			// If handling such registries turns out to be necessary, we could compute ic.diffIDsAreNeeded based on the full list of manifest MIME type candidates.
			return nil, "", fmt.Errorf("Can not convert image to %s, preparing DiffIDs for this case is not supported", ic.manifestUpdates.ManifestMIMEType)
		}
		pi, err := ic.src.UpdatedImage(ctx, *ic.manifestUpdates)
		if err != nil {
			return nil, "", fmt.Errorf("creating an updated image manifest: %w", err)
		}
		pendingImage = pi
	}
	man, _, err := pendingImage.Manifest(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	if err := ic.copyConfig(ctx, pendingImage); err != nil {
		return nil, "", err
	}

	ic.c.Printf("Writing manifest to image destination\n")
	manifestDigest, err := manifest.Digest(man)
	if err != nil {
		return nil, "", err
	}
	if instanceDigest != nil {
		instanceDigest = &manifestDigest
	}
	if err := ic.c.dest.PutManifest(ctx, man, instanceDigest); err != nil {
		logrus.Debugf("Error %v while writing manifest %q", err, string(man))
		return nil, "", fmt.Errorf("writing manifest: %w", err)
	}
	return man, manifestDigest, nil
}

// copyConfig copies config.json, if any, from src to dest.
func (ic *imageCopier) copyConfig(ctx context.Context, src types.Image) error {
	srcInfo := src.ConfigInfo()
	if srcInfo.Digest != "" {
		if err := ic.c.concurrentBlobCopiesSemaphore.Acquire(ctx, 1); err != nil {
			// This can only fail with ctx.Err(), so no need to blame acquiring the semaphore.
			return fmt.Errorf("copying config: %w", err)
		}
		defer ic.c.concurrentBlobCopiesSemaphore.Release(1)

		destInfo, err := func() (types.BlobInfo, error) { // A scope for defer
			progressPool := ic.c.newProgressPool()
			defer progressPool.Wait()
			bar := ic.c.createProgressBar(progressPool, false, srcInfo, "config", "done")
			defer bar.Abort(false)
			ic.c.printCopyInfo("config", srcInfo)

			configBlob, err := src.ConfigBlob(ctx)
			if err != nil {
				return types.BlobInfo{}, fmt.Errorf("reading config blob %s: %w", srcInfo.Digest, err)
			}

			destInfo, err := ic.copyBlobFromStream(ctx, bytes.NewReader(configBlob), srcInfo, nil, true, false, bar, -1, false)
			if err != nil {
				return types.BlobInfo{}, err
			}

			bar.mark100PercentComplete()
			return destInfo, nil
		}()
		if err != nil {
			return err
		}
		if destInfo.Digest != srcInfo.Digest {
			return fmt.Errorf("Internal error: copying uncompressed config blob %s changed digest to %s", srcInfo.Digest, destInfo.Digest)
		}
	}
	return nil
}

// diffIDResult contains both a digest value and an error from diffIDComputationGoroutine.
// We could also send the error through the pipeReader, but this more cleanly separates the copying of the layer and the DiffID computation.
type diffIDResult struct {
	digest digest.Digest
	err    error
}

// copyLayer copies a layer with srcInfo (with known Digest and Annotations and possibly known Size) in src to dest, perhaps (de/re/)compressing it,
// and returns a complete blobInfo of the copied layer, and a value for LayerDiffIDs if diffIDIsNeeded
// srcRef can be used as an additional hint to the destination during checking whether a layer can be reused but srcRef can be nil.
func (ic *imageCopier) copyLayer(ctx context.Context, srcInfo types.BlobInfo, toEncrypt bool, pool *mpb.Progress, layerIndex int, srcRef reference.Named, emptyLayer bool) (types.BlobInfo, digest.Digest, error) {
	// If the srcInfo doesn't contain compression information, try to compute it from the
	// MediaType, which was either read from a manifest by way of LayerInfos() or constructed
	// by LayerInfosForCopy(), if it was supplied at all.  If we succeed in copying the blob,
	// the BlobInfo we return will be passed to UpdatedImage() and then to UpdateLayerInfos(),
	// which uses the compression information to compute the updated MediaType values.
	// (Sadly UpdatedImage() is documented to not update MediaTypes from
	//  ManifestUpdateOptions.LayerInfos[].MediaType, so we are doing it indirectly.)
	//
	// This MIME type → compression mapping belongs in manifest-specific code in our manifest
	// package (but we should preferably replace/change UpdatedImage instead of productizing
	// this workaround).
	if srcInfo.CompressionAlgorithm == nil {
		switch srcInfo.MediaType {
		case manifest.DockerV2Schema2LayerMediaType, imgspecv1.MediaTypeImageLayerGzip:
			srcInfo.CompressionAlgorithm = &compression.Gzip
		case imgspecv1.MediaTypeImageLayerZstd:
			srcInfo.CompressionAlgorithm = &compression.Zstd
		}
	}

	ic.c.printCopyInfo("blob", srcInfo)

	cachedDiffID := ic.c.blobInfoCache.UncompressedDigest(srcInfo.Digest) // May be ""
	diffIDIsNeeded := ic.diffIDsAreNeeded && cachedDiffID == ""
	// When encrypting to decrypting, only use the simple code path. We might be able to optimize more
	// (e.g. if we know the DiffID of an encrypted compressed layer, it might not be necessary to pull, decrypt and decompress again),
	// but it’s not trivially safe to do such things, so until someone takes the effort to make a comprehensive argument, let’s not.
	encryptingOrDecrypting := toEncrypt || (isOciEncrypted(srcInfo.MediaType) && ic.c.ociDecryptConfig != nil)
	canAvoidProcessingCompleteLayer := !diffIDIsNeeded && !encryptingOrDecrypting

	// Don’t read the layer from the source if we already have the blob, and optimizations are acceptable.
	if canAvoidProcessingCompleteLayer {
		canChangeLayerCompression := ic.src.CanChangeLayerCompression(srcInfo.MediaType)
		logrus.Debugf("Checking if we can reuse blob %s: general substitution = %v, compression for MIME type %q = %v",
			srcInfo.Digest, ic.canSubstituteBlobs, srcInfo.MediaType, canChangeLayerCompression)
		canSubstitute := ic.canSubstituteBlobs && ic.src.CanChangeLayerCompression(srcInfo.MediaType)
		// TODO: at this point we don't know whether or not a blob we end up reusing is compressed using an algorithm
		// that is acceptable for use on layers in the manifest that we'll be writing later, so if we end up reusing
		// a blob that's compressed with e.g. zstd, but we're only allowed to write a v2s2 manifest, this will cause
		// a failure when we eventually try to update the manifest with the digest and MIME type of the reused blob.
		// Fixing that will probably require passing more information to TryReusingBlob() than the current version of
		// the ImageDestination interface lets us pass in.
		reused, reusedBlob, err := ic.c.dest.TryReusingBlobWithOptions(ctx, srcInfo, private.TryReusingBlobOptions{
			Cache:         ic.c.blobInfoCache,
			CanSubstitute: canSubstitute,
			EmptyLayer:    emptyLayer,
			LayerIndex:    &layerIndex,
			SrcRef:        srcRef,
		})
		if err != nil {
			return types.BlobInfo{}, "", fmt.Errorf("trying to reuse blob %s at destination: %w", srcInfo.Digest, err)
		}
		if reused {
			logrus.Debugf("Skipping blob %s (already present):", srcInfo.Digest)
			func() { // A scope for defer
				bar := ic.c.createProgressBar(pool, false, types.BlobInfo{Digest: reusedBlob.Digest, Size: 0}, "blob", "skipped: already exists")
				defer bar.Abort(false)
				bar.mark100PercentComplete()
			}()

			// Throw an event that the layer has been skipped
			if ic.c.progress != nil && ic.c.progressInterval > 0 {
				ic.c.progress <- types.ProgressProperties{
					Event:    types.ProgressEventSkipped,
					Artifact: srcInfo,
				}
			}

			return updatedBlobInfoFromReuse(srcInfo, reusedBlob), cachedDiffID, nil
		}
	}

	// A partial pull is managed by the destination storage, that decides what portions
	// of the source file are not known yet and must be fetched.
	// Attempt a partial only when the source allows to retrieve a blob partially and
	// the destination has support for it.
	if canAvoidProcessingCompleteLayer && ic.c.rawSource.SupportsGetBlobAt() && ic.c.dest.SupportsPutBlobPartial() {
		if reused, blobInfo := func() (bool, types.BlobInfo) { // A scope for defer
			bar := ic.c.createProgressBar(pool, true, srcInfo, "blob", "done")
			hideProgressBar := true
			defer func() { // Note that this is not the same as defer bar.Abort(hideProgressBar); we need hideProgressBar to be evaluated lazily.
				bar.Abort(hideProgressBar)
			}()

			proxy := blobChunkAccessorProxy{
				wrapped: ic.c.rawSource,
				bar:     bar,
			}
			uploadedBlob, err := ic.c.dest.PutBlobPartial(ctx, &proxy, srcInfo, ic.c.blobInfoCache)
			if err == nil {
				if srcInfo.Size != -1 {
					bar.SetRefill(srcInfo.Size - bar.Current())
				}
				bar.mark100PercentComplete()
				hideProgressBar = false
				logrus.Debugf("Retrieved partial blob %v", srcInfo.Digest)
				return true, updatedBlobInfoFromUpload(srcInfo, uploadedBlob)
			}
			logrus.Debugf("Failed to retrieve partial blob: %v", err)
			return false, types.BlobInfo{}
		}(); reused {
			return blobInfo, cachedDiffID, nil
		}
	}

	// Fallback: copy the layer, computing the diffID if we need to do so
	return func() (types.BlobInfo, digest.Digest, error) { // A scope for defer
		bar := ic.c.createProgressBar(pool, false, srcInfo, "blob", "done")
		defer bar.Abort(false)

		srcStream, srcBlobSize, err := ic.c.rawSource.GetBlob(ctx, srcInfo, ic.c.blobInfoCache)
		if err != nil {
			return types.BlobInfo{}, "", fmt.Errorf("reading blob %s: %w", srcInfo.Digest, err)
		}
		defer srcStream.Close()

		blobInfo, diffIDChan, err := ic.copyLayerFromStream(ctx, srcStream, types.BlobInfo{Digest: srcInfo.Digest, Size: srcBlobSize, MediaType: srcInfo.MediaType, Annotations: srcInfo.Annotations}, diffIDIsNeeded, toEncrypt, bar, layerIndex, emptyLayer)
		if err != nil {
			return types.BlobInfo{}, "", err
		}

		diffID := cachedDiffID
		if diffIDIsNeeded {
			select {
			case <-ctx.Done():
				return types.BlobInfo{}, "", ctx.Err()
			case diffIDResult := <-diffIDChan:
				if diffIDResult.err != nil {
					return types.BlobInfo{}, "", fmt.Errorf("computing layer DiffID: %w", diffIDResult.err)
				}
				logrus.Debugf("Computed DiffID %s for layer %s", diffIDResult.digest, srcInfo.Digest)
				// Don’t record any associations that involve encrypted data. This is a bit crude,
				// some blob substitutions (replacing pulls of encrypted data with local reuse of known decryption outcomes)
				// might be safe, but it’s not trivially obvious, so let’s be conservative for now.
				// This crude approach also means we don’t need to record whether a blob is encrypted
				// in the blob info cache (which would probably be necessary for any more complex logic),
				// and the simplicity is attractive.
				if !encryptingOrDecrypting {
					// This is safe because we have just computed diffIDResult.Digest ourselves, and in the process
					// we have read all of the input blob, so srcInfo.Digest must have been validated by digestingReader.
					ic.c.blobInfoCache.RecordDigestUncompressedPair(srcInfo.Digest, diffIDResult.digest)
				}
				diffID = diffIDResult.digest
			}
		}

		bar.mark100PercentComplete()
		return blobInfo, diffID, nil
	}()
}

// updatedBlobInfoFromReuse returns inputInfo updated with reusedBlob which was created based on inputInfo.
func updatedBlobInfoFromReuse(inputInfo types.BlobInfo, reusedBlob private.ReusedBlob) types.BlobInfo {
	// The transport is only tasked with finding the blob, determining its size if necessary, and returning the right
	// compression format if the blob was substituted.
	// Handling of compression, encryption, and the related MIME types and the like are all the responsibility
	// of the generic code in this package.
	res := types.BlobInfo{
		Digest:               reusedBlob.Digest,
		Size:                 reusedBlob.Size,
		URLs:                 nil,                   // This _must_ be cleared if Digest changes; clear it in other cases as well, to preserve previous behavior.
		Annotations:          inputInfo.Annotations, // FIXME: This should remove zstd:chunked annotations (but those annotations being left with incorrect values should not break pulls)
		MediaType:            inputInfo.MediaType,   // Mostly irrelevant, MediaType is updated based on Compression*/CryptoOperation.
		CompressionOperation: reusedBlob.CompressionOperation,
		CompressionAlgorithm: reusedBlob.CompressionAlgorithm,
		CryptoOperation:      inputInfo.CryptoOperation, // Expected to be unset anyway.
	}
	// The transport is only expected to fill CompressionOperation and CompressionAlgorithm
	// if the blob was substituted; otherwise, fill it in based
	// on what we know from the srcInfos we were given.
	if reusedBlob.Digest == inputInfo.Digest {
		res.CompressionOperation = inputInfo.CompressionOperation
		res.CompressionAlgorithm = inputInfo.CompressionAlgorithm
	}
	return res
}

// copyLayerFromStream is an implementation detail of copyLayer; mostly providing a separate “defer” scope.
// it copies a blob with srcInfo (with known Digest and Annotations and possibly known Size) from srcStream to dest,
// perhaps (de/re/)compressing the stream,
// and returns a complete blobInfo of the copied blob and perhaps a <-chan diffIDResult if diffIDIsNeeded, to be read by the caller.
func (ic *imageCopier) copyLayerFromStream(ctx context.Context, srcStream io.Reader, srcInfo types.BlobInfo,
	diffIDIsNeeded bool, toEncrypt bool, bar *progressBar, layerIndex int, emptyLayer bool) (types.BlobInfo, <-chan diffIDResult, error) {
	var getDiffIDRecorder func(compressiontypes.DecompressorFunc) io.Writer // = nil
	var diffIDChan chan diffIDResult

	err := errors.New("Internal error: unexpected panic in copyLayer") // For pipeWriter.CloseWithbelow
	if diffIDIsNeeded {
		diffIDChan = make(chan diffIDResult, 1) // Buffered, so that sending a value after this or our caller has failed and exited does not block.
		pipeReader, pipeWriter := io.Pipe()
		defer func() { // Note that this is not the same as {defer pipeWriter.CloseWithError(err)}; we need err to be evaluated lazily.
			_ = pipeWriter.CloseWithError(err) // CloseWithError(nil) is equivalent to Close(), always returns nil
		}()

		getDiffIDRecorder = func(decompressor compressiontypes.DecompressorFunc) io.Writer {
			// If this fails, e.g. because we have exited and due to pipeWriter.CloseWithError() above further
			// reading from the pipe has failed, we don’t really care.
			// We only read from diffIDChan if the rest of the flow has succeeded, and when we do read from it,
			// the return value includes an error indication, which we do check.
			//
			// If this gets never called, pipeReader will not be used anywhere, but pipeWriter will only be
			// closed above, so we are happy enough with both pipeReader and pipeWriter to just get collected by GC.
			go diffIDComputationGoroutine(diffIDChan, pipeReader, decompressor) // Closes pipeReader
			return pipeWriter
		}
	}

	blobInfo, err := ic.copyBlobFromStream(ctx, srcStream, srcInfo, getDiffIDRecorder, false, toEncrypt, bar, layerIndex, emptyLayer) // Sets err to nil on success
	return blobInfo, diffIDChan, err
	// We need the defer … pipeWriter.CloseWithError() to happen HERE so that the caller can block on reading from diffIDChan
}

// diffIDComputationGoroutine reads all input from layerStream, uncompresses using decompressor if necessary, and sends its digest, and status, if any, to dest.
func diffIDComputationGoroutine(dest chan<- diffIDResult, layerStream io.ReadCloser, decompressor compressiontypes.DecompressorFunc) {
	result := diffIDResult{
		digest: "",
		err:    errors.New("Internal error: unexpected panic in diffIDComputationGoroutine"),
	}
	defer func() { dest <- result }()
	defer layerStream.Close() // We do not care to bother the other end of the pipe with other failures; we send them to dest instead.

	result.digest, result.err = computeDiffID(layerStream, decompressor)
}

// computeDiffID reads all input from layerStream, uncompresses it using decompressor if necessary, and returns its digest.
func computeDiffID(stream io.Reader, decompressor compressiontypes.DecompressorFunc) (digest.Digest, error) {
	if decompressor != nil {
		s, err := decompressor(stream)
		if err != nil {
			return "", err
		}
		defer s.Close()
		stream = s
	}

	return digest.Canonical.FromReader(stream)
}
