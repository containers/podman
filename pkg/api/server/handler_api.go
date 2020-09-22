package server

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	log "github.com/sirupsen/logrus"
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
				log.Warnf("Recovering from API handler panic: %v, %s", err, buf[:n])
				// Try to inform client things went south... won't work if handler already started writing response body
				utils.InternalServerError(w, fmt.Errorf("%v", err))
			}
		}()

		// Wrapper to hide some boiler plate
		fn := func(w http.ResponseWriter, r *http.Request) {
			log.Debugf("APIHandler -- Method: %s URL: %s", r.Method, r.URL.String())

			if err := r.ParseForm(); err != nil {
				log.Infof("Failed Request: unable to parse form: %q", err)
			}

			// TODO: Use r.ConnContext when ported to go 1.13
			c := context.WithValue(r.Context(), "decoder", s.Decoder) // nolint
			c = context.WithValue(c, "runtime", s.Runtime)            // nolint
			c = context.WithValue(c, "shutdownFunc", s.Shutdown)      // nolint
			c = context.WithValue(c, "idletracker", s.idleTracker)    // nolint
			r = r.WithContext(c)

			cv := utils.APIVersion[utils.CompatTree][utils.CurrentAPIVersion]
			w.Header().Set("API-Version", fmt.Sprintf("%d.%d", cv.Major, cv.Minor))

			lv := utils.APIVersion[utils.LibpodTree][utils.CurrentAPIVersion].String()
			w.Header().Set("Libpod-API-Version", lv)
			w.Header().Set("Server", "Libpod/"+lv+" ("+runtime.GOOS+")")

			h(w, r)
		}
		fn(w, r)
	}
}

// VersionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func VersionedPath(p string) string {
	return "/v{version:[0-9][0-9.]*}" + p
}
