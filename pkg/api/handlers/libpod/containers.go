package libpod

import (
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	handlers.StopContainer(w, r)
}

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	// /containers/libpod/{name:..*}/exists
	name := mux.Vars(r)["name"]
	_, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, http.StatusText(http.StatusOK))
}

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	utils.RemoveContainer(w, r, query.Force, query.Vols)
}
func ListContainers(w http.ResponseWriter, r *http.Request) {
	//	filter, size, sync, last
	// returns []shared.PSOutput
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Filter []string `schema:"filter"`
		Last   int      `schema:"last"`
		Size   bool     `schema:"size"`
		Sync   bool     `schema:"sync"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	opts := shared.PsOptions{
		All:       true,
		Last:      query.Last,
		Size:      query.Size,
		Sort:      "",
		Namespace: true,
		Sync:      query.Sync,
	}

	pss, err := shared.GetPsContainerOutput(runtime, opts, query.Filter, 2)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, pss)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	container, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	data, err := container.Inspect(query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, data)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	_, err := utils.KillContainer(w, r)
	if err != nil {
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	_, err := utils.WaitContainer(w, r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	// force
	// filters
}

func LogsFromContainer(w http.ResponseWriter, r *http.Request) {
	// follow
	// since
	// timestamps
	// tail string
}
func StatsContainer(w http.ResponseWriter, r *http.Request) {
	//stream
}
func CreateContainer(w http.ResponseWriter, r *http.Request) {

}
func TopContainer(w http.ResponseWriter, r *http.Request) {
	//psargs
}

func MountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	conn, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	m, err := conn.Mount()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, m)
}

func ShowMountedContainers(w http.ResponseWriter, r *http.Request) {
	response := make(map[string]string)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	conns, err := runtime.GetAllContainers()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	for _, conn := range conns {
		mounted, mountPoint, err := conn.Mounted()
		if err != nil {
			utils.InternalServerError(w, err)
		}
		if !mounted {
			continue
		}
		response[conn.ID()] = mountPoint
	}
	utils.WriteResponse(w, http.StatusOK, response)
}
