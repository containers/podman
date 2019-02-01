package copy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containers/image/image"
	"github.com/containers/image/pkg/blobinfocache"
	"github.com/containers/image/pkg/compression"
	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/semaphore"
	pb "gopkg.in/cheggaaa/pb.v1"
)

type digestingReader struct {
	source              io.Reader
	digester            digest.Digester
	expectedDigest      digest.Digest
	validationFailed    bool
	validationSucceeded bool
}

// maxParallelDownloads is used to limit the maxmimum number of parallel
// downloads.  Let's follow Firefox by limiting it to 6.
var maxParallelDownloads = 6

// newDigestingReader returns an io.Reader implementation with contents of source, which will eventually return a non-EOF error
// or set validationSucceeded/validationFailed to true if the source stream does/does not match expectedDigest.
// (neither is set if EOF is never reached).
func newDigestingReader(source io.Reader, expectedDigest digest.Digest) (*digestingReader, error) {
	if err := expectedDigest.Validate(); err != nil {
		return nil, errors.Errorf("Invalid digest specification %s", expectedDigest)
	}
	digestAlgorithm := expectedDigest.Algorithm()
	if !digestAlgorithm.Available() {
		return nil, errors.Errorf("Invalid digest specification %s: unsupported digest algorithm %s", expectedDigest, digestAlgorithm)
	}
	return &digestingReader{
		source:           source,
		digester:         digestAlgorithm.Digester(),
		expectedDigest:   expectedDigest,
		validationFailed: false,
	}, nil
}

func (d *digestingReader) Read(p []byte) (int, error) {
	n, err := d.source.Read(p)
	if n > 0 {
		if n2, err := d.digester.Hash().Write(p[:n]); n2 != n || err != nil {
			// Coverage: This should not happen, the hash.Hash interface requires
			// d.digest.Write to never return an error, and the io.Writer interface
			// requires n2 == len(input) if no error is returned.
			return 0, errors.Wrapf(err, "Error updating digest during verification: %d vs. %d", n2, n)
		}
	}
	if err == io.EOF {
		actualDigest := d.digester.Digest()
		if actualDigest != d.expectedDigest {
			d.validationFailed = true
			return 0, errors.Errorf("Digest did not match, expected %s, got %s", d.expectedDigest, actualDigest)
		}
		d.validationSucceeded = true
	}
	return n, err
}

// copier allows us to keep track of diffID values for blobs, and other
// data shared across one or more images in a possible manifest list.
type copier struct {
	dest             types.ImageDestination
	rawSource        types.ImageSource
	reportWriter     io.Writer
	progressOutput   io.Writer
	progressInterval time.Duration
	progress         chan types.ProgressProperties
	blobInfoCache    types.BlobInfoCache
	copyInParallel   bool
}

// imageCopier tracks state specific to a single image (possibly an item of a manifest list)
type imageCopier struct {
	c                  *copier
	manifestUpdates    *types.ManifestUpdateOptions
	src                types.Image
	diffIDsAreNeeded   bool
	canModifyManifest  bool
	canSubstituteBlobs bool
}

// Options allows supplying non-default configuration modifying the behavior of CopyImage.
type Options struct {
	RemoveSignatures bool   // Remove any pre-existing signatures. SignBy will still add a new signature.
	SignBy           string // If non-empty, asks for a signature to be added during the copy, and specifies a key ID, as accepted by signature.NewGPGSigningMechanism().SignDockerManifest(),
	ReportWriter     io.Writer
	SourceCtx        *types.SystemContext
	DestinationCtx   *types.SystemContext
	ProgressInterval time.Duration                 // time to wait between reports to signal the progress channel
	Progress         chan types.ProgressProperties // Reported to when ProgressInterval has arrived for a single artifact+offset.
	// manifest MIME type of image set by user. "" is default and means use the autodetection to the the manifest MIME type
	ForceManifestMIMEType string
}

// Image copies image from srcRef to destRef, using policyContext to validate
// source image admissibility.  It returns the manifest which was written to
// the new copy of the image.
func Image(ctx context.Context, policyContext *signature.PolicyContext, destRef, srcRef types.ImageReference, options *Options) (manifest []byte, retErr error) {
	// NOTE this function uses an output parameter for the error return value.
	// Setting this and returning is the ideal way to return an error.
	//
	// the defers in this routine will wrap the error return with its own errors
	// which can be valuable context in the middle of a multi-streamed copy.
	if options == nil {
		options = &Options{}
	}

	reportWriter := ioutil.Discard

	if options.ReportWriter != nil {
		reportWriter = options.ReportWriter
	}

	dest, err := destRef.NewImageDestination(ctx, options.DestinationCtx)
	if err != nil {
		return nil, errors.Wrapf(err, "Error initializing destination %s", transports.ImageName(destRef))
	}
	defer func() {
		if err := dest.Close(); err != nil {
			retErr = errors.Wrapf(retErr, " (dest: %v)", err)
		}
	}()

	rawSource, err := srcRef.NewImageSource(ctx, options.SourceCtx)
	if err != nil {
		return nil, errors.Wrapf(err, "Error initializing source %s", transports.ImageName(srcRef))
	}
	defer func() {
		if err := rawSource.Close(); err != nil {
			retErr = errors.Wrapf(retErr, " (src: %v)", err)
		}
	}()

	// If reportWriter is not a TTY (e.g., when piping to a file), do not
	// print the progress bars to avoid long and hard to parse output.
	// createProgressBar() will print a single line instead.
	progressOutput := reportWriter
	if !isTTY(reportWriter) {
		progressOutput = ioutil.Discard
	}
	copyInParallel := dest.HasThreadSafePutBlob() && rawSource.HasThreadSafeGetBlob()
	c := &copier{
		dest:             dest,
		rawSource:        rawSource,
		reportWriter:     reportWriter,
		progressOutput:   progressOutput,
		progressInterval: options.ProgressInterval,
		progress:         options.Progress,
		copyInParallel:   copyInParallel,
		// FIXME? The cache is used for sources and destinations equally, but we only have a SourceCtx and DestinationCtx.
		// For now, use DestinationCtx (because blob reuse changes the behavior of the destination side more); eventually
		// we might want to add a separate CommonCtx — or would that be too confusing?
		blobInfoCache: blobinfocache.DefaultCache(options.DestinationCtx),
	}

	unparsedToplevel := image.UnparsedInstance(rawSource, nil)
	multiImage, err := isMultiImage(ctx, unparsedToplevel)
	if err != nil {
		return nil, errors.Wrapf(err, "Error determining manifest MIME type for %s", transports.ImageName(srcRef))
	}

	if !multiImage {
		// The simple case: Just copy a single image.
		if manifest, err = c.copyOneImage(ctx, policyContext, options, unparsedToplevel); err != nil {
			return nil, err
		}
	} else {
		// This is a manifest list. Choose a single image and copy it.
		// FIXME: Copy to destinations which support manifest lists, one image at a time.
		instanceDigest, err := image.ChooseManifestInstanceFromManifestList(ctx, options.SourceCtx, unparsedToplevel)
		if err != nil {
			return nil, errors.Wrapf(err, "Error choosing an image from manifest list %s", transports.ImageName(srcRef))
		}
		logrus.Debugf("Source is a manifest list; copying (only) instance %s", instanceDigest)
		unparsedInstance := image.UnparsedInstance(rawSource, &instanceDigest)

		if manifest, err = c.copyOneImage(ctx, policyContext, options, unparsedInstance); err != nil {
			return nil, err
		}
	}

	if err := c.dest.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "Error committing the finished image")
	}

	return manifest, nil
}

// Image copies a single (on-manifest-list) image unparsedImage, using policyContext to validate
// source image admissibility.
func (c *copier) copyOneImage(ctx context.Context, policyContext *signature.PolicyContext, options *Options, unparsedImage *image.UnparsedImage) (manifest []byte, retErr error) {
	// The caller is handling manifest lists; this could happen only if a manifest list contains a manifest list.
	// Make sure we fail cleanly in such cases.
	multiImage, err := isMultiImage(ctx, unparsedImage)
	if err != nil {
		// FIXME FIXME: How to name a reference for the sub-image?
		return nil, errors.Wrapf(err, "Error determining manifest MIME type for %s", transports.ImageName(unparsedImage.Reference()))
	}
	if multiImage {
		return nil, fmt.Errorf("Unexpectedly received a manifest list instead of a manifest for a single image")
	}

	// Please keep this policy check BEFORE reading any other information about the image.
	// (the multiImage check above only matches the MIME type, which we have received anyway.
	// Actual parsing of anything should be deferred.)
	if allowed, err := policyContext.IsRunningImageAllowed(ctx, unparsedImage); !allowed || err != nil { // Be paranoid and fail if either return value indicates so.
		return nil, errors.Wrap(err, "Source image rejected")
	}
	src, err := image.FromUnparsedImage(ctx, options.SourceCtx, unparsedImage)
	if err != nil {
		return nil, errors.Wrapf(err, "Error initializing image from source %s", transports.ImageName(c.rawSource.Reference()))
	}

	if err := checkImageDestinationForCurrentRuntimeOS(ctx, options.DestinationCtx, src, c.dest); err != nil {
		return nil, err
	}

	var sigs [][]byte
	if options.RemoveSignatures {
		sigs = [][]byte{}
	} else {
		c.Printf("Getting image source signatures\n")
		s, err := src.Signatures(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "Error reading signatures")
		}
		sigs = s
	}
	if len(sigs) != 0 {
		c.Printf("Checking if image destination supports signatures\n")
		if err := c.dest.SupportsSignatures(ctx); err != nil {
			return nil, errors.Wrap(err, "Can not copy signatures")
		}
	}

	ic := imageCopier{
		c:               c,
		manifestUpdates: &types.ManifestUpdateOptions{InformationOnly: types.ManifestUpdateInformation{Destination: c.dest}},
		src:             src,
		// diffIDsAreNeeded is computed later
		canModifyManifest: len(sigs) == 0,
		// Ensure _this_ copy sees exactly the intended data when either processing a signed image or signing it.
		// This may be too conservative, but for now, better safe than sorry, _especially_ on the SignBy path:
		// The signature makes the content non-repudiable, so it very much matters that the signature is made over exactly what the user intended.
		// We do intend the RecordDigestUncompressedPair calls to only work with reliable data, but at least there’s a risk
		// that the compressed version coming from a third party may be designed to attack some other decompressor implementation,
		// and we would reuse and sign it.
		canSubstituteBlobs: len(sigs) == 0 && options.SignBy == "",
	}

	if err := ic.updateEmbeddedDockerReference(); err != nil {
		return nil, err
	}

	// We compute preferredManifestMIMEType only to show it in error messages.
	// Without having to add this context in an error message, we would be happy enough to know only that no conversion is needed.
	preferredManifestMIMEType, otherManifestMIMETypeCandidates, err := ic.determineManifestConversion(ctx, c.dest.SupportedManifestMIMETypes(), options.ForceManifestMIMEType)
	if err != nil {
		return nil, err
	}

	// If src.UpdatedImageNeedsLayerDiffIDs(ic.manifestUpdates) will be true, it needs to be true by the time we get here.
	ic.diffIDsAreNeeded = src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates)

	if err := ic.copyLayers(ctx); err != nil {
		return nil, err
	}

	// With docker/distribution registries we do not know whether the registry accepts schema2 or schema1 only;
	// and at least with the OpenShift registry "acceptschema2" option, there is no way to detect the support
	// without actually trying to upload something and getting a types.ManifestTypeRejectedError.
	// So, try the preferred manifest MIME type. If the process succeeds, fine…
	manifest, err = ic.copyUpdatedConfigAndManifest(ctx)
	if err != nil {
		logrus.Debugf("Writing manifest using preferred type %s failed: %v", preferredManifestMIMEType, err)
		// … if it fails, _and_ the failure is because the manifest is rejected, we may have other options.
		if _, isManifestRejected := errors.Cause(err).(types.ManifestTypeRejectedError); !isManifestRejected || len(otherManifestMIMETypeCandidates) == 0 {
			// We don’t have other options.
			// In principle the code below would handle this as well, but the resulting  error message is fairly ugly.
			// Don’t bother the user with MIME types if we have no choice.
			return nil, err
		}
		// If the original MIME type is acceptable, determineManifestConversion always uses it as preferredManifestMIMEType.
		// So if we are here, we will definitely be trying to convert the manifest.
		// With !ic.canModifyManifest, that would just be a string of repeated failures for the same reason,
		// so let’s bail out early and with a better error message.
		if !ic.canModifyManifest {
			return nil, errors.Wrap(err, "Writing manifest failed (and converting it is not possible)")
		}

		// errs is a list of errors when trying various manifest types. Also serves as an "upload succeeded" flag when set to nil.
		errs := []string{fmt.Sprintf("%s(%v)", preferredManifestMIMEType, err)}
		for _, manifestMIMEType := range otherManifestMIMETypeCandidates {
			logrus.Debugf("Trying to use manifest type %s…", manifestMIMEType)
			ic.manifestUpdates.ManifestMIMEType = manifestMIMEType
			attemptedManifest, err := ic.copyUpdatedConfigAndManifest(ctx)
			if err != nil {
				logrus.Debugf("Upload of manifest type %s failed: %v", manifestMIMEType, err)
				errs = append(errs, fmt.Sprintf("%s(%v)", manifestMIMEType, err))
				continue
			}

			// We have successfully uploaded a manifest.
			manifest = attemptedManifest
			errs = nil // Mark this as a success so that we don't abort below.
			break
		}
		if errs != nil {
			return nil, fmt.Errorf("Uploading manifest failed, attempted the following formats: %s", strings.Join(errs, ", "))
		}
	}

	if options.SignBy != "" {
		newSig, err := c.createSignature(manifest, options.SignBy)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, newSig)
	}

	c.Printf("Storing signatures\n")
	if err := c.dest.PutSignatures(ctx, sigs); err != nil {
		return nil, errors.Wrap(err, "Error writing signatures")
	}

	return manifest, nil
}

// Printf writes a formatted string to c.reportWriter.
// Note that the method name Printf is not entirely arbitrary: (go tool vet)
// has a built-in list of functions/methods (whatever object they are for)
// which have their format strings checked; for other names we would have
// to pass a parameter to every (go tool vet) invocation.
func (c *copier) Printf(format string, a ...interface{}) {
	fmt.Fprintf(c.reportWriter, format, a...)
}

func checkImageDestinationForCurrentRuntimeOS(ctx context.Context, sys *types.SystemContext, src types.Image, dest types.ImageDestination) error {
	if dest.MustMatchRuntimeOS() {
		wantedOS := runtime.GOOS
		if sys != nil && sys.OSChoice != "" {
			wantedOS = sys.OSChoice
		}
		c, err := src.OCIConfig(ctx)
		if err != nil {
			return errors.Wrapf(err, "Error parsing image configuration")
		}
		osErr := fmt.Errorf("image operating system %q cannot be used on %q", c.OS, wantedOS)
		if wantedOS == "windows" && c.OS == "linux" {
			return osErr
		} else if wantedOS != "windows" && c.OS == "windows" {
			return osErr
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

	if !ic.canModifyManifest {
		return errors.Errorf("Copying a schema1 image with an embedded Docker reference to %s (Docker reference %s) would invalidate existing signatures. Explicitly enable signature removal to proceed anyway",
			transports.ImageName(ic.c.dest.Reference()), destRef.String())
	}
	ic.manifestUpdates.EmbeddedDockerReference = destRef
	return nil
}

// shortDigest returns the first 12 characters of the digest.
func shortDigest(d digest.Digest) string {
	return d.Encoded()[:12]
}

// createProgressBar creates a pb.ProgressBar.  Note that if the copier's
// reportWriter is ioutil.Discard, the progress bar's output will be discarded
// and a single line will be printed instead.
func (c *copier) createProgressBar(srcInfo types.BlobInfo, kind string) *pb.ProgressBar {
	bar := pb.New(int(srcInfo.Size)).SetUnits(pb.U_BYTES)
	bar.SetMaxWidth(80)
	bar.ShowTimeLeft = false
	bar.ShowPercent = false
	bar.Prefix(fmt.Sprintf("Copying %s %s:", kind, shortDigest(srcInfo.Digest)))
	bar.Output = c.progressOutput
	if bar.Output == ioutil.Discard {
		c.Printf("Copying %s %s\n", kind, srcInfo.Digest)
	}
	return bar
}

// isTTY returns true if the io.Writer is a file and a tty.
func isTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return terminal.IsTerminal(int(f.Fd()))
	}
	return false
}

// copyLayers copies layers from ic.src/ic.c.rawSource to dest, using and updating ic.manifestUpdates if necessary and ic.canModifyManifest.
func (ic *imageCopier) copyLayers(ctx context.Context) error {
	srcInfos := ic.src.LayerInfos()
	numLayers := len(srcInfos)
	updatedSrcInfos, err := ic.src.LayerInfosForCopy(ctx)
	if err != nil {
		return err
	}
	srcInfosUpdated := false
	if updatedSrcInfos != nil && !reflect.DeepEqual(srcInfos, updatedSrcInfos) {
		if !ic.canModifyManifest {
			return errors.Errorf("Internal error: copyLayers() needs to use an updated manifest but that was known to be forbidden")
		}
		srcInfos = updatedSrcInfos
		srcInfosUpdated = true
	}

	type copyLayerData struct {
		destInfo types.BlobInfo
		diffID   digest.Digest
		err      error
	}

	// copyGroup is used to determine if all layers are copied
	copyGroup := sync.WaitGroup{}
	copyGroup.Add(numLayers)
	// copySemaphore is used to limit the number of parallel downloads to
	// avoid malicious images causing troubles and to be nice to servers.
	var copySemaphore *semaphore.Weighted
	if ic.c.copyInParallel {
		copySemaphore = semaphore.NewWeighted(int64(maxParallelDownloads))
	} else {
		copySemaphore = semaphore.NewWeighted(int64(1))
	}

	data := make([]copyLayerData, numLayers)
	copyLayerHelper := func(index int, srcLayer types.BlobInfo, bar *pb.ProgressBar) {
		defer bar.Finish()
		defer copySemaphore.Release(1)
		defer copyGroup.Done()
		cld := copyLayerData{}
		if ic.c.dest.AcceptsForeignLayerURLs() && len(srcLayer.URLs) != 0 {
			// DiffIDs are, currently, needed only when converting from schema1.
			// In which case src.LayerInfos will not have URLs because schema1
			// does not support them.
			if ic.diffIDsAreNeeded {
				cld.err = errors.New("getting DiffID for foreign layers is unimplemented")
				bar.Prefix(fmt.Sprintf("Skipping blob %s (DiffID foreign layer unimplemented):", shortDigest(srcLayer.Digest)))
				bar.Finish()
			} else {
				cld.destInfo = srcLayer
				logrus.Debugf("Skipping foreign layer %q copy to %s", cld.destInfo.Digest, ic.c.dest.Reference().Transport().Name())
				bar.Prefix(fmt.Sprintf("Skipping blob %s (foreign layer):", shortDigest(srcLayer.Digest)))
				bar.Add64(bar.Total)
				bar.Finish()
			}
		} else {
			cld.destInfo, cld.diffID, cld.err = ic.copyLayer(ctx, srcLayer, bar)
		}
		data[index] = cld
	}

	progressBars := make([]*pb.ProgressBar, numLayers)
	for i, srcInfo := range srcInfos {
		bar := ic.c.createProgressBar(srcInfo, "blob")
		progressBars[i] = bar
	}

	progressPool := pb.NewPool(progressBars...)
	progressPool.Output = ic.c.progressOutput

	if err := progressPool.Start(); err != nil {
		return errors.Wrapf(err, "error creating progress-bar pool")
	}

	for i, srcLayer := range srcInfos {
		copySemaphore.Acquire(ctx, 1)
		go copyLayerHelper(i, srcLayer, progressBars[i])
	}

	destInfos := make([]types.BlobInfo, numLayers)
	diffIDs := make([]digest.Digest, numLayers)

	copyGroup.Wait()
	progressPool.Stop()

	for i, cld := range data {
		if cld.err != nil {
			return cld.err
		}
		destInfos[i] = cld.destInfo
		diffIDs[i] = cld.diffID
	}

	ic.manifestUpdates.InformationOnly.LayerInfos = destInfos
	if ic.diffIDsAreNeeded {
		ic.manifestUpdates.InformationOnly.LayerDiffIDs = diffIDs
	}
	if srcInfosUpdated || layerDigestsDiffer(srcInfos, destInfos) {
		ic.manifestUpdates.LayerInfos = destInfos
	}
	return nil
}

// layerDigestsDiffer return true iff the digests in a and b differ (ignoring sizes and possible other fields)
func layerDigestsDiffer(a, b []types.BlobInfo) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i].Digest != b[i].Digest {
			return true
		}
	}
	return false
}

// copyUpdatedConfigAndManifest updates the image per ic.manifestUpdates, if necessary,
// stores the resulting config and manifest to the destination, and returns the stored manifest.
func (ic *imageCopier) copyUpdatedConfigAndManifest(ctx context.Context) ([]byte, error) {
	pendingImage := ic.src
	if !reflect.DeepEqual(*ic.manifestUpdates, types.ManifestUpdateOptions{InformationOnly: ic.manifestUpdates.InformationOnly}) {
		if !ic.canModifyManifest {
			return nil, errors.Errorf("Internal error: copy needs an updated manifest but that was known to be forbidden")
		}
		if !ic.diffIDsAreNeeded && ic.src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates) {
			// We have set ic.diffIDsAreNeeded based on the preferred MIME type returned by determineManifestConversion.
			// So, this can only happen if we are trying to upload using one of the other MIME type candidates.
			// Because UpdatedImageNeedsLayerDiffIDs is true only when converting from s1 to s2, this case should only arise
			// when ic.c.dest.SupportedManifestMIMETypes() includes both s1 and s2, the upload using s1 failed, and we are now trying s2.
			// Supposedly s2-only registries do not exist or are extremely rare, so failing with this error message is good enough for now.
			// If handling such registries turns out to be necessary, we could compute ic.diffIDsAreNeeded based on the full list of manifest MIME type candidates.
			return nil, errors.Errorf("Can not convert image to %s, preparing DiffIDs for this case is not supported", ic.manifestUpdates.ManifestMIMEType)
		}
		pi, err := ic.src.UpdatedImage(ctx, *ic.manifestUpdates)
		if err != nil {
			return nil, errors.Wrap(err, "Error creating an updated image manifest")
		}
		pendingImage = pi
	}
	manifest, _, err := pendingImage.Manifest(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading manifest")
	}

	if err := ic.c.copyConfig(ctx, pendingImage); err != nil {
		return nil, err
	}

	ic.c.Printf("Writing manifest to image destination\n")
	if err := ic.c.dest.PutManifest(ctx, manifest); err != nil {
		return nil, errors.Wrap(err, "Error writing manifest")
	}
	return manifest, nil
}

// copyConfig copies config.json, if any, from src to dest.
func (c *copier) copyConfig(ctx context.Context, src types.Image) error {
	srcInfo := src.ConfigInfo()
	if srcInfo.Digest != "" {
		configBlob, err := src.ConfigBlob(ctx)
		if err != nil {
			return errors.Wrapf(err, "Error reading config blob %s", srcInfo.Digest)
		}
		bar := c.createProgressBar(srcInfo, "config")
		defer bar.Finish()
		bar.Start()
		destInfo, err := c.copyBlobFromStream(ctx, bytes.NewReader(configBlob), srcInfo, nil, false, true, bar)
		if err != nil {
			return err
		}
		if destInfo.Digest != srcInfo.Digest {
			return errors.Errorf("Internal error: copying uncompressed config blob %s changed digest to %s", srcInfo.Digest, destInfo.Digest)
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

// copyLayer copies a layer with srcInfo (with known Digest and possibly known Size) in src to dest, perhaps compressing it if canCompress,
// and returns a complete blobInfo of the copied layer, and a value for LayerDiffIDs if diffIDIsNeeded
func (ic *imageCopier) copyLayer(ctx context.Context, srcInfo types.BlobInfo, bar *pb.ProgressBar) (types.BlobInfo, digest.Digest, error) {
	cachedDiffID := ic.c.blobInfoCache.UncompressedDigest(srcInfo.Digest) // May be ""
	diffIDIsNeeded := ic.diffIDsAreNeeded && cachedDiffID == ""

	// If we already have the blob, and we don't need to compute the diffID, then we don't need to read it from the source.
	if !diffIDIsNeeded {
		reused, blobInfo, err := ic.c.dest.TryReusingBlob(ctx, srcInfo, ic.c.blobInfoCache, ic.canSubstituteBlobs)
		if err != nil {
			return types.BlobInfo{}, "", errors.Wrapf(err, "Error trying to reuse blob %s at destination", srcInfo.Digest)
		}
		if reused {
			bar.Prefix(fmt.Sprintf("Skipping blob %s (already present):", shortDigest(srcInfo.Digest)))
			bar.Add64(bar.Total)
			bar.Finish()
			return blobInfo, cachedDiffID, nil
		}
	}

	// Fallback: copy the layer, computing the diffID if we need to do so
	srcStream, srcBlobSize, err := ic.c.rawSource.GetBlob(ctx, srcInfo, ic.c.blobInfoCache)
	if err != nil {
		return types.BlobInfo{}, "", errors.Wrapf(err, "Error reading blob %s", srcInfo.Digest)
	}
	defer srcStream.Close()

	blobInfo, diffIDChan, err := ic.copyLayerFromStream(ctx, srcStream, types.BlobInfo{Digest: srcInfo.Digest, Size: srcBlobSize},
		diffIDIsNeeded, bar)
	if err != nil {
		return types.BlobInfo{}, "", err
	}
	if diffIDIsNeeded {
		select {
		case <-ctx.Done():
			return types.BlobInfo{}, "", ctx.Err()
		case diffIDResult := <-diffIDChan:
			if diffIDResult.err != nil {
				return types.BlobInfo{}, "", errors.Wrap(diffIDResult.err, "Error computing layer DiffID")
			}
			logrus.Debugf("Computed DiffID %s for layer %s", diffIDResult.digest, srcInfo.Digest)
			// This is safe because we have just computed diffIDResult.Digest ourselves, and in the process
			// we have read all of the input blob, so srcInfo.Digest must have been validated by digestingReader.
			ic.c.blobInfoCache.RecordDigestUncompressedPair(srcInfo.Digest, diffIDResult.digest)
			return blobInfo, diffIDResult.digest, nil
		}
	} else {
		return blobInfo, cachedDiffID, nil
	}
}

// copyLayerFromStream is an implementation detail of copyLayer; mostly providing a separate “defer” scope.
// it copies a blob with srcInfo (with known Digest and possibly known Size) from srcStream to dest,
// perhaps compressing the stream if canCompress,
// and returns a complete blobInfo of the copied blob and perhaps a <-chan diffIDResult if diffIDIsNeeded, to be read by the caller.
func (ic *imageCopier) copyLayerFromStream(ctx context.Context, srcStream io.Reader, srcInfo types.BlobInfo,
	diffIDIsNeeded bool, bar *pb.ProgressBar) (types.BlobInfo, <-chan diffIDResult, error) {
	var getDiffIDRecorder func(compression.DecompressorFunc) io.Writer // = nil
	var diffIDChan chan diffIDResult

	err := errors.New("Internal error: unexpected panic in copyLayer") // For pipeWriter.CloseWithError below
	if diffIDIsNeeded {
		diffIDChan = make(chan diffIDResult, 1) // Buffered, so that sending a value after this or our caller has failed and exited does not block.
		pipeReader, pipeWriter := io.Pipe()
		defer func() { // Note that this is not the same as {defer pipeWriter.CloseWithError(err)}; we need err to be evaluated lazily.
			pipeWriter.CloseWithError(err) // CloseWithError(nil) is equivalent to Close()
		}()

		getDiffIDRecorder = func(decompressor compression.DecompressorFunc) io.Writer {
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
	blobInfo, err := ic.c.copyBlobFromStream(ctx, srcStream, srcInfo, getDiffIDRecorder, ic.canModifyManifest, false, bar) // Sets err to nil on success
	return blobInfo, diffIDChan, err
	// We need the defer … pipeWriter.CloseWithError() to happen HERE so that the caller can block on reading from diffIDChan
}

// diffIDComputationGoroutine reads all input from layerStream, uncompresses using decompressor if necessary, and sends its digest, and status, if any, to dest.
func diffIDComputationGoroutine(dest chan<- diffIDResult, layerStream io.ReadCloser, decompressor compression.DecompressorFunc) {
	result := diffIDResult{
		digest: "",
		err:    errors.New("Internal error: unexpected panic in diffIDComputationGoroutine"),
	}
	defer func() { dest <- result }()
	defer layerStream.Close() // We do not care to bother the other end of the pipe with other failures; we send them to dest instead.

	result.digest, result.err = computeDiffID(layerStream, decompressor)
}

// computeDiffID reads all input from layerStream, uncompresses it using decompressor if necessary, and returns its digest.
func computeDiffID(stream io.Reader, decompressor compression.DecompressorFunc) (digest.Digest, error) {
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

// copyBlobFromStream copies a blob with srcInfo (with known Digest and possibly known Size) from srcStream to dest,
// perhaps sending a copy to an io.Writer if getOriginalLayerCopyWriter != nil,
// perhaps compressing it if canCompress,
// and returns a complete blobInfo of the copied blob.
func (c *copier) copyBlobFromStream(ctx context.Context, srcStream io.Reader, srcInfo types.BlobInfo,
	getOriginalLayerCopyWriter func(decompressor compression.DecompressorFunc) io.Writer,
	canModifyBlob bool, isConfig bool, bar *pb.ProgressBar) (types.BlobInfo, error) {
	// The copying happens through a pipeline of connected io.Readers.
	// === Input: srcStream

	// === Process input through digestingReader to validate against the expected digest.
	// Be paranoid; in case PutBlob somehow managed to ignore an error from digestingReader,
	// use a separate validation failure indicator.
	// Note that for this check we don't use the stronger "validationSucceeded" indicator, because
	// dest.PutBlob may detect that the layer already exists, in which case we don't
	// read stream to the end, and validation does not happen.
	digestingReader, err := newDigestingReader(srcStream, srcInfo.Digest)
	if err != nil {
		return types.BlobInfo{}, errors.Wrapf(err, "Error preparing to verify blob %s", srcInfo.Digest)
	}
	var destStream io.Reader = digestingReader

	// === Detect compression of the input stream.
	// This requires us to “peek ahead” into the stream to read the initial part, which requires us to chain through another io.Reader returned by DetectCompression.
	decompressor, destStream, err := compression.DetectCompression(destStream) // We could skip this in some cases, but let's keep the code path uniform
	if err != nil {
		return types.BlobInfo{}, errors.Wrapf(err, "Error reading blob %s", srcInfo.Digest)
	}
	isCompressed := decompressor != nil
	destStream = bar.NewProxyReader(destStream)

	// === Send a copy of the original, uncompressed, stream, to a separate path if necessary.
	var originalLayerReader io.Reader // DO NOT USE this other than to drain the input if no other consumer in the pipeline has done so.
	if getOriginalLayerCopyWriter != nil {
		destStream = io.TeeReader(destStream, getOriginalLayerCopyWriter(decompressor))
		originalLayerReader = destStream
	}

	// === Deal with layer compression/decompression if necessary
	var inputInfo types.BlobInfo
	var compressionOperation types.LayerCompression
	if canModifyBlob && c.dest.DesiredLayerCompression() == types.Compress && !isCompressed {
		logrus.Debugf("Compressing blob on the fly")
		compressionOperation = types.Compress
		pipeReader, pipeWriter := io.Pipe()
		defer pipeReader.Close()

		// If this fails while writing data, it will do pipeWriter.CloseWithError(); if it fails otherwise,
		// e.g. because we have exited and due to pipeReader.Close() above further writing to the pipe has failed,
		// we don’t care.
		go compressGoroutine(pipeWriter, destStream) // Closes pipeWriter
		destStream = pipeReader
		inputInfo.Digest = ""
		inputInfo.Size = -1
	} else if canModifyBlob && c.dest.DesiredLayerCompression() == types.Decompress && isCompressed {
		logrus.Debugf("Blob will be decompressed")
		compressionOperation = types.Decompress
		s, err := decompressor(destStream)
		if err != nil {
			return types.BlobInfo{}, err
		}
		defer s.Close()
		destStream = s
		inputInfo.Digest = ""
		inputInfo.Size = -1
	} else {
		logrus.Debugf("Using original blob without modification")
		compressionOperation = types.PreserveOriginal
		inputInfo = srcInfo
	}

	// === Report progress using the c.progress channel, if required.
	if c.progress != nil && c.progressInterval > 0 {
		destStream = &progressReader{
			source:   destStream,
			channel:  c.progress,
			interval: c.progressInterval,
			artifact: srcInfo,
			lastTime: time.Now(),
		}
	}

	// === Finally, send the layer stream to dest.
	uploadedInfo, err := c.dest.PutBlob(ctx, destStream, inputInfo, c.blobInfoCache, isConfig)
	if err != nil {
		return types.BlobInfo{}, errors.Wrap(err, "Error writing blob")
	}

	// This is fairly horrible: the writer from getOriginalLayerCopyWriter wants to consumer
	// all of the input (to compute DiffIDs), even if dest.PutBlob does not need it.
	// So, read everything from originalLayerReader, which will cause the rest to be
	// sent there if we are not already at EOF.
	if getOriginalLayerCopyWriter != nil {
		logrus.Debugf("Consuming rest of the original blob to satisfy getOriginalLayerCopyWriter")
		_, err := io.Copy(ioutil.Discard, originalLayerReader)
		if err != nil {
			return types.BlobInfo{}, errors.Wrapf(err, "Error reading input blob %s", srcInfo.Digest)
		}
	}

	if digestingReader.validationFailed { // Coverage: This should never happen.
		return types.BlobInfo{}, errors.Errorf("Internal error writing blob %s, digest verification failed but was ignored", srcInfo.Digest)
	}
	if inputInfo.Digest != "" && uploadedInfo.Digest != inputInfo.Digest {
		return types.BlobInfo{}, errors.Errorf("Internal error writing blob %s, blob with digest %s saved with digest %s", srcInfo.Digest, inputInfo.Digest, uploadedInfo.Digest)
	}
	if digestingReader.validationSucceeded {
		// If compressionOperation != types.PreserveOriginal, we now have two reliable digest values:
		// srcinfo.Digest describes the pre-compressionOperation input, verified by digestingReader
		// uploadedInfo.Digest describes the post-compressionOperation output, computed by PutBlob
		// (because inputInfo.Digest == "", this must have been computed afresh).
		switch compressionOperation {
		case types.PreserveOriginal:
			break // Do nothing, we have only one digest and we might not have even verified it.
		case types.Compress:
			c.blobInfoCache.RecordDigestUncompressedPair(uploadedInfo.Digest, srcInfo.Digest)
		case types.Decompress:
			c.blobInfoCache.RecordDigestUncompressedPair(srcInfo.Digest, uploadedInfo.Digest)
		default:
			return types.BlobInfo{}, errors.Errorf("Internal error: Unexpected compressionOperation value %#v", compressionOperation)
		}
	}
	return uploadedInfo, nil
}

// compressGoroutine reads all input from src and writes its compressed equivalent to dest.
func compressGoroutine(dest *io.PipeWriter, src io.Reader) {
	err := errors.New("Internal error: unexpected panic in compressGoroutine")
	defer func() { // Note that this is not the same as {defer dest.CloseWithError(err)}; we need err to be evaluated lazily.
		dest.CloseWithError(err) // CloseWithError(nil) is equivalent to Close()
	}()

	zipper := pgzip.NewWriter(dest)
	defer zipper.Close()

	_, err = io.Copy(zipper, src) // Sets err to nil, i.e. causes dest.Close()
}
