package handlers

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// Convenience routines to reduce boiler plate in handlers

// func hasVar(r *http.Request, k string) bool {
// 	_, found := mux.Vars(r)[k]
// 	return found
// }

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

// func getHeader(r *http.Request, k string) string {
// 	return r.Header.Get(k)
// }
//
// func hasHeader(r *http.Request, k string) bool {
// 	_, found := r.Header[k]
// 	return found
// }
