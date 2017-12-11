package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type dockerImageDestination struct {
	ref dockerReference
	c   *dockerClient
	// State
	manifestDigest digest.Digest // or "" if not yet known.
}

// newImageDestination creates a new ImageDestination for the specified image reference.
func newImageDestination(ctx *types.SystemContext, ref dockerReference) (types.ImageDestination, error) {
	c, err := newDockerClientFromRef(ctx, ref, true, "pull,push")
	if err != nil {
		return nil, err
	}
	return &dockerImageDestination{
		ref: ref,
		c:   c,
	}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *dockerImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *dockerImageDestination) Close() error {
	return nil
}

func (d *dockerImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		imgspecv1.MediaTypeImageManifest,
		manifest.DockerV2Schema2MediaType,
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *dockerImageDestination) SupportsSignatures() error {
	if err := d.c.detectProperties(context.TODO()); err != nil {
		return err
	}
	switch {
	case d.c.signatureBase != nil:
		return nil
	case d.c.supportsSignatures:
		return nil
	default:
		return errors.Errorf("X-Registry-Supports-Signatures extension not supported, and lookaside is not configured")
	}
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *dockerImageDestination) ShouldCompressLayers() bool {
	return true
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *dockerImageDestination) AcceptsForeignLayerURLs() bool {
	return true
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (d *dockerImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// sizeCounter is an io.Writer which only counts the total size of its input.
type sizeCounter struct{ size int64 }

func (c *sizeCounter) Write(p []byte) (n int, err error) {
	c.size += int64(len(p))
	return len(p), nil
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *dockerImageDestination) PutBlob(stream io.Reader, inputInfo types.BlobInfo) (types.BlobInfo, error) {
	if inputInfo.Digest.String() != "" {
		haveBlob, size, err := d.HasBlob(inputInfo)
		if err != nil {
			return types.BlobInfo{}, err
		}
		if haveBlob {
			return types.BlobInfo{Digest: inputInfo.Digest, Size: size}, nil
		}
	}

	// FIXME? Chunked upload, progress reporting, etc.
	uploadPath := fmt.Sprintf(blobUploadPath, reference.Path(d.ref.ref))
	logrus.Debugf("Uploading %s", uploadPath)
	res, err := d.c.makeRequest(context.TODO(), "POST", uploadPath, nil, nil)
	if err != nil {
		return types.BlobInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		logrus.Debugf("Error initiating layer upload, response %#v", *res)
		return types.BlobInfo{}, errors.Errorf("Error initiating layer upload to %s, status %d", uploadPath, res.StatusCode)
	}
	uploadLocation, err := res.Location()
	if err != nil {
		return types.BlobInfo{}, errors.Wrap(err, "Error determining upload URL")
	}

	digester := digest.Canonical.Digester()
	sizeCounter := &sizeCounter{}
	tee := io.TeeReader(stream, io.MultiWriter(digester.Hash(), sizeCounter))
	res, err = d.c.makeRequestToResolvedURL(context.TODO(), "PATCH", uploadLocation.String(), map[string][]string{"Content-Type": {"application/octet-stream"}}, tee, inputInfo.Size, true)
	if err != nil {
		logrus.Debugf("Error uploading layer chunked, response %#v", res)
		return types.BlobInfo{}, err
	}
	defer res.Body.Close()
	computedDigest := digester.Digest()

	uploadLocation, err = res.Location()
	if err != nil {
		return types.BlobInfo{}, errors.Wrap(err, "Error determining upload URL")
	}

	// FIXME: DELETE uploadLocation on failure

	locationQuery := uploadLocation.Query()
	// TODO: check inputInfo.Digest == computedDigest https://github.com/containers/image/pull/70#discussion_r77646717
	locationQuery.Set("digest", computedDigest.String())
	uploadLocation.RawQuery = locationQuery.Encode()
	res, err = d.c.makeRequestToResolvedURL(context.TODO(), "PUT", uploadLocation.String(), map[string][]string{"Content-Type": {"application/octet-stream"}}, nil, -1, true)
	if err != nil {
		return types.BlobInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		logrus.Debugf("Error uploading layer, response %#v", *res)
		return types.BlobInfo{}, errors.Errorf("Error uploading layer to %s, status %d", uploadLocation, res.StatusCode)
	}

	logrus.Debugf("Upload of layer %s complete", computedDigest)
	return types.BlobInfo{Digest: computedDigest, Size: sizeCounter.size}, nil
}

// HasBlob returns true iff the image destination already contains a blob with the matching digest which can be reapplied using ReapplyBlob.
// Unlike PutBlob, the digest can not be empty.  If HasBlob returns true, the size of the blob must also be returned.
// If the destination does not contain the blob, or it is unknown, HasBlob ordinarily returns (false, -1, nil);
// it returns a non-nil error only on an unexpected failure.
func (d *dockerImageDestination) HasBlob(info types.BlobInfo) (bool, int64, error) {
	if info.Digest == "" {
		return false, -1, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}
	checkPath := fmt.Sprintf(blobsPath, reference.Path(d.ref.ref), info.Digest.String())

	logrus.Debugf("Checking %s", checkPath)
	res, err := d.c.makeRequest(context.TODO(), "HEAD", checkPath, nil, nil)
	if err != nil {
		return false, -1, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		logrus.Debugf("... already exists")
		return true, getBlobSize(res), nil
	case http.StatusUnauthorized:
		logrus.Debugf("... not authorized")
		return false, -1, errors.Errorf("not authorized to read from destination repository %s", reference.Path(d.ref.ref))
	case http.StatusNotFound:
		logrus.Debugf("... not present")
		return false, -1, nil
	default:
		return false, -1, errors.Errorf("failed to read from destination repository %s: %v", reference.Path(d.ref.ref), http.StatusText(res.StatusCode))
	}
}

func (d *dockerImageDestination) ReapplyBlob(info types.BlobInfo) (types.BlobInfo, error) {
	return info, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *dockerImageDestination) PutManifest(m []byte) error {
	digest, err := manifest.Digest(m)
	if err != nil {
		return err
	}
	d.manifestDigest = digest

	refTail, err := d.ref.tagOrDigest()
	if err != nil {
		return err
	}
	path := fmt.Sprintf(manifestPath, reference.Path(d.ref.ref), refTail)

	headers := map[string][]string{}
	mimeType := manifest.GuessMIMEType(m)
	if mimeType != "" {
		headers["Content-Type"] = []string{mimeType}
	}
	res, err := d.c.makeRequest(context.TODO(), "PUT", path, headers, bytes.NewReader(m))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if !successStatus(res.StatusCode) {
		err = errors.Wrapf(client.HandleErrorResponse(res), "Error uploading manifest to %s", path)
		if isManifestInvalidError(errors.Cause(err)) {
			err = types.ManifestTypeRejectedError{Err: err}
		}
		return err
	}
	return nil
}

// successStatus returns true if the argument is a successful HTTP response
// code (in the range 200 - 399 inclusive).
func successStatus(status int) bool {
	return status >= 200 && status <= 399
}

// isManifestInvalidError returns true iff err from client.HandleErrorReponse is a “manifest invalid” error.
func isManifestInvalidError(err error) bool {
	errors, ok := err.(errcode.Errors)
	if !ok || len(errors) == 0 {
		return false
	}
	ec, ok := errors[0].(errcode.ErrorCoder)
	if !ok {
		return false
	}
	// ErrorCodeManifestInvalid is returned by OpenShift with acceptschema2=false.
	// ErrorCodeTagInvalid is returned by docker/distribution (at least as of commit ec87e9b6971d831f0eff752ddb54fb64693e51cd)
	// when uploading to a tag (because it can’t find a matching tag inside the manifest)
	return ec.ErrorCode() == v2.ErrorCodeManifestInvalid || ec.ErrorCode() == v2.ErrorCodeTagInvalid
}

func (d *dockerImageDestination) PutSignatures(signatures [][]byte) error {
	// Do not fail if we don’t really need to support signatures.
	if len(signatures) == 0 {
		return nil
	}
	if err := d.c.detectProperties(context.TODO()); err != nil {
		return err
	}
	switch {
	case d.c.signatureBase != nil:
		return d.putSignaturesToLookaside(signatures)
	case d.c.supportsSignatures:
		return d.putSignaturesToAPIExtension(signatures)
	default:
		return errors.Errorf("X-Registry-Supports-Signatures extension not supported, and lookaside is not configured")
	}
}

// putSignaturesToLookaside implements PutSignatures() from the lookaside location configured in s.c.signatureBase,
// which is not nil.
func (d *dockerImageDestination) putSignaturesToLookaside(signatures [][]byte) error {
	// FIXME? This overwrites files one at a time, definitely not atomic.
	// A failure when updating signatures with a reordered copy could lose some of them.

	// Skip dealing with the manifest digest if not necessary.
	if len(signatures) == 0 {
		return nil
	}

	if d.manifestDigest.String() == "" {
		// This shouldn’t happen, ImageDestination users are required to call PutManifest before PutSignatures
		return errors.Errorf("Unknown manifest digest, can't add signatures")
	}

	// NOTE: Keep this in sync with docs/signature-protocols.md!
	for i, signature := range signatures {
		url := signatureStorageURL(d.c.signatureBase, d.manifestDigest, i)
		if url == nil {
			return errors.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
		}
		err := d.putOneSignature(url, signature)
		if err != nil {
			return err
		}
	}
	// Remove any other signatures, if present.
	// We stop at the first missing signature; if a previous deleting loop aborted
	// prematurely, this may not clean up all of them, but one missing signature
	// is enough for dockerImageSource to stop looking for other signatures, so that
	// is sufficient.
	for i := len(signatures); ; i++ {
		url := signatureStorageURL(d.c.signatureBase, d.manifestDigest, i)
		if url == nil {
			return errors.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
		}
		missing, err := d.c.deleteOneSignature(url)
		if err != nil {
			return err
		}
		if missing {
			break
		}
	}

	return nil
}

// putOneSignature stores one signature to url.
// NOTE: Keep this in sync with docs/signature-protocols.md!
func (d *dockerImageDestination) putOneSignature(url *url.URL, signature []byte) error {
	switch url.Scheme {
	case "file":
		logrus.Debugf("Writing to %s", url.Path)
		err := os.MkdirAll(filepath.Dir(url.Path), 0755)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(url.Path, signature, 0644)
		if err != nil {
			return err
		}
		return nil

	case "http", "https":
		return errors.Errorf("Writing directly to a %s sigstore %s is not supported. Configure a sigstore-staging: location", url.Scheme, url.String())
	default:
		return errors.Errorf("Unsupported scheme when writing signature to %s", url.String())
	}
}

// deleteOneSignature deletes a signature from url, if it exists.
// If it successfully determines that the signature does not exist, returns (true, nil)
// NOTE: Keep this in sync with docs/signature-protocols.md!
func (c *dockerClient) deleteOneSignature(url *url.URL) (missing bool, err error) {
	switch url.Scheme {
	case "file":
		logrus.Debugf("Deleting %s", url.Path)
		err := os.Remove(url.Path)
		if err != nil && os.IsNotExist(err) {
			return true, nil
		}
		return false, err

	case "http", "https":
		return false, errors.Errorf("Writing directly to a %s sigstore %s is not supported. Configure a sigstore-staging: location", url.Scheme, url.String())
	default:
		return false, errors.Errorf("Unsupported scheme when deleting signature from %s", url.String())
	}
}

// putSignaturesToAPIExtension implements PutSignatures() using the X-Registry-Supports-Signatures API extension.
func (d *dockerImageDestination) putSignaturesToAPIExtension(signatures [][]byte) error {
	// Skip dealing with the manifest digest, or reading the old state, if not necessary.
	if len(signatures) == 0 {
		return nil
	}

	if d.manifestDigest.String() == "" {
		// This shouldn’t happen, ImageDestination users are required to call PutManifest before PutSignatures
		return errors.Errorf("Unknown manifest digest, can't add signatures")
	}

	// Because image signatures are a shared resource in Atomic Registry, the default upload
	// always adds signatures.  Eventually we should also allow removing signatures,
	// but the X-Registry-Supports-Signatures API extension does not support that yet.

	existingSignatures, err := d.c.getExtensionsSignatures(context.TODO(), d.ref, d.manifestDigest)
	if err != nil {
		return err
	}
	existingSigNames := map[string]struct{}{}
	for _, sig := range existingSignatures.Signatures {
		existingSigNames[sig.Name] = struct{}{}
	}

sigExists:
	for _, newSig := range signatures {
		for _, existingSig := range existingSignatures.Signatures {
			if existingSig.Version == extensionSignatureSchemaVersion && existingSig.Type == extensionSignatureTypeAtomic && bytes.Equal(existingSig.Content, newSig) {
				continue sigExists
			}
		}

		// The API expect us to invent a new unique name. This is racy, but hopefully good enough.
		var signatureName string
		for {
			randBytes := make([]byte, 16)
			n, err := rand.Read(randBytes)
			if err != nil || n != 16 {
				return errors.Wrapf(err, "Error generating random signature len %d", n)
			}
			signatureName = fmt.Sprintf("%s@%032x", d.manifestDigest.String(), randBytes)
			if _, ok := existingSigNames[signatureName]; !ok {
				break
			}
		}
		sig := extensionSignature{
			Version: extensionSignatureSchemaVersion,
			Name:    signatureName,
			Type:    extensionSignatureTypeAtomic,
			Content: newSig,
		}
		body, err := json.Marshal(sig)
		if err != nil {
			return err
		}

		path := fmt.Sprintf(extensionsSignaturePath, reference.Path(d.ref.ref), d.manifestDigest.String())
		res, err := d.c.makeRequest(context.TODO(), "PUT", path, nil, bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			body, err := ioutil.ReadAll(res.Body)
			if err == nil {
				logrus.Debugf("Error body %s", string(body))
			}
			logrus.Debugf("Error uploading signature, status %d, %#v", res.StatusCode, res)
			return errors.Errorf("Error uploading signature to %s, status %d", path, res.StatusCode)
		}
	}

	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *dockerImageDestination) Commit() error {
	return nil
}
