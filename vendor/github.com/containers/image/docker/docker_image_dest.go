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
	"github.com/containers/image/pkg/blobinfocache"
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
func newImageDestination(sys *types.SystemContext, ref dockerReference) (types.ImageDestination, error) {
	c, err := newDockerClientFromRef(sys, ref, true, "pull,push")
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
func (d *dockerImageDestination) SupportsSignatures(ctx context.Context) error {
	if err := d.c.detectProperties(ctx); err != nil {
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

func (d *dockerImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.Compress
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

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *dockerImageDestination) IgnoresEmbeddedDockerReference() bool {
	return false // We do want the manifest updated; older registry versions refuse manifests if the embedded reference does not match.
}

// sizeCounter is an io.Writer which only counts the total size of its input.
type sizeCounter struct{ size int64 }

func (c *sizeCounter) Write(p []byte) (n int, err error) {
	c.size += int64(len(p))
	return len(p), nil
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *dockerImageDestination) HasThreadSafePutBlob() bool {
	return true
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *dockerImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	if inputInfo.Digest.String() != "" {
		// This should not really be necessary, at least the copy code calls TryReusingBlob automatically.
		// Still, we need to check, if only because the "initiate upload" endpoint does not have a documented "blob already exists" return value.
		// But we do that with NoCache, so that it _only_ checks the primary destination, instead of trying all mount candidates _again_.
		haveBlob, reusedInfo, err := d.TryReusingBlob(ctx, inputInfo, blobinfocache.NoCache, false)
		if err != nil {
			return types.BlobInfo{}, err
		}
		if haveBlob {
			return reusedInfo, nil
		}
	}

	// FIXME? Chunked upload, progress reporting, etc.
	uploadPath := fmt.Sprintf(blobUploadPath, reference.Path(d.ref.ref))
	logrus.Debugf("Uploading %s", uploadPath)
	res, err := d.c.makeRequest(ctx, "POST", uploadPath, nil, nil, v2Auth, nil)
	if err != nil {
		return types.BlobInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		logrus.Debugf("Error initiating layer upload, response %#v", *res)
		return types.BlobInfo{}, errors.Wrapf(client.HandleErrorResponse(res), "Error initiating layer upload to %s in %s", uploadPath, d.c.registry)
	}
	uploadLocation, err := res.Location()
	if err != nil {
		return types.BlobInfo{}, errors.Wrap(err, "Error determining upload URL")
	}

	digester := digest.Canonical.Digester()
	sizeCounter := &sizeCounter{}
	tee := io.TeeReader(stream, io.MultiWriter(digester.Hash(), sizeCounter))
	res, err = d.c.makeRequestToResolvedURL(ctx, "PATCH", uploadLocation.String(), map[string][]string{"Content-Type": {"application/octet-stream"}}, tee, inputInfo.Size, v2Auth, nil)
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

	// FIXME: DELETE uploadLocation on failure (does not really work in docker/distribution servers, which incorrectly require the "delete" action in the token's scope)

	locationQuery := uploadLocation.Query()
	// TODO: check inputInfo.Digest == computedDigest https://github.com/containers/image/pull/70#discussion_r77646717
	locationQuery.Set("digest", computedDigest.String())
	uploadLocation.RawQuery = locationQuery.Encode()
	res, err = d.c.makeRequestToResolvedURL(ctx, "PUT", uploadLocation.String(), map[string][]string{"Content-Type": {"application/octet-stream"}}, nil, -1, v2Auth, nil)
	if err != nil {
		return types.BlobInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		logrus.Debugf("Error uploading layer, response %#v", *res)
		return types.BlobInfo{}, errors.Wrapf(client.HandleErrorResponse(res), "Error uploading layer to %s", uploadLocation)
	}

	logrus.Debugf("Upload of layer %s complete", computedDigest)
	cache.RecordKnownLocation(d.ref.Transport(), bicTransportScope(d.ref), computedDigest, newBICLocationReference(d.ref))
	return types.BlobInfo{Digest: computedDigest, Size: sizeCounter.size}, nil
}

// blobExists returns true iff repo contains a blob with digest, and if so, also its size.
// If the destination does not contain the blob, or it is unknown, blobExists ordinarily returns (false, -1, nil);
// it returns a non-nil error only on an unexpected failure.
func (d *dockerImageDestination) blobExists(ctx context.Context, repo reference.Named, digest digest.Digest, extraScope *authScope) (bool, int64, error) {
	checkPath := fmt.Sprintf(blobsPath, reference.Path(repo), digest.String())
	logrus.Debugf("Checking %s", checkPath)
	res, err := d.c.makeRequest(ctx, "HEAD", checkPath, nil, nil, v2Auth, extraScope)
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
		return false, -1, errors.Wrapf(client.HandleErrorResponse(res), "Error checking whether a blob %s exists in %s", digest, repo.Name())
	case http.StatusNotFound:
		logrus.Debugf("... not present")
		return false, -1, nil
	default:
		return false, -1, errors.Errorf("failed to read from destination repository %s: %d (%s)", reference.Path(d.ref.ref), res.StatusCode, http.StatusText(res.StatusCode))
	}
}

// mountBlob tries to mount blob srcDigest from srcRepo to the current destination.
func (d *dockerImageDestination) mountBlob(ctx context.Context, srcRepo reference.Named, srcDigest digest.Digest, extraScope *authScope) error {
	u := url.URL{
		Path: fmt.Sprintf(blobUploadPath, reference.Path(d.ref.ref)),
		RawQuery: url.Values{
			"mount": {srcDigest.String()},
			"from":  {reference.Path(srcRepo)},
		}.Encode(),
	}
	mountPath := u.String()
	logrus.Debugf("Trying to mount %s", mountPath)
	res, err := d.c.makeRequest(ctx, "POST", mountPath, nil, nil, v2Auth, extraScope)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusCreated:
		logrus.Debugf("... mount OK")
		return nil
	case http.StatusAccepted:
		// Oops, the mount was ignored - either the registry does not support that yet, or the blob does not exist; the registry has started an ordinary upload process.
		// Abort, and let the ultimate caller do an upload when its ready, instead.
		// NOTE: This does not really work in docker/distribution servers, which incorrectly require the "delete" action in the token's scope, and is thus entirely untested.
		uploadLocation, err := res.Location()
		if err != nil {
			return errors.Wrap(err, "Error determining upload URL after a mount attempt")
		}
		logrus.Debugf("... started an upload instead of mounting, trying to cancel at %s", uploadLocation.String())
		res2, err := d.c.makeRequestToResolvedURL(ctx, "DELETE", uploadLocation.String(), nil, nil, -1, v2Auth, extraScope)
		if err != nil {
			logrus.Debugf("Error trying to cancel an inadvertent upload: %s", err)
		} else {
			defer res2.Body.Close()
			if res2.StatusCode != http.StatusNoContent {
				logrus.Debugf("Error trying to cancel an inadvertent upload, status %s", http.StatusText(res.StatusCode))
			}
		}
		// Anyway, if canceling the upload fails, ignore it and return the more important error:
		return fmt.Errorf("Mounting %s from %s to %s started an upload instead", srcDigest, srcRepo.Name(), d.ref.ref.Name())
	default:
		logrus.Debugf("Error mounting, response %#v", *res)
		return errors.Wrapf(client.HandleErrorResponse(res), "Error mounting %s from %s to %s", srcDigest, srcRepo.Name(), d.ref.ref.Name())
	}
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been succesfully reused, returns (true, info, nil); info must contain at least a digest and size.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *dockerImageDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if info.Digest == "" {
		return false, types.BlobInfo{}, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}

	// First, check whether the blob happens to already exist at the destination.
	exists, size, err := d.blobExists(ctx, d.ref.ref, info.Digest, nil)
	if err != nil {
		return false, types.BlobInfo{}, err
	}
	if exists {
		cache.RecordKnownLocation(d.ref.Transport(), bicTransportScope(d.ref), info.Digest, newBICLocationReference(d.ref))
		return true, types.BlobInfo{Digest: info.Digest, Size: size}, nil
	}

	// Then try reusing blobs from other locations.
	for _, candidate := range cache.CandidateLocations(d.ref.Transport(), bicTransportScope(d.ref), info.Digest, canSubstitute) {
		candidateRepo, err := parseBICLocationReference(candidate.Location)
		if err != nil {
			logrus.Debugf("Error parsing BlobInfoCache location reference: %s", err)
			continue
		}
		logrus.Debugf("Trying to reuse cached location %s in %s", candidate.Digest.String(), candidateRepo.Name())

		// Sanity checks:
		if reference.Domain(candidateRepo) != reference.Domain(d.ref.ref) {
			logrus.Debugf("... Internal error: domain %s does not match destination %s", reference.Domain(candidateRepo), reference.Domain(d.ref.ref))
			continue
		}
		if candidateRepo.Name() == d.ref.ref.Name() && candidate.Digest == info.Digest {
			logrus.Debug("... Already tried the primary destination")
			continue
		}

		// Whatever happens here, don't abort the entire operation.  It's likely we just don't have permissions, and if it is a critical network error, we will find out soon enough anyway.

		// Checking candidateRepo, and mounting from it, requires an
		// expanded token scope.
		extraScope := &authScope{
			remoteName: reference.Path(candidateRepo),
			actions:    "pull",
		}
		// This existence check is not, strictly speaking, necessary: We only _really_ need it to get the blob size, and we could record that in the cache instead.
		// But a "failed" d.mountBlob currently leaves around an unterminated server-side upload, which we would try to cancel.
		// So, without this existence check, it would be 1 request on success, 2 requests on failure; with it, it is 2 requests on success, 1 request on failure.
		// On success we avoid the actual costly upload; so, in a sense, the success case is "free", but failures are always costly.
		// Even worse, docker/distribution does not actually reasonably implement canceling uploads
		// (it would require a "delete" action in the token, and Quay does not give that to anyone, so we can't ask);
		// so, be a nice client and don't create unnecesary upload sessions on the server.
		exists, size, err := d.blobExists(ctx, candidateRepo, candidate.Digest, extraScope)
		if err != nil {
			logrus.Debugf("... Failed: %v", err)
			continue
		}
		if !exists {
			// FIXME? Should we drop the blob from cache here (and elsewhere?)?
			continue // logrus.Debug() already happened in blobExists
		}
		if candidateRepo.Name() != d.ref.ref.Name() {
			if err := d.mountBlob(ctx, candidateRepo, candidate.Digest, extraScope); err != nil {
				logrus.Debugf("... Mount failed: %v", err)
				continue
			}
		}
		cache.RecordKnownLocation(d.ref.Transport(), bicTransportScope(d.ref), candidate.Digest, newBICLocationReference(d.ref))
		return true, types.BlobInfo{Digest: candidate.Digest, Size: size}, nil
	}

	return false, types.BlobInfo{}, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *dockerImageDestination) PutManifest(ctx context.Context, m []byte) error {
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
	res, err := d.c.makeRequest(ctx, "PUT", path, headers, bytes.NewReader(m), v2Auth, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if !successStatus(res.StatusCode) {
		err = errors.Wrapf(client.HandleErrorResponse(res), "Error uploading manifest %s to %s", refTail, d.ref.ref.Name())
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

func (d *dockerImageDestination) PutSignatures(ctx context.Context, signatures [][]byte) error {
	// Do not fail if we don’t really need to support signatures.
	if len(signatures) == 0 {
		return nil
	}
	if err := d.c.detectProperties(ctx); err != nil {
		return err
	}
	switch {
	case d.c.signatureBase != nil:
		return d.putSignaturesToLookaside(signatures)
	case d.c.supportsSignatures:
		return d.putSignaturesToAPIExtension(ctx, signatures)
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
func (d *dockerImageDestination) putSignaturesToAPIExtension(ctx context.Context, signatures [][]byte) error {
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

	existingSignatures, err := d.c.getExtensionsSignatures(ctx, d.ref, d.manifestDigest)
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
		res, err := d.c.makeRequest(ctx, "PUT", path, nil, bytes.NewReader(body), v2Auth, nil)
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
			return errors.Wrapf(client.HandleErrorResponse(res), "Error uploading signature to %s in %s", path, d.c.registry)
		}
	}

	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *dockerImageDestination) Commit(ctx context.Context) error {
	return nil
}
