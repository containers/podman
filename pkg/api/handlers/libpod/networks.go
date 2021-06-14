package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	options := entities.NetworkCreateOptions{}
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		utils.Error(w, "unable to marshall input", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	query := struct {
		Name string `schema:"name"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if len(options.Driver) < 1 {
		options.Driver = network.DefaultNetworkDriver
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.NetworkCreate(r.Context(), query.Name, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.NetworkListOptions{
		Filters: *filterMap,
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, err := ic.NetworkList(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)

	options := entities.NetworkRmOptions{
		Force: query.Force,
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, err := ic.NetworkRm(r.Context(), []string{name}, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if reports[0].Err != nil {
		// If the network cannot be found, we return a 404.
		if errors.Cause(reports[0].Err) == define.ErrNoSuchNetwork {
			utils.Error(w, "Something went wrong", http.StatusNotFound, reports[0].Err)
			return
		}
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	options := entities.InspectOptions{}
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, errs, err := ic.NetworkInspect(r.Context(), []string{name}, options)
	// If the network cannot be found, we return a 404.
	if len(errs) > 0 {
		utils.Error(w, "Something went wrong", http.StatusNotFound, define.ErrNoSuchNetwork)
		return
	}
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports[0])
}

// Connect adds a container to a network
func Connect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	var netConnect entities.NetworkConnectOptions
	if err := json.NewDecoder(r.Body).Decode(&netConnect); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	name := utils.GetName(r)
	err := runtime.ConnectContainerToNetwork(netConnect.Container, name, netConnect.Aliases)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, netConnect.Container, err)
			return
		}
		if errors.Cause(err) == define.ErrNoSuchNetwork {
			utils.Error(w, "network not found", http.StatusNotFound, err)
			return
		}
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// ExistsNetwork check if a network exists
func ExistsNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.NetworkExists(r.Context(), name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, "network not found", http.StatusNotFound, define.ErrNoSuchNetwork)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// Prune removes unused networks
func Prune(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	pruneOptions := entities.NetworkPruneOptions{
		Filters: *filterMap,
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	pruneReports, err := ic.NetworkPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	if pruneReports == nil {
		pruneReports = []*entities.NetworkPruneReport{}
	}
	utils.WriteResponse(w, http.StatusOK, pruneReports)
}
