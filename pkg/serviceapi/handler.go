package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/sirupsen/logrus"
)

// serviceHandler type defines a specialized http.Handler, included is the podman runtime
type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

// ServeHTTP will be called from the router when a request is made
func (h serviceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		logrus.Errorf("%s is not a supported Content-Type", r.Header.Get("Content-Type"))
	}

	// Call our specialized handler
	h(w, r, libpodRuntime)
}
