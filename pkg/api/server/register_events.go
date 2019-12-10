package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterEventsHandlers(r *mux.Router) error {
	// swagger:operation GET /events system getEvents
	// ---
	// summary: Returns events filtered on query parameters
	// produces:
	// - application/json
	// parameters:
	// - name: since
	//   in: query
	//   description: start streaming events from this time
	// - name: until
	//   in: query
	//   description: stop streaming events later than this
	// - name: filters
	//   in: query
	//   description: JSON encoded map[string][]string of constraints
	// responses:
	//   "200":
	//     description: OK
	//   "500":
	//     description: Failed
	//     "$ref": "#/types/errorModel"
	r.Handle(VersionedPath("/events"), APIHandler(s.Context, handlers.GetEvents))
	return nil
}
