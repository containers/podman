package server

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v3/pkg/api/types"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// referenceIDHandler adds X-Reference-Id Header allowing event correlation
// and Apache style request logging
func referenceIDHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(logrus.StandardLogger().Out,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rid := r.Header.Get("X-Reference-Id")
				if rid == "" {
					if c := r.Context().Value(types.ConnKey); c == nil {
						rid = uuid.New().String()
					} else {
						rid = fmt.Sprintf("%p", c)
					}
				}

				r.Header.Set("X-Reference-Id", rid)
				w.Header().Set("X-Reference-Id", rid)
				h.ServeHTTP(w, r)
			}))
	}
}
