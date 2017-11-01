// +build !containers_image_ostree_stub

package ostree

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/archive"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/ostreedev/ostree-go/pkg/otbuiltin"
)

type blobToImport struct {
	Size     int64
	Digest   digest.Digest
	BlobPath string
}

type descriptor struct {
	Size   int64         `json:"size"`
	Digest digest.Digest `json:"digest"`
}

type manifestSchema struct {
	ConfigDescriptor  descriptor   `json:"config"`
	LayersDescriptors []descriptor `json:"layers"`
}

type ostreeImageDestination struct {
	ref        ostreeReference
	manifest   string
	schema     manifestSchema
	tmpDirPath string
	blobs      map[string]*blobToImport
	digest     digest.Digest
}

// newImageDestination returns an ImageDestination for writing to an existing ostree.
func newImageDestination(ref ostreeReference, tmpDirPath string) (types.ImageDestination, error) {
	tmpDirPath = filepath.Join(tmpDirPath, ref.branchName)
	if err := ensureDirectoryExists(tmpDirPath); err != nil {
		return nil, err
	}
	return &ostreeImageDestination{ref, "", manifestSchema{}, tmpDirPath, map[string]*blobToImport{}, ""}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *ostreeImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *ostreeImageDestination) Close() error {
	return os.RemoveAll(d.tmpDirPath)
}

func (d *ostreeImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		manifest.DockerV2Schema2MediaType,
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *ostreeImageDestination) SupportsSignatures() error {
	return nil
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *ostreeImageDestination) ShouldCompressLayers() bool {
	return false
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *ostreeImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (d *ostreeImageDestination) MustMatchRuntimeOS() bool {
	return true
}

func (d *ostreeImageDestination) PutBlob(stream io.Reader, inputInfo types.BlobInfo) (types.BlobInfo, error) {
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

	digester := digest.Canonical.Digester()
	tee := io.TeeReader(stream, digester.Hash())

	size, err := io.Copy(blobFile, tee)
	if err != nil {
		return types.BlobInfo{}, err
	}
	computedDigest := digester.Digest()
	if inputInfo.Size != -1 && size != inputInfo.Size {
		return types.BlobInfo{}, errors.Errorf("Size mismatch when copying %s, expected %d, got %d", computedDigest, inputInfo.Size, size)
	}
	if err := blobFile.Sync(); err != nil {
		return types.BlobInfo{}, err
	}

	hash := computedDigest.Hex()
	d.blobs[hash] = &blobToImport{Size: size, Digest: computedDigest, BlobPath: blobPath}
	return types.BlobInfo{Digest: computedDigest, Size: size}, nil
}

func fixFiles(dir string, usermode bool) error {
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
		if info.IsDir() {
			if usermode {
				if err := os.Chmod(fullpath, info.Mode()|0700); err != nil {
					return err
				}
			}
			err = fixFiles(fullpath, usermode)
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

func (d *ostreeImageDestination) importBlob(repo *otbuiltin.Repo, blob *blobToImport) error {
	ostreeBranch := fmt.Sprintf("ociimage/%s", blob.Digest.Hex())
	destinationPath := filepath.Join(d.tmpDirPath, blob.Digest.Hex(), "root")
	if err := ensureDirectoryExists(destinationPath); err != nil {
		return err
	}
	defer func() {
		os.Remove(blob.BlobPath)
		os.RemoveAll(destinationPath)
	}()

	if os.Getuid() == 0 {
		if err := archive.UntarPath(blob.BlobPath, destinationPath); err != nil {
			return err
		}
		if err := fixFiles(destinationPath, false); err != nil {
			return err
		}
	} else {
		os.MkdirAll(destinationPath, 0755)
		if err := exec.Command("tar", "-C", destinationPath, "--no-same-owner", "--no-same-permissions", "--delay-directory-restore", "-xf", blob.BlobPath).Run(); err != nil {
			return err
		}

		if err := fixFiles(destinationPath, true); err != nil {
			return err
		}
	}
	return d.ostreeCommit(repo, ostreeBranch, destinationPath, []string{fmt.Sprintf("docker.size=%d", blob.Size)})
}

func (d *ostreeImageDestination) importConfig(blob *blobToImport) error {
	ostreeBranch := fmt.Sprintf("ociimage/%s", blob.Digest.Hex())

	return exec.Command("ostree", "commit",
		"--repo", d.ref.repo,
		fmt.Sprintf("--add-metadata-string=docker.size=%d", blob.Size),
		"--branch", ostreeBranch, filepath.Dir(blob.BlobPath)).Run()
}

func (d *ostreeImageDestination) HasBlob(info types.BlobInfo) (bool, int64, error) {
	branch := fmt.Sprintf("ociimage/%s", info.Digest.Hex())
	output, err := exec.Command("ostree", "show", "--repo", d.ref.repo, "--print-metadata-key=docker.size", branch).CombinedOutput()
	if err != nil {
		if bytes.Index(output, []byte("not found")) >= 0 || bytes.Index(output, []byte("No such")) >= 0 {
			return false, -1, nil
		}
		return false, -1, err
	}
	size, err := strconv.ParseInt(strings.Trim(string(output), "'\n"), 10, 64)
	if err != nil {
		return false, -1, err
	}

	return true, size, nil
}

func (d *ostreeImageDestination) ReapplyBlob(info types.BlobInfo) (types.BlobInfo, error) {
	return info, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *ostreeImageDestination) PutManifest(manifestBlob []byte) error {
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

func (d *ostreeImageDestination) PutSignatures(signatures [][]byte) error {
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
	return nil
}

func (d *ostreeImageDestination) Commit() error {
	repo, err := otbuiltin.OpenRepo(d.ref.repo)
	if err != nil {
		return err
	}

	_, err = repo.PrepareTransaction()
	if err != nil {
		return err
	}

	for _, layer := range d.schema.LayersDescriptors {
		hash := layer.Digest.Hex()
		blob := d.blobs[hash]
		// if the blob is not present in d.blobs then it is already stored in OSTree,
		// and we don't need to import it.
		if blob == nil {
			continue
		}
		err := d.importBlob(repo, blob)
		if err != nil {
			return err
		}
	}

	hash := d.schema.ConfigDescriptor.Digest.Hex()
	blob := d.blobs[hash]
	if blob != nil {
		err := d.importConfig(blob)
		if err != nil {
			return err
		}
	}

	manifestPath := filepath.Join(d.tmpDirPath, "manifest")

	metadata := []string{fmt.Sprintf("docker.manifest=%s", string(d.manifest)), fmt.Sprintf("docker.digest=%s", string(d.digest))}
	err = d.ostreeCommit(repo, fmt.Sprintf("ociimage/%s", d.ref.branchName), manifestPath, metadata)

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
