package utils

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// IsLibpodRequest returns true if the request related to a libpod endpoint
// (e.g., /v2/libpod/...).
func IsLibpodRequest(r *http.Request) bool {
	split := strings.Split(r.URL.String(), "/")
	return len(split) >= 3 && split[2] == "libpod"
}

// WriteResponse encodes the given value as JSON or string and renders it for http client
func WriteResponse(w http.ResponseWriter, code int, value interface{}) {
	switch v := value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := fmt.Fprintln(w, v); err != nil {
			logrus.Errorf("unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, code, value)
	}
}

func WriteJSON(w http.ResponseWriter, code int, value interface{}) {
	// FIXME: we don't need to write the header in all/some circumstances.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	if err := coder.Encode(value); err != nil {
		logrus.Errorf("unable to write json: %q", err)
	}
}

func FilterMapToString(filters map[string][]string) (string, error) {
	f, err := json.Marshal(filters)
	if err != nil {
		return "", err
	}
	return string(f), nil
}

func getVar(r *http.Request, k string) string {
	val := mux.Vars(r)[k]
	safeVal, err := url.PathUnescape(val)
	if err != nil {
		logrus.Error(errors.Wrapf(err, "failed to unescape mux key %s, value %s", k, val))
		return val
	}
	return safeVal
}

// GetName extracts the name from the mux
func GetName(r *http.Request) string {
	return getVar(r, "name")
}
