package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/imagesource/impl"
	"github.com/containers/image/v5/internal/imagesource/stubs"
	"github.com/containers/image/v5/internal/iolimits"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type dockerImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	impl.DoesNotAffectLayerInfosForCopy
	stubs.ImplementsGetBlobAt

	logicalRef  dockerReference // The reference the user requested.
	physicalRef dockerReference // The actual reference we are accessing (possibly a mirror)
	c           *dockerClient
	// State
	cachedManifest         []byte // nil if not loaded yet
	cachedManifestMIMEType string // Only valid if cachedManifest != nil
}

// newImageSource creates a new ImageSource for the specified image reference.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref dockerReference) (*dockerImageSource, error) {
	registryConfig, err := loadRegistryConfiguration(sys)
	if err != nil {
		return nil, err
	}
	registry, err := sysregistriesv2.FindRegistry(sys, ref.ref.Name())
	if err != nil {
		return nil, fmt.Errorf("loading registries configuration: %w", err)
	}
	if registry == nil {
		// No configuration was found for the provided reference, so use the
		// equivalent of a default configuration.
		registry = &sysregistriesv2.Registry{
			Endpoint: sysregistriesv2.Endpoint{
				Location: ref.ref.String(),
			},
			Prefix: ref.ref.String(),
		}
	}

	// Check all endpoints for the manifest availability. If we find one that does
	// contain the image, it will be used for all future pull actions.  Always try the
	// non-mirror original location last; this both transparently handles the case
	// of no mirrors configured, and ensures we return the error encountered when
	// accessing the upstream location if all endpoints fail.
	pullSources, err := registry.PullSourcesFromReference(ref.ref)
	if err != nil {
		return nil, err
	}
	type attempt struct {
		ref reference.Named
		err error
	}
	attempts := []attempt{}
	for _, pullSource := range pullSources {
		if sys != nil && sys.DockerLogMirrorChoice {
			logrus.Infof("Trying to access %q", pullSource.Reference)
		} else {
			logrus.Debugf("Trying to access %q", pullSource.Reference)
		}
		s, err := newImageSourceAttempt(ctx, sys, ref, pullSource, registryConfig)
		if err == nil {
			return s, nil
		}
		logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, err)
		attempts = append(attempts, attempt{
			ref: pullSource.Reference,
			err: err,
		})
	}
	switch len(attempts) {
	case 0:
		return nil, errors.New("Internal error: newImageSource returned without trying any endpoint")
	case 1:
		return nil, attempts[0].err // If no mirrors are used, perfectly preserve the error type and add no noise.
	default:
		// Don’t just build a string, try to preserve the typed error.
		primary := &attempts[len(attempts)-1]
		extras := []string{}
		for i := 0; i < len(attempts)-1; i++ {
			// This is difficult to fit into a single-line string, when the error can contain arbitrary strings including any metacharacters we decide to use.
			// The paired [] at least have some chance of being unambiguous.
			extras = append(extras, fmt.Sprintf("[%s: %v]", attempts[i].ref.String(), attempts[i].err))
		}
		return nil, fmt.Errorf("(Mirrors also failed: %s): %s: %w", strings.Join(extras, "\n"), primary.ref.String(), primary.err)
	}
}

// newImageSourceAttempt is an internal helper for newImageSource. Everyone else must call newImageSource.
// Given a logicalReference and a pullSource, return a dockerImageSource if it is reachable.
// The caller must call .Close() on the returned ImageSource.
func newImageSourceAttempt(ctx context.Context, sys *types.SystemContext, logicalRef dockerReference, pullSource sysregistriesv2.PullSource,
	registryConfig *registryConfiguration) (*dockerImageSource, error) {
	physicalRef, err := newReference(pullSource.Reference)
	if err != nil {
		return nil, err
	}

	endpointSys := sys
	// sys.DockerAuthConfig does not explicitly specify a registry; we must not blindly send the credentials intended for the primary endpoint to mirrors.
	if endpointSys != nil && endpointSys.DockerAuthConfig != nil && reference.Domain(physicalRef.ref) != reference.Domain(logicalRef.ref) {
		copy := *endpointSys
		copy.DockerAuthConfig = nil
		copy.DockerBearerRegistryToken = ""
		endpointSys = &copy
	}

	client, err := newDockerClientFromRef(endpointSys, physicalRef, registryConfig, false, "pull")
	if err != nil {
		return nil, err
	}
	client.tlsClientConfig.InsecureSkipVerify = pullSource.Endpoint.Insecure

	s := &dockerImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: true,
		}),

		logicalRef:  logicalRef,
		physicalRef: physicalRef,
		c:           client,
	}
	s.Compat = impl.AddCompat(s)

	if err := s.ensureManifestIsLoaded(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dockerImageSource) Reference() types.ImageReference {
	return s.logicalRef
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dockerImageSource) Close() error {
	return nil
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
	return s.c.fetchManifest(ctx, s.physicalRef, tagOrDigest)
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

	reference, err := s.physicalRef.tagOrDigest()
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

// splitHTTP200ResponseToPartial splits a 200 response in multiple streams as specified by the chunks
func splitHTTP200ResponseToPartial(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []private.ImageSourceChunk) {
	defer close(streams)
	defer close(errs)
	currentOffset := uint64(0)

	body = makeBufferedNetworkReader(body, 64, 16384)
	defer body.Close()
	for _, c := range chunks {
		if c.Offset != currentOffset {
			if c.Offset < currentOffset {
				errs <- fmt.Errorf("invalid chunk offset specified %v (expected >= %v)", c.Offset, currentOffset)
				break
			}
			toSkip := c.Offset - currentOffset
			if _, err := io.Copy(io.Discard, io.LimitReader(body, int64(toSkip))); err != nil {
				errs <- err
				break
			}
			currentOffset += toSkip
		}
		s := signalCloseReader{
			closed:        make(chan interface{}),
			stream:        io.NopCloser(io.LimitReader(body, int64(c.Length))),
			consumeStream: true,
		}
		streams <- s

		// Wait until the stream is closed before going to the next chunk
		<-s.closed
		currentOffset += c.Length
	}
}

// handle206Response reads a 206 response and send each part as a separate ReadCloser to the streams chan.
func handle206Response(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []private.ImageSourceChunk, mediaType string, params map[string]string) {
	defer close(streams)
	defer close(errs)
	if !strings.HasPrefix(mediaType, "multipart/") {
		streams <- body
		return
	}
	boundary, found := params["boundary"]
	if !found {
		errs <- errors.New("could not find boundary")
		body.Close()
		return
	}
	buffered := makeBufferedNetworkReader(body, 64, 16384)
	defer buffered.Close()
	mr := multipart.NewReader(buffered, boundary)
	parts := 0
	for {
		p, err := mr.NextPart()
		if err != nil {
			if err != io.EOF {
				errs <- err
			}
			if parts != len(chunks) {
				errs <- errors.New("invalid number of chunks returned by the server")
			}
			return
		}
		s := signalCloseReader{
			closed: make(chan interface{}),
			stream: p,
		}
		streams <- s
		// NextPart() cannot be called while the current part
		// is being read, so wait until it is closed
		<-s.closed
		parts++
	}
}

var multipartByteRangesRe = regexp.MustCompile("multipart/byteranges; boundary=([A-Za-z-0-9:]+)")

func parseMediaType(contentType string) (string, map[string]string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		if err == mime.ErrInvalidMediaParameter {
			// CloudFront returns an invalid MIME type, that contains an unquoted ":" in the boundary
			// param, let's handle it here.
			matches := multipartByteRangesRe.FindStringSubmatch(contentType)
			if len(matches) == 2 {
				mediaType = "multipart/byteranges"
				params = map[string]string{
					"boundary": matches[1],
				}
				err = nil
			}
		}
		if err != nil {
			return "", nil, err
		}
	}
	return mediaType, params, err
}

// GetBlobAt returns a sequential channel of readers that contain data for the requested
// blob chunks, and a channel that might get a single error value.
// The specified chunks must be not overlapping and sorted by their offset.
// The readers must be fully consumed, in the order they are returned, before blocking
// to read the next chunk.
func (s *dockerImageSource) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	headers := make(map[string][]string)

	var rangeVals []string
	for _, c := range chunks {
		rangeVals = append(rangeVals, fmt.Sprintf("%d-%d", c.Offset, c.Offset+c.Length-1))
	}

	headers["Range"] = []string{fmt.Sprintf("bytes=%s", strings.Join(rangeVals, ","))}

	if len(info.URLs) != 0 {
		return nil, nil, fmt.Errorf("external URLs not supported with GetBlobAt")
	}

	path := fmt.Sprintf(blobsPath, reference.Path(s.physicalRef.ref), info.Digest.String())
	logrus.Debugf("Downloading %s", path)
	res, err := s.c.makeRequest(ctx, http.MethodGet, path, headers, nil, v2Auth, nil)
	if err != nil {
		return nil, nil, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		// if the server replied with a 200 status code, convert the full body response to a series of
		// streams as it would have been done with 206.
		streams := make(chan io.ReadCloser)
		errs := make(chan error)
		go splitHTTP200ResponseToPartial(streams, errs, res.Body, chunks)
		return streams, errs, nil
	case http.StatusPartialContent:
		mediaType, params, err := parseMediaType(res.Header.Get("Content-Type"))
		if err != nil {
			return nil, nil, err
		}

		streams := make(chan io.ReadCloser)
		errs := make(chan error)

		go handle206Response(streams, errs, res.Body, chunks, mediaType, params)
		return streams, errs, nil
	case http.StatusBadRequest:
		res.Body.Close()
		return nil, nil, private.BadPartialRequestError{Status: res.Status}
	default:
		err := httpResponseToError(res, "Error fetching partial blob")
		if err == nil {
			err = fmt.Errorf("invalid status code returned when fetching blob %d (%s)", res.StatusCode, http.StatusText(res.StatusCode))
		}
		res.Body.Close()
		return nil, nil, err
	}
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *dockerImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	return s.c.getBlob(ctx, s.physicalRef, info, cache)
}

// GetSignaturesWithFormat returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *dockerImageSource) GetSignaturesWithFormat(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	if err := s.c.detectProperties(ctx); err != nil {
		return nil, err
	}
	var res []signature.Signature
	switch {
	case s.c.supportsSignatures:
		sigs, err := s.getSignaturesFromAPIExtension(ctx, instanceDigest)
		if err != nil {
			return nil, err
		}
		res = append(res, sigs...)
	case s.c.signatureBase != nil:
		sigs, err := s.getSignaturesFromLookaside(ctx, instanceDigest)
		if err != nil {
			return nil, err
		}
		res = append(res, sigs...)
	default:
		return nil, errors.New("Internal error: X-Registry-Supports-Signatures extension not supported, and lookaside should not be empty configuration")
	}

	sigstoreSigs, err := s.getSignaturesFromSigstoreAttachments(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}
	res = append(res, sigstoreSigs...)
	return res, nil
}

// manifestDigest returns a digest of the manifest, from instanceDigest if non-nil; or from the supplied reference,
// or finally, from a fetched manifest.
func (s *dockerImageSource) manifestDigest(ctx context.Context, instanceDigest *digest.Digest) (digest.Digest, error) {
	if instanceDigest != nil {
		return *instanceDigest, nil
	}
	if digested, ok := s.physicalRef.ref.(reference.Digested); ok {
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

// getSignaturesFromLookaside implements GetSignaturesWithFormat() from the lookaside location configured in s.c.signatureBase,
// which is not nil.
func (s *dockerImageSource) getSignaturesFromLookaside(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}

	// NOTE: Keep this in sync with docs/signature-protocols.md!
	signatures := []signature.Signature{}
	for i := 0; ; i++ {
		url := lookasideStorageURL(s.c.signatureBase, manifestDigest, i)
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

// getOneSignature downloads one signature from url, and returns (signature, false, nil)
// If it successfully determines that the signature does not exist, returns (nil, true, nil).
// NOTE: Keep this in sync with docs/signature-protocols.md!
func (s *dockerImageSource) getOneSignature(ctx context.Context, url *url.URL) (signature.Signature, bool, error) {
	switch url.Scheme {
	case "file":
		logrus.Debugf("Reading %s", url.Path)
		sigBlob, err := os.ReadFile(url.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, nil
			}
			return nil, false, err
		}
		sig, err := signature.FromBlob(sigBlob)
		if err != nil {
			return nil, false, fmt.Errorf("parsing signature %q: %w", url.Path, err)
		}
		return sig, false, nil

	case "http", "https":
		logrus.Debugf("GET %s", url.Redacted())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
		if err != nil {
			return nil, false, err
		}
		res, err := s.c.client.Do(req)
		if err != nil {
			return nil, false, err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return nil, true, nil
		} else if res.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("reading signature from %s: status %d (%s)", url.Redacted(), res.StatusCode, http.StatusText(res.StatusCode))
		}
		sigBlob, err := iolimits.ReadAtMost(res.Body, iolimits.MaxSignatureBodySize)
		if err != nil {
			return nil, false, err
		}
		sig, err := signature.FromBlob(sigBlob)
		if err != nil {
			return nil, false, fmt.Errorf("parsing signature %s: %w", url.Redacted(), err)
		}
		return sig, false, nil

	default:
		return nil, false, fmt.Errorf("Unsupported scheme when reading signature from %s", url.Redacted())
	}
}

// getSignaturesFromAPIExtension implements GetSignaturesWithFormat() using the X-Registry-Supports-Signatures API extension.
func (s *dockerImageSource) getSignaturesFromAPIExtension(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}

	parsedBody, err := s.c.getExtensionsSignatures(ctx, s.physicalRef, manifestDigest)
	if err != nil {
		return nil, err
	}

	var sigs []signature.Signature
	for _, sig := range parsedBody.Signatures {
		if sig.Version == extensionSignatureSchemaVersion && sig.Type == extensionSignatureTypeAtomic {
			sigs = append(sigs, signature.SimpleSigningFromBlob(sig.Content))
		}
	}
	return sigs, nil
}

func (s *dockerImageSource) getSignaturesFromSigstoreAttachments(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	if !s.c.useSigstoreAttachments {
		logrus.Debugf("Not looking for sigstore attachments: disabled by configuration")
		return nil, nil
	}

	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}

	ociManifest, err := s.c.getSigstoreAttachmentManifest(ctx, s.physicalRef, manifestDigest)
	if err != nil {
		return nil, err
	}
	if ociManifest == nil {
		return nil, nil
	}

	logrus.Debugf("Found a sigstore attachment manifest with %d layers", len(ociManifest.Layers))
	res := []signature.Signature{}
	for layerIndex, layer := range ociManifest.Layers {
		// Note that this copies all kinds of attachments: attestations, and whatever else is there,
		// not just signatures. We leave the signature consumers to decide based on the MIME type.
		logrus.Debugf("Fetching sigstore attachment %d/%d: %s", layerIndex+1, len(ociManifest.Layers), layer.Digest.String())
		// We don’t benefit from a real BlobInfoCache here because we never try to reuse/mount attachment payloads.
		// That might eventually need to change if payloads grow to be not just signatures, but something
		// significantly large.
		payload, err := s.c.getOCIDescriptorContents(ctx, s.physicalRef, layer, iolimits.MaxSignatureBodySize,
			none.NoCache)
		if err != nil {
			return nil, err
		}
		res = append(res, signature.SigstoreFromComponents(layer.MediaType, payload, layer.Annotations))
	}
	return res, nil
}

// deleteImage deletes the named image from the registry, if supported.
func deleteImage(ctx context.Context, sys *types.SystemContext, ref dockerReference) error {
	registryConfig, err := loadRegistryConfiguration(sys)
	if err != nil {
		return err
	}
	// docker/distribution does not document what action should be used for deleting images.
	//
	// Current docker/distribution requires "pull" for reading the manifest and "delete" for deleting it.
	// quay.io requires "push" (an explicit "pull" is unnecessary), does not grant any token (fails parsing the request) if "delete" is included.
	// OpenShift ignores the action string (both the password and the token is an OpenShift API token identifying a user).
	//
	// We have to hard-code a single string, luckily both docker/distribution and quay.io support "*" to mean "everything".
	c, err := newDockerClientFromRef(sys, ref, registryConfig, true, "*")
	if err != nil {
		return err
	}

	headers := map[string][]string{
		"Accept": manifest.DefaultRequestedManifestMIMETypes,
	}
	refTail, err := ref.tagOrDigest()
	if err != nil {
		return err
	}
	getPath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), refTail)
	get, err := c.makeRequest(ctx, http.MethodGet, getPath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer get.Body.Close()
	manifestBody, err := iolimits.ReadAtMost(get.Body, iolimits.MaxManifestBodySize)
	if err != nil {
		return err
	}
	switch get.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("Unable to delete %v. Image may not exist or is not stored with a v2 Schema in a v2 registry", ref.ref)
	default:
		return fmt.Errorf("Failed to delete %v: %s (%v)", ref.ref, manifestBody, get.Status)
	}

	manifestDigest, err := manifest.Digest(manifestBody)
	if err != nil {
		return fmt.Errorf("computing manifest digest: %w", err)
	}
	deletePath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), manifestDigest)

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	delete, err := c.makeRequest(ctx, http.MethodDelete, deletePath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer delete.Body.Close()

	body, err := iolimits.ReadAtMost(delete.Body, iolimits.MaxErrorBodySize)
	if err != nil {
		return err
	}
	if delete.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Failed to delete %v: %s (%v)", deletePath, string(body), delete.Status)
	}

	for i := 0; ; i++ {
		url := lookasideStorageURL(c.signatureBase, manifestDigest, i)
		missing, err := c.deleteOneSignature(url)
		if err != nil {
			return err
		}
		if missing {
			break
		}
	}

	return nil
}

type bufferedNetworkReaderBuffer struct {
	data     []byte
	len      int
	consumed int
	err      error
}

type bufferedNetworkReader struct {
	stream      io.ReadCloser
	emptyBuffer chan *bufferedNetworkReaderBuffer
	readyBuffer chan *bufferedNetworkReaderBuffer
	terminate   chan bool
	current     *bufferedNetworkReaderBuffer
	mutex       sync.Mutex
	gotEOF      bool
}

// handleBufferedNetworkReader runs in a goroutine
func handleBufferedNetworkReader(br *bufferedNetworkReader) {
	defer close(br.readyBuffer)
	for {
		select {
		case b := <-br.emptyBuffer:
			b.len, b.err = br.stream.Read(b.data)
			br.readyBuffer <- b
			if b.err != nil {
				return
			}
		case <-br.terminate:
			return
		}
	}
}

func (n *bufferedNetworkReader) Close() error {
	close(n.terminate)
	close(n.emptyBuffer)
	return n.stream.Close()
}

func (n *bufferedNetworkReader) read(p []byte) (int, error) {
	if n.current != nil {
		copied := copy(p, n.current.data[n.current.consumed:n.current.len])
		n.current.consumed += copied
		if n.current.consumed == n.current.len {
			n.emptyBuffer <- n.current
			n.current = nil
		}
		if copied > 0 {
			return copied, nil
		}
	}
	if n.gotEOF {
		return 0, io.EOF
	}

	var b *bufferedNetworkReaderBuffer

	select {
	case b = <-n.readyBuffer:
		if b.err != nil {
			if b.err != io.EOF {
				return b.len, b.err
			}
			n.gotEOF = true
		}
		b.consumed = 0
		n.current = b
		return n.read(p)
	case <-n.terminate:
		return 0, io.EOF
	}
}

func (n *bufferedNetworkReader) Read(p []byte) (int, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	return n.read(p)
}

func makeBufferedNetworkReader(stream io.ReadCloser, nBuffers, bufferSize uint) *bufferedNetworkReader {
	br := bufferedNetworkReader{
		stream:      stream,
		emptyBuffer: make(chan *bufferedNetworkReaderBuffer, nBuffers),
		readyBuffer: make(chan *bufferedNetworkReaderBuffer, nBuffers),
		terminate:   make(chan bool),
	}

	go func() {
		handleBufferedNetworkReader(&br)
	}()

	for i := uint(0); i < nBuffers; i++ {
		b := bufferedNetworkReaderBuffer{
			data: make([]byte, bufferSize),
		}
		br.emptyBuffer <- &b
	}

	return &br
}

type signalCloseReader struct {
	closed        chan interface{}
	stream        io.ReadCloser
	consumeStream bool
}

func (s signalCloseReader) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s signalCloseReader) Close() error {
	defer close(s.closed)
	if s.consumeStream {
		if _, err := io.Copy(io.Discard, s.stream); err != nil {
			s.stream.Close()
			return err
		}
	}
	return s.stream.Close()
}
