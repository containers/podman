package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerPluginsHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/plugins"), serviceHandler(plugins))
	return nil
}

func plugins(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.Error(w, "Server error", http.StatusInternalServerError)
}
