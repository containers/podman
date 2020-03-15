package compat

import (
	"io"
	"net/http"
	"os"
)

func SaveFromBody(f *os.File, r *http.Request) error { // nolint
	if _, err := io.Copy(f, r.Body); err != nil {
		return err
	}
	return f.Close()
}
