package docker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/iolimits"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/pkg/tlsclientconfig"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/homedir"
	clientLib "github.com/docker/distribution/registry/client"
	"github.com/docker/go-connections/tlsconfig"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	dockerHostname   = "docker.io"
	dockerV1Hostname = "index.docker.io"
	dockerRegistry   = "registry-1.docker.io"

	resolvedPingV2URL       = "%s://%s/v2/"
	resolvedPingV1URL       = "%s://%s/v1/_ping"
	tagsPath                = "/v2/%s/tags/list"
	manifestPath            = "/v2/%s/manifests/%s"
	blobsPath               = "/v2/%s/blobs/%s"
	blobUploadPath          = "/v2/%s/blobs/uploads/"
	extensionsSignaturePath = "/extensions/v2/%s/signatures/%s"

	minimumTokenLifetimeSeconds = 60

	extensionSignatureSchemaVersion = 2        // extensionSignature.Version
	extensionSignatureTypeAtomic    = "atomic" // extensionSignature.Type

	backoffNumIterations = 5
	backoffInitialDelay  = 2 * time.Second
	backoffMaxDelay      = 60 * time.Second
)

type certPath struct {
	path     string
	absolute bool
}

var (
	homeCertDir     = filepath.FromSlash(".config/containers/certs.d")
	perHostCertDirs = []certPath{
		{path: "/etc/containers/certs.d", absolute: true},
		{path: "/etc/docker/certs.d", absolute: true},
	}
)

// extensionSignature and extensionSignatureList come from github.com/openshift/origin/pkg/dockerregistry/server/signaturedispatcher.go:
// signature represents a Docker image signature.
type extensionSignature struct {
	Version int    `json:"schemaVersion"` // Version specifies the schema version
	Name    string `json:"name"`          // Name must be in "sha256:<digest>@signatureName" format
	Type    string `json:"type"`          // Type is optional, of not set it will be defaulted to "AtomicImageV1"
	Content []byte `json:"content"`       // Content contains the signature
}

// signatureList represents list of Docker image signatures.
type extensionSignatureList struct {
	Signatures []extensionSignature `json:"signatures"`
}

type bearerToken struct {
	Token          string    `json:"token"`
	AccessToken    string    `json:"access_token"`
	ExpiresIn      int       `json:"expires_in"`
	IssuedAt       time.Time `json:"issued_at"`
	expirationTime time.Time
}

// dockerClient is configuration for dealing with a single Docker registry.
type dockerClient struct {
	// The following members are set by newDockerClient and do not change afterwards.
	sys      *types.SystemContext
	registry string

	// tlsClientConfig is setup by newDockerClient and will be used and updated
	// by detectProperties(). Callers can edit tlsClientConfig.InsecureSkipVerify in the meantime.
	tlsClientConfig *tls.Config
	// The following members are not set by newDockerClient and must be set by callers if needed.
	auth          types.DockerAuthConfig
	registryToken string
	signatureBase signatureStorageBase
	scope         authScope

	// The following members are detected registry properties:
	// They are set after a successful detectProperties(), and never change afterwards.
	client             *http.Client
	scheme             string
	challenges         []challenge
	supportsSignatures bool

	// Private state for setupRequestAuth (key: string, value: bearerToken)
	tokenCache sync.Map
	// Private state for detectProperties:
	detectPropertiesOnce  sync.Once // detectPropertiesOnce is used to execute detectProperties() at most once.
	detectPropertiesError error     // detectPropertiesError caches the initial error.
}

type authScope struct {
	remoteName string
	actions    string
}

// sendAuth determines whether we need authentication for v2 or v1 endpoint.
type sendAuth int

const (
	// v2 endpoint with authentication.
	v2Auth sendAuth = iota
	// v1 endpoint with authentication.
	// TODO: Get v1Auth working
	// v1Auth
	// no authentication, works for both v1 and v2.
	noAuth
)

func newBearerTokenFromJSONBlob(blob []byte) (*bearerToken, error) {
	token := new(bearerToken)
	if err := json.Unmarshal(blob, &token); err != nil {
		return nil, err
	}
	if token.Token == "" {
		token.Token = token.AccessToken
	}
	if token.ExpiresIn < minimumTokenLifetimeSeconds {
		token.ExpiresIn = minimumTokenLifetimeSeconds
		logrus.Debugf("Increasing token expiration to: %d seconds", token.ExpiresIn)
	}
	if token.IssuedAt.IsZero() {
		token.IssuedAt = time.Now().UTC()
	}
	token.expirationTime = token.IssuedAt.Add(time.Duration(token.ExpiresIn) * time.Second)
	return token, nil
}

// this is cloned from docker/go-connections because upstream docker has changed
// it and make deps here fails otherwise.
// We'll drop this once we upgrade to docker 1.13.x deps.
func serverDefault() *tls.Config {
	return &tls.Config{
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
		CipherSuites:             tlsconfig.DefaultServerAcceptedCiphers,
	}
}

// dockerCertDir returns a path to a directory to be consumed by tlsclientconfig.SetupCertificates() depending on ctx and hostPort.
func dockerCertDir(sys *types.SystemContext, hostPort string) (string, error) {
	if sys != nil && sys.DockerCertPath != "" {
		return sys.DockerCertPath, nil
	}
	if sys != nil && sys.DockerPerHostCertDirPath != "" {
		return filepath.Join(sys.DockerPerHostCertDirPath, hostPort), nil
	}

	var (
		hostCertDir     string
		fullCertDirPath string
	)

	for _, perHostCertDir := range append([]certPath{{path: filepath.Join(homedir.Get(), homeCertDir), absolute: false}}, perHostCertDirs...) {
		if sys != nil && sys.RootForImplicitAbsolutePaths != "" && perHostCertDir.absolute {
			hostCertDir = filepath.Join(sys.RootForImplicitAbsolutePaths, perHostCertDir.path)
		} else {
			hostCertDir = perHostCertDir.path
		}

		fullCertDirPath = filepath.Join(hostCertDir, hostPort)
		_, err := os.Stat(fullCertDirPath)
		if err == nil {
			break
		}
		if os.IsNotExist(err) {
			continue
		}
		if os.IsPermission(err) {
			logrus.Debugf("error accessing certs directory due to permissions: %v", err)
			continue
		}
		if err != nil {
			return "", err
		}
	}
	return fullCertDirPath, nil
}

// newDockerClientFromRef returns a new dockerClient instance for refHostname (a host a specified in the Docker image reference, not canonicalized to dockerRegistry)
// “write” specifies whether the client will be used for "write" access (in particular passed to lookaside.go:toplevelFromSection)
func newDockerClientFromRef(sys *types.SystemContext, ref dockerReference, write bool, actions string) (*dockerClient, error) {
	registry := reference.Domain(ref.ref)
	auth, err := config.GetCredentials(sys, registry)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting username and password")
	}

	sigBase, err := configuredSignatureStorageBase(sys, ref, write)
	if err != nil {
		return nil, err
	}

	client, err := newDockerClient(sys, registry, ref.ref.Name())
	if err != nil {
		return nil, err
	}
	client.auth = auth
	if sys != nil {
		client.registryToken = sys.DockerBearerRegistryToken
	}
	client.signatureBase = sigBase
	client.scope.actions = actions
	client.scope.remoteName = reference.Path(ref.ref)
	return client, nil
}

// newDockerClient returns a new dockerClient instance for the given registry
// and reference.  The reference is used to query the registry configuration
// and can either be a registry (e.g, "registry.com[:5000]"), a repository
// (e.g., "registry.com[:5000][/some/namespace]/repo").
// Please note that newDockerClient does not set all members of dockerClient
// (e.g., username and password); those must be set by callers if necessary.
func newDockerClient(sys *types.SystemContext, registry, reference string) (*dockerClient, error) {
	hostName := registry
	if registry == dockerHostname {
		registry = dockerRegistry
	}
	tlsClientConfig := serverDefault()

	// It is undefined whether the host[:port] string for dockerHostname should be dockerHostname or dockerRegistry,
	// because docker/docker does not read the certs.d subdirectory at all in that case.  We use the user-visible
	// dockerHostname here, because it is more symmetrical to read the configuration in that case as well, and because
	// generally the UI hides the existence of the different dockerRegistry.  But note that this behavior is
	// undocumented and may change if docker/docker changes.
	certDir, err := dockerCertDir(sys, hostName)
	if err != nil {
		return nil, err
	}
	if err := tlsclientconfig.SetupCertificates(certDir, tlsClientConfig); err != nil {
		return nil, err
	}

	// Check if TLS verification shall be skipped (default=false) which can
	// be specified in the sysregistriesv2 configuration.
	skipVerify := false
	reg, err := sysregistriesv2.FindRegistry(sys, reference)
	if err != nil {
		return nil, errors.Wrapf(err, "error loading registries")
	}
	if reg != nil {
		if reg.Blocked {
			return nil, fmt.Errorf("registry %s is blocked in %s or %s", reg.Prefix, sysregistriesv2.ConfigPath(sys), sysregistriesv2.ConfigDirPath(sys))
		}
		skipVerify = reg.Insecure
	}
	tlsClientConfig.InsecureSkipVerify = skipVerify

	return &dockerClient{
		sys:             sys,
		registry:        registry,
		tlsClientConfig: tlsClientConfig,
	}, nil
}

// CheckAuth validates the credentials by attempting to log into the registry
// returns an error if an error occurred while making the http request or the status code received was 401
func CheckAuth(ctx context.Context, sys *types.SystemContext, username, password, registry string) error {
	client, err := newDockerClient(sys, registry, registry)
	if err != nil {
		return errors.Wrapf(err, "error creating new docker client")
	}
	client.auth = types.DockerAuthConfig{
		Username: username,
		Password: password,
	}

	resp, err := client.makeRequest(ctx, "GET", "/v2/", nil, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return httpResponseToError(resp, "")
}

// SearchResult holds the information of each matching image
// It matches the output returned by the v1 endpoint
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// StarCount states the number of stars the image has
	StarCount int  `json:"star_count"`
	IsTrusted bool `json:"is_trusted"`
	// IsAutomated states whether the image is an automated build
	IsAutomated bool `json:"is_automated"`
	// IsOfficial states whether the image is an official build
	IsOfficial bool `json:"is_official"`
}

// SearchRegistry queries a registry for images that contain "image" in their name
// The limit is the max number of results desired
// Note: The limit value doesn't work with all registries
// for example registry.access.redhat.com returns all the results without limiting it to the limit value
func SearchRegistry(ctx context.Context, sys *types.SystemContext, registry, image string, limit int) ([]SearchResult, error) {
	type V2Results struct {
		// Repositories holds the results returned by the /v2/_catalog endpoint
		Repositories []string `json:"repositories"`
	}
	type V1Results struct {
		// Results holds the results returned by the /v1/search endpoint
		Results []SearchResult `json:"results"`
	}
	v2Res := &V2Results{}
	v1Res := &V1Results{}

	// Get credentials from authfile for the underlying hostname
	auth, err := config.GetCredentials(sys, registry)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting username and password")
	}

	// The /v2/_catalog endpoint has been disabled for docker.io therefore
	// the call made to that endpoint will fail.  So using the v1 hostname
	// for docker.io for simplicity of implementation and the fact that it
	// returns search results.
	hostname := registry
	if registry == dockerHostname {
		hostname = dockerV1Hostname
	}

	client, err := newDockerClient(sys, hostname, registry)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new docker client")
	}
	client.auth = auth
	if sys != nil {
		client.registryToken = sys.DockerBearerRegistryToken
	}

	// Only try the v1 search endpoint if the search query is not empty. If it is
	// empty skip to the v2 endpoint.
	if image != "" {
		// set up the query values for the v1 endpoint
		u := url.URL{
			Path: "/v1/search",
		}
		q := u.Query()
		q.Set("q", image)
		q.Set("n", strconv.Itoa(limit))
		u.RawQuery = q.Encode()

		logrus.Debugf("trying to talk to v1 search endpoint")
		resp, err := client.makeRequest(ctx, "GET", u.String(), nil, nil, noAuth, nil)
		if err != nil {
			logrus.Debugf("error getting search results from v1 endpoint %q: %v", registry, err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Debugf("error getting search results from v1 endpoint %q: %v", registry, httpResponseToError(resp, ""))
			} else {
				if err := json.NewDecoder(resp.Body).Decode(v1Res); err != nil {
					return nil, err
				}
				return v1Res.Results, nil
			}
		}
	}

	logrus.Debugf("trying to talk to v2 search endpoint")
	resp, err := client.makeRequest(ctx, "GET", "/v2/_catalog", nil, nil, v2Auth, nil)
	if err != nil {
		logrus.Debugf("error getting search results from v2 endpoint %q: %v", registry, err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logrus.Errorf("error getting search results from v2 endpoint %q: %v", registry, httpResponseToError(resp, ""))
		} else {
			if err := json.NewDecoder(resp.Body).Decode(v2Res); err != nil {
				return nil, err
			}
			searchRes := []SearchResult{}
			for _, repo := range v2Res.Repositories {
				if strings.Contains(repo, image) {
					res := SearchResult{
						Name: repo,
					}
					searchRes = append(searchRes, res)
				}
			}
			return searchRes, nil
		}
	}

	return nil, errors.Wrapf(err, "couldn't search registry %q", registry)
}

// makeRequest creates and executes a http.Request with the specified parameters, adding authentication and TLS options for the Docker client.
// The host name and schema is taken from the client or autodetected, and the path is relative to it, i.e. the path usually starts with /v2/.
func (c *dockerClient) makeRequest(ctx context.Context, method, path string, headers map[string][]string, stream io.Reader, auth sendAuth, extraScope *authScope) (*http.Response, error) {
	if err := c.detectProperties(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s://%s%s", c.scheme, c.registry, path)
	return c.makeRequestToResolvedURL(ctx, method, url, headers, stream, -1, auth, extraScope)
}

// parseRetryAfter determines the delay required by the "Retry-After" header in res and returns it,
// silently falling back to fallbackDelay if the header is missing or invalid.
func parseRetryAfter(res *http.Response, fallbackDelay time.Duration) time.Duration {
	after := res.Header.Get("Retry-After")
	if after == "" {
		return fallbackDelay
	}
	logrus.Debugf("Detected 'Retry-After' header %q", after)
	// First, check if we have a numerical value.
	if num, err := strconv.ParseInt(after, 10, 64); err == nil {
		return time.Duration(num) * time.Second
	}
	// Second, check if we have an HTTP date.
	// If the delta between the date and now is positive, use it.
	// Otherwise, fall back to using the default exponential back off.
	if t, err := http.ParseTime(after); err == nil {
		delta := time.Until(t)
		if delta > 0 {
			return delta
		}
		logrus.Debugf("Retry-After date in the past, ignoring it")
		return fallbackDelay
	}
	// If the header contents are bogus, fall back to using the default exponential back off.
	logrus.Debugf("Invalid Retry-After format, ignoring it")
	return fallbackDelay
}

// makeRequestToResolvedURL creates and executes a http.Request with the specified parameters, adding authentication and TLS options for the Docker client.
// streamLen, if not -1, specifies the length of the data expected on stream.
// makeRequest should generally be preferred.
// In case of an HTTP 429 status code in the response, it may automatically retry a few times.
// TODO(runcom): too many arguments here, use a struct
func (c *dockerClient) makeRequestToResolvedURL(ctx context.Context, method, url string, headers map[string][]string, stream io.Reader, streamLen int64, auth sendAuth, extraScope *authScope) (*http.Response, error) {
	delay := backoffInitialDelay
	attempts := 0
	for {
		res, err := c.makeRequestToResolvedURLOnce(ctx, method, url, headers, stream, streamLen, auth, extraScope)
		attempts++
		if res == nil || res.StatusCode != http.StatusTooManyRequests || // Only retry on StatusTooManyRequests, success or other failure is returned to caller immediately
			stream != nil || // We can't retry with a body (which is not restartable in the general case)
			attempts == backoffNumIterations {
			return res, err
		}

		delay = parseRetryAfter(res, delay)
		if delay > backoffMaxDelay {
			delay = backoffMaxDelay
		}
		logrus.Debugf("Too many requests to %s: sleeping for %f seconds before next attempt", url, delay.Seconds())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Nothing
		}
		delay = delay * 2 // exponential back off
	}
}

// makeRequestToResolvedURLOnce creates and executes a http.Request with the specified parameters, adding authentication and TLS options for the Docker client.
// streamLen, if not -1, specifies the length of the data expected on stream.
// makeRequest should generally be preferred.
// Note that no exponential back off is performed when receiving an http 429 status code.
func (c *dockerClient) makeRequestToResolvedURLOnce(ctx context.Context, method, url string, headers map[string][]string, stream io.Reader, streamLen int64, auth sendAuth, extraScope *authScope) (*http.Response, error) {
	req, err := http.NewRequest(method, url, stream)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if streamLen != -1 { // Do not blindly overwrite if streamLen == -1, http.NewRequest above can figure out the length of bytes.Reader and similar objects without us having to compute it.
		req.ContentLength = streamLen
	}
	req.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	for n, h := range headers {
		for _, hh := range h {
			req.Header.Add(n, hh)
		}
	}
	if c.sys != nil && c.sys.DockerRegistryUserAgent != "" {
		req.Header.Add("User-Agent", c.sys.DockerRegistryUserAgent)
	}
	if auth == v2Auth {
		if err := c.setupRequestAuth(req, extraScope); err != nil {
			return nil, err
		}
	}
	logrus.Debugf("%s %s", method, url)
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// we're using the challenges from the /v2/ ping response and not the one from the destination
// URL in this request because:
//
// 1) docker does that as well
// 2) gcr.io is sending 401 without a WWW-Authenticate header in the real request
//
// debugging: https://github.com/containers/image/pull/211#issuecomment-273426236 and follows up
func (c *dockerClient) setupRequestAuth(req *http.Request, extraScope *authScope) error {
	if len(c.challenges) == 0 {
		return nil
	}
	schemeNames := make([]string, 0, len(c.challenges))
	for _, challenge := range c.challenges {
		schemeNames = append(schemeNames, challenge.Scheme)
		switch challenge.Scheme {
		case "basic":
			req.SetBasicAuth(c.auth.Username, c.auth.Password)
			return nil
		case "bearer":
			registryToken := c.registryToken
			if registryToken == "" {
				cacheKey := ""
				scopes := []authScope{c.scope}
				if extraScope != nil {
					// Using ':' as a separator here is unambiguous because getBearerToken below uses the same separator when formatting a remote request (and because repository names can't contain colons).
					cacheKey = fmt.Sprintf("%s:%s", extraScope.remoteName, extraScope.actions)
					scopes = append(scopes, *extraScope)
				}
				var token bearerToken
				t, inCache := c.tokenCache.Load(cacheKey)
				if inCache {
					token = t.(bearerToken)
				}
				if !inCache || time.Now().After(token.expirationTime) {
					var (
						t   *bearerToken
						err error
					)
					if c.auth.IdentityToken != "" {
						t, err = c.getBearerTokenOAuth2(req.Context(), challenge, scopes)
					} else {
						t, err = c.getBearerToken(req.Context(), challenge, scopes)
					}
					if err != nil {
						return err
					}

					token = *t
					c.tokenCache.Store(cacheKey, token)
				}
				registryToken = token.Token
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", registryToken))
			return nil
		default:
			logrus.Debugf("no handler for %s authentication", challenge.Scheme)
		}
	}
	logrus.Infof("None of the challenges sent by server (%s) are supported, trying an unauthenticated request anyway", strings.Join(schemeNames, ", "))
	return nil
}

func (c *dockerClient) getBearerTokenOAuth2(ctx context.Context, challenge challenge,
	scopes []authScope) (*bearerToken, error) {
	realm, ok := challenge.Parameters["realm"]
	if !ok {
		return nil, errors.Errorf("missing realm in bearer auth challenge")
	}

	authReq, err := http.NewRequest(http.MethodPost, realm, nil)
	if err != nil {
		return nil, err
	}

	authReq = authReq.WithContext(ctx)

	// Make the form data required against the oauth2 authentication
	// More details here: https://docs.docker.com/registry/spec/auth/oauth/
	params := authReq.URL.Query()
	if service, ok := challenge.Parameters["service"]; ok && service != "" {
		params.Add("service", service)
	}
	for _, scope := range scopes {
		if scope.remoteName != "" && scope.actions != "" {
			params.Add("scope", fmt.Sprintf("repository:%s:%s", scope.remoteName, scope.actions))
		}
	}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", c.auth.IdentityToken)
	params.Add("client_id", "containers/image")

	authReq.Body = ioutil.NopCloser(bytes.NewBufferString(params.Encode()))
	if c.sys != nil && c.sys.DockerRegistryUserAgent != "" {
		authReq.Header.Add("User-Agent", c.sys.DockerRegistryUserAgent)
	}
	authReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	logrus.Debugf("%s %s", authReq.Method, authReq.URL.String())
	res, err := c.client.Do(authReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := httpResponseToError(res, "Trying to obtain access token"); err != nil {
		return nil, err
	}

	tokenBlob, err := iolimits.ReadAtMost(res.Body, iolimits.MaxAuthTokenBodySize)
	if err != nil {
		return nil, err
	}

	return newBearerTokenFromJSONBlob(tokenBlob)
}

func (c *dockerClient) getBearerToken(ctx context.Context, challenge challenge,
	scopes []authScope) (*bearerToken, error) {
	realm, ok := challenge.Parameters["realm"]
	if !ok {
		return nil, errors.Errorf("missing realm in bearer auth challenge")
	}

	authReq, err := http.NewRequest(http.MethodGet, realm, nil)
	if err != nil {
		return nil, err
	}

	authReq = authReq.WithContext(ctx)
	params := authReq.URL.Query()
	if c.auth.Username != "" {
		params.Add("account", c.auth.Username)
	}

	if service, ok := challenge.Parameters["service"]; ok && service != "" {
		params.Add("service", service)
	}

	for _, scope := range scopes {
		if scope.remoteName != "" && scope.actions != "" {
			params.Add("scope", fmt.Sprintf("repository:%s:%s", scope.remoteName, scope.actions))
		}
	}

	authReq.URL.RawQuery = params.Encode()

	if c.auth.Username != "" && c.auth.Password != "" {
		authReq.SetBasicAuth(c.auth.Username, c.auth.Password)
	}
	if c.sys != nil && c.sys.DockerRegistryUserAgent != "" {
		authReq.Header.Add("User-Agent", c.sys.DockerRegistryUserAgent)
	}

	logrus.Debugf("%s %s", authReq.Method, authReq.URL.String())
	res, err := c.client.Do(authReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := httpResponseToError(res, "Requesting bear token"); err != nil {
		return nil, err
	}
	tokenBlob, err := iolimits.ReadAtMost(res.Body, iolimits.MaxAuthTokenBodySize)
	if err != nil {
		return nil, err
	}

	return newBearerTokenFromJSONBlob(tokenBlob)
}

// detectPropertiesHelper performs the work of detectProperties which executes
// it at most once.
func (c *dockerClient) detectPropertiesHelper(ctx context.Context) error {
	// We overwrite the TLS clients `InsecureSkipVerify` only if explicitly
	// specified by the system context
	if c.sys != nil && c.sys.DockerInsecureSkipTLSVerify != types.OptionalBoolUndefined {
		c.tlsClientConfig.InsecureSkipVerify = c.sys.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue
	}
	tr := tlsclientconfig.NewTransport()
	tr.TLSClientConfig = c.tlsClientConfig
	c.client = &http.Client{Transport: tr}

	ping := func(scheme string) error {
		url := fmt.Sprintf(resolvedPingV2URL, scheme, c.registry)
		resp, err := c.makeRequestToResolvedURL(ctx, "GET", url, nil, nil, -1, noAuth, nil)
		if err != nil {
			logrus.Debugf("Ping %s err %s (%#v)", url, err.Error(), err)
			return err
		}
		defer resp.Body.Close()
		logrus.Debugf("Ping %s status %d", url, resp.StatusCode)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
			return httpResponseToError(resp, "")
		}
		c.challenges = parseAuthHeader(resp.Header)
		c.scheme = scheme
		c.supportsSignatures = resp.Header.Get("X-Registry-Supports-Signatures") == "1"
		return nil
	}
	err := ping("https")
	if err != nil && c.tlsClientConfig.InsecureSkipVerify {
		err = ping("http")
	}
	if err != nil {
		err = errors.Wrapf(err, "error pinging docker registry %s", c.registry)
		if c.sys != nil && c.sys.DockerDisableV1Ping {
			return err
		}
		// best effort to understand if we're talking to a V1 registry
		pingV1 := func(scheme string) bool {
			url := fmt.Sprintf(resolvedPingV1URL, scheme, c.registry)
			resp, err := c.makeRequestToResolvedURL(ctx, "GET", url, nil, nil, -1, noAuth, nil)
			if err != nil {
				logrus.Debugf("Ping %s err %s (%#v)", url, err.Error(), err)
				return false
			}
			defer resp.Body.Close()
			logrus.Debugf("Ping %s status %d", url, resp.StatusCode)
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
				return false
			}
			return true
		}
		isV1 := pingV1("https")
		if !isV1 && c.tlsClientConfig.InsecureSkipVerify {
			isV1 = pingV1("http")
		}
		if isV1 {
			err = ErrV1NotSupported
		}
	}
	return err
}

// detectProperties detects various properties of the registry.
// See the dockerClient documentation for members which are affected by this.
func (c *dockerClient) detectProperties(ctx context.Context) error {
	c.detectPropertiesOnce.Do(func() { c.detectPropertiesError = c.detectPropertiesHelper(ctx) })
	return c.detectPropertiesError
}

// getExtensionsSignatures returns signatures from the X-Registry-Supports-Signatures API extension,
// using the original data structures.
func (c *dockerClient) getExtensionsSignatures(ctx context.Context, ref dockerReference, manifestDigest digest.Digest) (*extensionSignatureList, error) {
	path := fmt.Sprintf(extensionsSignaturePath, reference.Path(ref.ref), manifestDigest)
	res, err := c.makeRequest(ctx, "GET", path, nil, nil, v2Auth, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.Wrapf(clientLib.HandleErrorResponse(res), "Error downloading signatures for %s in %s", manifestDigest, ref.ref.Name())
	}

	body, err := iolimits.ReadAtMost(res.Body, iolimits.MaxSignatureListBodySize)
	if err != nil {
		return nil, err
	}

	var parsedBody extensionSignatureList
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		return nil, errors.Wrapf(err, "Error decoding signature list")
	}
	return &parsedBody, nil
}
