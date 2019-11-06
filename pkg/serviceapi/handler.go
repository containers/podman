package serviceapi

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/sirupsen/logrus"
)

type ServiceWriter struct {
	http.ResponseWriter
}

// serviceHandler type defines a specialized http.Handler, included is the podman runtime
type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

// ServeHTTP will be called from the router when a request is made
func (h serviceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		logrus.Errorf("%s is not a supported Content-Type", r.Header.Get("Content-Type"))
	}
	if err := r.ParseForm(); err != nil {
		logrus.Errorf("unable to parse form: %q", err)
	}
	// Call our specialized handler
	h(ServiceWriter{w}, r, libpodRuntime)
}

func (w ServiceWriter) WriteJSON(code int, value interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(false)
	return coder.Encode(value)
}
