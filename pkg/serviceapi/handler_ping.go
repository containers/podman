package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *APIServer) registerPingHandlers(r *mux.Router) error {
	r.Handle("/_ping", s.serviceHandler(pingGET)).Methods("GET")
	r.Handle("/_ping", s.serviceHandler(pingHEAD)).Methods("HEAD")
	return nil
}

func pingGET(w http.ResponseWriter, _ *http.Request) {
	setHeaders(w)
	fmt.Fprintln(w, "OK")
}

func pingHEAD(w http.ResponseWriter, _ *http.Request) {
	setHeaders(w)
	fmt.Fprintln(w, "")
}

func setHeaders(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("API-Version", DefaultApiVersion)
	w.Header().Set("BuildKit-Version", "")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
}
