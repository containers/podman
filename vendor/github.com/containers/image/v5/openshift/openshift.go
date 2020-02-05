package openshift

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/iolimits"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/image/v5/version"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// openshiftClient is configuration for dealing with a single image stream, for reading or writing.
type openshiftClient struct {
	ref     openshiftReference
	baseURL *url.URL
	// Values from Kubernetes configuration
	httpClient  *http.Client
	bearerToken string // "" if not used
	username    string // "" if not used
	password    string // if username != ""
}

// newOpenshiftClient creates a new openshiftClient for the specified reference.
func newOpenshiftClient(ref openshiftReference) (*openshiftClient, error) {
	// We have already done this parsing in ParseReference, but thrown away
	// httpClient. So, parse again.
	// (We could also rework/split restClientFor to "get base URL" to be done
	// in ParseReference, and "get httpClient" to be done here.  But until/unless
	// we support non-default clusters, this is good enough.)

	// Overall, this is modelled on openshift/origin/pkg/cmd/util/clientcmd.New().ClientConfig() and openshift/origin/pkg/client.
	cmdConfig := defaultClientConfig()
	logrus.Debugf("cmdConfig: %#v", cmdConfig)
	restConfig, err := cmdConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	// REMOVED: SetOpenShiftDefaults (values are not overridable in config files, so hard-coded these defaults.)
	logrus.Debugf("restConfig: %#v", restConfig)
	baseURL, httpClient, err := restClientFor(restConfig)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("URL: %#v", *baseURL)

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &openshiftClient{
		ref:         ref,
		baseURL:     baseURL,
		httpClient:  httpClient,
		bearerToken: restConfig.BearerToken,
		username:    restConfig.Username,
		password:    restConfig.Password,
	}, nil
}

// doRequest performs a correctly authenticated request to a specified path, and returns response body or an error object.
func (c *openshiftClient) doRequest(ctx context.Context, method, path string, requestBody []byte) ([]byte, error) {
	url := *c.baseURL
	url.Path = path
	var requestBodyReader io.Reader
	if requestBody != nil {
		logrus.Debugf("Will send body: %s", requestBody)
		requestBodyReader = bytes.NewReader(requestBody)
	}
	req, err := http.NewRequest(method, url.String(), requestBodyReader)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	if len(c.bearerToken) != 0 {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	} else if len(c.username) != 0 {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("User-Agent", fmt.Sprintf("skopeo/%s", version.Version))
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	logrus.Debugf("%s %s", method, url.String())
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := iolimits.ReadAtMost(res.Body, iolimits.MaxOpenShiftStatusBody)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Got body: %s", body)
	// FIXME: Just throwing this useful information away only to try to guess later...
	logrus.Debugf("Got content-type: %s", res.Header.Get("Content-Type"))

	var status status
	statusValid := false
	if err := json.Unmarshal(body, &status); err == nil && len(status.Status) > 0 {
		statusValid = true
	}

	switch {
	case res.StatusCode == http.StatusSwitchingProtocols: // FIXME?! No idea why this weird case exists in k8s.io/kubernetes/pkg/client/restclient.
		if statusValid && status.Status != "Success" {
			return nil, errors.New(status.Message)
		}
	case res.StatusCode >= http.StatusOK && res.StatusCode <= http.StatusPartialContent:
		// OK.
	default:
		if statusValid {
			return nil, errors.New(status.Message)
		}
		return nil, errors.Errorf("HTTP error: status code: %d (%s), body: %s", res.StatusCode, http.StatusText(res.StatusCode), string(body))
	}

	return body, nil
}

// getImage loads the specified image object.
func (c *openshiftClient) getImage(ctx context.Context, imageStreamImageName string) (*image, error) {
	// FIXME: validate components per validation.IsValidPathSegmentName?
	path := fmt.Sprintf("/oapi/v1/namespaces/%s/imagestreamimages/%s@%s", c.ref.namespace, c.ref.stream, imageStreamImageName)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Note: This does absolutely no kind/version checking or conversions.
	var isi imageStreamImage
	if err := json.Unmarshal(body, &isi); err != nil {
		return nil, err
	}
	return &isi.Image, nil
}

// convertDockerImageReference takes an image API DockerImageReference value and returns a reference we can actually use;
// currently OpenShift stores the cluster-internal service IPs here, which are unusable from the outside.
func (c *openshiftClient) convertDockerImageReference(ref string) (string, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", errors.Errorf("Invalid format of docker reference %s: missing '/'", ref)
	}
	return reference.Domain(c.ref.dockerReference) + "/" + parts[1], nil
}

type openshiftImageSource struct {
	client *openshiftClient
	// Values specific to this image
	sys *types.SystemContext
	// State
	docker               types.ImageSource // The Docker Registry endpoint, or nil if not resolved yet
	imageStreamImageName string            // Resolved image identifier, or "" if not known yet
}

// newImageSource creates a new ImageSource for the specified reference.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(sys *types.SystemContext, ref openshiftReference) (types.ImageSource, error) {
	client, err := newOpenshiftClient(ref)
	if err != nil {
		return nil, err
	}

	return &openshiftImageSource{
		client: client,
		sys:    sys,
	}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *openshiftImageSource) Reference() types.ImageReference {
	return s.client.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *openshiftImageSource) Close() error {
	if s.docker != nil {
		err := s.docker.Close()
		s.docker = nil

		return err
	}

	return nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *openshiftImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if err := s.ensureImageIsResolved(ctx); err != nil {
		return nil, "", err
	}
	return s.docker.GetManifest(ctx, instanceDigest)
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *openshiftImageSource) HasThreadSafeGetBlob() bool {
	return false
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *openshiftImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if err := s.ensureImageIsResolved(ctx); err != nil {
		return nil, 0, err
	}
	return s.docker.GetBlob(ctx, info, cache)
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *openshiftImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	var imageStreamImageName string
	if instanceDigest == nil {
		if err := s.ensureImageIsResolved(ctx); err != nil {
			return nil, err
		}
		imageStreamImageName = s.imageStreamImageName
	} else {
		imageStreamImageName = instanceDigest.String()
	}
	image, err := s.client.getImage(ctx, imageStreamImageName)
	if err != nil {
		return nil, err
	}
	var sigs [][]byte
	for _, sig := range image.Signatures {
		if sig.Type == imageSignatureTypeAtomic {
			sigs = append(sigs, sig.Content)
		}
	}
	return sigs, nil
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve BlobInfos for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *openshiftImageSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	return nil, nil
}

// ensureImageIsResolved sets up s.docker and s.imageStreamImageName
func (s *openshiftImageSource) ensureImageIsResolved(ctx context.Context) error {
	if s.docker != nil {
		return nil
	}

	// FIXME: validate components per validation.IsValidPathSegmentName?
	path := fmt.Sprintf("/oapi/v1/namespaces/%s/imagestreams/%s", s.client.ref.namespace, s.client.ref.stream)
	body, err := s.client.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	// Note: This does absolutely no kind/version checking or conversions.
	var is imageStream
	if err := json.Unmarshal(body, &is); err != nil {
		return err
	}
	var te *tagEvent
	for _, tag := range is.Status.Tags {
		if tag.Tag != s.client.ref.dockerReference.Tag() {
			continue
		}
		if len(tag.Items) > 0 {
			te = &tag.Items[0]
			break
		}
	}
	if te == nil {
		return errors.Errorf("No matching tag found")
	}
	logrus.Debugf("tag event %#v", te)
	dockerRefString, err := s.client.convertDockerImageReference(te.DockerImageReference)
	if err != nil {
		return err
	}
	logrus.Debugf("Resolved reference %#v", dockerRefString)
	dockerRef, err := docker.ParseReference("//" + dockerRefString)
	if err != nil {
		return err
	}
	d, err := dockerRef.NewImageSource(ctx, s.sys)
	if err != nil {
		return err
	}
	s.docker = d
	s.imageStreamImageName = te.Image
	return nil
}

type openshiftImageDestination struct {
	client *openshiftClient
	docker types.ImageDestination // The Docker Registry endpoint
	// State
	imageStreamImageName string // "" if not yet known
}

// newImageDestination creates a new ImageDestination for the specified reference.
func newImageDestination(ctx context.Context, sys *types.SystemContext, ref openshiftReference) (types.ImageDestination, error) {
	client, err := newOpenshiftClient(ref)
	if err != nil {
		return nil, err
	}

	// FIXME: Should this always use a digest, not a tag? Uploading to Docker by tag requires the tag _inside_ the manifest to match,
	// i.e. a single signed image cannot be available under multiple tags.  But with types.ImageDestination, we don't know
	// the manifest digest at this point.
	dockerRefString := fmt.Sprintf("//%s/%s/%s:%s", reference.Domain(client.ref.dockerReference), client.ref.namespace, client.ref.stream, client.ref.dockerReference.Tag())
	dockerRef, err := docker.ParseReference(dockerRefString)
	if err != nil {
		return nil, err
	}
	docker, err := dockerRef.NewImageDestination(ctx, sys)
	if err != nil {
		return nil, err
	}

	return &openshiftImageDestination{
		client: client,
		docker: docker,
	}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *openshiftImageDestination) Reference() types.ImageReference {
	return d.client.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *openshiftImageDestination) Close() error {
	return d.docker.Close()
}

func (d *openshiftImageDestination) SupportedManifestMIMETypes() []string {
	return d.docker.SupportedManifestMIMETypes()
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *openshiftImageDestination) SupportsSignatures(ctx context.Context) error {
	return nil
}

func (d *openshiftImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.Compress
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *openshiftImageDestination) AcceptsForeignLayerURLs() bool {
	return true
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *openshiftImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *openshiftImageDestination) IgnoresEmbeddedDockerReference() bool {
	return d.docker.IgnoresEmbeddedDockerReference()
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *openshiftImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *openshiftImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	return d.docker.PutBlob(ctx, stream, inputInfo, cache, isConfig)
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been succesfully reused, returns (true, info, nil); info must contain at least a digest and size.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *openshiftImageDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	return d.docker.TryReusingBlob(ctx, info, cache, canSubstitute)
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *openshiftImageDestination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	if instanceDigest == nil {
		manifestDigest, err := manifest.Digest(m)
		if err != nil {
			return err
		}
		d.imageStreamImageName = manifestDigest.String()
	}
	return d.docker.PutManifest(ctx, m, instanceDigest)
}

func (d *openshiftImageDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	var imageStreamName string
	if instanceDigest == nil {
		if d.imageStreamImageName == "" {
			return errors.Errorf("Internal error: Unknown manifest digest, can't add signatures")
		}
		imageStreamName = d.imageStreamImageName
	} else {
		imageStreamName = instanceDigest.String()
	}

	// Because image signatures are a shared resource in Atomic Registry, the default upload
	// always adds signatures.  Eventually we should also allow removing signatures.

	if len(signatures) == 0 {
		return nil // No need to even read the old state.
	}

	image, err := d.client.getImage(ctx, imageStreamName)
	if err != nil {
		return err
	}
	existingSigNames := map[string]struct{}{}
	for _, sig := range image.Signatures {
		existingSigNames[sig.objectMeta.Name] = struct{}{}
	}

sigExists:
	for _, newSig := range signatures {
		for _, existingSig := range image.Signatures {
			if existingSig.Type == imageSignatureTypeAtomic && bytes.Equal(existingSig.Content, newSig) {
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
			signatureName = fmt.Sprintf("%s@%032x", imageStreamName, randBytes)
			if _, ok := existingSigNames[signatureName]; !ok {
				break
			}
		}
		// Note: This does absolutely no kind/version checking or conversions.
		sig := imageSignature{
			typeMeta: typeMeta{
				Kind:       "ImageSignature",
				APIVersion: "v1",
			},
			objectMeta: objectMeta{Name: signatureName},
			Type:       imageSignatureTypeAtomic,
			Content:    newSig,
		}
		body, err := json.Marshal(sig)
		if err != nil {
			return err
		}
		_, err = d.client.doRequest(ctx, "POST", "/oapi/v1/imagesignatures", body)
		if err != nil {
			return err
		}
	}

	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *openshiftImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	return d.docker.Commit(ctx, unparsedToplevel)
}

// These structs are subsets of github.com/openshift/origin/pkg/image/api/v1 and its dependencies.
type imageStream struct {
	Status imageStreamStatus `json:"status,omitempty"`
}
type imageStreamStatus struct {
	DockerImageRepository string              `json:"dockerImageRepository"`
	Tags                  []namedTagEventList `json:"tags,omitempty"`
}
type namedTagEventList struct {
	Tag   string     `json:"tag"`
	Items []tagEvent `json:"items"`
}
type tagEvent struct {
	DockerImageReference string `json:"dockerImageReference"`
	Image                string `json:"image"`
}
type imageStreamImage struct {
	Image image `json:"image"`
}
type image struct {
	objectMeta           `json:"metadata,omitempty"`
	DockerImageReference string `json:"dockerImageReference,omitempty"`
	//	DockerImageMetadata        runtime.RawExtension `json:"dockerImageMetadata,omitempty"`
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty"`
	DockerImageManifest        string `json:"dockerImageManifest,omitempty"`
	//	DockerImageLayers          []ImageLayer         `json:"dockerImageLayers"`
	Signatures []imageSignature `json:"signatures,omitempty"`
}

const imageSignatureTypeAtomic string = "atomic"

type imageSignature struct {
	typeMeta   `json:",inline"`
	objectMeta `json:"metadata,omitempty"`
	Type       string `json:"type"`
	Content    []byte `json:"content"`
	// Conditions []SignatureCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// ImageIdentity string `json:"imageIdentity,omitempty"`
	// SignedClaims map[string]string `json:"signedClaims,omitempty"`
	// Created *unversioned.Time `json:"created,omitempty"`
	// IssuedBy SignatureIssuer `json:"issuedBy,omitempty"`
	// IssuedTo SignatureSubject `json:"issuedTo,omitempty"`
}
type typeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}
type objectMeta struct {
	Name                       string            `json:"name,omitempty"`
	GenerateName               string            `json:"generateName,omitempty"`
	Namespace                  string            `json:"namespace,omitempty"`
	SelfLink                   string            `json:"selfLink,omitempty"`
	ResourceVersion            string            `json:"resourceVersion,omitempty"`
	Generation                 int64             `json:"generation,omitempty"`
	DeletionGracePeriodSeconds *int64            `json:"deletionGracePeriodSeconds,omitempty"`
	Labels                     map[string]string `json:"labels,omitempty"`
	Annotations                map[string]string `json:"annotations,omitempty"`
}

// A subset of k8s.io/kubernetes/pkg/api/unversioned/Status
type status struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	// Reason StatusReason `json:"reason,omitempty"`
	// Details *StatusDetails `json:"details,omitempty"`
	Code int32 `json:"code,omitempty"`
}
