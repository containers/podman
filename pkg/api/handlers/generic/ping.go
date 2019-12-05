package generic

import (
	"fmt"
	"net/http"
)

func PingGET(w http.ResponseWriter, _ *http.Request) {
	setHeaders(w)
	fmt.Fprintln(w, "OK")
}

func PingHEAD(w http.ResponseWriter, _ *http.Request) {
	setHeaders(w)
	fmt.Fprintln(w, "")
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("API-Version", DefaultApiVersion)
	w.Header().Set("BuildKit-Version", "")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
}
