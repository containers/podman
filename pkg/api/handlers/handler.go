package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
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

// Convenience routines to reduce boiler plate in handlers

func getVar(r *http.Request, k string) string {
	return mux.Vars(r)[k]
}

func hasVar(r *http.Request, k string) bool {
	_, found := mux.Vars(r)[k]
	return found
}
func getName(r *http.Request) string {
	return getVar(r, "name")
}

func decodeQuery(r *http.Request, i interface{}) error {
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	if err := decoder.Decode(i, r.URL.Query()); err != nil {
		return errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String())
	}
	return nil
}

func getRuntime(r *http.Request) *libpod.Runtime {
	return r.Context().Value("runtime").(*libpod.Runtime)
}

func getHeader(r *http.Request, k string) string {
	return r.Header.Get(k)
}

func hasHeader(r *http.Request, k string) bool {
	_, found := r.Header[k]
	return found
}
