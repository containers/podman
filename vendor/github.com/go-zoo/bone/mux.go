/********************************
*** Multiplexer for Go        ***
*** Bone is under MIT license ***
*** Code by CodingFerret      ***
*** github.com/go-zoo         ***
*********************************/

package bone

import "net/http"

// Router is the same as a http.Handler
type Router interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Register the route in the router
func (m *Mux) Register(method string, path string, handler http.Handler) *Route {
	return m.register(method, path, handler)
}

// GetFunc add a new route to the Mux with the Get method
func (m *Mux) GetFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("GET", path, handler)
}

// PostFunc add a new route to the Mux with the Post method
func (m *Mux) PostFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("POST", path, handler)
}

// PutFunc add a new route to the Mux with the Put method
func (m *Mux) PutFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("PUT", path, handler)
}

// DeleteFunc add a new route to the Mux with the Delete method
func (m *Mux) DeleteFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("DELETE", path, handler)
}

// HeadFunc add a new route to the Mux with the Head method
func (m *Mux) HeadFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("HEAD", path, handler)
}

// PatchFunc add a new route to the Mux with the Patch method
func (m *Mux) PatchFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("PATCH", path, handler)
}

// OptionsFunc add a new route to the Mux with the Options method
func (m *Mux) OptionsFunc(path string, handler http.HandlerFunc) *Route {
	return m.register("OPTIONS", path, handler)
}

// NotFoundFunc the mux custom 404 handler
func (m *Mux) NotFoundFunc(handler http.HandlerFunc) {
	m.notFound = handler
}

// Handle add a new route to the Mux without a HTTP method
func (m *Mux) Handle(path string, handler http.Handler) {
	for _, mt := range method {
		m.register(mt, path, handler)
	}
}

// HandleFunc is use to pass a func(http.ResponseWriter, *Http.Request) instead of http.Handler
func (m *Mux) HandleFunc(path string, handler http.HandlerFunc) {
	m.Handle(path, handler)
}

// Get add a new route to the Mux with the Get method
func (m *Mux) Get(path string, handler http.Handler) *Route {
	return m.register("GET", path, handler)
}

// Post add a new route to the Mux with the Post method
func (m *Mux) Post(path string, handler http.Handler) *Route {
	return m.register("POST", path, handler)
}

// Put add a new route to the Mux with the Put method
func (m *Mux) Put(path string, handler http.Handler) *Route {
	return m.register("PUT", path, handler)
}

// Delete add a new route to the Mux with the Delete method
func (m *Mux) Delete(path string, handler http.Handler) *Route {
	return m.register("DELETE", path, handler)
}

// Head add a new route to the Mux with the Head method
func (m *Mux) Head(path string, handler http.Handler) *Route {
	return m.register("HEAD", path, handler)
}

// Patch add a new route to the Mux with the Patch method
func (m *Mux) Patch(path string, handler http.Handler) *Route {
	return m.register("PATCH", path, handler)
}

// Options add a new route to the Mux with the Options method
func (m *Mux) Options(path string, handler http.Handler) *Route {
	return m.register("OPTIONS", path, handler)
}

// NotFound the mux custom 404 handler
func (m *Mux) NotFound(handler http.Handler) {
	m.notFound = handler
}

// Register the new route in the router with the provided method and handler
func (m *Mux) register(method string, path string, handler http.Handler) *Route {
	r := NewRoute(m.prefix+path, handler)
	r.Method = method
	if valid(path) {
		m.Routes[method] = append(m.Routes[method], r)
		return r
	}
	m.Routes[static] = append(m.Routes[static], r)
	return r
}

// SubRoute register a router as a SubRouter of bone
func (m *Mux) SubRoute(path string, router Router) *Route {
	r := NewRoute(m.prefix+path, router)
	if valid(path) {
		r.Atts += SUB
		for _, mt := range method {
			m.Routes[mt] = append(m.Routes[mt], r)
		}
		return r
	}
	return nil
}
