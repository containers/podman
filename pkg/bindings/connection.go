package bindings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/containers/common/pkg/ssh"
	"github.com/containers/podman/v4/version"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

type APIResponse struct {
	*http.Response
	Request *http.Request
}

type Connection struct {
	URI    *url.URL
	Client *http.Client
}

type valueKey string

const (
	clientKey  = valueKey("Client")
	versionKey = valueKey("ServiceVersion")
)

// GetClient from context build by NewConnection()
func GetClient(ctx context.Context) (*Connection, error) {
	if c, ok := ctx.Value(clientKey).(*Connection); ok {
		return c, nil
	}
	return nil, fmt.Errorf("%s not set in context", clientKey)
}

// ServiceVersion from context build by NewConnection()
func ServiceVersion(ctx context.Context) *semver.Version {
	if v, ok := ctx.Value(versionKey).(*semver.Version); ok {
		return v
	}
	return new(semver.Version)
}

// JoinURL elements with '/'
func JoinURL(elements ...string) string {
	return "/" + strings.Join(elements, "/")
}

// NewConnection creates a new service connection without an identity
func NewConnection(ctx context.Context, uri string) (context.Context, error) {
	return NewConnectionWithIdentity(ctx, uri, "", false)
}

// NewConnectionWithIdentity takes a URI as a string and returns a context with the
// Connection embedded as a value.  This context needs to be passed to each
// endpoint to work correctly.
//
// A valid URI connection should be scheme://
// For example tcp://localhost:<port>
// or unix:///run/podman/podman.sock
// or ssh://<user>@<host>[:port]/run/podman/podman.sock?secure=True
func NewConnectionWithIdentity(ctx context.Context, uri string, identity string, machine bool) (context.Context, error) {
	var (
		err error
	)
	if v, found := os.LookupEnv("CONTAINER_HOST"); found && uri == "" {
		uri = v
	}

	if v, found := os.LookupEnv("CONTAINER_SSHKEY"); found && len(identity) == 0 {
		identity = v
	}

	_url, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("value of CONTAINER_HOST is not a valid url: %s: %w", uri, err)
	}

	// Now we set up the http Client to use the connection above
	var connection Connection
	switch _url.Scheme {
	case "ssh":
		port := 22
		if _url.Port() != "" {
			port, err = strconv.Atoi(_url.Port())
			if err != nil {
				return nil, err
			}
		}
		conn, err := ssh.Dial(&ssh.ConnectionDialOptions{
			Host:                        uri,
			Identity:                    identity,
			User:                        _url.User,
			Port:                        port,
			InsecureIsMachineConnection: machine,
		}, "golang")
		if err != nil {
			return nil, err
		}
		connection = Connection{URI: _url}
		connection.Client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return ssh.DialNet(conn, "unix", _url)
				},
			}}
	case "unix":
		if !strings.HasPrefix(uri, "unix:///") {
			// autofix unix://path_element vs unix:///path_element
			_url.Path = JoinURL(_url.Host, _url.Path)
			_url.Host = ""
		}
		connection = unixClient(_url)
	case "tcp":
		if !strings.HasPrefix(uri, "tcp://") {
			return nil, errors.New("tcp URIs should begin with tcp://")
		}
		conn, err := tcpClient(_url)
		if err != nil {
			return nil, err
		}
		connection = conn
	default:
		return nil, fmt.Errorf("unable to create connection. %q is not a supported schema", _url.Scheme)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to connect to Podman. failed to create %sClient: %w", _url.Scheme, err)
	}

	ctx = context.WithValue(ctx, clientKey, &connection)
	serviceVersion, err := pingNewConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to Podman socket: %w", err)
	}
	ctx = context.WithValue(ctx, versionKey, serviceVersion)
	return ctx, nil
}

func tcpClient(_url *url.URL) (Connection, error) {
	connection := Connection{
		URI: _url,
	}
	dialContext := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("tcp", _url.Host)
	}
	// use proxy if env `CONTAINER_PROXY` set
	if proxyURI, found := os.LookupEnv("CONTAINER_PROXY"); found {
		proxyURL, err := url.Parse(proxyURI)
		if err != nil {
			return connection, fmt.Errorf("value of CONTAINER_PROXY is not a valid url: %s: %w", proxyURI, err)
		}
		proxyDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return connection, fmt.Errorf("unable to dial to proxy %s, %w", proxyURI, err)
		}
		dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			logrus.Debugf("use proxy %s, but proxy dialer does not support dial timeout", proxyURI)
			return proxyDialer.Dial("tcp", _url.Host)
		}
		if f, ok := proxyDialer.(proxy.ContextDialer); ok {
			dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
				// the default tcp dial timeout seems to be 75s, podman-remote will retry 3 times before exit.
				// here we change proxy dial timeout to 3s
				logrus.Debugf("use proxy %s with dial timeout 3s", proxyURI)
				ctx, cancel := context.WithTimeout(ctx, time.Second*3)
				defer cancel() // It's safe to cancel, `f.DialContext` only use ctx for returning the Conn, not the lifetime of the Conn.
				return f.DialContext(ctx, "tcp", _url.Host)
			}
		}
	}
	connection.Client = &http.Client{
		Transport: &http.Transport{
			DialContext:        dialContext,
			DisableCompression: true,
		},
	}
	return connection, nil
}

// pingNewConnection pings to make sure the RESTFUL service is up
// and running. it should only be used when initializing a connection
func pingNewConnection(ctx context.Context) (*semver.Version, error) {
	client, err := GetClient(ctx)
	if err != nil {
		return nil, err
	}
	// the ping endpoint sits at / in this case
	response, err := client.DoRequest(ctx, nil, http.MethodGet, "/_ping", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		versionHdr := response.Header.Get("Libpod-API-Version")
		if versionHdr == "" {
			logrus.Warn("Service did not provide Libpod-API-Version Header")
			return new(semver.Version), nil
		}
		versionSrv, err := semver.ParseTolerant(versionHdr)
		if err != nil {
			return nil, err
		}

		switch version.APIVersion[version.Libpod][version.MinimalAPI].Compare(versionSrv) {
		case -1, 0:
			// Server's job when Client version is equal or older
			return &versionSrv, nil
		case 1:
			return nil, fmt.Errorf("server API version is too old. Client %q server %q",
				version.APIVersion[version.Libpod][version.MinimalAPI].String(), versionSrv.String())
		}
	}
	return nil, fmt.Errorf("ping response was %d", response.StatusCode)
}

func unixClient(_url *url.URL) Connection {
	connection := Connection{URI: _url}
	connection.Client = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", _url.Path)
			},
			DisableCompression: true,
		},
	}
	return connection
}

// DoRequest assembles the http request and returns the response.
// The caller must close the response body.
func (c *Connection) DoRequest(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (*APIResponse, error) {
	var (
		err      error
		response *http.Response
	)

	params := make([]interface{}, len(pathValues)+1)

	if v := headers.Values("API-Version"); len(v) > 0 {
		params[0] = v[0]
	} else {
		// Including the semver suffices breaks older services... so do not include them
		v := version.APIVersion[version.Libpod][version.CurrentAPI]
		params[0] = fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}

	for i, pv := range pathValues {
		// url.URL lacks the semantics for escaping embedded path parameters... so we manually
		//   escape each one and assume the caller included the correct formatting in "endpoint"
		params[i+1] = url.PathEscape(pv)
	}

	uri := fmt.Sprintf("http://d/v%s/libpod"+endpoint, params...)
	logrus.Debugf("DoRequest Method: %s URI: %v", httpMethod, uri)

	req, err := http.NewRequestWithContext(ctx, httpMethod, uri, httpBody)
	if err != nil {
		return nil, err
	}
	if len(queryParams) > 0 {
		req.URL.RawQuery = queryParams.Encode()
	}

	for key, val := range headers {
		if key == "API-Version" {
			continue
		}

		for _, v := range val {
			req.Header.Add(key, v)
		}
	}

	// Give the Do three chances in the case of a comm/service hiccup
	for i := 1; i <= 3; i++ {
		response, err = c.Client.Do(req) //nolint:bodyclose // The caller has to close the body.
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i*100) * time.Millisecond)
	}
	return &APIResponse{response, req}, err
}

// GetDialer returns raw Transport.DialContext from client
func (c *Connection) GetDialer(ctx context.Context) (net.Conn, error) {
	client := c.Client
	transport := client.Transport.(*http.Transport)
	if transport.DialContext != nil && transport.TLSClientConfig == nil {
		return transport.DialContext(ctx, c.URI.Scheme, c.URI.String())
	}

	return nil, errors.New("unable to get dial context")
}

// IsInformational returns true if the response code is 1xx
func (h *APIResponse) IsInformational() bool {
	return h.Response.StatusCode/100 == 1
}

// IsSuccess returns true if the response code is 2xx
func (h *APIResponse) IsSuccess() bool {
	return h.Response.StatusCode/100 == 2
}

// IsRedirection returns true if the response code is 3xx
func (h *APIResponse) IsRedirection() bool {
	return h.Response.StatusCode/100 == 3
}

// IsClientError returns true if the response code is 4xx
func (h *APIResponse) IsClientError() bool {
	return h.Response.StatusCode/100 == 4
}

// IsConflictError returns true if the response code is 409
func (h *APIResponse) IsConflictError() bool {
	return h.Response.StatusCode == 409
}

// IsServerError returns true if the response code is 5xx
func (h *APIResponse) IsServerError() bool {
	return h.Response.StatusCode/100 == 5
}
