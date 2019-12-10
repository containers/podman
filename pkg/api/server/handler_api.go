package server

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// APIHandler is a wrapper to enhance HandlerFunc's and remove redundant code
func APIHandler(ctx context.Context, h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("APIHandler -- Method: %s URL: %s", r.Method, r.URL.String())
		if err := r.ParseForm(); err != nil {
			log.Infof("Failed Request: unable to parse form: %q", err)
		}

		// TODO: Use ConnContext when ported to go 1.13
		c := context.WithValue(r.Context(), "decoder", ctx.Value("decoder"))
		c = context.WithValue(c, "runtime", ctx.Value("runtime"))
		c = context.WithValue(c, "shutdownFunc", ctx.Value("shutdownFunc"))
		r = r.WithContext(c)

		h(w, r)

		shutdownFunc := r.Context().Value("shutdownFunc").(func() error)
		if err := shutdownFunc(); err != nil {
			log.Errorf("Failed to shutdown Server in APIHandler(): %s", err.Error())
		}
	})
}

// VersionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func VersionedPath(p string) string {
	return "/v{version:[0-9][0-9.]*}" + p
}
