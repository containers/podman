package serviceapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
)

type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

type message struct {
	Message string `json:"message"`
}

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

func apiResponse(w http.ResponseWriter, code int, msg message) {
	w.WriteHeader(code)
	b, _ := json.Marshal(msg)
	fmt.Fprintln(w, b)
}

func apiError(w http.ResponseWriter, error string, code int) {
	msg := message{Message: error}
	apiResponse(w, code, msg)
}

func noSuchContainerError(w http.ResponseWriter, nameOrId string) {
	msg := message{
		Message: fmt.Sprintf("No such container: %s", nameOrId),
	}
	apiResponse(w, http.StatusNotFound, msg)
}

func noSuchImageError(w http.ResponseWriter, nameOrId string) {
	msg := message{
		Message: fmt.Sprintf("No such image: %s", nameOrId),
	}
	apiResponse(w, http.StatusNotFound, msg)
}

func containerNotRunningError(w http.ResponseWriter, nameOrId string) {
	msg := message{
		Message: fmt.Sprintf("Container %s is not running", nameOrId),
	}
	apiResponse(w, http.StatusConflict, msg)
}
