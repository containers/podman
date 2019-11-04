package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/libpod/libpod"
)

type unimplemented struct {
	Message string `json:"message"`
}

func sendUnimplemented(w http.ResponseWriter, msg string) error {
	u := unimplemented{Message: msg}
	buffer, err := json.Marshal(u)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusInternalServerError)
	_, err = io.WriteString(w, string(buffer))
	return err
}

func renameContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	if err := sendUnimplemented(w, "/containers/name/rename not implemented"); err != nil {
		http.Error(w,
			fmt.Sprintf("%s", err.Error()), http.StatusUnsupportedMediaType)
		return
	}
	return
}
