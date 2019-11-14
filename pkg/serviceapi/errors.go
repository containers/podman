package serviceapi

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Error formats an API response to an error
// apiMessage and code must match the container API
// err is logged on the system running the podman service
func Error(w http.ResponseWriter, apiMessage string, code int, err error) {
	// Log detailed message of what happened to machine running podman service
	log.Errorf(err.Error())
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
