package libpod

import (
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func GenerateSystemd(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Name            bool   `schema:"useName"`
		New             bool   `schema:"new"`
		NoHeader        bool   `schema:"noHeader"`
		RestartPolicy   string `schema:"restartPolicy"`
		StopTimeout     uint   `schema:"stopTimeout"`
		ContainerPrefix string `schema:"containerPrefix"`
		PodPrefix       string `schema:"podPrefix"`
		Separator       string `schema:"separator"`
	}{
		RestartPolicy:   "on-failure",
		StopTimeout:     util.DefaultContainerConfig().Engine.StopTimeout,
		ContainerPrefix: "container",
		PodPrefix:       "pod",
		Separator:       "-",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateSystemdOptions{
		Name:            query.Name,
		New:             query.New,
		NoHeader:        query.NoHeader,
		RestartPolicy:   query.RestartPolicy,
		StopTimeout:     &query.StopTimeout,
		ContainerPrefix: query.ContainerPrefix,
		PodPrefix:       query.PodPrefix,
		Separator:       query.Separator,
	}
	report, err := containerEngine.GenerateSystemd(r.Context(), utils.GetName(r), options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "error generating systemd units"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report.Units)
}

func GenerateKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Names   []string `schema:"names"`
		Service bool     `schema:"service"`
	}{
		// Defaults would go here.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateKubeOptions{Service: query.Service}
	report, err := containerEngine.GenerateKube(r.Context(), query.Names, options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "error generating YAML"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report.Reader)
}
