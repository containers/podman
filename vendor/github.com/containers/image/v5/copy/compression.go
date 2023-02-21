package copy

import (
	"errors"
	"fmt"
	"io"

	internalblobinfocache "github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/pkg/compression"
	compressiontypes "github.com/containers/image/v5/pkg/compression/types"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
)

// bpDetectCompressionStepData contains data that the copy pipeline needs about the “detect compression” step.
type bpDetectCompressionStepData struct {
	isCompressed      bool
	format            compressiontypes.Algorithm        // Valid if isCompressed
	decompressor      compressiontypes.DecompressorFunc // Valid if isCompressed
	srcCompressorName string                            // Compressor name to possibly record in the blob info cache for the source blob.
}

// blobPipelineDetectCompressionStep updates *stream to detect its current compression format.
// srcInfo is only used for error messages.
// Returns data for other steps.
func blobPipelineDetectCompressionStep(stream *sourceStream, srcInfo types.BlobInfo) (bpDetectCompressionStepData, error) {
	// This requires us to “peek ahead” into the stream to read the initial part, which requires us to chain through another io.Reader returned by DetectCompression.
	format, decompressor, reader, err := compression.DetectCompressionFormat(stream.reader) // We could skip this in some cases, but let's keep the code path uniform
	if err != nil {
		return bpDetectCompressionStepData{}, fmt.Errorf("reading blob %s: %w", srcInfo.Digest, err)
	}
	stream.reader = reader

	res := bpDetectCompressionStepData{
		isCompressed: decompressor != nil,
		format:       format,
		decompressor: decompressor,
	}
	if res.isCompressed {
		res.srcCompressorName = format.Name()
	} else {
		res.srcCompressorName = internalblobinfocache.Uncompressed
	}

	if expectedFormat, known := expectedCompressionFormats[stream.info.MediaType]; known && res.isCompressed && format.Name() != expectedFormat.Name() {
		logrus.Debugf("blob %s with type %s should be compressed with %s, but compressor appears to be %s", srcInfo.Digest.String(), srcInfo.MediaType, expectedFormat.Name(), format.Name())
	}
	return res, nil
}

// bpCompressionStepData contains data that the copy pipeline needs about the compression step.
type bpCompressionStepData struct {
	operation              types.LayerCompression      // Operation to use for updating the blob metadata.
	uploadedAlgorithm      *compressiontypes.Algorithm // An algorithm parameter for the compressionOperation edits.
	uploadedAnnotations    map[string]string           // Annotations that should be set on the uploaded blob. WARNING: This is only set after the srcStream.reader is fully consumed.
	srcCompressorName      string                      // Compressor name to record in the blob info cache for the source blob.
	uploadedCompressorName string                      // Compressor name to record in the blob info cache for the uploaded blob.
	closers                []io.Closer                 // Objects to close after the upload is done, if any.
}

// blobPipelineCompressionStep updates *stream to compress and/or decompress it.
// srcInfo is primarily used for error messages.
// Returns data for other steps; the caller should eventually call updateCompressionEdits and perhaps recordValidatedBlobData,
// and must eventually call close.
func (ic *imageCopier) blobPipelineCompressionStep(stream *sourceStream, canModifyBlob bool, srcInfo types.BlobInfo,
	detected bpDetectCompressionStepData) (*bpCompressionStepData, error) {
	// WARNING: If you are adding new reasons to change the blob, update also the OptimizeDestinationImageAlreadyExists
	// short-circuit conditions
	layerCompressionChangeSupported := ic.src.CanChangeLayerCompression(stream.info.MediaType)
	if !layerCompressionChangeSupported {
		logrus.Debugf("Compression change for blob %s (%q) not supported", srcInfo.Digest, stream.info.MediaType)
	}
	if canModifyBlob && layerCompressionChangeSupported {
		for _, fn := range []func(*sourceStream, bpDetectCompressionStepData) (*bpCompressionStepData, error){
			ic.bpcPreserveEncrypted,
			ic.bpcCompressUncompressed,
			ic.bpcRecompressCompressed,
			ic.bpcDecompressCompressed,
		} {
			res, err := fn(stream, detected)
			if err != nil {
				return nil, err
			}
			if res != nil {
				return res, nil
			}
		}
	}
	return ic.bpcPreserveOriginal(stream, detected, layerCompressionChangeSupported), nil
}

// bpcPreserveEncrypted checks if the input is encrypted, and returns a *bpCompressionStepData if so.
func (ic *imageCopier) bpcPreserveEncrypted(stream *sourceStream, _ bpDetectCompressionStepData) (*bpCompressionStepData, error) {
	if isOciEncrypted(stream.info.MediaType) {
		logrus.Debugf("Using original blob without modification for encrypted blob")
		// PreserveOriginal due to any compression not being able to be done on an encrypted blob unless decrypted
		return &bpCompressionStepData{
			operation:              types.PreserveOriginal,
			uploadedAlgorithm:      nil,
			srcCompressorName:      internalblobinfocache.UnknownCompression,
			uploadedCompressorName: internalblobinfocache.UnknownCompression,
		}, nil
	}
	return nil, nil
}

// bpcCompressUncompressed checks if we should be compressing an uncompressed input, and returns a *bpCompressionStepData if so.
func (ic *imageCopier) bpcCompressUncompressed(stream *sourceStream, detected bpDetectCompressionStepData) (*bpCompressionStepData, error) {
	if ic.c.dest.DesiredLayerCompression() == types.Compress && !detected.isCompressed {
		logrus.Debugf("Compressing blob on the fly")
		var uploadedAlgorithm *compressiontypes.Algorithm
		if ic.c.compressionFormat != nil {
			uploadedAlgorithm = ic.c.compressionFormat
		} else {
			uploadedAlgorithm = defaultCompressionFormat
		}

		reader, annotations := ic.c.compressedStream(stream.reader, *uploadedAlgorithm)
		// Note: reader must be closed on all return paths.
		stream.reader = reader
		stream.info = types.BlobInfo{ // FIXME? Should we preserve more data in src.info?
			Digest: "",
			Size:   -1,
		}
		return &bpCompressionStepData{
			operation:              types.Compress,
			uploadedAlgorithm:      uploadedAlgorithm,
			uploadedAnnotations:    annotations,
			srcCompressorName:      detected.srcCompressorName,
			uploadedCompressorName: uploadedAlgorithm.Name(),
			closers:                []io.Closer{reader},
		}, nil
	}
	return nil, nil
}

// bpcRecompressCompressed checks if we should be recompressing a compressed input to another format, and returns a *bpCompressionStepData if so.
func (ic *imageCopier) bpcRecompressCompressed(stream *sourceStream, detected bpDetectCompressionStepData) (*bpCompressionStepData, error) {
	if ic.c.dest.DesiredLayerCompression() == types.Compress && detected.isCompressed &&
		ic.c.compressionFormat != nil && ic.c.compressionFormat.Name() != detected.format.Name() {
		// When the blob is compressed, but the desired format is different, it first needs to be decompressed and finally
		// re-compressed using the desired format.
		logrus.Debugf("Blob will be converted")

		decompressed, err := detected.decompressor(stream.reader)
		if err != nil {
			return nil, err
		}
		succeeded := false
		defer func() {
			if !succeeded {
				decompressed.Close()
			}
		}()

		recompressed, annotations := ic.c.compressedStream(decompressed, *ic.c.compressionFormat)
		// Note: recompressed must be closed on all return paths.
		stream.reader = recompressed
		stream.info = types.BlobInfo{ // FIXME? Should we preserve more data in src.info?
			Digest: "",
			Size:   -1,
		}
		succeeded = true
		return &bpCompressionStepData{
			operation:              types.PreserveOriginal,
			uploadedAlgorithm:      ic.c.compressionFormat,
			uploadedAnnotations:    annotations,
			srcCompressorName:      detected.srcCompressorName,
			uploadedCompressorName: ic.c.compressionFormat.Name(),
			closers:                []io.Closer{decompressed, recompressed},
		}, nil
	}
	return nil, nil
}

// bpcDecompressCompressed checks if we should be decompressing a compressed input, and returns a *bpCompressionStepData if so.
func (ic *imageCopier) bpcDecompressCompressed(stream *sourceStream, detected bpDetectCompressionStepData) (*bpCompressionStepData, error) {
	if ic.c.dest.DesiredLayerCompression() == types.Decompress && detected.isCompressed {
		logrus.Debugf("Blob will be decompressed")
		s, err := detected.decompressor(stream.reader)
		if err != nil {
			return nil, err
		}
		// Note: s must be closed on all return paths.
		stream.reader = s
		stream.info = types.BlobInfo{ // FIXME? Should we preserve more data in src.info?
			Digest: "",
			Size:   -1,
		}
		return &bpCompressionStepData{
			operation:              types.Decompress,
			uploadedAlgorithm:      nil,
			srcCompressorName:      detected.srcCompressorName,
			uploadedCompressorName: internalblobinfocache.Uncompressed,
			closers:                []io.Closer{s},
		}, nil
	}
	return nil, nil
}

// bpcPreserveOriginal returns a *bpCompressionStepData for not changing the original blob.
func (ic *imageCopier) bpcPreserveOriginal(stream *sourceStream, detected bpDetectCompressionStepData,
	layerCompressionChangeSupported bool) *bpCompressionStepData {
	logrus.Debugf("Using original blob without modification")
	// Remember if the original blob was compressed, and if so how, so that if
	// LayerInfosForCopy() returned something that differs from what was in the
	// source's manifest, and UpdatedImage() needs to call UpdateLayerInfos(),
	// it will be able to correctly derive the MediaType for the copied blob.
	//
	// But don’t touch blobs in objects where we can’t change compression,
	// so that src.UpdatedImage() doesn’t fail; assume that for such blobs
	// LayerInfosForCopy() should not be making any changes in the first place.
	var algorithm *compressiontypes.Algorithm
	if layerCompressionChangeSupported && detected.isCompressed {
		algorithm = &detected.format
	} else {
		algorithm = nil
	}
	return &bpCompressionStepData{
		operation:              types.PreserveOriginal,
		uploadedAlgorithm:      algorithm,
		srcCompressorName:      detected.srcCompressorName,
		uploadedCompressorName: detected.srcCompressorName,
	}
}

// updateCompressionEdits sets *operation, *algorithm and updates *annotations, if necessary.
func (d *bpCompressionStepData) updateCompressionEdits(operation *types.LayerCompression, algorithm **compressiontypes.Algorithm, annotations *map[string]string) {
	*operation = d.operation
	// If we can modify the layer's blob, set the desired algorithm for it to be set in the manifest.
	*algorithm = d.uploadedAlgorithm
	if *annotations == nil {
		*annotations = map[string]string{}
	}
	for k, v := range d.uploadedAnnotations {
		(*annotations)[k] = v
	}
}

// recordValidatedBlobData updates b.blobInfoCache with data about the created uploadedInfo adnd the original srcInfo.
// This must ONLY be called if all data has been validated by OUR code, and is not coming from third parties.
func (d *bpCompressionStepData) recordValidatedDigestData(c *copier, uploadedInfo types.BlobInfo, srcInfo types.BlobInfo,
	encryptionStep *bpEncryptionStepData, decryptionStep *bpDecryptionStepData) error {
	// Don’t record any associations that involve encrypted data. This is a bit crude,
	// some blob substitutions (replacing pulls of encrypted data with local reuse of known decryption outcomes)
	// might be safe, but it’s not trivially obvious, so let’s be conservative for now.
	// This crude approach also means we don’t need to record whether a blob is encrypted
	// in the blob info cache (which would probably be necessary for any more complex logic),
	// and the simplicity is attractive.
	if !encryptionStep.encrypting && !decryptionStep.decrypting {
		// If d.operation != types.PreserveOriginal, we now have two reliable digest values:
		// srcinfo.Digest describes the pre-d.operation input, verified by digestingReader
		// uploadedInfo.Digest describes the post-d.operation output, computed by PutBlob
		// (because stream.info.Digest == "", this must have been computed afresh).
		switch d.operation {
		case types.PreserveOriginal:
			break // Do nothing, we have only one digest and we might not have even verified it.
		case types.Compress:
			c.blobInfoCache.RecordDigestUncompressedPair(uploadedInfo.Digest, srcInfo.Digest)
		case types.Decompress:
			c.blobInfoCache.RecordDigestUncompressedPair(srcInfo.Digest, uploadedInfo.Digest)
		default:
			return fmt.Errorf("Internal error: Unexpected d.operation value %#v", d.operation)
		}
	}
	if d.uploadedCompressorName != "" && d.uploadedCompressorName != internalblobinfocache.UnknownCompression {
		c.blobInfoCache.RecordDigestCompressorName(uploadedInfo.Digest, d.uploadedCompressorName)
	}
	if srcInfo.Digest != "" && d.srcCompressorName != "" && d.srcCompressorName != internalblobinfocache.UnknownCompression {
		c.blobInfoCache.RecordDigestCompressorName(srcInfo.Digest, d.srcCompressorName)
	}
	return nil
}

// close closes objects that carry state throughout the compression/decompression operation.
func (d *bpCompressionStepData) close() {
	for _, c := range d.closers {
		c.Close()
	}
}

// doCompression reads all input from src and writes its compressed equivalent to dest.
func doCompression(dest io.Writer, src io.Reader, metadata map[string]string, compressionFormat compressiontypes.Algorithm, compressionLevel *int) error {
	compressor, err := compression.CompressStreamWithMetadata(dest, metadata, compressionFormat, compressionLevel)
	if err != nil {
		return err
	}

	buf := make([]byte, compressionBufferSize)

	_, err = io.CopyBuffer(compressor, src, buf) // Sets err to nil, i.e. causes dest.Close()
	if err != nil {
		compressor.Close()
		return err
	}

	return compressor.Close()
}

// compressGoroutine reads all input from src and writes its compressed equivalent to dest.
func (c *copier) compressGoroutine(dest *io.PipeWriter, src io.Reader, metadata map[string]string, compressionFormat compressiontypes.Algorithm) {
	err := errors.New("Internal error: unexpected panic in compressGoroutine")
	defer func() { // Note that this is not the same as {defer dest.CloseWithError(err)}; we need err to be evaluated lazily.
		_ = dest.CloseWithError(err) // CloseWithError(nil) is equivalent to Close(), always returns nil
	}()

	err = doCompression(dest, src, metadata, compressionFormat, c.compressionLevel)
}

// compressedStream returns a stream the input reader compressed using format, and a metadata map.
// The caller must close the returned reader.
// AFTER the stream is consumed, metadata will be updated with annotations to use on the data.
func (c *copier) compressedStream(reader io.Reader, algorithm compressiontypes.Algorithm) (io.ReadCloser, map[string]string) {
	pipeReader, pipeWriter := io.Pipe()
	annotations := map[string]string{}
	// If this fails while writing data, it will do pipeWriter.CloseWithError(); if it fails otherwise,
	// e.g. because we have exited and due to pipeReader.Close() above further writing to the pipe has failed,
	// we don’t care.
	go c.compressGoroutine(pipeWriter, reader, annotations, algorithm) // Closes pipeWriter
	return pipeReader, annotations
}
