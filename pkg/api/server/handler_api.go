package server

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/containers/podman/v4/version"
	"github.com/sirupsen/logrus"
)

// APIHandler is a wrapper to enhance HandlerFunc's and remove redundant code
func (s *APIServer) APIHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Wrapper to hide some boilerplate
		fn := func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				logrus.WithFields(logrus.Fields{
					"X-Reference-Id": r.Header.Get("X-Reference-Id"),
				}).Info("Failed Request: unable to parse form: " + err.Error())
			}

			cv := version.APIVersion[version.Compat][version.CurrentAPI]
			w.Header().Set("API-Version", fmt.Sprintf("%d.%d", cv.Major, cv.Minor))

			lv := version.APIVersion[version.Libpod][version.CurrentAPI].String()
			w.Header().Set("Libpod-API-Version", lv)
			w.Header().Set("Server", "Libpod/"+lv+" ("+runtime.GOOS+")")

			if s.CorsHeaders != "" {
				w.Header().Set("Access-Control-Allow-Origin", s.CorsHeaders)
				w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth, Connection, Upgrade, X-Registry-Config")
				w.Header().Set("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE, PUT, OPTIONS")
			}

			h(w, r)
		}
		fn(w, r)
	}
}

// VersionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func VersionedPath(p string) string {
	return "/v{version:[0-9][0-9A-Za-z.-]*}" + p
}
