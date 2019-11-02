package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
)

type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

func (h serviceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r, libpodRuntime)
}
