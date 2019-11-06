package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func registerNotFoundHandlers(r *mux.Router) error {
	r.NotFoundHandler = http.HandlerFunc(notFound)
	return nil
}

func notFound(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("%d %s for '%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.URL.String())
	log.Debugf(msg)
	http.Error(w, msg, http.StatusNotFound)
}
