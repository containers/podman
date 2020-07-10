package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v2/pkg/domain/entities"

	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	log "github.com/sirupsen/logrus"
)

func UnsupportedHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Path %s is not supported", r.URL.Path)
	log.Infof("Request Failed: %s", msg)

	utils.WriteJSON(w, http.StatusInternalServerError,
		entities.ErrorModel{Message: msg})
}
