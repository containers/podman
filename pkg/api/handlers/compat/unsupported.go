package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	"github.com/containers/podman/v4/pkg/errorhandling"
	log "github.com/sirupsen/logrus"
)

func UnsupportedHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Path %s is not supported", r.URL.Path)
	log.Infof("Request Failed: %s", msg)

	utils.WriteJSON(w, http.StatusNotFound, errorhandling.ErrorModel{Message: msg})
}
