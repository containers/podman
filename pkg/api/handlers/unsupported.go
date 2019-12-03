package handlers

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func UnsupportedHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Path %s is not supported", r.URL.Path)
	log.Infof("Request Failed: %s", msg)

	WriteJSON(w, http.StatusInternalServerError,
		ErrorModel{msg})
}
