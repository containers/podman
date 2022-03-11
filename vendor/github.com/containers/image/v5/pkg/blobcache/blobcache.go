package blobcache

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	_ types.ImageReference   = &BlobCache{}
	_ types.ImageSource      = &blobCacheSource{}
	_ types.ImageDestination = &blobCacheDestination{}
)

const (
	compressedNote   = ".compressed"
	decompressedNote = ".decompressed"
)

// BlobCache is an object which saves copies of blobs that are written to it while passing them
// through to some real destination, and which can be queried directly in order to read them
// back.
//
// Implements types.ImageReference.
type BlobCache struct {
	reference types.ImageReference
	// WARNING: The contents of this directory may be accessed concurrently,
	// both within this process and by multiple different processes
	directory string
	compress  types.LayerCompression
}

type blobCacheSource struct {
	reference *BlobCache
	source    types.ImageSource
	sys       types.SystemContext
	// this mutex synchronizes the counters below
	mu          sync.Mutex
	cacheHits   int64
	cacheMisses int64
	cacheErrors int64
}

type blobCacheDestination struct {
	reference   *BlobCache
	destination types.ImageDestination
}

func makeFilename(blobSum digest.Digest, isConfig bool) string {
	if isConfig {
		return blobSum.String() + ".config"
	}
	return blobSum.String()
}

// NewBlobCache creates a new blob cache that wraps an image reference.  Any blobs which are
// written to the destination image created from the resulting reference will also be stored
// as-is to the specified directory or a temporary directory.
// The compress argument controls whether or not the cache will try to substitute a compressed
// or different version of a blob when preparing the list of layers when reading an image.
func NewBlobCache(ref types.ImageReference, directory string, compress types.LayerCompression) (*BlobCache, error) {
	if directory == "" {
		return nil, errors.Errorf("error creating cache around reference %q: no directory specified", transports.ImageName(ref))
	}
	switch compress {
	case types.Compress, types.Decompress, types.PreserveOriginal:
		// valid value, accept it
	default:
		return nil, errors.Errorf("unhandled LayerCompression value %v", compress)
	}
	return &BlobCache{
		reference: ref,
		directory: directory,
		compress:  compress,
	}, nil
}

func (b *BlobCache) Transport() types.ImageTransport {
	return b.reference.Transport()
}

func (b *BlobCache) StringWithinTransport() string {
	return b.reference.StringWithinTransport()
}

func (b *BlobCache) DockerReference() reference.Named {
	return b.reference.DockerReference()
}

func (b *BlobCache) PolicyConfigurationIdentity() string {
	return b.reference.PolicyConfigurationIdentity()
}

func (b *BlobCache) PolicyConfigurationNamespaces() []string {
	return b.reference.PolicyConfigurationNamespaces()
}

func (b *BlobCache) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	return b.reference.DeleteImage(ctx, sys)
}

func (b *BlobCache) HasBlob(blobinfo types.BlobInfo) (bool, int64, error) {
	if blobinfo.Digest == "" {
		return false, -1, nil
	}

	for _, isConfig := range []bool{false, true} {
		filename := filepath.Join(b.directory, makeFilename(blobinfo.Digest, isConfig))
		fileInfo, err := os.Stat(filename)
		if err == nil && (blobinfo.Size == -1 || blobinfo.Size == fileInfo.Size()) {
			return true, fileInfo.Size(), nil
		}
		if !os.IsNotExist(err) {
			return false, -1, errors.Wrap(err, "checking size")
		}
	}

	return false, -1, nil
}

func (b *BlobCache) Directory() string {
	return b.directory
}

func (b *BlobCache) ClearCache() error {
	f, err := os.Open(b.directory)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return errors.Wrapf(err, "error reading directory %q", b.directory)
	}
	for _, name := range names {
		pathname := filepath.Join(b.directory, name)
		if err = os.RemoveAll(pathname); err != nil {
			return errors.Wrapf(err, "clearing cache for %q", transports.ImageName(b))
		}
	}
	return nil
}

func (b *BlobCache) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	src, err := b.NewImageSource(ctx, sys)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new image %q", transports.ImageName(b.reference))
	}
	return image.FromSource(ctx, sys, src)
}

func (b *BlobCache) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	src, err := b.reference.NewImageSource(ctx, sys)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new image source %q", transports.ImageName(b.reference))
	}
	logrus.Debugf("starting to read from image %q using blob cache in %q (compression=%v)", transports.ImageName(b.reference), b.directory, b.compress)
	return &blobCacheSource{reference: b, source: src, sys: *sys}, nil
}

func (b *BlobCache) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	dest, err := b.reference.NewImageDestination(ctx, sys)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new image destination %q", transports.ImageName(b.reference))
	}
	logrus.Debugf("starting to write to image %q using blob cache in %q", transports.ImageName(b.reference), b.directory)
	return &blobCacheDestination{reference: b, destination: dest}, nil
}

func (s *blobCacheSource) Reference() types.ImageReference {
	return s.reference
}

func (s *blobCacheSource) Close() error {
	logrus.Debugf("finished reading from image %q using blob cache: cache had %d hits, %d misses, %d errors", transports.ImageName(s.reference), s.cacheHits, s.cacheMisses, s.cacheErrors)
	return s.source.Close()
}

func (s *blobCacheSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		filename := filepath.Join(s.reference.directory, makeFilename(*instanceDigest, false))
		manifestBytes, err := ioutil.ReadFile(filename)
		if err == nil {
			s.cacheHits++
			return manifestBytes, manifest.GuessMIMEType(manifestBytes), nil
		}
		if !os.IsNotExist(err) {
			s.cacheErrors++
			return nil, "", errors.Wrap(err, "checking for manifest file")
		}
	}
	s.cacheMisses++
	return s.source.GetManifest(ctx, instanceDigest)
}

func (s *blobCacheSource) HasThreadSafeGetBlob() bool {
	return s.source.HasThreadSafeGetBlob()
}

func (s *blobCacheSource) GetBlob(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	present, size, err := s.reference.HasBlob(blobinfo)
	if err != nil {
		return nil, -1, err
	}
	if present {
		for _, isConfig := range []bool{false, true} {
			filename := filepath.Join(s.reference.directory, makeFilename(blobinfo.Digest, isConfig))
			f, err := os.Open(filename)
			if err == nil {
				s.mu.Lock()
				s.cacheHits++
				s.mu.Unlock()
				return f, size, nil
			}
			if !os.IsNotExist(err) {
				s.mu.Lock()
				s.cacheErrors++
				s.mu.Unlock()
				return nil, -1, errors.Wrap(err, "checking for cache")
			}
		}
	}
	s.mu.Lock()
	s.cacheMisses++
	s.mu.Unlock()
	rc, size, err := s.source.GetBlob(ctx, blobinfo, cache)
	if err != nil {
		return rc, size, errors.Wrapf(err, "error reading blob from source image %q", transports.ImageName(s.reference))
	}
	return rc, size, nil
}

func (s *blobCacheSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	return s.source.GetSignatures(ctx, instanceDigest)
}

func (s *blobCacheSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	signatures, err := s.source.GetSignatures(ctx, instanceDigest)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking if image %q has signatures", transports.ImageName(s.reference))
	}
	canReplaceBlobs := !(len(signatures) > 0 && len(signatures[0]) > 0)

	infos, err := s.source.LayerInfosForCopy(ctx, instanceDigest)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting layer infos for copying image %q through cache", transports.ImageName(s.reference))
	}
	if infos == nil {
		img, err := image.FromUnparsedImage(ctx, &s.sys, image.UnparsedInstance(s.source, instanceDigest))
		if err != nil {
			return nil, errors.Wrapf(err, "error opening image to get layer infos for copying image %q through cache", transports.ImageName(s.reference))
		}
		infos = img.LayerInfos()
	}

	if canReplaceBlobs && s.reference.compress != types.PreserveOriginal {
		replacedInfos := make([]types.BlobInfo, 0, len(infos))
		for _, info := range infos {
			var replaceDigest []byte
			var err error
			blobFile := filepath.Join(s.reference.directory, makeFilename(info.Digest, false))
			alternate := ""
			switch s.reference.compress {
			case types.Compress:
				alternate = blobFile + compressedNote
				replaceDigest, err = ioutil.ReadFile(alternate)
			case types.Decompress:
				alternate = blobFile + decompressedNote
				replaceDigest, err = ioutil.ReadFile(alternate)
			}
			if err == nil && digest.Digest(replaceDigest).Validate() == nil {
				alternate = filepath.Join(filepath.Dir(alternate), makeFilename(digest.Digest(replaceDigest), false))
				fileInfo, err := os.Stat(alternate)
				if err == nil {
					switch info.MediaType {
					case v1.MediaTypeImageLayer, v1.MediaTypeImageLayerGzip:
						switch s.reference.compress {
						case types.Compress:
							info.MediaType = v1.MediaTypeImageLayerGzip
							info.CompressionAlgorithm = &compression.Gzip
						case types.Decompress:
							info.MediaType = v1.MediaTypeImageLayer
							info.CompressionAlgorithm = nil
						}
					case manifest.DockerV2SchemaLayerMediaTypeUncompressed, manifest.DockerV2Schema2LayerMediaType:
						switch s.reference.compress {
						case types.Compress:
							info.MediaType = manifest.DockerV2Schema2LayerMediaType
							info.CompressionAlgorithm = &compression.Gzip
						case types.Decompress:
							// nope, not going to suggest anything, it's not allowed by the spec
							replacedInfos = append(replacedInfos, info)
							continue
						}
					}
					logrus.Debugf("suggesting cached blob with digest %q, type %q, and compression %v in place of blob with digest %q", string(replaceDigest), info.MediaType, s.reference.compress, info.Digest.String())
					info.CompressionOperation = s.reference.compress
					info.Digest = digest.Digest(replaceDigest)
					info.Size = fileInfo.Size()
					logrus.Debugf("info = %#v", info)
				}
			}
			replacedInfos = append(replacedInfos, info)
		}
		infos = replacedInfos
	}

	return infos, nil
}

func (d *blobCacheDestination) Reference() types.ImageReference {
	return d.reference
}

func (d *blobCacheDestination) Close() error {
	logrus.Debugf("finished writing to image %q using blob cache", transports.ImageName(d.reference))
	return d.destination.Close()
}

func (d *blobCacheDestination) SupportedManifestMIMETypes() []string {
	return d.destination.SupportedManifestMIMETypes()
}

func (d *blobCacheDestination) SupportsSignatures(ctx context.Context) error {
	return d.destination.SupportsSignatures(ctx)
}

func (d *blobCacheDestination) DesiredLayerCompression() types.LayerCompression {
	return d.destination.DesiredLayerCompression()
}

func (d *blobCacheDestination) AcceptsForeignLayerURLs() bool {
	return d.destination.AcceptsForeignLayerURLs()
}

func (d *blobCacheDestination) MustMatchRuntimeOS() bool {
	return d.destination.MustMatchRuntimeOS()
}

func (d *blobCacheDestination) IgnoresEmbeddedDockerReference() bool {
	return d.destination.IgnoresEmbeddedDockerReference()
}

// Decompress and save the contents of the decompressReader stream into the passed-in temporary
// file.  If we successfully save all of the data, rename the file to match the digest of the data,
// and make notes about the relationship between the file that holds a copy of the compressed data
// and this new file.
func saveStream(wg *sync.WaitGroup, decompressReader io.ReadCloser, tempFile *os.File, compressedFilename string, compressedDigest digest.Digest, isConfig bool, alternateDigest *digest.Digest) {
	defer wg.Done()
	// Decompress from and digest the reading end of that pipe.
	decompressed, err3 := archive.DecompressStream(decompressReader)
	digester := digest.Canonical.Digester()
	if err3 == nil {
		// Read the decompressed data through the filter over the pipe, blocking until the
		// writing end is closed.
		_, err3 = io.Copy(io.MultiWriter(tempFile, digester.Hash()), decompressed)
	} else {
		// Drain the pipe to keep from stalling the PutBlob() thread.
		if _, err := io.Copy(ioutil.Discard, decompressReader); err != nil {
			logrus.Debugf("error draining the pipe: %v", err)
		}
	}
	decompressReader.Close()
	decompressed.Close()
	tempFile.Close()
	// Determine the name that we should give to the uncompressed copy of the blob.
	decompressedFilename := filepath.Join(filepath.Dir(tempFile.Name()), makeFilename(digester.Digest(), isConfig))
	if err3 == nil {
		// Rename the temporary file.
		if err3 = os.Rename(tempFile.Name(), decompressedFilename); err3 != nil {
			logrus.Debugf("error renaming new decompressed copy of blob %q into place at %q: %v", digester.Digest().String(), decompressedFilename, err3)
			// Remove the temporary file.
			if err3 = os.Remove(tempFile.Name()); err3 != nil {
				logrus.Debugf("error cleaning up temporary file %q for decompressed copy of blob %q: %v", tempFile.Name(), compressedDigest.String(), err3)
			}
		} else {
			*alternateDigest = digester.Digest()
			// Note the relationship between the two files.
			if err3 = ioutils.AtomicWriteFile(decompressedFilename+compressedNote, []byte(compressedDigest.String()), 0600); err3 != nil {
				logrus.Debugf("error noting that the compressed version of %q is %q: %v", digester.Digest().String(), compressedDigest.String(), err3)
			}
			if err3 = ioutils.AtomicWriteFile(compressedFilename+decompressedNote, []byte(digester.Digest().String()), 0600); err3 != nil {
				logrus.Debugf("error noting that the decompressed version of %q is %q: %v", compressedDigest.String(), digester.Digest().String(), err3)
			}
		}
	} else {
		// Remove the temporary file.
		if err3 = os.Remove(tempFile.Name()); err3 != nil {
			logrus.Debugf("error cleaning up temporary file %q for decompressed copy of blob %q: %v", tempFile.Name(), compressedDigest.String(), err3)
		}
	}
}

func (d *blobCacheDestination) HasThreadSafePutBlob() bool {
	return d.destination.HasThreadSafePutBlob()
}

func (d *blobCacheDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	var tempfile *os.File
	var err error
	var n int
	var alternateDigest digest.Digest
	var closer io.Closer
	wg := new(sync.WaitGroup)
	needToWait := false
	compression := archive.Uncompressed
	if inputInfo.Digest != "" {
		filename := filepath.Join(d.reference.directory, makeFilename(inputInfo.Digest, isConfig))
		tempfile, err = ioutil.TempFile(d.reference.directory, makeFilename(inputInfo.Digest, isConfig))
		if err == nil {
			stream = io.TeeReader(stream, tempfile)
			defer func() {
				if err == nil {
					if err = os.Rename(tempfile.Name(), filename); err != nil {
						if err2 := os.Remove(tempfile.Name()); err2 != nil {
							logrus.Debugf("error cleaning up temporary file %q for blob %q: %v", tempfile.Name(), inputInfo.Digest.String(), err2)
						}
						err = errors.Wrapf(err, "error renaming new layer for blob %q into place at %q", inputInfo.Digest.String(), filename)
					}
				} else {
					if err2 := os.Remove(tempfile.Name()); err2 != nil {
						logrus.Debugf("error cleaning up temporary file %q for blob %q: %v", tempfile.Name(), inputInfo.Digest.String(), err2)
					}
				}
				tempfile.Close()
			}()
		} else {
			logrus.Debugf("error while creating a temporary file under %q to hold blob %q: %v", d.reference.directory, inputInfo.Digest.String(), err)
		}
		if !isConfig {
			initial := make([]byte, 8)
			n, err = stream.Read(initial)
			if n > 0 {
				// Build a Reader that will still return the bytes that we just
				// read, for PutBlob()'s sake.
				stream = io.MultiReader(bytes.NewReader(initial[:n]), stream)
				if n >= len(initial) {
					compression = archive.DetectCompression(initial[:n])
				}
				if compression == archive.Gzip {
					// The stream is compressed, so create a file which we'll
					// use to store a decompressed copy.
					decompressedTemp, err2 := ioutil.TempFile(d.reference.directory, makeFilename(inputInfo.Digest, isConfig))
					if err2 != nil {
						logrus.Debugf("error while creating a temporary file under %q to hold decompressed blob %q: %v", d.reference.directory, inputInfo.Digest.String(), err2)
						decompressedTemp.Close()
					} else {
						// Write a copy of the compressed data to a pipe,
						// closing the writing end of the pipe after
						// PutBlob() returns.
						decompressReader, decompressWriter := io.Pipe()
						closer = decompressWriter
						stream = io.TeeReader(stream, decompressWriter)
						// Let saveStream() close the reading end and handle the temporary file.
						wg.Add(1)
						needToWait = true
						go saveStream(wg, decompressReader, decompressedTemp, filename, inputInfo.Digest, isConfig, &alternateDigest)
					}
				}
			}
		}
	}
	newBlobInfo, err := d.destination.PutBlob(ctx, stream, inputInfo, cache, isConfig)
	if closer != nil {
		closer.Close()
	}
	if needToWait {
		wg.Wait()
	}
	if err != nil {
		return newBlobInfo, errors.Wrapf(err, "error storing blob to image destination for cache %q", transports.ImageName(d.reference))
	}
	if alternateDigest.Validate() == nil {
		logrus.Debugf("added blob %q (also %q) to the cache at %q", inputInfo.Digest.String(), alternateDigest.String(), d.reference.directory)
	} else {
		logrus.Debugf("added blob %q to the cache at %q", inputInfo.Digest.String(), d.reference.directory)
	}
	return newBlobInfo, nil
}

func (d *blobCacheDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	present, reusedInfo, err := d.destination.TryReusingBlob(ctx, info, cache, canSubstitute)
	if err != nil || present {
		return present, reusedInfo, err
	}

	for _, isConfig := range []bool{false, true} {
		filename := filepath.Join(d.reference.directory, makeFilename(info.Digest, isConfig))
		f, err := os.Open(filename)
		if err == nil {
			defer f.Close()
			uploadedInfo, err := d.destination.PutBlob(ctx, f, info, cache, isConfig)
			if err != nil {
				return false, types.BlobInfo{}, err
			}
			return true, uploadedInfo, nil
		}
	}

	return false, types.BlobInfo{}, nil
}

func (d *blobCacheDestination) PutManifest(ctx context.Context, manifestBytes []byte, instanceDigest *digest.Digest) error {
	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		logrus.Warnf("error digesting manifest %q: %v", string(manifestBytes), err)
	} else {
		filename := filepath.Join(d.reference.directory, makeFilename(manifestDigest, false))
		if err = ioutils.AtomicWriteFile(filename, manifestBytes, 0600); err != nil {
			logrus.Warnf("error saving manifest as %q: %v", filename, err)
		}
	}
	return d.destination.PutManifest(ctx, manifestBytes, instanceDigest)
}

func (d *blobCacheDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	return d.destination.PutSignatures(ctx, signatures, instanceDigest)
}

func (d *blobCacheDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	return d.destination.Commit(ctx, unparsedToplevel)
}
