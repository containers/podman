// +build !go1.7

/********************************
*** Multiplexer for Go        ***
*** Bone is under MIT license ***
*** Code by CodingFerret      ***
*** github.com/go-zoo         ***
*********************************/

package bone

import (
	"net/http"
	"sync"
)

var globalVars = struct {
	sync.RWMutex
	v map[*http.Request]map[string]string
}{v: make(map[*http.Request]map[string]string)}

// GetAllValues return the req PARAMs
func GetAllValues(req *http.Request) map[string]string {
	globalVars.RLock()
	values := globalVars.v[req]
	globalVars.RUnlock()
	return values
}

// serveMatchedRequest is an extension point for Route which allows us to conditionally compile for
// go1.7 and <go1.7
func (r *Route) serveMatchedRequest(rw http.ResponseWriter, req *http.Request, vars map[string]string) {
	globalVars.Lock()
	globalVars.v[req] = vars
	globalVars.Unlock()

	// Regardless if ServeHTTP panics (and potentially recovers) we can make sure to not leak
	// memory in globalVars for this request
	defer func() {
		globalVars.Lock()
		delete(globalVars.v, req)
		globalVars.Unlock()
	}()
	r.Handler.ServeHTTP(rw, req)
}
