package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
)

func containers(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}
