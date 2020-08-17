package libpod

import (
	"net/http"

	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, "not implemented", http.StatusNotImplemented, errors.New("not implemented"))
}
