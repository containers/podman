package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

// WriteResponse encodes the given value as JSON or string and renders it for http client
func WriteResponse(w http.ResponseWriter, code int, value interface{}) {
	switch value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := fmt.Fprintln(w, value); err != nil {
			log.Errorf("unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := io.Copy(w, value.(*os.File)); err != nil {
			log.Errorf("unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, code, value)
	}
}

func WriteJSON(w http.ResponseWriter, code int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	if err := coder.Encode(value); err != nil {
		log.Errorf("unable to write json: %q", err)
	}
}
