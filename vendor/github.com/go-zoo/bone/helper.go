/********************************
*** Multiplexer for Go        ***
*** Bone is under MIT license ***
*** Code by CodingFerret      ***
*** github.com/go-zoo         ***
*********************************/

package bone

import (
	"net/http"
	"net/url"
	"strings"
)

func (m *Mux) ListenAndServe(port string) error {
	return http.ListenAndServe(port, m)
}

func (m *Mux) parse(rw http.ResponseWriter, req *http.Request) bool {
	for _, r := range m.Routes[req.Method] {
		ok := r.parse(rw, req)
		if ok {
			return true
		}
	}
	// If no HEAD method, default to GET
	if req.Method == "HEAD" {
		for _, r := range m.Routes["GET"] {
			ok := r.parse(rw, req)
			if ok {
				return true
			}
		}
	}
	return false
}

// StaticRoute check if the request path is for Static route
func (m *Mux) staticRoute(rw http.ResponseWriter, req *http.Request) bool {
	for _, s := range m.Routes[static] {
		if len(req.URL.Path) >= s.Size {
			if req.URL.Path[:s.Size] == s.Path {
				s.Handler.ServeHTTP(rw, req)
				return true
			}
		}
	}
	return false
}

// HandleNotFound handle when a request does not match a registered handler.
func (m *Mux) HandleNotFound(rw http.ResponseWriter, req *http.Request) {
	if m.notFound != nil {
		m.notFound.ServeHTTP(rw, req)
	} else {
		http.NotFound(rw, req)
	}
}

// Check if the path don't end with a /
func (m *Mux) validate(rw http.ResponseWriter, req *http.Request) bool {
	plen := len(req.URL.Path)
	if plen > 1 && req.URL.Path[plen-1:] == "/" {
		cleanURL(&req.URL.Path)
		rw.Header().Set("Location", req.URL.String())
		rw.WriteHeader(http.StatusFound)
		return true
	}
	// Retry to find a route that match
	return m.parse(rw, req)
}

func valid(path string) bool {
	plen := len(path)
	if plen > 1 && path[plen-1:] == "/" {
		return false
	}
	return true
}

// Clean url path
func cleanURL(url *string) {
	ulen := len((*url))
	if ulen > 1 {
		if (*url)[ulen-1:] == "/" {
			*url = (*url)[:ulen-1]
			cleanURL(url)
		}
	}
}

// GetValue return the key value, of the current *http.Request
func GetValue(req *http.Request, key string) string {
	return GetAllValues(req)[key]
}

// GetRequestRoute returns the route of given Request
func (m *Mux) GetRequestRoute(req *http.Request) string {
	cleanURL(&req.URL.Path)
	for _, r := range m.Routes[req.Method] {
		if r.Atts != 0 {
			if r.Atts&SUB != 0 {
				return r.Handler.(*Mux).GetRequestRoute(req)
			}
			if r.Match(req) {
				return r.Path
			}
		}
		if req.URL.Path == r.Path {
			return r.Path
		}
	}

	for _, s := range m.Routes[static] {
		if len(req.URL.Path) >= s.Size {
			if req.URL.Path[:s.Size] == s.Path {
				return s.Path
			}
		}
	}

	return "NotFound"
}

// GetQuery return the key value, of the current *http.Request query
func GetQuery(req *http.Request, key string) []string {
	if ok, value := extractQueries(req); ok {
		return value[key]
	}
	return nil
}

// GetAllQueries return all queries of the current *http.Request
func GetAllQueries(req *http.Request) map[string][]string {
	if ok, values := extractQueries(req); ok {
		return values
	}
	return nil
}

func extractQueries(req *http.Request) (bool, map[string][]string) {
	if q, err := url.ParseQuery(req.URL.RawQuery); err == nil {
		var queries = make(map[string][]string)
		for k, v := range q {
			for _, item := range v {
				values := strings.Split(item, ",")
				queries[k] = append(queries[k], values...)
			}
		}
		return true, queries
	}
	return false, nil
}

func (m *Mux) otherMethods(rw http.ResponseWriter, req *http.Request) bool {
	for _, met := range method {
		if met != req.Method {
			for _, r := range m.Routes[met] {
				ok := r.exists(rw, req)
				if ok {
					rw.WriteHeader(http.StatusMethodNotAllowed)
					return true
				}
			}
		}
	}
	return false
}
