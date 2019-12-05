package generic

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"net/http"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
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
		handlers.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		handlers.ContainerNotFound(w, name, err)
		return
	}

	state, err := ctnr.State()
	if err != nil {
		handlers.InternalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning {
		handlers.ContainerNotRunning(w, name, errors.Errorf("Container %s must be running to perform top operation", name))
		return
	}

	output, err := ctnr.Top([]string{})
	if err != nil {
		handlers.InternalServerError(w, err)
		return
	}

	var body = handlers.ContainerTopOKBody{}
	if len(output) > 0 {
		body.Titles = strings.Split(output[0], "\t")
		for _, line := range output[1:] {
			body.Processes = append(body.Processes, strings.Split(line, "\t"))
		}
	}
	handlers.WriteJSON(w, http.StatusOK, body)
}
