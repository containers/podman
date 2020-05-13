package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/containers/libpod/pkg/network"
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
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
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
	options := entities.NetworkListOptions{}
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
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
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
		if errors.Cause(err) == network.ErrNetworkNotFound {
			utils.Error(w, "Something went wrong", http.StatusNotFound, err)
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
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	options := entities.NetworkInspectOptions{}
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, err := ic.NetworkInspect(r.Context(), []string{name}, options)
	if err != nil {
		// If the network cannot be found, we return a 404.
		if errors.Cause(err) == network.ErrNetworkNotFound {
			utils.Error(w, "Something went wrong", http.StatusNotFound, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}
