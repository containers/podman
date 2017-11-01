/********************************
*** Multiplexer for Go        ***
*** Bone is under MIT license ***
*** Code by CodingFerret      ***
*** github.com/go-zoo         ***
*********************************/

package bone

import (
	"net/http"
	"strings"
)

// Mux have routes and a notFound handler
// Route: all the registred route
// notFound: 404 handler, default http.NotFound if not provided
type Mux struct {
	Routes        map[string][]*Route
	prefix        string
	notFound      http.Handler
	Serve         func(rw http.ResponseWriter, req *http.Request)
	CaseSensitive bool
}

var (
	static = "static"
	method = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH", "OPTIONS"}
)

type adapter func(*Mux) *Mux

// New create a pointer to a Mux instance
func New(adapters ...adapter) *Mux {
	m := &Mux{Routes: make(map[string][]*Route), Serve: nil, CaseSensitive: true}
	for _, adap := range adapters {
		adap(m)
	}
	if m.Serve == nil {
		m.Serve = m.DefaultServe
	}
	return m
}

// Prefix set a default prefix for all routes registred on the router
func (m *Mux) Prefix(p string) *Mux {
	m.prefix = strings.TrimSuffix(p, "/")
	return m
}

// DefaultServe is the default http request handler
func (m *Mux) DefaultServe(rw http.ResponseWriter, req *http.Request) {
	// Check if a route match
	if !m.parse(rw, req) {
		// Check if it's a static ressource
		if !m.staticRoute(rw, req) {
			// Check if the request path doesn't end with /
			if !m.validate(rw, req) {
				// Check if same route exists for another HTTP method
				if !m.otherMethods(rw, req) {
					m.HandleNotFound(rw, req)
				}
			}
		}
	}
}

// ServeHTTP pass the request to the serve method of Mux
func (m *Mux) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !m.CaseSensitive {
		req.URL.Path = strings.ToLower(req.URL.Path)
	}
	m.Serve(rw, req)
}
