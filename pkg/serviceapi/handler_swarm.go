package serviceapi

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

func registerSwarmHandlers(r *mux.Router) error {
	r.HandleFunc(unversionedPath("/swarm"), noSwarm)
	r.HandleFunc(unversionedPath("/services"), noSwarm)
	r.HandleFunc(unversionedPath("/nodes"), noSwarm)
	r.HandleFunc(unversionedPath("/tasks"), noSwarm)
	r.HandleFunc(unversionedPath("/secrets"), noSwarm)
	r.HandleFunc(unversionedPath("/configs"), noSwarm)
	return nil
}

// noSwarm returns http.StatusServiceUnavailable rather than something like http.StatusInternalServerError,
// this allows the client to decide if they still can talk to us
func noSwarm(w http.ResponseWriter, r *http.Request) {
	Error(w, "node is not part of a swarm", http.StatusServiceUnavailable, errors.New("swarm is not supported by podman"))
}
