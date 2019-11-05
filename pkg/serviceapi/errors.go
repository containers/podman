package serviceapi

import (
	"encoding/json"
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

	// Now prepare API required message
	buffer, err := json.Marshal(struct {
		message string
	}{
		apiMessage,
	})
	if err != nil {
		// Log error and abandon connection, rather than sending bad JSON to client
		log.Errorf(err.Error())
		return
	}

	// API HTTP code for error
	w.WriteHeader(code)

	// Finally do our best to send API message to client
	fmt.Fprintln(w, string(buffer))
}
