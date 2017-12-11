// +build !containers_image_storage_stub

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"

	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/ioutils"
	ddigest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

var (
	// ErrBlobDigestMismatch is returned when PutBlob() is given a blob
	// with a digest-based name that doesn't match its contents.
	ErrBlobDigestMismatch = errors.New("blob digest mismatch")
	// ErrBlobSizeMismatch is returned when PutBlob() is given a blob
	// with an expected size that doesn't match the reader.
	ErrBlobSizeMismatch = errors.New("blob size mismatch")
	// ErrNoManifestLists is returned when GetManifest() is called.
	// with a non-nil instanceDigest.
	ErrNoManifestLists = errors.New("manifest lists are not supported by this transport")
	// ErrNoSuchImage is returned when we attempt to access an image which
	// doesn't exist in the storage area.
	ErrNoSuchImage = storage.ErrNotAnImage
)

type storageImageSource struct {
	imageRef       storageReference
	Tag            string                      `json:"tag,omitempty"`
	Created        time.Time                   `json:"created-time,omitempty"`
	ID             string                      `json:"id"`
	BlobList       []types.BlobInfo            `json:"blob-list,omitempty"` // Ordered list of every blob the image has been told to handle
	Layers         map[ddigest.Digest][]string `json:"layers,omitempty"`    // Map from digests of blobs to lists of layer IDs
	LayerPosition  map[ddigest.Digest]int      `json:"-"`                   // Where we are in reading a blob's layers
	SignatureSizes []int                       `json:"signature-sizes"`     // List of sizes of each signature slice
}

type storageImageDestination struct {
	imageRef       storageReference
	Tag            string                      `json:"tag,omitempty"`
	Created        time.Time                   `json:"created-time,omitempty"`
	ID             string                      `json:"id"`
	BlobList       []types.BlobInfo            `json:"blob-list,omitempty"` // Ordered list of every blob the image has been told to handle
	Layers         map[ddigest.Digest][]string `json:"layers,omitempty"`    // Map from digests of blobs to lists of layer IDs
	BlobData       map[ddigest.Digest][]byte   `json:"-"`                   // Map from names of blobs that aren't layers to contents, temporary
	Manifest       []byte                      `json:"-"`                   // Manifest contents, temporary
	Signatures     []byte                      `json:"-"`                   // Signature contents, temporary
	SignatureSizes []int                       `json:"signature-sizes"`     // List of sizes of each signature slice
}

type storageLayerMetadata struct {
	Digest         string `json:"digest,omitempty"`
	Size           int64  `json:"size"`
	CompressedSize int64  `json:"compressed-size,omitempty"`
}

type storageImageCloser struct {
	types.ImageCloser
	size int64
}

// newImageSource sets us up to read out an image, which needs to already exist.
func newImageSource(imageRef storageReference) (*storageImageSource, error) {
	img, err := imageRef.resolveImage()
	if err != nil {
		return nil, err
	}
	image := &storageImageSource{
		imageRef:       imageRef,
		Created:        time.Now(),
		ID:             img.ID,
		BlobList:       []types.BlobInfo{},
		Layers:         make(map[ddigest.Digest][]string),
		LayerPosition:  make(map[ddigest.Digest]int),
		SignatureSizes: []int{},
	}
	if err := json.Unmarshal([]byte(img.Metadata), image); err != nil {
		return nil, errors.Wrap(err, "error decoding metadata for source image")
	}
	return image, nil
}

// newImageDestination sets us up to write a new image.
func newImageDestination(imageRef storageReference) (*storageImageDestination, error) {
	image := &storageImageDestination{
		imageRef:       imageRef,
		Tag:            imageRef.reference,
		Created:        time.Now(),
		ID:             imageRef.id,
		BlobList:       []types.BlobInfo{},
		Layers:         make(map[ddigest.Digest][]string),
		BlobData:       make(map[ddigest.Digest][]byte),
		SignatureSizes: []int{},
	}
	return image, nil
}

func (s storageImageSource) Reference() types.ImageReference {
	return s.imageRef
}

func (s storageImageDestination) Reference() types.ImageReference {
	return s.imageRef
}

func (s storageImageSource) Close() error {
	return nil
}

func (s storageImageDestination) Close() error {
	return nil
}

func (s storageImageDestination) ShouldCompressLayers() bool {
	// We ultimately have to decompress layers to populate trees on disk,
	// so callers shouldn't bother compressing them before handing them to
	// us, if they're not already compressed.
	return false
}

// putBlob stores a layer or data blob, optionally enforcing that a digest in
// blobinfo matches the incoming data.
func (s *storageImageDestination) putBlob(stream io.Reader, blobinfo types.BlobInfo, enforceDigestAndSize bool) (types.BlobInfo, error) {
	blobSize := blobinfo.Size
	digest := blobinfo.Digest
	errorBlobInfo := types.BlobInfo{
		Digest: "",
		Size:   -1,
	}
	// Try to read an initial snippet of the blob.
	buf := [archive.HeaderSize]byte{}
	n, err := io.ReadAtLeast(stream, buf[:], len(buf))
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return errorBlobInfo, err
	}
	// Set up to read the whole blob (the initial snippet, plus the rest)
	// while digesting it with either the default, or the passed-in digest,
	// if one was specified.
	hasher := ddigest.Canonical.Digester()
	if digest.Validate() == nil {
		if a := digest.Algorithm(); a.Available() {
			hasher = a.Digester()
		}
	}
	hash := ""
	counter := ioutils.NewWriteCounter(hasher.Hash())
	defragmented := io.MultiReader(bytes.NewBuffer(buf[:n]), stream)
	multi := io.TeeReader(defragmented, counter)
	if (n > 0) && archive.IsArchive(buf[:n]) {
		// It's a filesystem layer.  If it's not the first one in the
		// image, we assume that the most recently added layer is its
		// parent.
		parentLayer := ""
		for _, blob := range s.BlobList {
			if layerList, ok := s.Layers[blob.Digest]; ok {
				parentLayer = layerList[len(layerList)-1]
			}
		}
		// If we have an expected content digest, generate a layer ID
		// based on the parent's ID and the expected content digest.
		id := ""
		if digest.Validate() == nil {
			id = ddigest.Canonical.FromBytes([]byte(parentLayer + "+" + digest.String())).Hex()
		}
		// Attempt to create the identified layer and import its contents.
		layer, uncompressedSize, err := s.imageRef.transport.store.PutLayer(id, parentLayer, nil, "", true, multi)
		if err != nil && errors.Cause(err) != storage.ErrDuplicateID {
			logrus.Debugf("error importing layer blob %q as %q: %v", blobinfo.Digest, id, err)
			return errorBlobInfo, err
		}
		if errors.Cause(err) == storage.ErrDuplicateID {
			// We specified an ID, and there's already a layer with
			// the same ID.  Drain the input so that we can look at
			// its length and digest.
			_, err := io.Copy(ioutil.Discard, multi)
			if err != nil && err != io.EOF {
				logrus.Debugf("error digesting layer blob %q: %v", blobinfo.Digest, id, err)
				return errorBlobInfo, err
			}
			hash = hasher.Digest().String()
		} else {
			// Applied the layer with the specified ID.  Note the
			// size info and computed digest.
			hash = hasher.Digest().String()
			layerMeta := storageLayerMetadata{
				Digest:         hash,
				CompressedSize: counter.Count,
				Size:           uncompressedSize,
			}
			if metadata, err := json.Marshal(&layerMeta); len(metadata) != 0 && err == nil {
				s.imageRef.transport.store.SetMetadata(layer.ID, string(metadata))
			}
			// Hang on to the new layer's ID.
			id = layer.ID
		}
		// Check if the size looks right.
		if enforceDigestAndSize && blobinfo.Size >= 0 && blobinfo.Size != counter.Count {
			logrus.Debugf("layer blob %q size is %d, not %d, rejecting", blobinfo.Digest, counter.Count, blobinfo.Size)
			if layer != nil {
				// Something's wrong; delete the newly-created layer.
				s.imageRef.transport.store.DeleteLayer(layer.ID)
			}
			return errorBlobInfo, ErrBlobSizeMismatch
		}
		// If the content digest was specified, verify it.
		if enforceDigestAndSize && digest.Validate() == nil && digest.String() != hash {
			logrus.Debugf("layer blob %q digests to %q, rejecting", blobinfo.Digest, hash)
			if layer != nil {
				// Something's wrong; delete the newly-created layer.
				s.imageRef.transport.store.DeleteLayer(layer.ID)
			}
			return errorBlobInfo, ErrBlobDigestMismatch
		}
		// If we didn't get a blob size, return the one we calculated.
		if blobSize == -1 {
			blobSize = counter.Count
		}
		// If we didn't get a digest, construct one.
		if digest == "" {
			digest = ddigest.Digest(hash)
		}
		// Record that this layer blob is a layer, and the layer ID it
		// ended up having.  This is a list, in case the same blob is
		// being applied more than once.
		s.Layers[digest] = append(s.Layers[digest], id)
		s.BlobList = append(s.BlobList, types.BlobInfo{Digest: digest, Size: counter.Count})
		if layer != nil {
			logrus.Debugf("blob %q imported as a filesystem layer %q", blobinfo.Digest, id)
		} else {
			logrus.Debugf("layer blob %q already present as layer %q", blobinfo.Digest, id)
		}
	} else {
		// It's just data.  Finish scanning it in, check that our
		// computed digest matches the passed-in digest, and store it,
		// but leave it out of the blob-to-layer-ID map so that we can
		// tell that it's not a layer.
		blob, err := ioutil.ReadAll(multi)
		if err != nil && err != io.EOF {
			return errorBlobInfo, err
		}
		hash = hasher.Digest().String()
		if enforceDigestAndSize && blobinfo.Size >= 0 && int64(len(blob)) != blobinfo.Size {
			logrus.Debugf("blob %q size is %d, not %d, rejecting", blobinfo.Digest, int64(len(blob)), blobinfo.Size)
			return errorBlobInfo, ErrBlobSizeMismatch
		}
		// If we were given a digest, verify that the content matches
		// it.
		if enforceDigestAndSize && digest.Validate() == nil && digest.String() != hash {
			logrus.Debugf("blob %q digests to %q, rejecting", blobinfo.Digest, hash)
			return errorBlobInfo, ErrBlobDigestMismatch
		}
		// If we didn't get a blob size, return the one we calculated.
		if blobSize == -1 {
			blobSize = int64(len(blob))
		}
		// If we didn't get a digest, construct one.
		if digest == "" {
			digest = ddigest.Digest(hash)
		}
		// Save the blob for when we Commit().
		s.BlobData[digest] = blob
		s.BlobList = append(s.BlobList, types.BlobInfo{Digest: digest, Size: int64(len(blob))})
		logrus.Debugf("blob %q imported as opaque data %q", blobinfo.Digest, digest)
	}
	return types.BlobInfo{
		Digest: digest,
		Size:   blobSize,
	}, nil
}

// PutBlob is used to both store filesystem layers and binary data that is part
// of the image.  Filesystem layers are assumed to be imported in order, as
// that is required by some of the underlying storage drivers.
func (s *storageImageDestination) PutBlob(stream io.Reader, blobinfo types.BlobInfo) (types.BlobInfo, error) {
	return s.putBlob(stream, blobinfo, true)
}

// HasBlob returns true iff the image destination already contains a blob with the matching digest which can be reapplied using ReapplyBlob.
// Unlike PutBlob, the digest can not be empty.  If HasBlob returns true, the size of the blob must also be returned.
// If the destination does not contain the blob, or it is unknown, HasBlob ordinarily returns (false, -1, nil);
// it returns a non-nil error only on an unexpected failure.
func (s *storageImageDestination) HasBlob(blobinfo types.BlobInfo) (bool, int64, error) {
	if blobinfo.Digest == "" {
		return false, -1, errors.Errorf(`Can not check for a blob with unknown digest`)
	}
	for _, blob := range s.BlobList {
		if blob.Digest == blobinfo.Digest {
			return true, blob.Size, nil
		}
	}
	return false, -1, nil
}

func (s *storageImageDestination) ReapplyBlob(blobinfo types.BlobInfo) (types.BlobInfo, error) {
	err := blobinfo.Digest.Validate()
	if err != nil {
		return types.BlobInfo{}, err
	}
	if layerList, ok := s.Layers[blobinfo.Digest]; !ok || len(layerList) < 1 {
		b, err := s.imageRef.transport.store.ImageBigData(s.ID, blobinfo.Digest.String())
		if err != nil {
			return types.BlobInfo{}, err
		}
		return types.BlobInfo{Digest: blobinfo.Digest, Size: int64(len(b))}, nil
	}
	layerList := s.Layers[blobinfo.Digest]
	rc, _, err := diffLayer(s.imageRef.transport.store, layerList[len(layerList)-1])
	if err != nil {
		return types.BlobInfo{}, err
	}
	return s.putBlob(rc, blobinfo, false)
}

func (s *storageImageDestination) Commit() error {
	// Create the image record.
	lastLayer := ""
	for _, blob := range s.BlobList {
		if layerList, ok := s.Layers[blob.Digest]; ok {
			lastLayer = layerList[len(layerList)-1]
		}
	}
	img, err := s.imageRef.transport.store.CreateImage(s.ID, nil, lastLayer, "", nil)
	if err != nil {
		if errors.Cause(err) != storage.ErrDuplicateID {
			logrus.Debugf("error creating image: %q", err)
			return errors.Wrapf(err, "error creating image %q", s.ID)
		}
		img, err = s.imageRef.transport.store.Image(s.ID)
		if err != nil {
			return errors.Wrapf(err, "error reading image %q", s.ID)
		}
		if img.TopLayer != lastLayer {
			logrus.Debugf("error creating image: image with ID %q exists, but uses different layers", s.ID)
			return errors.Wrapf(storage.ErrDuplicateID, "image with ID %q already exists, but uses a different top layer", s.ID)
		}
		logrus.Debugf("reusing image ID %q", img.ID)
	} else {
		logrus.Debugf("created new image ID %q", img.ID)
	}
	s.ID = img.ID
	names := img.Names
	if s.Tag != "" {
		names = append(names, s.Tag)
	}
	// We have names to set, so move those names to this image.
	if len(names) > 0 {
		if err := s.imageRef.transport.store.SetNames(img.ID, names); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error setting names on image %q: %v", img.ID, err)
			return err
		}
		logrus.Debugf("set names of image %q to %v", img.ID, names)
	}
	// Save the data blobs to disk, and drop their contents from memory.
	keys := []ddigest.Digest{}
	for k, v := range s.BlobData {
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, k.String(), v); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving big data %q for image %q: %v", k, img.ID, err)
			return err
		}
		keys = append(keys, k)
	}
	for _, key := range keys {
		delete(s.BlobData, key)
	}
	// Save the manifest, if we have one.
	if err := s.imageRef.transport.store.SetImageBigData(s.ID, "manifest", s.Manifest); err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return err
	}
	// Save the signatures, if we have any.
	if err := s.imageRef.transport.store.SetImageBigData(s.ID, "signatures", s.Signatures); err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
		return err
	}
	// Save our metadata.
	metadata, err := json.Marshal(s)
	if err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error encoding metadata for image %q: %v", img.ID, err)
		return err
	}
	if len(metadata) != 0 {
		if err = s.imageRef.transport.store.SetMetadata(s.ID, string(metadata)); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving metadata for image %q: %v", img.ID, err)
			return err
		}
		logrus.Debugf("saved image metadata %q", string(metadata))
	}
	return nil
}

var manifestMIMETypes = []string{
	// TODO(runcom): we'll add OCI as part of another PR here
	manifest.DockerV2Schema2MediaType,
	manifest.DockerV2Schema1SignedMediaType,
	manifest.DockerV2Schema1MediaType,
}

func (s *storageImageDestination) SupportedManifestMIMETypes() []string {
	return manifestMIMETypes
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (s *storageImageDestination) PutManifest(manifest []byte) error {
	s.Manifest = make([]byte, len(manifest))
	copy(s.Manifest, manifest)
	return nil
}

// SupportsSignatures returns an error if we can't expect GetSignatures() to
// return data that was previously supplied to PutSignatures().
func (s *storageImageDestination) SupportsSignatures() error {
	return nil
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (s *storageImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (s *storageImageDestination) MustMatchRuntimeOS() bool {
	return true
}

func (s *storageImageDestination) PutSignatures(signatures [][]byte) error {
	sizes := []int{}
	sigblob := []byte{}
	for _, sig := range signatures {
		sizes = append(sizes, len(sig))
		newblob := make([]byte, len(sigblob)+len(sig))
		copy(newblob, sigblob)
		copy(newblob[len(sigblob):], sig)
		sigblob = newblob
	}
	s.Signatures = sigblob
	s.SignatureSizes = sizes
	return nil
}

func (s *storageImageSource) GetBlob(info types.BlobInfo) (rc io.ReadCloser, n int64, err error) {
	rc, n, _, err = s.getBlobAndLayerID(info)
	return rc, n, err
}

func (s *storageImageSource) getBlobAndLayerID(info types.BlobInfo) (rc io.ReadCloser, n int64, layerID string, err error) {
	err = info.Digest.Validate()
	if err != nil {
		return nil, -1, "", err
	}
	if layerList, ok := s.Layers[info.Digest]; !ok || len(layerList) < 1 {
		b, err := s.imageRef.transport.store.ImageBigData(s.ID, info.Digest.String())
		if err != nil {
			return nil, -1, "", err
		}
		r := bytes.NewReader(b)
		logrus.Debugf("exporting opaque data as blob %q", info.Digest.String())
		return ioutil.NopCloser(r), int64(r.Len()), "", nil
	}
	// If the blob was "put" more than once, we have multiple layer IDs
	// which should all produce the same diff.  For the sake of tests that
	// want to make sure we created different layers each time the blob was
	// "put", though, cycle through the layers.
	layerList := s.Layers[info.Digest]
	position, ok := s.LayerPosition[info.Digest]
	if !ok {
		position = 0
	}
	s.LayerPosition[info.Digest] = (position + 1) % len(layerList)
	logrus.Debugf("exporting filesystem layer %q for blob %q", layerList[position], info.Digest)
	rc, n, err = diffLayer(s.imageRef.transport.store, layerList[position])
	return rc, n, layerList[position], err
}

func diffLayer(store storage.Store, layerID string) (rc io.ReadCloser, n int64, err error) {
	layer, err := store.Layer(layerID)
	if err != nil {
		return nil, -1, err
	}
	layerMeta := storageLayerMetadata{
		CompressedSize: -1,
	}
	if layer.Metadata != "" {
		if err := json.Unmarshal([]byte(layer.Metadata), &layerMeta); err != nil {
			return nil, -1, errors.Wrapf(err, "error decoding metadata for layer %q", layerID)
		}
	}
	if layerMeta.CompressedSize <= 0 {
		n = -1
	} else {
		n = layerMeta.CompressedSize
	}
	diff, err := store.Diff("", layer.ID, nil)
	if err != nil {
		return nil, -1, err
	}
	return diff, n, nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *storageImageSource) GetManifest(instanceDigest *ddigest.Digest) (manifestBlob []byte, MIMEType string, err error) {
	if instanceDigest != nil {
		return nil, "", ErrNoManifestLists
	}
	manifestBlob, err = s.imageRef.transport.store.ImageBigData(s.ID, "manifest")
	return manifestBlob, manifest.GuessMIMEType(manifestBlob), err
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *storageImageSource) GetSignatures(ctx context.Context, instanceDigest *ddigest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, ErrNoManifestLists
	}
	var offset int
	signature, err := s.imageRef.transport.store.ImageBigData(s.ID, "signatures")
	if err != nil {
		return nil, err
	}
	sigslice := [][]byte{}
	for _, length := range s.SignatureSizes {
		sigslice = append(sigslice, signature[offset:offset+length])
		offset += length
	}
	if offset != len(signature) {
		return nil, errors.Errorf("signatures data contained %d extra bytes", len(signature)-offset)
	}
	return sigslice, nil
}

func (s *storageImageSource) getSize() (int64, error) {
	var sum int64
	names, err := s.imageRef.transport.store.ListImageBigData(s.imageRef.id)
	if err != nil {
		return -1, errors.Wrapf(err, "error reading image %q", s.imageRef.id)
	}
	for _, name := range names {
		bigSize, err := s.imageRef.transport.store.ImageBigDataSize(s.imageRef.id, name)
		if err != nil {
			return -1, errors.Wrapf(err, "error reading data blob size %q for %q", name, s.imageRef.id)
		}
		sum += bigSize
	}
	for _, sigSize := range s.SignatureSizes {
		sum += int64(sigSize)
	}
	for _, layerList := range s.Layers {
		for _, layerID := range layerList {
			layer, err := s.imageRef.transport.store.Layer(layerID)
			if err != nil {
				return -1, err
			}
			layerMeta := storageLayerMetadata{
				Size: -1,
			}
			if layer.Metadata != "" {
				if err := json.Unmarshal([]byte(layer.Metadata), &layerMeta); err != nil {
					return -1, errors.Wrapf(err, "error decoding metadata for layer %q", layerID)
				}
			}
			if layerMeta.Size < 0 {
				return -1, errors.Errorf("size for layer %q is unknown, failing getSize()", layerID)
			}
			sum += layerMeta.Size
		}
	}
	return sum, nil
}

func (s *storageImageCloser) Size() (int64, error) {
	return s.size, nil
}

// newImage creates an ImageCloser that also knows its size
func newImage(ctx *types.SystemContext, s storageReference) (types.ImageCloser, error) {
	src, err := newImageSource(s)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(ctx, src)
	if err != nil {
		return nil, err
	}
	size, err := src.getSize()
	if err != nil {
		return nil, err
	}
	return &storageImageCloser{ImageCloser: img, size: size}, nil
}
