package compat

import (
	"errors"
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/utils"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "not implemented", http.StatusNotImplemented, errors.New("not implemented"))
}
