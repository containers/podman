package bindings

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/api/handlers"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

var (
	defaultConnectionPath string = filepath.Join(fmt.Sprintf("v%s", handlers.MinimalApiVersion), "libpod")
)

type APIResponse struct {
	*http.Response
	Request *http.Request
}

type Connection struct {
	scheme  string
	address string
	client  *http.Client
}

// NewConnection takes a URI as a string and returns a context with the
// Connection embedded as a value.  This context needs to be passed to each
// endpoint to work correctly.
//
// A valid URI connection should be scheme://
// For example tcp://localhost:<port>
// or unix://run/podman/podman.sock
func NewConnection(uri string) (context.Context, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	// TODO once ssh is implemented, remove this block and
	// add it to the conditional beneath it
	if u.Scheme == "ssh" {
		return nil, ErrNotImplemented
	}
	if u.Scheme != "tcp" && u.Scheme != "unix" {
		return nil, errors.Errorf("%s is not a support schema", u.Scheme)
	}

	if u.Scheme == "tcp" && !strings.HasPrefix(uri, "tcp://") {
		return nil, errors.New("tcp URIs should begin with tcp://")
	}

	address := u.Path
	if u.Scheme == "tcp" {
		address = u.Host
	}
	newConn := newConnection(u.Scheme, address)
	ctx := context.WithValue(context.Background(), "conn", &newConn)
	if err := pingNewConnection(ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

// pingNewConnection pings to make sure the RESTFUL service is up
// and running. it should only be used where initializing a connection
func pingNewConnection(ctx context.Context) error {
	conn, err := GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	// the ping endpoint sits at / in this case
	response, err := conn.DoRequest(nil, http.MethodGet, "../../../_ping", nil)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusOK {
		return nil
	}
	return errors.Errorf("ping response was %q", response.StatusCode)
}

// newConnection takes a scheme and address and creates a connection from it
func newConnection(scheme, address string) Connection {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(scheme, address)
			},
		},
	}
	newConn := Connection{
		client:  &client,
		address: address,
		scheme:  scheme,
	}
	return newConn
}

func (c *Connection) makeEndpoint(u string) string {
	// The d character in the url is discarded and is meaningless
	return fmt.Sprintf("http://d/%s%s", defaultConnectionPath, u)
}

// DoRequest assembles the http request and returns the response
func (c *Connection) DoRequest(httpBody io.Reader, httpMethod, endpoint string, queryParams map[string]string, pathValues ...string) (*APIResponse, error) {
	var (
		err      error
		response *http.Response
	)
	safePathValues := make([]interface{}, len(pathValues))
	// Make sure path values are http url safe
	for i, pv := range pathValues {
		safePathValues[i] = url.QueryEscape(pv)
	}
	// Lets eventually use URL for this which might lead to safer
	// usage
	safeEndpoint := fmt.Sprintf(endpoint, safePathValues...)
	e := c.makeEndpoint(safeEndpoint)
	req, err := http.NewRequest(httpMethod, e, httpBody)
	if err != nil {
		return nil, err
	}
	if len(queryParams) > 0 {
		// if more desirable we could use url to form the encoded endpoint with params
		r := req.URL.Query()
		for k, v := range queryParams {
			r.Add(k, url.QueryEscape(v))
		}
		req.URL.RawQuery = r.Encode()
	}
	// Give the Do three chances in the case of a comm/service hiccup
	for i := 0; i < 3; i++ {
		response, err = c.client.Do(req) // nolint
		if err == nil {
			break
		}
	}
	return &APIResponse{response, req}, err
}

// GetConnectionFromContext returns a bindings connection from the context
// being passed into each method.
func GetConnectionFromContext(ctx context.Context) (*Connection, error) {
	c := ctx.Value("conn")
	if c == nil {
		return nil, errors.New("unable to get connection from context")
	}
	conn := c.(*Connection)
	return conn, nil
}

// FiltersToHTML converts our typical filter format of a
// map[string][]string to a query/html safe string.
func FiltersToHTML(filters map[string][]string) (string, error) {
	lowerCaseKeys := make(map[string][]string)
	for k, v := range filters {
		lowerCaseKeys[strings.ToLower(k)] = v
	}
	unsafeString, err := jsoniter.MarshalToString(lowerCaseKeys)
	if err != nil {
		return "", err
	}
	return url.QueryEscape(unsafeString), nil
}

// IsInformation returns true if the response code is 1xx
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

// IsServerError returns true if the response code is 5xx
func (h *APIResponse) IsServerError() bool {
	return h.Response.StatusCode/100 == 5
}
