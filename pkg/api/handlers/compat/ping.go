package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/buildah"
	"github.com/containers/libpod/pkg/api/handlers/utils"
)

// Ping returns headers to client about the service
//
// This handler must always be the same for the compatibility and libpod URL trees!
// Clients will use the Header availability to test which backend engine is in use.
// Note: Additionally handler supports GET and HEAD methods
func Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("API-Version", utils.ApiVersion[utils.CompatTree][utils.CurrentApiVersion].String())
	w.Header().Set("BuildKit-Version", "")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")

	w.Header().Set("Libpod-API-Version", utils.ApiVersion[utils.LibpodTree][utils.CurrentApiVersion].String())
	w.Header().Set("Libpod-Buildha-Version", buildah.Version)
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		fmt.Fprint(w, "OK")
	}
	fmt.Fprint(w, "\n")
}
