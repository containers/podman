package serviceapi

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func registerNotFoundHandlers(r *mux.Router) error {
	r.NotFoundHandler = http.HandlerFunc(notFound)
	return nil
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w,
		fmt.Sprintf("%d %s for '%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.URL.String()),
		http.StatusNotFound)
}
