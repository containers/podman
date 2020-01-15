package bindings

import (
	"fmt"
	"io"
	"net/http"
)

const (
	defaultConnection string = "http://localhost:8080/v1.24/libpod"
	pingConnection    string = "http://localhost:8080/_ping"
)

type APIResponse struct {
	*http.Response
	Request *http.Request
}

type Connection struct {
	url    string
	client *http.Client
}

func NewConnection(url string) (Connection, error) {
	if len(url) < 1 {
		url = defaultConnection
	}
	newConn := Connection{
		url:    url,
		client: &http.Client{},
	}
	response, err := http.Get(pingConnection)
	if err != nil {
		return newConn, err
	}
	if err := response.Body.Close(); err != nil {
		return newConn, err
	}
	return newConn, err
}

func (c Connection) makeEndpoint(u string) string {
	return fmt.Sprintf("%s%s", defaultConnection, u)
}

func (c Connection) newRequest(httpMethod, endpoint string, httpBody io.Reader, params map[string]string) (*APIResponse, error) {
	e := c.makeEndpoint(endpoint)
	req, err := http.NewRequest(httpMethod, e, httpBody)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		// if more desirable we could use url to form the encoded endpoint with params
		r := req.URL.Query()
		for k, v := range params {
			r.Add(k, v)
		}
		req.URL.RawQuery = r.Encode()
	}
	response, err := c.client.Do(req) // nolint
	return &APIResponse{response, req}, err
}
