package handlers

import (
	"net/http"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
)

func CreateExec(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "function not implemented", http.StatusInternalServerError, define.ErrNotImplemented)
}

func StartExec(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "function not implemented", http.StatusInternalServerError, define.ErrNotImplemented)
}

func ResizeExec(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "function not implemented", http.StatusInternalServerError, define.ErrNotImplemented)

}

func InspectExec(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "function not implemented", http.StatusInternalServerError, define.ErrNotImplemented)
}
