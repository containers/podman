package serviceapi

import (
	"fmt"
	"net/http"
)

func (s *APIServer) unsupportedHandler(w http.ResponseWriter, r *http.Request) {
	s.WriteResponse(w, http.StatusInternalServerError, struct {
		Message string `json:"message"`
	}{
		Message: fmt.Sprintf("Path %s is not supported", r.URL.Path),
	})
}
