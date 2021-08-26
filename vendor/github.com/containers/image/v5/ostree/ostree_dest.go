//go:build containers_image_ostree
// +build containers_image_ostree

package ostree

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/containers/image/v5/internal/putblobdigest"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/archive"
	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/ostreedev/ostree-go/pkg/otbuiltin"
	"github.com/pkg/errors"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// #cgo pkg-config: glib-2.0 gobject-2.0 ostree-1 libselinux
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <stdlib.h>
// #include <ostree.h>
// #include <gio/ginputstream.h>
// #include <selinux/selinux.h>
// #include <selinux/label.h>
import "C"

type blobToImport struct {
	Size     int64
	Digest   digest.Digest
	BlobPath string
}

type descriptor struct {
	Size   int64         `json:"size"`
	Digest digest.Digest `json:"digest"`
}

type fsLayersSchema1 struct {
	BlobSum digest.Digest `json:"blobSum"`
}

type manifestSchema struct {
	LayersDescriptors []descriptor      `json:"layers"`
	FSLayers          []fsLayersSchema1 `json:"fsLayers"`
}

type ostreeImageDestination struct {
	ref           ostreeReference
	manifest      string
	schema        manifestSchema
	tmpDirPath    string
	blobs         map[string]*blobToImport
	digest        digest.Digest
	signaturesLen int
	repo          *C.struct_OstreeRepo
}

// newImageDestination returns an ImageDestination for writing to an existing ostree.
func newImageDestination(ref ostreeReference, tmpDirPath string) (types.ImageDestination, error) {
	tmpDirPath = filepath.Join(tmpDirPath, ref.branchName)
	if err := ensureDirectoryExists(tmpDirPath); err != nil {
		return nil, err
	}
	return &ostreeImageDestination{ref, "", manifestSchema{}, tmpDirPath, map[string]*blobToImport{}, "", 0, nil}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *ostreeImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *ostreeImageDestination) Close() error {
	if d.repo != nil {
		C.g_object_unref(C.gpointer(d.repo))
	}
	return os.RemoveAll(d.tmpDirPath)
}

func (d *ostreeImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		manifest.DockerV2Schema2MediaType,
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *ostreeImageDestination) SupportsSignatures(ctx context.Context) error {
	return nil
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *ostreeImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.PreserveOriginal
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *ostreeImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *ostreeImageDestination) MustMatchRuntimeOS() bool {
	return true
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *ostreeImageDestination) IgnoresEmbeddedDockerReference() bool {
	return false // N/A, DockerReference() returns nil.
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *ostreeImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *ostreeImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	tmpDir, err := ioutil.TempDir(d.tmpDirPath, "blob")
	if err != nil {
		return types.BlobInfo{}, err
	}

	blobPath := filepath.Join(tmpDir, "content")
	blobFile, err := os.Create(blobPath)
	if err != nil {
		return types.BlobInfo{}, err
	}
	defer blobFile.Close()

	digester, stream := putblobdigest.DigestIfCanonicalUnknown(stream, inputInfo)
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	size, err := io.Copy(blobFile, stream)
	if err != nil {
		return types.BlobInfo{}, err
	}
	blobDigest := digester.Digest()
	if inputInfo.Size != -1 && size != inputInfo.Size {
		return types.BlobInfo{}, errors.Errorf("Size mismatch when copying %s, expected %d, got %d", blobDigest, inputInfo.Size, size)
	}
	if err := blobFile.Sync(); err != nil {
		return types.BlobInfo{}, err
	}

	hash := blobDigest.Hex()
	d.blobs[hash] = &blobToImport{Size: size, Digest: blobDigest, BlobPath: blobPath}
	return types.BlobInfo{Digest: blobDigest, Size: size}, nil
}

func fixFiles(selinuxHnd *C.struct_selabel_handle, root string, dir string, usermode bool) error {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range entries {
		fullpath := filepath.Join(dir, info.Name())
		if info.Mode()&(os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
			if err := os.Remove(fullpath); err != nil {
				return err
			}
			continue
		}

		if selinuxHnd != nil {
			relPath, err := filepath.Rel(root, fullpath)
			if err != nil {
				return err
			}
			// Handle /exports/hostfs as a special case.  Files under this directory are copied to the host,
			// thus we benefit from maintaining the same SELinux label they would have on the host as we could
			// use hard links instead of copying the files.
			relPath = fmt.Sprintf("/%s", strings.TrimPrefix(relPath, "exports/hostfs/"))

			relPathC := C.CString(relPath)
			defer C.free(unsafe.Pointer(relPathC))
			var context *C.char

			res, err := C.selabel_lookup_raw(selinuxHnd, &context, relPathC, C.int(info.Mode()&os.ModePerm))
			if int(res) < 0 && err != syscall.ENOENT {
				return errors.Wrapf(err, "cannot selabel_lookup_raw %s", relPath)
			}
			if int(res) == 0 {
				defer C.freecon(context)
				fullpathC := C.CString(fullpath)
				defer C.free(unsafe.Pointer(fullpathC))
				res, err = C.lsetfilecon_raw(fullpathC, context)
				if int(res) < 0 {
					return errors.Wrapf(err, "cannot setfilecon_raw %s to %s", fullpath, C.GoString(context))
				}
			}
		}

		if info.IsDir() {
			if usermode {
				if err := os.Chmod(fullpath, info.Mode()|0700); err != nil {
					return err
				}
			}
			err = fixFiles(selinuxHnd, root, fullpath, usermode)
			if err != nil {
				return err
			}
		} else if usermode && (info.Mode().IsRegular()) {
			if err := os.Chmod(fullpath, info.Mode()|0600); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *ostreeImageDestination) ostreeCommit(repo *otbuiltin.Repo, branch string, root string, metadata []string) error {
	opts := otbuiltin.NewCommitOptions()
	opts.AddMetadataString = metadata
	opts.Timestamp = time.Now()
	// OCI layers have no parent OSTree commit
	opts.Parent = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := repo.Commit(root, branch, opts)
	return err
}

func generateTarSplitMetadata(output *bytes.Buffer, file string) (digest.Digest, int64, error) {
	mfz := pgzip.NewWriter(output)
	defer mfz.Close()
	metaPacker := storage.NewJSONPacker(mfz)

	stream, err := os.OpenFile(file, os.O_RDONLY, 0)
	if err != nil {
		return "", -1, err
	}
	defer stream.Close()

	gzReader, err := archive.DecompressStream(stream)
	if err != nil {
		return "", -1, err
	}
	defer gzReader.Close()

	its, err := asm.NewInputTarStream(gzReader, metaPacker, nil)
	if err != nil {
		return "", -1, err
	}

	digester := digest.Canonical.Digester()

	written, err := io.Copy(digester.Hash(), its)
	if err != nil {
		return "", -1, err
	}

	return digester.Digest(), written, nil
}

func (d *ostreeImageDestination) importBlob(selinuxHnd *C.struct_selabel_handle, repo *otbuiltin.Repo, blob *blobToImport) error {
	// TODO: This can take quite some time, and should ideally be cancellable using a context.Context.

	ostreeBranch := fmt.Sprintf("ociimage/%s", blob.Digest.Hex())
	destinationPath := filepath.Join(d.tmpDirPath, blob.Digest.Hex(), "root")
	if err := ensureDirectoryExists(destinationPath); err != nil {
		return err
	}
	defer func() {
		os.Remove(blob.BlobPath)
		os.RemoveAll(destinationPath)
	}()

	var tarSplitOutput bytes.Buffer
	uncompressedDigest, uncompressedSize, err := generateTarSplitMetadata(&tarSplitOutput, blob.BlobPath)
	if err != nil {
		return err
	}

	if os.Getuid() == 0 {
		if err := archive.UntarPath(blob.BlobPath, destinationPath); err != nil {
			return err
		}
		if err := fixFiles(selinuxHnd, destinationPath, destinationPath, false); err != nil {
			return err
		}
	} else {
		os.MkdirAll(destinationPath, 0755)
		if err := exec.Command("tar", "-C", destinationPath, "--no-same-owner", "--no-same-permissions", "--delay-directory-restore", "-xf", blob.BlobPath).Run(); err != nil {
			return err
		}

		if err := fixFiles(selinuxHnd, destinationPath, destinationPath, true); err != nil {
			return err
		}
	}
	return d.ostreeCommit(repo, ostreeBranch, destinationPath, []string{fmt.Sprintf("docker.size=%d", blob.Size),
		fmt.Sprintf("docker.uncompressed_size=%d", uncompressedSize),
		fmt.Sprintf("docker.uncompressed_digest=%s", uncompressedDigest.String()),
		fmt.Sprintf("tarsplit.output=%s", base64.StdEncoding.EncodeToString(tarSplitOutput.Bytes()))})

}

func (d *ostreeImageDestination) importConfig(repo *otbuiltin.Repo, blob *blobToImport) error {
	ostreeBranch := fmt.Sprintf("ociimage/%s", blob.Digest.Hex())
	destinationPath := filepath.Dir(blob.BlobPath)

	return d.ostreeCommit(repo, ostreeBranch, destinationPath, []string{fmt.Sprintf("docker.size=%d", blob.Size)})
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been successfully reused, returns (true, info, nil); info must contain at least a digest and size, and may
// include CompressionOperation and CompressionAlgorithm fields to indicate that a change to the compression type should be
// reflected in the manifest that will be written.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *ostreeImageDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if d.repo == nil {
		repo, err := openRepo(d.ref.repo)
		if err != nil {
			return false, types.BlobInfo{}, err
		}
		d.repo = repo
	}
	branch := fmt.Sprintf("ociimage/%s", info.Digest.Hex())

	found, data, err := readMetadata(d.repo, branch, "docker.uncompressed_digest")
	if err != nil || !found {
		return found, types.BlobInfo{}, err
	}

	found, data, err = readMetadata(d.repo, branch, "docker.uncompressed_size")
	if err != nil || !found {
		return found, types.BlobInfo{}, err
	}

	found, data, err = readMetadata(d.repo, branch, "docker.size")
	if err != nil || !found {
		return found, types.BlobInfo{}, err
	}

	size, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return false, types.BlobInfo{}, err
	}

	return true, types.BlobInfo{Digest: info.Digest, Size: size}, nil
}

// PutManifest writes manifest to the destination.
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *ostreeImageDestination) PutManifest(ctx context.Context, manifestBlob []byte, instanceDigest *digest.Digest) error {
	if instanceDigest != nil {
		return errors.New(`Manifest lists are not supported by "ostree:"`)
	}

	d.manifest = string(manifestBlob)

	if err := json.Unmarshal(manifestBlob, &d.schema); err != nil {
		return err
	}

	manifestPath := filepath.Join(d.tmpDirPath, d.ref.manifestPath())
	if err := ensureParentDirectoryExists(manifestPath); err != nil {
		return err
	}

	digest, err := manifest.Digest(manifestBlob)
	if err != nil {
		return err
	}
	d.digest = digest

	return ioutil.WriteFile(manifestPath, manifestBlob, 0644)
}

// PutSignatures writes signatures to the destination.
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.
func (d *ostreeImageDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	if instanceDigest != nil {
		return errors.New(`Manifest lists are not supported by "ostree:"`)
	}

	path := filepath.Join(d.tmpDirPath, d.ref.signaturePath(0))
	if err := ensureParentDirectoryExists(path); err != nil {
		return err
	}

	for i, sig := range signatures {
		signaturePath := filepath.Join(d.tmpDirPath, d.ref.signaturePath(i))
		if err := ioutil.WriteFile(signaturePath, sig, 0644); err != nil {
			return err
		}
	}
	d.signaturesLen = len(signatures)
	return nil
}

func (d *ostreeImageDestination) Commit(context.Context, types.UnparsedImage) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	repo, err := otbuiltin.OpenRepo(d.ref.repo)
	if err != nil {
		return err
	}

	_, err = repo.PrepareTransaction()
	if err != nil {
		return err
	}

	var selinuxHnd *C.struct_selabel_handle

	if os.Getuid() == 0 && selinux.GetEnabled() {
		selinuxHnd, err = C.selabel_open(C.SELABEL_CTX_FILE, nil, 0)
		if selinuxHnd == nil {
			return errors.Wrapf(err, "cannot open the SELinux DB")
		}

		defer C.selabel_close(selinuxHnd)
	}

	checkLayer := func(hash string) error {
		blob := d.blobs[hash]
		// if the blob is not present in d.blobs then it is already stored in OSTree,
		// and we don't need to import it.
		if blob == nil {
			return nil
		}
		err := d.importBlob(selinuxHnd, repo, blob)
		if err != nil {
			return err
		}

		delete(d.blobs, hash)
		return nil
	}
	for _, layer := range d.schema.LayersDescriptors {
		hash := layer.Digest.Hex()
		if err = checkLayer(hash); err != nil {
			return err
		}
	}
	for _, layer := range d.schema.FSLayers {
		hash := layer.BlobSum.Hex()
		if err = checkLayer(hash); err != nil {
			return err
		}
	}

	// Import the other blobs that are not layers
	for _, blob := range d.blobs {
		err := d.importConfig(repo, blob)
		if err != nil {
			return err
		}
	}

	manifestPath := filepath.Join(d.tmpDirPath, "manifest")

	metadata := []string{fmt.Sprintf("docker.manifest=%s", string(d.manifest)),
		fmt.Sprintf("signatures=%d", d.signaturesLen),
		fmt.Sprintf("docker.digest=%s", string(d.digest))}
	if err := d.ostreeCommit(repo, fmt.Sprintf("ociimage/%s", d.ref.branchName), manifestPath, metadata); err != nil {
		return err
	}

	_, err = repo.CommitTransaction()
	return err
}

func ensureDirectoryExists(path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func ensureParentDirectoryExists(path string) error {
	return ensureDirectoryExists(filepath.Dir(path))
}
