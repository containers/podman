package serviceapi

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Error formats an API response to an error
// apiMessage and code must match the container API
// err is logged on the system running the podman service
func Error(w http.ResponseWriter, apiMessage string, code int, err error) {
	// Log detailed message of what happened to machine running podman service
	log.Errorf(err.Error())

	w.(ServiceWriter).WriteJSON(code, struct {
		message string
	}{
		apiMessage,
	})
}
