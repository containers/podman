package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/libpod/libpod"
)

func sendUnimplemented(w http.ResponseWriter, msg string) error {
	buffer, err := json.Marshal(struct{ message string }{msg})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusInternalServerError)
	_, err = io.WriteString(w, string(buffer))
	return err
}

func unsupportedHandler(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	if err := sendUnimplemented(w, fmt.Sprintf("Path %s is not supported", r.URL.Path)); err != nil {
		Error(w, fmt.Sprintf("Requested Path '%s' is not supported", r.URL.Path), http.StatusUnsupportedMediaType, err)
	}
	return
}
