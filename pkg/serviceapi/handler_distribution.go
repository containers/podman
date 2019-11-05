package serviceapi

import (
	"errors"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerDistributionHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/distribution/{name:..*}/json"), serviceHandler(distributionHandler))
	return nil
}

func distributionHandler(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	Error(w, "Server error", http.StatusInternalServerError, errors.New("not implemented"))
}
