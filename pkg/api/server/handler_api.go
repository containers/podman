package server

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/version"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// APIHandler is a wrapper to enhance HandlerFunc's and remove redundant code
func (s *APIServer) APIHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// http.Server hides panics, we want to see them and fix the cause.
		defer func() {
			err := recover()
			if err != nil {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				logrus.Warnf("Recovering from API handler panic: %v, %s", err, buf[:n])
				// Try to inform client things went south... won't work if handler already started writing response body
				utils.InternalServerError(w, fmt.Errorf("%v", err))
			}
		}()

		// Wrapper to hide some boiler plate
		fn := func(w http.ResponseWriter, r *http.Request) {
			rid := uuid.New().String()
			logrus.Infof("APIHandler(%s) -- %s %s BEGIN", rid, r.Method, r.URL.String())
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				for k, v := range r.Header {
					switch auth.HeaderAuthName(k) {
					case auth.XRegistryConfigHeader, auth.XRegistryAuthHeader:
						logrus.Debugf("APIHandler(%s) -- Header: %s=<hidden>", rid, k)
					default:
						logrus.Debugf("APIHandler(%s) -- Header: %s=%v", rid, k, v)
					}
				}
			}
			// Set in case handler wishes to correlate logging events
			r.Header.Set("X-Reference-Id", rid)

			if err := r.ParseForm(); err != nil {
				logrus.Infof("Failed Request: unable to parse form: %q (%s)", err, rid)
			}

			// TODO: Use r.ConnContext when ported to go 1.13
			c := context.WithValue(r.Context(), "decoder", s.Decoder) // nolint
			c = context.WithValue(c, "runtime", s.Runtime)            // nolint
			c = context.WithValue(c, "shutdownFunc", s.Shutdown)      // nolint
			c = context.WithValue(c, "idletracker", s.idleTracker)    // nolint
			r = r.WithContext(c)

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
			logrus.Debugf("APIHandler(%s) -- %s %s END", rid, r.Method, r.URL.String())
		}
		fn(w, r)
	}
}

// VersionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func VersionedPath(p string) string {
	return "/v{version:[0-9][0-9A-Za-z.-]*}" + p
}
