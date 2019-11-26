package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Error formats an API response to an error
// apiMessage and code must match the container API
// err is logged on the system running the podman service
func Error(w http.ResponseWriter, apiMessage string, code int, err error) {
	// Log detailed message of what happened to machine running podman service
	log.Infof(err.Error())
	w.WriteHeader(code)
	WriteJSON(w, struct {
		Message string `json:"message"`
	}{
		apiMessage,
	})
}

func containerNotFound(w http.ResponseWriter, nameOrId string, err error) {
	msg := fmt.Sprintf("No such container: %s", nameOrId)
	Error(w, msg, http.StatusNotFound, err)
}

func imageNotFound(w http.ResponseWriter, nameOrId string, err error) {
	msg := fmt.Sprintf("No such image: %s", nameOrId)
	Error(w, msg, http.StatusNotFound, err)
}

func podNotFound(w http.ResponseWriter, nameOrId string, err error) {
	msg := fmt.Sprintf("No such pod: %s", nameOrId)
	Error(w, msg, http.StatusNotFound, err)
}

func containerNotRunning(w http.ResponseWriter, containerID string, err error) {
	msg := fmt.Sprintf("Container %s is not running", containerID)
	Error(w, msg, http.StatusConflict, err)
}

func internalServerError(w http.ResponseWriter, err error) {
	Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, err)
}

func badRequest(w http.ResponseWriter, key string, value string, err error) {
	e := errors.Wrapf(err, "Failed to parse query parameter '%s': %q", key, value)
	Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, e)
}
