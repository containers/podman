// +build containers_image_ostree

package ostree

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"unsafe"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
	"github.com/pkg/errors"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// #cgo pkg-config: glib-2.0 gobject-2.0 ostree-1
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <stdlib.h>
// #include <ostree.h>
// #include <gio/ginputstream.h>
import "C"

type ostreeImageSource struct {
	ref    ostreeReference
	tmpDir string
	repo   *C.struct_OstreeRepo
	// get the compressed layer by its uncompressed checksum
	compressed map[digest.Digest]digest.Digest
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(tmpDir string, ref ostreeReference) (types.ImageSource, error) {
	return &ostreeImageSource{ref: ref, tmpDir: tmpDir, compressed: nil}, nil
}

// Reference returns the reference used to set up this source.
func (s *ostreeImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *ostreeImageSource) Close() error {
	if s.repo != nil {
		C.g_object_unref(C.gpointer(s.repo))
	}
	return nil
}

func (s *ostreeImageSource) getBlobUncompressedSize(blob string, isCompressed bool) (int64, error) {
	var metadataKey string
	if isCompressed {
		metadataKey = "docker.uncompressed_size"
	} else {
		metadataKey = "docker.size"
	}
	b := fmt.Sprintf("ociimage/%s", blob)
	found, data, err := readMetadata(s.repo, b, metadataKey)
	if err != nil || !found {
		return 0, err
	}
	return strconv.ParseInt(data, 10, 64)
}

func (s *ostreeImageSource) getLenSignatures() (int64, error) {
	b := fmt.Sprintf("ociimage/%s", s.ref.branchName)
	found, data, err := readMetadata(s.repo, b, "signatures")
	if err != nil {
		return -1, err
	}
	if !found {
		// if 'signatures' is not present, just return 0 signatures.
		return 0, nil
	}
	return strconv.ParseInt(data, 10, 64)
}

func (s *ostreeImageSource) getTarSplitData(blob string) ([]byte, error) {
	b := fmt.Sprintf("ociimage/%s", blob)
	found, out, err := readMetadata(s.repo, b, "tarsplit.output")
	if err != nil || !found {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out)
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be non-default instances.
func (s *ostreeImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return nil, "", errors.New(`Manifest lists are not supported by "ostree:"`)
	}
	if s.repo == nil {
		repo, err := openRepo(s.ref.repo)
		if err != nil {
			return nil, "", err
		}
		s.repo = repo
	}

	b := fmt.Sprintf("ociimage/%s", s.ref.branchName)
	found, out, err := readMetadata(s.repo, b, "docker.manifest")
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", errors.New("manifest not found")
	}
	m := []byte(out)
	return m, manifest.GuessMIMEType(m), nil
}

func (s *ostreeImageSource) GetTargetManifest(digest digest.Digest) ([]byte, string, error) {
	return nil, "", errors.New("manifest lists are not supported by this transport")
}

func openRepo(path string) (*C.struct_OstreeRepo, error) {
	var cerr *C.GError
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	repo := C.ostree_repo_new(pathc)
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_open(repo, nil, &cerr)))
	if !r {
		C.g_object_unref(C.gpointer(repo))
		return nil, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return repo, nil
}

type ostreePathFileGetter struct {
	repo       *C.struct_OstreeRepo
	parentRoot *C.GFile
}

type ostreeReader struct {
	stream *C.GFileInputStream
}

func (o ostreeReader) Close() error {
	C.g_object_unref(C.gpointer(o.stream))
	return nil
}
func (o ostreeReader) Read(p []byte) (int, error) {
	var cerr *C.GError
	instanceCast := C.g_type_check_instance_cast((*C.GTypeInstance)(unsafe.Pointer(o.stream)), C.g_input_stream_get_type())
	stream := (*C.GInputStream)(unsafe.Pointer(instanceCast))

	b := C.g_input_stream_read_bytes(stream, (C.gsize)(cap(p)), nil, &cerr)
	if b == nil {
		return 0, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	defer C.g_bytes_unref(b)

	count := int(C.g_bytes_get_size(b))
	if count == 0 {
		return 0, io.EOF
	}
	data := (*[1 << 30]byte)(unsafe.Pointer(C.g_bytes_get_data(b, nil)))[:count:count]
	copy(p, data)
	return count, nil
}

func readMetadata(repo *C.struct_OstreeRepo, commit, key string) (bool, string, error) {
	var cerr *C.GError
	var ref *C.char
	defer C.free(unsafe.Pointer(ref))

	cCommit := C.CString(commit)
	defer C.free(unsafe.Pointer(cCommit))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo, cCommit, C.gboolean(1), &ref, &cerr))) {
		return false, "", glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}

	if ref == nil {
		return false, "", nil
	}

	var variant *C.GVariant
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_load_variant(repo, C.OSTREE_OBJECT_TYPE_COMMIT, ref, &variant, &cerr))) {
		return false, "", glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	defer C.g_variant_unref(variant)
	if variant != nil {
		cKey := C.CString(key)
		defer C.free(unsafe.Pointer(cKey))

		metadata := C.g_variant_get_child_value(variant, 0)
		defer C.g_variant_unref(metadata)

		data := C.g_variant_lookup_value(metadata, (*C.gchar)(cKey), nil)
		if data != nil {
			defer C.g_variant_unref(data)
			ptr := (*C.char)(C.g_variant_get_string(data, nil))
			val := C.GoString(ptr)
			return true, val, nil
		}
	}
	return false, "", nil
}

func newOSTreePathFileGetter(repo *C.struct_OstreeRepo, commit string) (*ostreePathFileGetter, error) {
	var cerr *C.GError
	var parentRoot *C.GFile
	cCommit := C.CString(commit)
	defer C.free(unsafe.Pointer(cCommit))
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_read_commit(repo, cCommit, &parentRoot, nil, nil, &cerr))) {
		return &ostreePathFileGetter{}, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}

	C.g_object_ref(C.gpointer(repo))

	return &ostreePathFileGetter{repo: repo, parentRoot: parentRoot}, nil
}

func (o ostreePathFileGetter) Get(filename string) (io.ReadCloser, error) {
	var file *C.GFile
	if strings.HasPrefix(filename, "./") {
		filename = filename[2:]
	}
	cfilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cfilename))

	file = (*C.GFile)(C.g_file_resolve_relative_path(o.parentRoot, cfilename))

	var cerr *C.GError
	stream := C.g_file_read(file, nil, &cerr)
	if stream == nil {
		return nil, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}

	return &ostreeReader{stream: stream}, nil
}

func (o ostreePathFileGetter) Close() {
	C.g_object_unref(C.gpointer(o.repo))
	C.g_object_unref(C.gpointer(o.parentRoot))
}

func (s *ostreeImageSource) readSingleFile(commit, path string) (io.ReadCloser, error) {
	getter, err := newOSTreePathFileGetter(s.repo, commit)
	if err != nil {
		return nil, err
	}
	defer getter.Close()

	return getter.Get(path)
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *ostreeImageSource) HasThreadSafeGetBlob() bool {
	return false
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *ostreeImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {

	blob := info.Digest.Hex()

	// Ensure s.compressed is initialized.  It is build by LayerInfosForCopy.
	if s.compressed == nil {
		_, err := s.LayerInfosForCopy(ctx, nil)
		if err != nil {
			return nil, -1, err
		}

	}
	compressedBlob, isCompressed := s.compressed[info.Digest]
	if isCompressed {
		blob = compressedBlob.Hex()
	}
	branch := fmt.Sprintf("ociimage/%s", blob)

	if s.repo == nil {
		repo, err := openRepo(s.ref.repo)
		if err != nil {
			return nil, 0, err
		}
		s.repo = repo
	}

	layerSize, err := s.getBlobUncompressedSize(blob, isCompressed)
	if err != nil {
		return nil, 0, err
	}

	tarsplit, err := s.getTarSplitData(blob)
	if err != nil {
		return nil, 0, err
	}

	// if tarsplit is nil we are looking at the manifest.  Return directly the file in /content
	if tarsplit == nil {
		file, err := s.readSingleFile(branch, "/content")
		if err != nil {
			return nil, 0, err
		}
		return file, layerSize, nil
	}

	mf := bytes.NewReader(tarsplit)
	mfz, err := pgzip.NewReader(mf)
	if err != nil {
		return nil, 0, err
	}
	metaUnpacker := storage.NewJSONUnpacker(mfz)

	getter, err := newOSTreePathFileGetter(s.repo, branch)
	if err != nil {
		mfz.Close()
		return nil, 0, err
	}

	ots := asm.NewOutputTarStream(getter, metaUnpacker)

	rc := ioutils.NewReadCloserWrapper(ots, func() error {
		getter.Close()
		mfz.Close()
		return ots.Close()
	})
	return rc, layerSize, nil
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as there can be no secondary manifests.
func (s *ostreeImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, errors.New(`Manifest lists are not supported by "ostree:"`)
	}
	lenSignatures, err := s.getLenSignatures()
	if err != nil {
		return nil, err
	}
	branch := fmt.Sprintf("ociimage/%s", s.ref.branchName)

	if s.repo == nil {
		repo, err := openRepo(s.ref.repo)
		if err != nil {
			return nil, err
		}
		s.repo = repo
	}

	signatures := [][]byte{}
	for i := int64(1); i <= lenSignatures; i++ {
		sigReader, err := s.readSingleFile(branch, fmt.Sprintf("/signature-%d", i))
		if err != nil {
			return nil, err
		}
		defer sigReader.Close()

		sig, err := ioutil.ReadAll(sigReader)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, sig)
	}
	return signatures, nil
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be secondary manifests.
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *ostreeImageSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	if instanceDigest != nil {
		return nil, errors.New(`Manifest lists are not supported by "ostree:"`)
	}

	updatedBlobInfos := []types.BlobInfo{}
	manifestBlob, manifestType, err := s.GetManifest(ctx, nil)
	if err != nil {
		return nil, err
	}

	man, err := manifest.FromBlob(manifestBlob, manifestType)

	s.compressed = make(map[digest.Digest]digest.Digest)

	layerBlobs := man.LayerInfos()

	for _, layerBlob := range layerBlobs {
		branch := fmt.Sprintf("ociimage/%s", layerBlob.Digest.Hex())
		found, uncompressedDigestStr, err := readMetadata(s.repo, branch, "docker.uncompressed_digest")
		if err != nil || !found {
			return nil, err
		}

		found, uncompressedSizeStr, err := readMetadata(s.repo, branch, "docker.uncompressed_size")
		if err != nil || !found {
			return nil, err
		}

		uncompressedSize, err := strconv.ParseInt(uncompressedSizeStr, 10, 64)
		if err != nil {
			return nil, err
		}
		uncompressedDigest := digest.Digest(uncompressedDigestStr)
		blobInfo := types.BlobInfo{
			Digest:    uncompressedDigest,
			Size:      uncompressedSize,
			MediaType: layerBlob.MediaType,
		}
		s.compressed[uncompressedDigest] = layerBlob.Digest
		updatedBlobInfos = append(updatedBlobInfos, blobInfo)
	}
	return updatedBlobInfos, nil
}
