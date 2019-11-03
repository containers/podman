package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerPodsHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/pods/"), serviceHandler(pods))
	return nil
}

func pods(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}
