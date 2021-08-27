package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/iolimits"
	internalTypes "github.com/containers/image/v5/internal/types"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type dockerImageSource struct {
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
	registry, err := sysregistriesv2.FindRegistry(sys, ref.ref.Name())
	if err != nil {
		return nil, errors.Wrapf(err, "loading registries configuration")
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
		s, err := newImageSourceAttempt(ctx, sys, ref, pullSource)
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
		return nil, errors.Wrapf(primary.err, "(Mirrors also failed: %s): %s", strings.Join(extras, "\n"), primary.ref.String())
	}
}

// newImageSourceAttempt is an internal helper for newImageSource. Everyone else must call newImageSource.
// Given a logicalReference and a pullSource, return a dockerImageSource if it is reachable.
// The caller must call .Close() on the returned ImageSource.
func newImageSourceAttempt(ctx context.Context, sys *types.SystemContext, logicalRef dockerReference, pullSource sysregistriesv2.PullSource) (*dockerImageSource, error) {
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

	client, err := newDockerClientFromRef(endpointSys, physicalRef, false, "pull")
	if err != nil {
		return nil, err
	}
	client.tlsClientConfig.InsecureSkipVerify = pullSource.Endpoint.Insecure

	s := &dockerImageSource{
		logicalRef:  logicalRef,
		physicalRef: physicalRef,
		c:           client,
	}

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

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve BlobInfos for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *dockerImageSource) LayerInfosForCopy(context.Context, *digest.Digest) ([]types.BlobInfo, error) {
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
	path := fmt.Sprintf(manifestPath, reference.Path(s.physicalRef.ref), tagOrDigest)
	headers := map[string][]string{
		"Accept": manifest.DefaultRequestedManifestMIMETypes,
	}
	res, err := s.c.makeRequest(ctx, http.MethodGet, path, headers, nil, v2Auth, nil)
	if err != nil {
		return nil, "", err
	}
	logrus.Debugf("Content-Type from manifest GET is %q", res.Header.Get("Content-Type"))
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", errors.Wrapf(registryHTTPResponseToError(res), "reading manifest %s in %s", tagOrDigest, s.physicalRef.ref.Name())
	}

	manblob, err := iolimits.ReadAtMost(res.Body, iolimits.MaxManifestBodySize)
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

func (s *dockerImageSource) getExternalBlob(ctx context.Context, urls []string) (io.ReadCloser, int64, error) {
	var (
		resp *http.Response
		err  error
	)
	if len(urls) == 0 {
		return nil, 0, errors.New("internal error: getExternalBlob called with no URLs")
	}
	for _, url := range urls {
		// NOTE: we must not authenticate on additional URLs as those
		//       can be abused to leak credentials or tokens.  Please
		//       refer to CVE-2020-15157 for more information.
		resp, err = s.c.makeRequestToResolvedURL(ctx, http.MethodGet, url, nil, nil, -1, noAuth, nil)
		if err == nil {
			if resp.StatusCode != http.StatusOK {
				err = errors.Errorf("error fetching external blob from %q: %d (%s)", url, resp.StatusCode, http.StatusText(resp.StatusCode))
				logrus.Debug(err)
				resp.Body.Close()
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

// splitHTTP200ResponseToPartial splits a 200 response in multiple streams as specified by the chunks
func splitHTTP200ResponseToPartial(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []internalTypes.ImageSourceChunk) {
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
			if _, err := io.Copy(ioutil.Discard, io.LimitReader(body, int64(toSkip))); err != nil {
				errs <- err
				break
			}
			currentOffset += toSkip
		}
		s := signalCloseReader{
			closed:        make(chan interface{}),
			stream:        ioutil.NopCloser(io.LimitReader(body, int64(c.Length))),
			consumeStream: true,
		}
		streams <- s

		// Wait until the stream is closed before going to the next chunk
		<-s.closed
		currentOffset += c.Length
	}
}

// handle206Response reads a 206 response and send each part as a separate ReadCloser to the streams chan.
func handle206Response(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []internalTypes.ImageSourceChunk, mediaType string, params map[string]string) {
	defer close(streams)
	defer close(errs)
	if !strings.HasPrefix(mediaType, "multipart/") {
		streams <- body
		return
	}
	boundary, found := params["boundary"]
	if !found {
		errs <- errors.Errorf("could not find boundary")
		body.Close()
		return
	}
	buffered := makeBufferedNetworkReader(body, 64, 16384)
	defer buffered.Close()
	mr := multipart.NewReader(buffered, boundary)
	for {
		p, err := mr.NextPart()
		if err != nil {
			if err != io.EOF {
				errs <- err
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
	}
}

// GetBlobAt returns a stream for the specified blob.
// The specified chunks must be not overlapping and sorted by their offset.
func (s *dockerImageSource) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []internalTypes.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
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
	if err := httpResponseToError(res, "Error fetching partial blob"); err != nil {
		if res.Body != nil {
			res.Body.Close()
		}
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
		mediaType, params, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if err != nil {
			return nil, nil, err
		}

		streams := make(chan io.ReadCloser)
		errs := make(chan error)

		go handle206Response(streams, errs, res.Body, chunks, mediaType, params)
		return streams, errs, nil
	default:
		res.Body.Close()
		return nil, nil, errors.Errorf("invalid status code returned when fetching blob %d (%s)", res.StatusCode, http.StatusText(res.StatusCode))
	}
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *dockerImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		return s.getExternalBlob(ctx, info.URLs)
	}

	path := fmt.Sprintf(blobsPath, reference.Path(s.physicalRef.ref), info.Digest.String())
	logrus.Debugf("Downloading %s", path)
	res, err := s.c.makeRequest(ctx, http.MethodGet, path, nil, nil, v2Auth, nil)
	if err != nil {
		return nil, 0, err
	}
	if err := httpResponseToError(res, "Error fetching blob"); err != nil {
		res.Body.Close()
		return nil, 0, err
	}
	cache.RecordKnownLocation(s.physicalRef.Transport(), bicTransportScope(s.physicalRef), info.Digest, newBICLocationReference(s.physicalRef))
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
	case s.c.supportsSignatures:
		return s.getSignaturesFromAPIExtension(ctx, instanceDigest)
	case s.c.signatureBase != nil:
		return s.getSignaturesFromLookaside(ctx, instanceDigest)
	default:
		return nil, errors.Errorf("Internal error: X-Registry-Supports-Signatures extension not supported, and lookaside should not be empty configuration")
	}
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
			return nil, false, errors.Errorf("Error reading signature from %s: status %d (%s)", url.String(), res.StatusCode, http.StatusText(res.StatusCode))
		}
		sig, err := iolimits.ReadAtMost(res.Body, iolimits.MaxSignatureBodySize)
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

	parsedBody, err := s.c.getExtensionsSignatures(ctx, s.physicalRef, manifestDigest)
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
		return errors.Errorf("Unable to delete %v. Image may not exist or is not stored with a v2 Schema in a v2 registry", ref.ref)
	default:
		return errors.Errorf("Failed to delete %v: %s (%v)", ref.ref, manifestBody, get.Status)
	}

	digest := get.Header.Get("Docker-Content-Digest")
	deletePath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), digest)

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
		return errors.Errorf("Failed to delete %v: %s (%v)", deletePath, string(body), delete.Status)
	}

	manifestDigest, err := manifest.Digest(manifestBody)
	if err != nil {
		return err
	}

	for i := 0; ; i++ {
		url := signatureStorageURL(c.signatureBase, manifestDigest, i)
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
		if _, err := io.Copy(ioutil.Discard, s.stream); err != nil {
			s.stream.Close()
			return err
		}
	}
	return s.stream.Close()
}
