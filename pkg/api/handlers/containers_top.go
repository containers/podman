package handlers

import (
	"net/http"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func TopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		PsArgs string `schema:"ps_args"`
	}{
		PsArgs: "-ef",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]

	adapterRuntime := adapter.LocalRuntime{}
	adapterRuntime.Runtime = runtime

	topValues := cliconfig.TopValues{}
	topValues.InputArgs = []string{name, query.PsArgs}

	output, err := adapterRuntime.Top(&topValues)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	var body = ContainerTopOKBody{}
	if len(output) > 0 {
		body.Titles = strings.Split(output[0], "\t")
		for _, line := range output[1:] {
			body.Processes = append(body.Processes, strings.Split(line, "\t"))
		}
	}
	utils.WriteJSON(w, http.StatusOK, body)
}
