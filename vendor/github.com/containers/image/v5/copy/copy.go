package copy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	internalblobinfocache "github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/pkg/platform"
	internalTypes "github.com/containers/image/v5/internal/types"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/ocicrypt"
	encconfig "github.com/containers/ocicrypt/config"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/semaphore"
)

type digestingReader struct {
	source              io.Reader
	digester            digest.Digester
	expectedDigest      digest.Digest
	validationFailed    bool
	validationSucceeded bool
}

// FIXME: disable early layer commits temporarily until a solid solution to
// address #1205 has been found.
const enableEarlyCommit = false

var (
	// ErrDecryptParamsMissing is returned if there is missing decryption parameters
	ErrDecryptParamsMissing = errors.New("Necessary DecryptParameters not present")

	// maxParallelDownloads is used to limit the maximum number of parallel
	// downloads.  Let's follow Firefox by limiting it to 6.
	maxParallelDownloads = uint(6)
)

// compressionBufferSize is the buffer size used to compress a blob
var compressionBufferSize = 1048576

// expectedCompressionFormats is used to check if a blob with a specified media type is compressed
// using the algorithm that the media type says it should be compressed with
var expectedCompressionFormats = map[string]*compression.Algorithm{
	imgspecv1.MediaTypeImageLayerGzip:      &compression.Gzip,
	imgspecv1.MediaTypeImageLayerZstd:      &compression.Zstd,
	manifest.DockerV2Schema2LayerMediaType: &compression.Gzip,
}

// newDigestingReader returns an io.Reader implementation with contents of source, which will eventually return a non-EOF error
// or set validationSucceeded/validationFailed to true if the source stream does/does not match expectedDigest.
// (neither is set if EOF is never reached).
func newDigestingReader(source io.Reader, expectedDigest digest.Digest) (*digestingReader, error) {
	var digester digest.Digester
	if err := expectedDigest.Validate(); err != nil {
		return nil, errors.Errorf("Invalid digest specification %s", expectedDigest)
	}
	digestAlgorithm := expectedDigest.Algorithm()
	if !digestAlgorithm.Available() {
		return nil, errors.Errorf("Invalid digest specification %s: unsupported digest algorithm %s", expectedDigest, digestAlgorithm)
	}
	digester = digestAlgorithm.Digester()

	return &digestingReader{
		source:           source,
		digester:         digester,
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
	dest                  types.ImageDestination
	rawSource             types.ImageSource
	reportWriter          io.Writer
	progressOutput        io.Writer
	progressInterval      time.Duration
	progress              chan types.ProgressProperties
	blobInfoCache         internalblobinfocache.BlobInfoCache2
	copyInParallel        bool
	compressionFormat     compression.Algorithm
	compressionLevel      *int
	ociDecryptConfig      *encconfig.DecryptConfig
	ociEncryptConfig      *encconfig.EncryptConfig
	maxParallelDownloads  uint
	downloadForeignLayers bool
}

// imageCopier tracks state specific to a single image (possibly an item of a manifest list)
type imageCopier struct {
	c                  *copier
	manifestUpdates    *types.ManifestUpdateOptions
	src                types.Image
	diffIDsAreNeeded   bool
	canModifyManifest  bool
	canSubstituteBlobs bool
	ociEncryptLayers   *[]int
}

const (
	// CopySystemImage is the default value which, when set in
	// Options.ImageListSelection, indicates that the caller expects only one
	// image to be copied, so if the source reference refers to a list of
	// images, one that matches the current system will be selected.
	CopySystemImage ImageListSelection = iota
	// CopyAllImages is a value which, when set in Options.ImageListSelection,
	// indicates that the caller expects to copy multiple images, and if
	// the source reference refers to a list, that the list and every image
	// to which it refers will be copied.  If the source reference refers
	// to a list, the target reference can not accept lists, an error
	// should be returned.
	CopyAllImages
	// CopySpecificImages is a value which, when set in
	// Options.ImageListSelection, indicates that the caller expects the
	// source reference to be either a single image or a list of images,
	// and if the source reference is a list, wants only specific instances
	// from it copied (or none of them, if the list of instances to copy is
	// empty), along with the list itself.  If the target reference can
	// only accept one image (i.e., it cannot accept lists), an error
	// should be returned.
	CopySpecificImages
)

// ImageListSelection is one of CopySystemImage, CopyAllImages, or
// CopySpecificImages, to control whether, when the source reference is a list,
// copy.Image() copies only an image which matches the current runtime
// environment, or all images which match the supplied reference, or only
// specific images from the source reference.
type ImageListSelection int

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
	ImageListSelection    ImageListSelection // set to either CopySystemImage (the default), CopyAllImages, or CopySpecificImages to control which instances we copy when the source reference is a list; ignored if the source reference is not a list
	Instances             []digest.Digest    // if ImageListSelection is CopySpecificImages, copy only these instances and the list itself
	// If OciEncryptConfig is non-nil, it indicates that an image should be encrypted.
	// The encryption options is derived from the construction of EncryptConfig object.
	// Note: During initial encryption process of a layer, the resultant digest is not known
	// during creation, so newDigestingReader has to be set with validateDigest = false
	OciEncryptConfig *encconfig.EncryptConfig
	// OciEncryptLayers represents the list of layers to encrypt.
	// If nil, don't encrypt any layers.
	// If non-nil and len==0, denotes encrypt all layers.
	// integers in the slice represent 0-indexed layer indices, with support for negative
	// indexing. i.e. 0 is the first layer, -1 is the last (top-most) layer.
	OciEncryptLayers *[]int
	// OciDecryptConfig contains the config that can be used to decrypt an image if it is
	// encrypted if non-nil. If nil, it does not attempt to decrypt an image.
	OciDecryptConfig *encconfig.DecryptConfig
	// MaxParallelDownloads indicates the maximum layers to pull at the same time.  A reasonable default is used if this is left as 0.
	MaxParallelDownloads uint
	// When OptimizeDestinationImageAlreadyExists is set, optimize the copy assuming that the destination image already
	// exists (and is equivalent). Making the eventual (no-op) copy more performant for this case. Enabling the option
	// is slightly pessimistic if the destination image doesn't exist, or is not equivalent.
	OptimizeDestinationImageAlreadyExists bool
	// Download layer contents with "nondistributable" media types ("foreign" layers) and translate the layer media type
	// to not indicate "nondistributable".
	DownloadForeignLayers bool
}

// validateImageListSelection returns an error if the passed-in value is not one that we recognize as a valid ImageListSelection value
func validateImageListSelection(selection ImageListSelection) error {
	switch selection {
	case CopySystemImage, CopyAllImages, CopySpecificImages:
		return nil
	default:
		return errors.Errorf("Invalid value for options.ImageListSelection: %d", selection)
	}
}

// Image copies image from srcRef to destRef, using policyContext to validate
// source image admissibility.  It returns the manifest which was written to
// the new copy of the image.
func Image(ctx context.Context, policyContext *signature.PolicyContext, destRef, srcRef types.ImageReference, options *Options) (copiedManifest []byte, retErr error) {
	// NOTE this function uses an output parameter for the error return value.
	// Setting this and returning is the ideal way to return an error.
	//
	// the defers in this routine will wrap the error return with its own errors
	// which can be valuable context in the middle of a multi-streamed copy.
	if options == nil {
		options = &Options{}
	}

	if err := validateImageListSelection(options.ImageListSelection); err != nil {
		return nil, err
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
		blobInfoCache:         internalblobinfocache.FromBlobInfoCache(blobinfocache.DefaultCache(options.DestinationCtx)),
		ociDecryptConfig:      options.OciDecryptConfig,
		ociEncryptConfig:      options.OciEncryptConfig,
		maxParallelDownloads:  options.MaxParallelDownloads,
		downloadForeignLayers: options.DownloadForeignLayers,
	}
	// Default to using gzip compression unless specified otherwise.
	if options.DestinationCtx == nil || options.DestinationCtx.CompressionFormat == nil {
		algo, err := compression.AlgorithmByName("gzip")
		if err != nil {
			return nil, err
		}
		c.compressionFormat = algo
	} else {
		c.compressionFormat = *options.DestinationCtx.CompressionFormat
	}
	if options.DestinationCtx != nil {
		// Note that the compressionLevel can be nil.
		c.compressionLevel = options.DestinationCtx.CompressionLevel
	}

	unparsedToplevel := image.UnparsedInstance(rawSource, nil)
	multiImage, err := isMultiImage(ctx, unparsedToplevel)
	if err != nil {
		return nil, errors.Wrapf(err, "Error determining manifest MIME type for %s", transports.ImageName(srcRef))
	}

	if !multiImage {
		// The simple case: just copy a single image.
		if copiedManifest, _, _, err = c.copyOneImage(ctx, policyContext, options, unparsedToplevel, unparsedToplevel, nil); err != nil {
			return nil, err
		}
	} else if options.ImageListSelection == CopySystemImage {
		// This is a manifest list, and we weren't asked to copy multiple images.  Choose a single image that
		// matches the current system to copy, and copy it.
		mfest, manifestType, err := unparsedToplevel.Manifest(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "Error reading manifest for %s", transports.ImageName(srcRef))
		}
		manifestList, err := manifest.ListFromBlob(mfest, manifestType)
		if err != nil {
			return nil, errors.Wrapf(err, "Error parsing primary manifest as list for %s", transports.ImageName(srcRef))
		}
		instanceDigest, err := manifestList.ChooseInstance(options.SourceCtx) // try to pick one that matches options.SourceCtx
		if err != nil {
			return nil, errors.Wrapf(err, "Error choosing an image from manifest list %s", transports.ImageName(srcRef))
		}
		logrus.Debugf("Source is a manifest list; copying (only) instance %s for current system", instanceDigest)
		unparsedInstance := image.UnparsedInstance(rawSource, &instanceDigest)

		if copiedManifest, _, _, err = c.copyOneImage(ctx, policyContext, options, unparsedToplevel, unparsedInstance, nil); err != nil {
			return nil, err
		}
	} else { /* options.ImageListSelection == CopyAllImages or options.ImageListSelection == CopySpecificImages, */
		// If we were asked to copy multiple images and can't, that's an error.
		if !supportsMultipleImages(c.dest) {
			return nil, errors.Errorf("Error copying multiple images: destination transport %q does not support copying multiple images as a group", destRef.Transport().Name())
		}
		// Copy some or all of the images.
		switch options.ImageListSelection {
		case CopyAllImages:
			logrus.Debugf("Source is a manifest list; copying all instances")
		case CopySpecificImages:
			logrus.Debugf("Source is a manifest list; copying some instances")
		}
		if copiedManifest, _, err = c.copyMultipleImages(ctx, policyContext, options, unparsedToplevel); err != nil {
			return nil, err
		}
	}

	if err := c.dest.Commit(ctx, unparsedToplevel); err != nil {
		return nil, errors.Wrap(err, "Error committing the finished image")
	}

	return copiedManifest, nil
}

// Checks if the destination supports accepting multiple images by checking if it can support
// manifest types that are lists of other manifests.
func supportsMultipleImages(dest types.ImageDestination) bool {
	mtypes := dest.SupportedManifestMIMETypes()
	if len(mtypes) == 0 {
		// Anything goes!
		return true
	}
	for _, mtype := range mtypes {
		if manifest.MIMETypeIsMultiImage(mtype) {
			return true
		}
	}
	return false
}

// compareImageDestinationManifestEqual compares the `src` and `dest` image manifests (reading the manifest from the
// (possibly remote) destination). Returning true and the destination's manifest, type and digest if they compare equal.
func compareImageDestinationManifestEqual(ctx context.Context, options *Options, src types.Image, targetInstance *digest.Digest, dest types.ImageDestination) (bool, []byte, string, digest.Digest, error) {
	srcManifest, _, err := src.Manifest(ctx)
	if err != nil {
		return false, nil, "", "", errors.Wrapf(err, "Error reading manifest from image")
	}

	srcManifestDigest, err := manifest.Digest(srcManifest)
	if err != nil {
		return false, nil, "", "", errors.Wrapf(err, "Error calculating manifest digest")
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
		return false, nil, "", "", errors.Wrapf(err, "Error calculating manifest digest")
	}

	logrus.Debugf("Comparing source and destination manifest digests: %v vs. %v", srcManifestDigest, destManifestDigest)
	if srcManifestDigest != destManifestDigest {
		return false, nil, "", "", nil
	}

	// Destination and source manifests, types and digests should all be equivalent
	return true, destManifest, destManifestType, destManifestDigest, nil
}

// copyMultipleImages copies some or all of an image list's instances, using
// policyContext to validate source image admissibility.
func (c *copier) copyMultipleImages(ctx context.Context, policyContext *signature.PolicyContext, options *Options, unparsedToplevel *image.UnparsedImage) (copiedManifest []byte, copiedManifestType string, retErr error) {
	// Parse the list and get a copy of the original value after it's re-encoded.
	manifestList, manifestType, err := unparsedToplevel.Manifest(ctx)
	if err != nil {
		return nil, "", errors.Wrapf(err, "Error reading manifest list")
	}
	originalList, err := manifest.ListFromBlob(manifestList, manifestType)
	if err != nil {
		return nil, "", errors.Wrapf(err, "Error parsing manifest list %q", string(manifestList))
	}
	updatedList := originalList.Clone()

	// Read and/or clear the set of signatures for this list.
	var sigs [][]byte
	if options.RemoveSignatures {
		sigs = [][]byte{}
	} else {
		c.Printf("Getting image list signatures\n")
		s, err := c.rawSource.GetSignatures(ctx, nil)
		if err != nil {
			return nil, "", errors.Wrap(err, "Error reading signatures")
		}
		sigs = s
	}
	if len(sigs) != 0 {
		c.Printf("Checking if image list destination supports signatures\n")
		if err := c.dest.SupportsSignatures(ctx); err != nil {
			return nil, "", errors.Wrapf(err, "Can not copy signatures to %s", transports.ImageName(c.dest.Reference()))
		}
	}
	canModifyManifestList := (len(sigs) == 0)

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
		return nil, "", errors.Wrapf(err, "Error determining manifest list type to write to destination")
	}
	if selectedListType != originalList.MIMEType() {
		if !canModifyManifestList {
			return nil, "", errors.Errorf("Error: manifest list must be converted to type %q to be written to destination, but that would invalidate signatures", selectedListType)
		}
	}

	// Copy each image, or just the ones we want to copy, in turn.
	instanceDigests := updatedList.Instances()
	imagesToCopy := len(instanceDigests)
	if options.ImageListSelection == CopySpecificImages {
		imagesToCopy = len(options.Instances)
	}
	c.Printf("Copying %d of %d images in list\n", imagesToCopy, len(instanceDigests))
	updates := make([]manifest.ListUpdate, len(instanceDigests))
	instancesCopied := 0
	for i, instanceDigest := range instanceDigests {
		if options.ImageListSelection == CopySpecificImages {
			skip := true
			for _, instance := range options.Instances {
				if instance == instanceDigest {
					skip = false
					break
				}
			}
			if skip {
				update, err := updatedList.Instance(instanceDigest)
				if err != nil {
					return nil, "", err
				}
				logrus.Debugf("Skipping instance %s (%d/%d)", instanceDigest, i+1, len(instanceDigests))
				// Record the digest/size/type of the manifest that we didn't copy.
				updates[i] = update
				continue
			}
		}
		logrus.Debugf("Copying instance %s (%d/%d)", instanceDigest, i+1, len(instanceDigests))
		c.Printf("Copying image %s (%d/%d)\n", instanceDigest, instancesCopied+1, imagesToCopy)
		unparsedInstance := image.UnparsedInstance(c.rawSource, &instanceDigest)
		updatedManifest, updatedManifestType, updatedManifestDigest, err := c.copyOneImage(ctx, policyContext, options, unparsedToplevel, unparsedInstance, &instanceDigest)
		if err != nil {
			return nil, "", err
		}
		instancesCopied++
		// Record the result of a possible conversion here.
		update := manifest.ListUpdate{
			Digest:    updatedManifestDigest,
			Size:      int64(len(updatedManifest)),
			MediaType: updatedManifestType,
		}
		updates[i] = update
	}

	// Now reset the digest/size/types of the manifests in the list to account for any conversions that we made.
	if err = updatedList.UpdateInstances(updates); err != nil {
		return nil, "", errors.Wrapf(err, "Error updating manifest list")
	}

	// Iterate through supported list types, preferred format first.
	c.Printf("Writing manifest list to image destination\n")
	var errs []string
	for _, thisListType := range append([]string{selectedListType}, otherManifestMIMETypeCandidates...) {
		attemptedList := updatedList

		logrus.Debugf("Trying to use manifest list type %s…", thisListType)

		// Perform the list conversion, if we need one.
		if thisListType != updatedList.MIMEType() {
			attemptedList, err = updatedList.ConvertToMIMEType(thisListType)
			if err != nil {
				return nil, "", errors.Wrapf(err, "Error converting manifest list to list with MIME type %q", thisListType)
			}
		}

		// Check if the updates or a type conversion meaningfully changed the list of images
		// by serializing them both so that we can compare them.
		attemptedManifestList, err := attemptedList.Serialize()
		if err != nil {
			return nil, "", errors.Wrapf(err, "Error encoding updated manifest list (%q: %#v)", updatedList.MIMEType(), updatedList.Instances())
		}
		originalManifestList, err := originalList.Serialize()
		if err != nil {
			return nil, "", errors.Wrapf(err, "Error encoding original manifest list for comparison (%q: %#v)", originalList.MIMEType(), originalList.Instances())
		}

		// If we can't just use the original value, but we have to change it, flag an error.
		if !bytes.Equal(attemptedManifestList, originalManifestList) {
			if !canModifyManifestList {
				return nil, "", errors.Errorf("Error: manifest list must be converted to type %q to be written to destination, but that would invalidate signatures", thisListType)
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
		return nil, "", fmt.Errorf("Uploading manifest list failed, attempted the following formats: %s", strings.Join(errs, ", "))
	}

	// Sign the manifest list.
	if options.SignBy != "" {
		newSig, err := c.createSignature(manifestList, options.SignBy)
		if err != nil {
			return nil, "", err
		}
		sigs = append(sigs, newSig)
	}

	c.Printf("Storing list signatures\n")
	if err := c.dest.PutSignatures(ctx, sigs, nil); err != nil {
		return nil, "", errors.Wrap(err, "Error writing signatures")
	}

	return manifestList, selectedListType, nil
}

// copyOneImage copies a single (non-manifest-list) image unparsedImage, using policyContext to validate
// source image admissibility.
func (c *copier) copyOneImage(ctx context.Context, policyContext *signature.PolicyContext, options *Options, unparsedToplevel, unparsedImage *image.UnparsedImage, targetInstance *digest.Digest) (retManifest []byte, retManifestType string, retManifestDigest digest.Digest, retErr error) {
	// The caller is handling manifest lists; this could happen only if a manifest list contains a manifest list.
	// Make sure we fail cleanly in such cases.
	multiImage, err := isMultiImage(ctx, unparsedImage)
	if err != nil {
		// FIXME FIXME: How to name a reference for the sub-image?
		return nil, "", "", errors.Wrapf(err, "Error determining manifest MIME type for %s", transports.ImageName(unparsedImage.Reference()))
	}
	if multiImage {
		return nil, "", "", fmt.Errorf("Unexpectedly received a manifest list instead of a manifest for a single image")
	}

	// Please keep this policy check BEFORE reading any other information about the image.
	// (The multiImage check above only matches the MIME type, which we have received anyway.
	// Actual parsing of anything should be deferred.)
	if allowed, err := policyContext.IsRunningImageAllowed(ctx, unparsedImage); !allowed || err != nil { // Be paranoid and fail if either return value indicates so.
		return nil, "", "", errors.Wrap(err, "Source image rejected")
	}
	src, err := image.FromUnparsedImage(ctx, options.SourceCtx, unparsedImage)
	if err != nil {
		return nil, "", "", errors.Wrapf(err, "Error initializing image from source %s", transports.ImageName(c.rawSource.Reference()))
	}

	// If the destination is a digested reference, make a note of that, determine what digest value we're
	// expecting, and check that the source manifest matches it.  If the source manifest doesn't, but it's
	// one item from a manifest list that matches it, accept that as a match.
	destIsDigestedReference := false
	if named := c.dest.Reference().DockerReference(); named != nil {
		if digested, ok := named.(reference.Digested); ok {
			destIsDigestedReference = true
			sourceManifest, _, err := src.Manifest(ctx)
			if err != nil {
				return nil, "", "", errors.Wrapf(err, "Error reading manifest from source image")
			}
			matches, err := manifest.MatchesDigest(sourceManifest, digested.Digest())
			if err != nil {
				return nil, "", "", errors.Wrapf(err, "Error computing digest of source image's manifest")
			}
			if !matches {
				manifestList, _, err := unparsedToplevel.Manifest(ctx)
				if err != nil {
					return nil, "", "", errors.Wrapf(err, "Error reading manifest from source image")
				}
				matches, err = manifest.MatchesDigest(manifestList, digested.Digest())
				if err != nil {
					return nil, "", "", errors.Wrapf(err, "Error computing digest of source image's manifest")
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

	var sigs [][]byte
	if options.RemoveSignatures {
		sigs = [][]byte{}
	} else {
		c.Printf("Getting image source signatures\n")
		s, err := src.Signatures(ctx)
		if err != nil {
			return nil, "", "", errors.Wrap(err, "Error reading signatures")
		}
		sigs = s
	}
	if len(sigs) != 0 {
		c.Printf("Checking if image destination supports signatures\n")
		if err := c.dest.SupportsSignatures(ctx); err != nil {
			return nil, "", "", errors.Wrapf(err, "Can not copy signatures to %s", transports.ImageName(c.dest.Reference()))
		}
	}

	ic := imageCopier{
		c:               c,
		manifestUpdates: &types.ManifestUpdateOptions{InformationOnly: types.ManifestUpdateInformation{Destination: c.dest}},
		src:             src,
		// diffIDsAreNeeded is computed later
		canModifyManifest: len(sigs) == 0 && !destIsDigestedReference,
		ociEncryptLayers:  options.OciEncryptLayers,
	}
	// Ensure _this_ copy sees exactly the intended data when either processing a signed image or signing it.
	// This may be too conservative, but for now, better safe than sorry, _especially_ on the SignBy path:
	// The signature makes the content non-repudiable, so it very much matters that the signature is made over exactly what the user intended.
	// We do intend the RecordDigestUncompressedPair calls to only work with reliable data, but at least there’s a risk
	// that the compressed version coming from a third party may be designed to attack some other decompressor implementation,
	// and we would reuse and sign it.
	ic.canSubstituteBlobs = ic.canModifyManifest && options.SignBy == ""

	if err := ic.updateEmbeddedDockerReference(); err != nil {
		return nil, "", "", err
	}

	destRequiresOciEncryption := (isEncrypted(src) && ic.c.ociDecryptConfig != nil) || options.OciEncryptLayers != nil

	// We compute preferredManifestMIMEType only to show it in error messages.
	// Without having to add this context in an error message, we would be happy enough to know only that no conversion is needed.
	preferredManifestMIMEType, otherManifestMIMETypeCandidates, err := ic.determineManifestConversion(ctx, c.dest.SupportedManifestMIMETypes(), options.ForceManifestMIMEType, destRequiresOciEncryption)
	if err != nil {
		return nil, "", "", err
	}

	// If src.UpdatedImageNeedsLayerDiffIDs(ic.manifestUpdates) will be true, it needs to be true by the time we get here.
	ic.diffIDsAreNeeded = src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates)
	// If encrypted and decryption keys provided, we should try to decrypt
	ic.diffIDsAreNeeded = ic.diffIDsAreNeeded || (isEncrypted(src) && ic.c.ociDecryptConfig != nil) || ic.c.ociEncryptConfig != nil

	// If enabled, fetch and compare the destination's manifest. And as an optimization skip updating the destination iff equal
	if options.OptimizeDestinationImageAlreadyExists {
		shouldUpdateSigs := len(sigs) > 0 || options.SignBy != "" // TODO: Consider allowing signatures updates only and skipping the image's layers/manifest copy if possible
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
	retManifestType = preferredManifestMIMEType
	if err != nil {
		logrus.Debugf("Writing manifest using preferred type %s failed: %v", preferredManifestMIMEType, err)
		// … if it fails, and the failure is either because the manifest is rejected by the registry, or
		// because we failed to create a manifest of the specified type because the specific manifest type
		// doesn't support the type of compression we're trying to use (e.g. docker v2s2 and zstd), we may
		// have other options available that could still succeed.
		_, isManifestRejected := errors.Cause(err).(types.ManifestTypeRejectedError)
		_, isCompressionIncompatible := errors.Cause(err).(manifest.ManifestLayerCompressionIncompatibilityError)
		if (!isManifestRejected && !isCompressionIncompatible) || len(otherManifestMIMETypeCandidates) == 0 {
			// We don’t have other options.
			// In principle the code below would handle this as well, but the resulting  error message is fairly ugly.
			// Don’t bother the user with MIME types if we have no choice.
			return nil, "", "", err
		}
		// If the original MIME type is acceptable, determineManifestConversion always uses it as preferredManifestMIMEType.
		// So if we are here, we will definitely be trying to convert the manifest.
		// With !ic.canModifyManifest, that would just be a string of repeated failures for the same reason,
		// so let’s bail out early and with a better error message.
		if !ic.canModifyManifest {
			return nil, "", "", errors.Wrap(err, "Writing manifest failed (and converting it is not possible, image is signed or the destination specifies a digest)")
		}

		// errs is a list of errors when trying various manifest types. Also serves as an "upload succeeded" flag when set to nil.
		errs := []string{fmt.Sprintf("%s(%v)", preferredManifestMIMEType, err)}
		for _, manifestMIMEType := range otherManifestMIMETypeCandidates {
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

	if options.SignBy != "" {
		newSig, err := c.createSignature(manifestBytes, options.SignBy)
		if err != nil {
			return nil, "", "", err
		}
		sigs = append(sigs, newSig)
	}

	c.Printf("Storing signatures\n")
	if err := c.dest.PutSignatures(ctx, sigs, targetInstance); err != nil {
		return nil, "", "", errors.Wrap(err, "Error writing signatures")
	}

	return manifestBytes, retManifestType, retManifestDigest, nil
}

// Printf writes a formatted string to c.reportWriter.
// Note that the method name Printf is not entirely arbitrary: (go tool vet)
// has a built-in list of functions/methods (whatever object they are for)
// which have their format strings checked; for other names we would have
// to pass a parameter to every (go tool vet) invocation.
func (c *copier) Printf(format string, a ...interface{}) {
	fmt.Fprintf(c.reportWriter, format, a...)
}

// checkImageDestinationForCurrentRuntime enforces dest.MustMatchRuntimeOS, if necessary.
func checkImageDestinationForCurrentRuntime(ctx context.Context, sys *types.SystemContext, src types.Image, dest types.ImageDestination) error {
	if dest.MustMatchRuntimeOS() {
		c, err := src.OCIConfig(ctx)
		if err != nil {
			return errors.Wrapf(err, "Error parsing image configuration")
		}
		wantedPlatforms, err := platform.WantedPlatforms(sys)
		if err != nil {
			return errors.Wrapf(err, "error getting current platform information %#v", sys)
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

	if !ic.canModifyManifest {
		return errors.Errorf("Copying a schema1 image with an embedded Docker reference to %s (Docker reference %s) would change the manifest, which is not possible (image is signed or the destination specifies a digest)",
			transports.ImageName(ic.c.dest.Reference()), destRef.String())
	}
	ic.manifestUpdates.EmbeddedDockerReference = destRef
	return nil
}

func (ic *imageCopier) noPendingManifestUpdates() bool {
	return reflect.DeepEqual(*ic.manifestUpdates, types.ManifestUpdateOptions{InformationOnly: ic.manifestUpdates.InformationOnly})
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
	// If we only need to check authorization, no updates required.
	if updatedSrcInfos != nil && !reflect.DeepEqual(srcInfos, updatedSrcInfos) {
		if !ic.canModifyManifest {
			return errors.Errorf("Copying this image requires changing layer representation, which is not possible (image is signed or the destination specifies a digest)")
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

	// copySemaphore is used to limit the number of parallel downloads to
	// avoid malicious images causing troubles and to be nice to servers.
	var copySemaphore *semaphore.Weighted
	if ic.c.copyInParallel {
		max := ic.c.maxParallelDownloads
		if max == 0 {
			max = maxParallelDownloads
		}
		copySemaphore = semaphore.NewWeighted(int64(max))
	} else {
		copySemaphore = semaphore.NewWeighted(int64(1))
	}

	data := make([]copyLayerData, numLayers)
	copyLayerHelper := func(index int, srcLayer types.BlobInfo, toEncrypt bool, pool *mpb.Progress, srcRef reference.Named) {
		defer copySemaphore.Release(1)
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
			cld.destInfo, cld.diffID, cld.err = ic.copyLayer(ctx, srcLayer, toEncrypt, pool, index, srcRef)
		}
		data[index] = cld
	}

	// Create layer Encryption map
	encLayerBitmap := map[int]bool{}
	var encryptAll bool
	if ic.ociEncryptLayers != nil {
		encryptAll = len(*ic.ociEncryptLayers) == 0
		totalLayers := len(srcInfos)
		for _, l := range *ic.ociEncryptLayers {
			// if layer is negative, it is reverse indexed.
			encLayerBitmap[(totalLayers+l)%totalLayers] = true
		}

		if encryptAll {
			for i := 0; i < len(srcInfos); i++ {
				encLayerBitmap[i] = true
			}
		}
	}

	if err := func() error { // A scope for defer
		progressPool, progressCleanup := ic.c.newProgressPool(ctx)
		defer func() {
			// Wait for all layers to be copied. progressCleanup() must not be called while any of the copyLayerHelpers interact with the progressPool.
			copyGroup.Wait()
			progressCleanup()
		}()

		for i, srcLayer := range srcInfos {
			err = copySemaphore.Acquire(ctx, 1)
			if err != nil {
				return errors.Wrapf(err, "Can't acquire semaphore")
			}
			copyGroup.Add(1)
			go copyLayerHelper(i, srcLayer, encLayerBitmap[i], progressPool, ic.c.rawSource.Reference().DockerReference())
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
// stores the resulting config and manifest to the destination, and returns the stored manifest
// and its digest.
func (ic *imageCopier) copyUpdatedConfigAndManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, digest.Digest, error) {
	pendingImage := ic.src
	if !ic.noPendingManifestUpdates() {
		if !ic.canModifyManifest {
			return nil, "", errors.Errorf("Internal error: copy needs an updated manifest but that was known to be forbidden")
		}
		if !ic.diffIDsAreNeeded && ic.src.UpdatedImageNeedsLayerDiffIDs(*ic.manifestUpdates) {
			// We have set ic.diffIDsAreNeeded based on the preferred MIME type returned by determineManifestConversion.
			// So, this can only happen if we are trying to upload using one of the other MIME type candidates.
			// Because UpdatedImageNeedsLayerDiffIDs is true only when converting from s1 to s2, this case should only arise
			// when ic.c.dest.SupportedManifestMIMETypes() includes both s1 and s2, the upload using s1 failed, and we are now trying s2.
			// Supposedly s2-only registries do not exist or are extremely rare, so failing with this error message is good enough for now.
			// If handling such registries turns out to be necessary, we could compute ic.diffIDsAreNeeded based on the full list of manifest MIME type candidates.
			return nil, "", errors.Errorf("Can not convert image to %s, preparing DiffIDs for this case is not supported", ic.manifestUpdates.ManifestMIMEType)
		}
		pi, err := ic.src.UpdatedImage(ctx, *ic.manifestUpdates)
		if err != nil {
			return nil, "", errors.Wrap(err, "Error creating an updated image manifest")
		}
		pendingImage = pi
	}
	man, _, err := pendingImage.Manifest(ctx)
	if err != nil {
		return nil, "", errors.Wrap(err, "Error reading manifest")
	}

	if err := ic.c.copyConfig(ctx, pendingImage); err != nil {
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
		return nil, "", errors.Wrapf(err, "Error writing manifest %q", string(man))
	}
	return man, manifestDigest, nil
}

// newProgressPool creates a *mpb.Progress and a cleanup function.
// The caller must eventually call the returned cleanup function after the pool will no longer be updated.
func (c *copier) newProgressPool(ctx context.Context) (*mpb.Progress, func()) {
	ctx, cancel := context.WithCancel(ctx)
	pool := mpb.NewWithContext(ctx, mpb.WithWidth(40), mpb.WithOutput(c.progressOutput))
	return pool, func() {
		cancel()
		pool.Wait()
	}
}

// createProgressBar creates a mpb.Bar in pool.  Note that if the copier's reportWriter
// is ioutil.Discard, the progress bar's output will be discarded
func (c *copier) createProgressBar(pool *mpb.Progress, info types.BlobInfo, kind string, onComplete string) *mpb.Bar {
	// shortDigestLen is the length of the digest used for blobs.
	const shortDigestLen = 12

	prefix := fmt.Sprintf("Copying %s %s", kind, info.Digest.Encoded())
	// Truncate the prefix (chopping of some part of the digest) to make all progress bars aligned in a column.
	maxPrefixLen := len("Copying blob ") + shortDigestLen
	if len(prefix) > maxPrefixLen {
		prefix = prefix[:maxPrefixLen]
	}

	// onComplete will replace prefix once the bar/spinner has completed
	onComplete = prefix + " " + onComplete

	// Use a normal progress bar when we know the size (i.e., size > 0).
	// Otherwise, use a spinner to indicate that something's happening.
	var bar *mpb.Bar
	if info.Size > 0 {
		bar = pool.AddBar(info.Size,
			mpb.BarFillerClearOnComplete(),
			mpb.PrependDecorators(
				decor.OnComplete(decor.Name(prefix), onComplete),
			),
			mpb.AppendDecorators(
				decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), ""),
			),
		)
	} else {
		bar = pool.Add(0,
			mpb.NewSpinnerFiller([]string{".", "..", "...", "....", ""}, mpb.SpinnerOnLeft),
			mpb.BarFillerClearOnComplete(),
			mpb.PrependDecorators(
				decor.OnComplete(decor.Name(prefix), onComplete),
			),
		)
	}
	if c.progressOutput == ioutil.Discard {
		c.Printf("Copying %s %s\n", kind, info.Digest)
	}
	return bar
}

// copyConfig copies config.json, if any, from src to dest.
func (c *copier) copyConfig(ctx context.Context, src types.Image) error {
	srcInfo := src.ConfigInfo()
	if srcInfo.Digest != "" {
		configBlob, err := src.ConfigBlob(ctx)
		if err != nil {
			return errors.Wrapf(err, "Error reading config blob %s", srcInfo.Digest)
		}

		destInfo, err := func() (types.BlobInfo, error) { // A scope for defer
			progressPool, progressCleanup := c.newProgressPool(ctx)
			defer progressCleanup()
			bar := c.createProgressBar(progressPool, srcInfo, "config", "done")
			destInfo, err := c.copyBlobFromStream(ctx, bytes.NewReader(configBlob), srcInfo, nil, false, true, false, bar, -1)
			if err != nil {
				return types.BlobInfo{}, err
			}
			bar.SetTotal(int64(len(configBlob)), true)
			return destInfo, nil
		}()
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

// copyLayer copies a layer with srcInfo (with known Digest and Annotations and possibly known Size) in src to dest, perhaps (de/re/)compressing it,
// and returns a complete blobInfo of the copied layer, and a value for LayerDiffIDs if diffIDIsNeeded
// srcRef can be used as an additional hint to the destination during checking whehter a layer can be reused but srcRef can be nil.
func (ic *imageCopier) copyLayer(ctx context.Context, srcInfo types.BlobInfo, toEncrypt bool, pool *mpb.Progress, layerIndex int, srcRef reference.Named) (types.BlobInfo, digest.Digest, error) {
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

	cachedDiffID := ic.c.blobInfoCache.UncompressedDigest(srcInfo.Digest) // May be ""
	// Diffs are needed if we are encrypting an image or trying to decrypt an image
	diffIDIsNeeded := ic.diffIDsAreNeeded && cachedDiffID == "" || toEncrypt || (isOciEncrypted(srcInfo.MediaType) && ic.c.ociDecryptConfig != nil)

	// If we already have the blob, and we don't need to compute the diffID, then we don't need to read it from the source.
	if !diffIDIsNeeded {
		// TODO: at this point we don't know whether or not a blob we end up reusing is compressed using an algorithm
		// that is acceptable for use on layers in the manifest that we'll be writing later, so if we end up reusing
		// a blob that's compressed with e.g. zstd, but we're only allowed to write a v2s2 manifest, this will cause
		// a failure when we eventually try to update the manifest with the digest and MIME type of the reused blob.
		// Fixing that will probably require passing more information to TryReusingBlob() than the current version of
		// the ImageDestination interface lets us pass in.
		var (
			blobInfo types.BlobInfo
			reused   bool
			err      error
		)
		// Note: the storage destination optimizes the committing of
		// layers which requires passing the index of the layer.
		// Hence, we need to special case and cast.
		dest, ok := ic.c.dest.(internalTypes.ImageDestinationWithOptions)
		if ok {
			options := internalTypes.TryReusingBlobOptions{
				Cache:         ic.c.blobInfoCache,
				CanSubstitute: ic.canSubstituteBlobs,
				SrcRef:        srcRef,
			}
			if enableEarlyCommit {
				options.LayerIndex = &layerIndex
			}
			reused, blobInfo, err = dest.TryReusingBlobWithOptions(ctx, srcInfo, options)
		} else {
			reused, blobInfo, err = ic.c.dest.TryReusingBlob(ctx, srcInfo, ic.c.blobInfoCache, ic.canSubstituteBlobs)
		}

		if err != nil {
			return types.BlobInfo{}, "", errors.Wrapf(err, "Error trying to reuse blob %s at destination", srcInfo.Digest)
		}
		if reused {
			logrus.Debugf("Skipping blob %s (already present):", srcInfo.Digest)
			bar := ic.c.createProgressBar(pool, srcInfo, "blob", "skipped: already exists")
			bar.SetTotal(0, true)

			// Throw an event that the layer has been skipped
			if ic.c.progress != nil && ic.c.progressInterval > 0 {
				ic.c.progress <- types.ProgressProperties{
					Event:    types.ProgressEventSkipped,
					Artifact: srcInfo,
				}
			}

			// If the reused blob has the same digest as the one we asked for, but
			// the transport didn't/couldn't supply compression info, fill it in based
			// on what we know from the srcInfos we were given.
			// If the srcInfos came from LayerInfosForCopy(), then UpdatedImage() will
			// call UpdateLayerInfos(), which uses this information to compute the
			// MediaType value for the updated layer infos, and it the transport
			// didn't pass the information along from its input to its output, then
			// it can derive the MediaType incorrectly.
			if blobInfo.Digest == srcInfo.Digest && blobInfo.CompressionAlgorithm == nil {
				blobInfo.CompressionOperation = srcInfo.CompressionOperation
				blobInfo.CompressionAlgorithm = srcInfo.CompressionAlgorithm
			}
			return blobInfo, cachedDiffID, nil
		}
	}

	// Fallback: copy the layer, computing the diffID if we need to do so
	srcStream, srcBlobSize, err := ic.c.rawSource.GetBlob(ctx, srcInfo, ic.c.blobInfoCache)
	if err != nil {
		return types.BlobInfo{}, "", errors.Wrapf(err, "Error reading blob %s", srcInfo.Digest)
	}
	defer srcStream.Close()

	bar := ic.c.createProgressBar(pool, srcInfo, "blob", "done")

	blobInfo, diffIDChan, err := ic.copyLayerFromStream(ctx, srcStream, types.BlobInfo{Digest: srcInfo.Digest, Size: srcBlobSize, MediaType: srcInfo.MediaType, Annotations: srcInfo.Annotations}, diffIDIsNeeded, toEncrypt, bar, layerIndex)
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
				return types.BlobInfo{}, "", errors.Wrap(diffIDResult.err, "Error computing layer DiffID")
			}
			logrus.Debugf("Computed DiffID %s for layer %s", diffIDResult.digest, srcInfo.Digest)
			// This is safe because we have just computed diffIDResult.Digest ourselves, and in the process
			// we have read all of the input blob, so srcInfo.Digest must have been validated by digestingReader.
			ic.c.blobInfoCache.RecordDigestUncompressedPair(srcInfo.Digest, diffIDResult.digest)
			diffID = diffIDResult.digest
		}
	}

	bar.SetTotal(srcInfo.Size, true)
	return blobInfo, diffID, nil
}

// copyLayerFromStream is an implementation detail of copyLayer; mostly providing a separate “defer” scope.
// it copies a blob with srcInfo (with known Digest and Annotations and possibly known Size) from srcStream to dest,
// perhaps (de/re/)compressing the stream,
// and returns a complete blobInfo of the copied blob and perhaps a <-chan diffIDResult if diffIDIsNeeded, to be read by the caller.
func (ic *imageCopier) copyLayerFromStream(ctx context.Context, srcStream io.Reader, srcInfo types.BlobInfo,
	diffIDIsNeeded bool, toEncrypt bool, bar *mpb.Bar, layerIndex int) (types.BlobInfo, <-chan diffIDResult, error) {
	var getDiffIDRecorder func(compression.DecompressorFunc) io.Writer // = nil
	var diffIDChan chan diffIDResult

	err := errors.New("Internal error: unexpected panic in copyLayer") // For pipeWriter.CloseWithError below
	if diffIDIsNeeded {
		diffIDChan = make(chan diffIDResult, 1) // Buffered, so that sending a value after this or our caller has failed and exited does not block.
		pipeReader, pipeWriter := io.Pipe()
		defer func() { // Note that this is not the same as {defer pipeWriter.CloseWithError(err)}; we need err to be evaluated lazily.
			_ = pipeWriter.CloseWithError(err) // CloseWithError(nil) is equivalent to Close(), always returns nil
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

	blobInfo, err := ic.c.copyBlobFromStream(ctx, srcStream, srcInfo, getDiffIDRecorder, ic.canModifyManifest, false, toEncrypt, bar, layerIndex) // Sets err to nil on success
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

// errorAnnotationReader wraps the io.Reader passed to PutBlob for annotating the error happened during read.
// These errors are reported as PutBlob errors, so we would otherwise misleadingly attribute them to the copy destination.
type errorAnnotationReader struct {
	reader io.Reader
}

// Read annotates the error happened during read
func (r errorAnnotationReader) Read(b []byte) (n int, err error) {
	n, err = r.reader.Read(b)
	if err != io.EOF {
		return n, errors.Wrapf(err, "error happened during read")
	}
	return n, err
}

// copyBlobFromStream copies a blob with srcInfo (with known Digest and Annotations and possibly known Size) from srcStream to dest,
// perhaps sending a copy to an io.Writer if getOriginalLayerCopyWriter != nil,
// perhaps (de/re/)compressing it if canModifyBlob,
// and returns a complete blobInfo of the copied blob.
func (c *copier) copyBlobFromStream(ctx context.Context, srcStream io.Reader, srcInfo types.BlobInfo,
	getOriginalLayerCopyWriter func(decompressor compression.DecompressorFunc) io.Writer,
	canModifyBlob bool, isConfig bool, toEncrypt bool, bar *mpb.Bar, layerIndex int) (types.BlobInfo, error) {
	if isConfig { // This is guaranteed by the caller, but set it here to be explicit.
		canModifyBlob = false
	}

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

	// === Decrypt the stream, if required.
	var decrypted bool
	if isOciEncrypted(srcInfo.MediaType) && c.ociDecryptConfig != nil {
		newDesc := imgspecv1.Descriptor{
			Annotations: srcInfo.Annotations,
		}

		var d digest.Digest
		destStream, d, err = ocicrypt.DecryptLayer(c.ociDecryptConfig, destStream, newDesc, false)
		if err != nil {
			return types.BlobInfo{}, errors.Wrapf(err, "Error decrypting layer %s", srcInfo.Digest)
		}

		srcInfo.Digest = d
		srcInfo.Size = -1
		for k := range srcInfo.Annotations {
			if strings.HasPrefix(k, "org.opencontainers.image.enc") {
				delete(srcInfo.Annotations, k)
			}
		}
		decrypted = true
	}

	// === Detect compression of the input stream.
	// This requires us to “peek ahead” into the stream to read the initial part, which requires us to chain through another io.Reader returned by DetectCompression.
	compressionFormat, decompressor, destStream, err := compression.DetectCompressionFormat(destStream) // We could skip this in some cases, but let's keep the code path uniform
	if err != nil {
		return types.BlobInfo{}, errors.Wrapf(err, "Error reading blob %s", srcInfo.Digest)
	}
	isCompressed := decompressor != nil
	if expectedCompressionFormat, known := expectedCompressionFormats[srcInfo.MediaType]; known && isCompressed && compressionFormat.Name() != expectedCompressionFormat.Name() {
		logrus.Debugf("blob %s with type %s should be compressed with %s, but compressor appears to be %s", srcInfo.Digest.String(), srcInfo.MediaType, expectedCompressionFormat.Name(), compressionFormat.Name())
	}

	// === Update progress bars
	destStream = bar.ProxyReader(destStream)

	// === Send a copy of the original, uncompressed, stream, to a separate path if necessary.
	var originalLayerReader io.Reader // DO NOT USE this other than to drain the input if no other consumer in the pipeline has done so.
	if getOriginalLayerCopyWriter != nil {
		destStream = io.TeeReader(destStream, getOriginalLayerCopyWriter(decompressor))
		originalLayerReader = destStream
	}

	// === Deal with layer compression/decompression if necessary
	// WARNING: If you are adding new reasons to change the blob, update also the OptimizeDestinationImageAlreadyExists
	// short-circuit conditions
	var inputInfo types.BlobInfo
	var compressionOperation types.LayerCompression
	uploadCompressionFormat := &c.compressionFormat
	srcCompressorName := internalblobinfocache.Uncompressed
	if isCompressed {
		srcCompressorName = compressionFormat.Name()
	}
	var uploadCompressorName string
	if canModifyBlob && isOciEncrypted(srcInfo.MediaType) {
		// PreserveOriginal due to any compression not being able to be done on an encrypted blob unless decrypted
		logrus.Debugf("Using original blob without modification for encrypted blob")
		compressionOperation = types.PreserveOriginal
		inputInfo = srcInfo
		srcCompressorName = internalblobinfocache.UnknownCompression
		uploadCompressorName = internalblobinfocache.UnknownCompression
		uploadCompressionFormat = nil
	} else if canModifyBlob && c.dest.DesiredLayerCompression() == types.Compress && !isCompressed {
		logrus.Debugf("Compressing blob on the fly")
		compressionOperation = types.Compress
		pipeReader, pipeWriter := io.Pipe()
		defer pipeReader.Close()

		// If this fails while writing data, it will do pipeWriter.CloseWithError(); if it fails otherwise,
		// e.g. because we have exited and due to pipeReader.Close() above further writing to the pipe has failed,
		// we don’t care.
		go c.compressGoroutine(pipeWriter, destStream, *uploadCompressionFormat) // Closes pipeWriter
		destStream = pipeReader
		inputInfo.Digest = ""
		inputInfo.Size = -1
		uploadCompressorName = uploadCompressionFormat.Name()
	} else if canModifyBlob && c.dest.DesiredLayerCompression() == types.Compress && isCompressed && uploadCompressionFormat.Name() != compressionFormat.Name() {
		// When the blob is compressed, but the desired format is different, it first needs to be decompressed and finally
		// re-compressed using the desired format.
		logrus.Debugf("Blob will be converted")

		compressionOperation = types.PreserveOriginal
		s, err := decompressor(destStream)
		if err != nil {
			return types.BlobInfo{}, err
		}
		defer s.Close()

		pipeReader, pipeWriter := io.Pipe()
		defer pipeReader.Close()

		go c.compressGoroutine(pipeWriter, s, *uploadCompressionFormat) // Closes pipeWriter

		destStream = pipeReader
		inputInfo.Digest = ""
		inputInfo.Size = -1
		uploadCompressorName = uploadCompressionFormat.Name()
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
		uploadCompressorName = internalblobinfocache.Uncompressed
		uploadCompressionFormat = nil
	} else {
		// PreserveOriginal might also need to recompress the original blob if the desired compression format is different.
		logrus.Debugf("Using original blob without modification")
		compressionOperation = types.PreserveOriginal
		inputInfo = srcInfo
		uploadCompressorName = srcCompressorName
		// Remember if the original blob was compressed, and if so how, so that if
		// LayerInfosForCopy() returned something that differs from what was in the
		// source's manifest, and UpdatedImage() needs to call UpdateLayerInfos(),
		// it will be able to correctly derive the MediaType for the copied blob.
		if isCompressed {
			uploadCompressionFormat = &compressionFormat
		} else {
			uploadCompressionFormat = nil
		}
	}

	// === Encrypt the stream for valid mediatypes if ociEncryptConfig provided
	var (
		encrypted bool
		finalizer ocicrypt.EncryptLayerFinalizer
	)
	if toEncrypt {
		if decrypted {
			return types.BlobInfo{}, errors.New("Unable to support both decryption and encryption in the same copy")
		}

		if !isOciEncrypted(srcInfo.MediaType) && c.ociEncryptConfig != nil {
			var annotations map[string]string
			if !decrypted {
				annotations = srcInfo.Annotations
			}
			desc := imgspecv1.Descriptor{
				MediaType:   srcInfo.MediaType,
				Digest:      srcInfo.Digest,
				Size:        srcInfo.Size,
				Annotations: annotations,
			}

			s, fin, err := ocicrypt.EncryptLayer(c.ociEncryptConfig, destStream, desc)
			if err != nil {
				return types.BlobInfo{}, errors.Wrapf(err, "Error encrypting blob %s", srcInfo.Digest)
			}

			destStream = s
			finalizer = fin
			inputInfo.Digest = ""
			inputInfo.Size = -1
			encrypted = true
		}
	}

	// === Report progress using the c.progress channel, if required.
	if c.progress != nil && c.progressInterval > 0 {
		progressReader := newProgressReader(
			destStream,
			c.progress,
			c.progressInterval,
			srcInfo,
		)
		defer progressReader.reportDone()
		destStream = progressReader
	}

	// === Finally, send the layer stream to dest.
	var uploadedInfo types.BlobInfo
	// Note: the storage destination optimizes the committing of layers
	// which requires passing the index of the layer.  Hence, we need to
	// special case and cast.
	dest, ok := c.dest.(internalTypes.ImageDestinationWithOptions)
	if ok {
		options := internalTypes.PutBlobOptions{
			Cache:    c.blobInfoCache,
			IsConfig: isConfig,
		}
		if !isConfig && enableEarlyCommit {
			options.LayerIndex = &layerIndex
		}
		uploadedInfo, err = dest.PutBlobWithOptions(ctx, &errorAnnotationReader{destStream}, inputInfo, options)
	} else {
		uploadedInfo, err = c.dest.PutBlob(ctx, &errorAnnotationReader{destStream}, inputInfo, c.blobInfoCache, isConfig)
	}
	if err != nil {
		return types.BlobInfo{}, errors.Wrap(err, "Error writing blob")
	}

	uploadedInfo.Annotations = srcInfo.Annotations

	uploadedInfo.CompressionOperation = compressionOperation
	// If we can modify the layer's blob, set the desired algorithm for it to be set in the manifest.
	uploadedInfo.CompressionAlgorithm = uploadCompressionFormat
	if decrypted {
		uploadedInfo.CryptoOperation = types.Decrypt
	} else if encrypted {
		encryptAnnotations, err := finalizer()
		if err != nil {
			return types.BlobInfo{}, errors.Wrap(err, "Unable to finalize encryption")
		}
		uploadedInfo.CryptoOperation = types.Encrypt
		if uploadedInfo.Annotations == nil {
			uploadedInfo.Annotations = map[string]string{}
		}
		for k, v := range encryptAnnotations {
			uploadedInfo.Annotations[k] = v
		}
	}

	// This is fairly horrible: the writer from getOriginalLayerCopyWriter wants to consume
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
		if uploadCompressorName != "" && uploadCompressorName != internalblobinfocache.UnknownCompression {
			c.blobInfoCache.RecordDigestCompressorName(uploadedInfo.Digest, uploadCompressorName)
		}
		if srcInfo.Digest != "" && srcCompressorName != "" && srcCompressorName != internalblobinfocache.UnknownCompression {
			c.blobInfoCache.RecordDigestCompressorName(srcInfo.Digest, srcCompressorName)
		}
	}
	return uploadedInfo, nil
}

// compressGoroutine reads all input from src and writes its compressed equivalent to dest.
func (c *copier) compressGoroutine(dest *io.PipeWriter, src io.Reader, compressionFormat compression.Algorithm) {
	err := errors.New("Internal error: unexpected panic in compressGoroutine")
	defer func() { // Note that this is not the same as {defer dest.CloseWithError(err)}; we need err to be evaluated lazily.
		_ = dest.CloseWithError(err) // CloseWithError(nil) is equivalent to Close(), always returns nil
	}()

	compressor, err := compression.CompressStream(dest, compressionFormat, c.compressionLevel)
	if err != nil {
		return
	}
	defer compressor.Close()

	buf := make([]byte, compressionBufferSize)

	_, err = io.CopyBuffer(compressor, src, buf) // Sets err to nil, i.e. causes dest.Close()
}
