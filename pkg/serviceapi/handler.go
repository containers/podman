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
func (s *APIServer) WriteResponse(w http.ResponseWriter, code int, value interface{}) (err error) {
	w.WriteHeader(code)
	switch value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		_, err = fmt.Fprintln(w, value)
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		io.Copy(w, value.(*os.File))
	default:
		WriteJSON(w, value)
	}
	return err
}

func WriteJSON(w http.ResponseWriter, value interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	return coder.Encode(value)
}
