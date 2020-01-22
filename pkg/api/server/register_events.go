package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterEventsHandlers(r *mux.Router) error {
	// swagger:operation GET /events system getEvents
	// ---
	// tags:
	//   - system
	// summary: Returns events filtered on query parameters
	// description: Returns events filtered on query parameters
	// produces:
	// - application/json
	// parameters:
	// - name: since
	//   type: string
	//   in: query
	//   description: start streaming events from this time
	// - name: until
	//   type: string
	//   in: query
	//   description: stop streaming events later than this
	// - name: filters
	//   type: string
	//   in: query
	//   description: JSON encoded map[string][]string of constraints
	// responses:
	//   200:
	//     $ref: "#/responses/ok"
	//   500:
	//     "$ref": "#/responses/InternalError"
	r.Handle(VersionedPath("/events"), APIHandler(s.Context, handlers.GetEvents))
	return nil
}
