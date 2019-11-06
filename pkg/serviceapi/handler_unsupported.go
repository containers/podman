package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
)

func unsupportedHandler(w http.ResponseWriter, r *http.Request, _ *libpod.Runtime) {
	w.(ServiceWriter).WriteJSON(http.StatusInternalServerError, struct {
		message string
	}{
		message: fmt.Sprintf("Path %s is not supported", r.URL.Path),
	})
}
