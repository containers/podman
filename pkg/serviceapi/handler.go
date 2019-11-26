package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

// serviceHandler is wrapper to enhance HandlerFunc's and remove redundant code
func (s *APIServer) serviceHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("ServiceHandler -- Method: %s URL: %s", r.Method, r.URL.String())
		if err := r.ParseForm(); err != nil {
			log.Errorf("unable to parse form: %q", err)
		}

		h(w, r)

		if err := s.Shutdown(); err != nil {
			log.Errorf("Failed to shutdown APIServer in serviceHandler(): %s", err.Error())
		}
	}
}

// versionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func versionedPath(p string) string {
	return "/v{version:[0-9][0-9.]*}" + p
}

// WriteResponse encodes the given value as JSON or string and renders it for http client
func (s *APIServer) WriteResponse(w http.ResponseWriter, code int, value interface{}) {
	w.WriteHeader(code)
	switch value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		if _, err := fmt.Fprintln(w, value); err != nil {
			log.Errorf("unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		if _, err := io.Copy(w, value.(*os.File)); err != nil {
			log.Errorf("unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, value)
	}
}

func WriteJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	if err := coder.Encode(value); err != nil {
		log.Errorf("unable to write json: %q", err)
	}
}
