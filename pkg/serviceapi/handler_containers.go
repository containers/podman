package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerContainersHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/containers/"), serviceHandler(containers))
	return nil
}

func containers(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}
