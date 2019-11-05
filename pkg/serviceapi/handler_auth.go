package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerAuthHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/auth"), serviceHandler(authHandler))
	return nil
}

func authHandler(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.Error(w, "Server error", http.StatusInternalServerError)
}
