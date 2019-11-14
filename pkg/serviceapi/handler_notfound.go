package serviceapi

import (
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (s *APIServer) registerNotFoundHandlers(r *mux.Router) error {
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Errorf("%d %s for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	},
	)
	return nil
}
