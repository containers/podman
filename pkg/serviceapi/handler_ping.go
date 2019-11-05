package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerPingHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/_ping"), serviceHandler(pingGET)).Methods("GET")
	r.Handle(unversionedPath("/_ping"), serviceHandler(pingHEAD)).Methods("HEAD")
	return nil
}

func pingGET(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	setHeaders(w)
	fmt.Fprintln(w, "OK")
}

func pingHEAD(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	setHeaders(w)
	fmt.Fprintln(w, "(emtpy)")
}

func setHeaders(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("API-Version", DefaultApiVersion)
	w.Header().Set("BuildKit-Version", "")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
}
