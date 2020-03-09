package compat

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	// /{version}/containers/(name)/stop
	query := struct {
		Timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "unable to get state for Container %s", name))
		return
	}
	// If the Container is stopped already, send a 304
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}

	var stopError error
	if query.Timeout > 0 {
		stopError = con.StopWithTimeout(uint(query.Timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		utils.InternalServerError(w, errors.Wrapf(stopError, "failed to stop %s", name))
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}
