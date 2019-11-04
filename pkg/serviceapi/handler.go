package serviceapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
)

type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

func (h serviceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		apiError(w,
			fmt.Sprintf("%s is not a supported Content-Type", r.Header.Get("Content-Type")),
			http.StatusUnsupportedMediaType)
		return
	}

	h(w, r, libpodRuntime)
}

func apiError(w http.ResponseWriter, error string, code int) {
	msg := struct {
		message string
	}{
		error,
	}

	w.WriteHeader(code)
	fmt.Fprintln(w, json.Marshal(msg))
}
