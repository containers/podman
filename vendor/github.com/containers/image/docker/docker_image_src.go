package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/pkg/sysregistriesv2"
	"github.com/containers/image/types"
	"github.com/docker/distribution/registry/client"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type dockerImageSource struct {
	ref dockerReference
	c   *dockerClient
	// State
	cachedManifest         []byte // nil if not loaded yet
	cachedManifestMIMEType string // Only valid if cachedManifest != nil
}

// newImageSource creates a new `ImageSource` for the specified image reference
// `ref`.
//
// The following steps will be done during the instance creation:
//
// - Lookup the registry within the configured location in
//   `sys.SystemRegistriesConfPath`. If there is no configured registry available,
//   we fallback to the provided docker reference `ref`.
//
// - References which contain a configured prefix will be automatically rewritten
//   to the correct target reference. For example, if the configured
//   `prefix = "example.com/foo"`, `location = "example.com"` and the image will be
//   pulled from the ref `example.com/foo/image`, then the resulting pull will
//   effectively point to `example.com/image`.
//
// - If the rewritten reference succeeds, it will be used as the `dockerRef`
//   in the client. If the rewrite fails, the function immediately returns an error.
//
// - Each mirror will be used (in the configured order) to test the
//   availability of the image manifest on the remote location. For example,
//   if the manifest is not reachable due to connectivity issues, then the next
//   mirror will be tested instead. If no mirror is configured or contains the
//   target manifest, then the initial `ref` will be tested as fallback. The
//   creation of the new `dockerImageSource` only succeeds if a remote
//   location with the available manifest was found.
//
// A cleanup call to `.Close()` is needed if the caller is done using the returned
// `ImageSource`.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref dockerReference) (*dockerImageSource, error) {
	registry, err := sysregistriesv2.FindRegistry(sys, ref.ref.Name())
	if err != nil {
		return nil, errors.Wrapf(err, "error loading registries configuration")
	}

	if registry == nil {
		// No configuration was found for the provided reference, so we create
		// a fallback registry by hand to make the client creation below work
		// as intended.
		registry = &sysregistriesv2.Registry{
			Endpoint: sysregistriesv2.Endpoint{
				Location: ref.ref.String(),
			},
		}
	}

	// Found the registry within the sysregistriesv2 configuration. Now we test
	// all endpoints for the manifest availability. If a working image source
	// was found, it will be used for all future pull actions.
	var (
		imageSource     *dockerImageSource
		manifestLoadErr error
	)
	for _, endpoint := range append(registry.Mirrors, registry.Endpoint) {
		logrus.Debugf("Trying to pull %q from endpoint %q", ref.ref, endpoint.Location)

		newRef, err := endpoint.RewriteReference(ref.ref, registry.Prefix)
		if err != nil {
			return nil, err
		}
		dockerRef, err := newReference(newRef)
		if err != nil {
			return nil, err
		}

		client, err := newDockerClientFromRef(sys, dockerRef, false, "pull")
		if err != nil {
			return nil, err
		}
		client.tlsClientConfig.InsecureSkipVerify = endpoint.Insecure

		testImageSource := &dockerImageSource{
			ref: dockerRef,
			c:   client,
		}

		manifestLoadErr = testImageSource.ensureManifestIsLoaded(ctx)
		if manifestLoadErr == nil {
			imageSource = testImageSource
			break
		}
	}

	return imageSource, manifestLoadErr
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dockerImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dockerImageSource) Close() error {
	return nil
}

// LayerInfosForCopy() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *dockerImageSource) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
	return nil, nil
}

// simplifyContentType drops parameters from a HTTP media type (see https://tools.ietf.org/html/rfc7231#section-3.1.1.1)
// Alternatively, an empty string is returned unchanged, and invalid values are "simplified" to an empty string.
func simplifyContentType(contentType string) string {
	if contentType == "" {
		return contentType
	}
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mimeType
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *dockerImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return s.fetchManifest(ctx, instanceDigest.String())
	}
	err := s.ensureManifestIsLoaded(ctx)
	if err != nil {
		return nil, "", err
	}
	return s.cachedManifest, s.cachedManifestMIMEType, nil
}

func (s *dockerImageSource) fetchManifest(ctx context.Context, tagOrDigest string) ([]byte, string, error) {
	path := fmt.Sprintf(manifestPath, reference.Path(s.ref.ref), tagOrDigest)
	headers := make(map[string][]string)
	headers["Accept"] = manifest.DefaultRequestedManifestMIMETypes
	res, err := s.c.makeRequest(ctx, "GET", path, headers, nil, v2Auth, nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", errors.Wrapf(client.HandleErrorResponse(res), "Error reading manifest %s in %s", tagOrDigest, s.ref.ref.Name())
	}
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return manblob, simplifyContentType(res.Header.Get("Content-Type")), nil
}

// ensureManifestIsLoaded sets s.cachedManifest and s.cachedManifestMIMEType
//
// ImageSource implementations are not required or expected to do any caching,
// but because our signatures are “attached” to the manifest digest,
// we need to ensure that the digest of the manifest returned by GetManifest(ctx, nil)
// and used by GetSignatures(ctx, nil) are consistent, otherwise we would get spurious
// signature verification failures when pulling while a tag is being updated.
func (s *dockerImageSource) ensureManifestIsLoaded(ctx context.Context) error {
	if s.cachedManifest != nil {
		return nil
	}

	reference, err := s.ref.tagOrDigest()
	if err != nil {
		return err
	}

	manblob, mt, err := s.fetchManifest(ctx, reference)
	if err != nil {
		return err
	}
	// We might validate manblob against the Docker-Content-Digest header here to protect against transport errors.
	s.cachedManifest = manblob
	s.cachedManifestMIMEType = mt
	return nil
}

func (s *dockerImageSource) getExternalBlob(ctx context.Context, urls []string) (io.ReadCloser, int64, error) {
	var (
		resp *http.Response
		err  error
	)
	for _, url := range urls {
		resp, err = s.c.makeRequestToResolvedURL(ctx, "GET", url, nil, nil, -1, noAuth, nil)
		if err == nil {
			if resp.StatusCode != http.StatusOK {
				err = errors.Errorf("error fetching external blob from %q: %d (%s)", url, resp.StatusCode, http.StatusText(resp.StatusCode))
				logrus.Debug(err)
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, getBlobSize(resp), nil
}

func getBlobSize(resp *http.Response) int64 {
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return size
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *dockerImageSource) HasThreadSafeGetBlob() bool {
	return true
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *dockerImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		return s.getExternalBlob(ctx, info.URLs)
	}

	path := fmt.Sprintf(blobsPath, reference.Path(s.ref.ref), info.Digest.String())
	logrus.Debugf("Downloading %s", path)
	res, err := s.c.makeRequest(ctx, "GET", path, nil, nil, v2Auth, nil)
	if err != nil {
		return nil, 0, err
	}
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, 0, errors.Errorf("Invalid status code returned when fetching blob %d (%s)", res.StatusCode, http.StatusText(res.StatusCode))
	}
	cache.RecordKnownLocation(s.ref.Transport(), bicTransportScope(s.ref), info.Digest, newBICLocationReference(s.ref))
	return res.Body, getBlobSize(res), nil
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *dockerImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if err := s.c.detectProperties(ctx); err != nil {
		return nil, err
	}
	switch {
	case s.c.signatureBase != nil:
		return s.getSignaturesFromLookaside(ctx, instanceDigest)
	case s.c.supportsSignatures:
		return s.getSignaturesFromAPIExtension(ctx, instanceDigest)
	default:
		return [][]byte{}, nil
	}
}

// manifestDigest returns a digest of the manifest, from instanceDigest if non-nil; or from the supplied reference,
// or finally, from a fetched manifest.
func (s *dockerImageSource) manifestDigest(ctx context.Context, instanceDigest *digest.Digest) (digest.Digest, error) {
	if instanceDigest != nil {
		return *instanceDigest, nil
	}
	if digested, ok := s.ref.ref.(reference.Digested); ok {
		d := digested.Digest()
		if d.Algorithm() == digest.Canonical {
			return d, nil
		}
	}
	if err := s.ensureManifestIsLoaded(ctx); err != nil {
		return "", err
	}
	return manifest.Digest(s.cachedManifest)
}

// getSignaturesFromLookaside implements GetSignatures() from the lookaside location configured in s.c.signatureBase,
// which is not nil.
func (s *dockerImageSource) getSignaturesFromLookaside(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}

	// NOTE: Keep this in sync with docs/signature-protocols.md!
	signatures := [][]byte{}
	for i := 0; ; i++ {
		url := signatureStorageURL(s.c.signatureBase, manifestDigest, i)
		if url == nil {
			return nil, errors.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
		}
		signature, missing, err := s.getOneSignature(ctx, url)
		if err != nil {
			return nil, err
		}
		if missing {
			break
		}
		signatures = append(signatures, signature)
	}
	return signatures, nil
}

// getOneSignature downloads one signature from url.
// If it successfully determines that the signature does not exist, returns with missing set to true and error set to nil.
// NOTE: Keep this in sync with docs/signature-protocols.md!
func (s *dockerImageSource) getOneSignature(ctx context.Context, url *url.URL) (signature []byte, missing bool, err error) {
	switch url.Scheme {
	case "file":
		logrus.Debugf("Reading %s", url.Path)
		sig, err := ioutil.ReadFile(url.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, nil
			}
			return nil, false, err
		}
		return sig, false, nil

	case "http", "https":
		logrus.Debugf("GET %s", url)
		req, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			return nil, false, err
		}
		req = req.WithContext(ctx)
		res, err := s.c.doHTTP(req)
		if err != nil {
			return nil, false, err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return nil, true, nil
		} else if res.StatusCode != http.StatusOK {
			return nil, false, errors.Errorf("Error reading signature from %s: status %d (%s)", url.String(), res.StatusCode, http.StatusText(res.StatusCode))
		}
		sig, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, false, err
		}
		return sig, false, nil

	default:
		return nil, false, errors.Errorf("Unsupported scheme when reading signature from %s", url.String())
	}
}

// getSignaturesFromAPIExtension implements GetSignatures() using the X-Registry-Supports-Signatures API extension.
func (s *dockerImageSource) getSignaturesFromAPIExtension(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}

	parsedBody, err := s.c.getExtensionsSignatures(ctx, s.ref, manifestDigest)
	if err != nil {
		return nil, err
	}

	var sigs [][]byte
	for _, sig := range parsedBody.Signatures {
		if sig.Version == extensionSignatureSchemaVersion && sig.Type == extensionSignatureTypeAtomic {
			sigs = append(sigs, sig.Content)
		}
	}
	return sigs, nil
}

// deleteImage deletes the named image from the registry, if supported.
func deleteImage(ctx context.Context, sys *types.SystemContext, ref dockerReference) error {
	// docker/distribution does not document what action should be used for deleting images.
	//
	// Current docker/distribution requires "pull" for reading the manifest and "delete" for deleting it.
	// quay.io requires "push" (an explicit "pull" is unnecessary), does not grant any token (fails parsing the request) if "delete" is included.
	// OpenShift ignores the action string (both the password and the token is an OpenShift API token identifying a user).
	//
	// We have to hard-code a single string, luckily both docker/distribution and quay.io support "*" to mean "everything".
	c, err := newDockerClientFromRef(sys, ref, true, "*")
	if err != nil {
		return err
	}

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	headers := make(map[string][]string)
	headers["Accept"] = []string{manifest.DockerV2Schema2MediaType}

	refTail, err := ref.tagOrDigest()
	if err != nil {
		return err
	}
	getPath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), refTail)
	get, err := c.makeRequest(ctx, "GET", getPath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer get.Body.Close()
	manifestBody, err := ioutil.ReadAll(get.Body)
	if err != nil {
		return err
	}
	switch get.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return errors.Errorf("Unable to delete %v. Image may not exist or is not stored with a v2 Schema in a v2 registry", ref.ref)
	default:
		return errors.Errorf("Failed to delete %v: %s (%v)", ref.ref, manifestBody, get.Status)
	}

	digest := get.Header.Get("Docker-Content-Digest")
	deletePath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), digest)

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	delete, err := c.makeRequest(ctx, "DELETE", deletePath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer delete.Body.Close()

	body, err := ioutil.ReadAll(delete.Body)
	if err != nil {
		return err
	}
	if delete.StatusCode != http.StatusAccepted {
		return errors.Errorf("Failed to delete %v: %s (%v)", deletePath, string(body), delete.Status)
	}

	if c.signatureBase != nil {
		manifestDigest, err := manifest.Digest(manifestBody)
		if err != nil {
			return err
		}

		for i := 0; ; i++ {
			url := signatureStorageURL(c.signatureBase, manifestDigest, i)
			if url == nil {
				return errors.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
			}
			missing, err := c.deleteOneSignature(url)
			if err != nil {
				return err
			}
			if missing {
				break
			}
		}
	}

	return nil
}
