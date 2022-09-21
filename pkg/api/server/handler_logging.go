package server

import (
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type responseWriter struct {
	http.ResponseWriter
}

var apiLogger = &logrus.Logger{
	Formatter: &logrus.TextFormatter{
		DisableColors:          true,
		DisableLevelTruncation: true,
		FullTimestamp:          true,
		QuoteEmptyFields:       true,
		TimestampFormat:        time.RFC3339,
	},
	Level: logrus.TraceLevel,
	Out:   logrus.StandardLogger().Out,
}

func (l responseWriter) Write(b []byte) (int, error) {
	apiLogger.WithFields(logrus.Fields{
		"API":            "response",
		"X-Reference-Id": l.Header().Get("X-Reference-Id"),
	}).Trace(string(b))
	return l.ResponseWriter.Write(b)
}

func loggingHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			annotated := apiLogger.WithFields(logrus.Fields{
				"API":            "request",
				"X-Reference-Id": r.Header.Get("X-Reference-Id"),
			})
			r.Body = io.NopCloser(
				io.TeeReader(r.Body, annotated.WriterLevel(logrus.TraceLevel)))

			w = responseWriter{ResponseWriter: w}
			h.ServeHTTP(w, r)
		})
	}
}
