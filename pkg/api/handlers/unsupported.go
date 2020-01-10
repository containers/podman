package handlers

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/utils"
	log "github.com/sirupsen/logrus"
)

func UnsupportedHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Path %s is not supported", r.URL.Path)
	log.Infof("Request Failed: %s", msg)

	utils.WriteJSON(w, http.StatusInternalServerError,
		utils.ErrorModel{Message: msg})
}
