package handlers

import (
	"fmt"
	"net/http"

	"github.com/containers/buildah"
)

// Ping returns headers to client about the service
//
// This handler must always be the same for the compatibility and libpod URL trees!
// Clients will use the Header availability to test which backend engine is in use.
func Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("API-Version", DefaultApiVersion)
	w.Header().Set("BuildKit-Version", "")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")

	// API-Version and Libpod-API-Version may not always be equal
	w.Header().Set("Libpod-API-Version", DefaultApiVersion)
	w.Header().Set("Libpod-Buildha-Version", buildah.Version)
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		fmt.Fprint(w, "OK")
	}
	fmt.Fprint(w, "\n")
}
