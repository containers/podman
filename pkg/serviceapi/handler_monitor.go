package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerMonitorHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/monitor"), serviceHandler(monitorHandler))
	return nil
}

func monitorHandler(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.Error(w, "Not implemented.", http.StatusInternalServerError)
}
