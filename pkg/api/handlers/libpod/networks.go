package libpod

import (
	"encoding/json"
	"fmt"
	"net/http"

	"errors"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/gorilla/schema"
)

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	network := types.Network{}
	if err := json.NewDecoder(r.Body).Decode(&network); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to decode request JSON payload: %w", err))
		return
	}

	query := struct {
		IgnoreIfExists bool `schema:"ignoreIfExists"`
	}{}
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.NetworkCreate(r.Context(), network, &types.NetworkCreateOptions{IgnoreIfExists: query.IgnoreIfExists})
	if err != nil {
		if errors.Is(err, types.ErrNetworkExists) {
			utils.Error(w, http.StatusConflict, err)
		} else {
			utils.InternalServerError(w, err)
		}
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func UpdateNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	networkUpdateOptions := entities.NetworkUpdateOptions{}
	if err := json.NewDecoder(r.Body).Decode(&networkUpdateOptions); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to decode request JSON payload: %w", err))
		return
	}

	name := utils.GetName(r)

	err := ic.NetworkUpdate(r.Context(), name, networkUpdateOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func ListNetworks(w http.ResponseWriter, r *http.Request) {
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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
		if errors.Is(reports[0].Err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, reports[0].Err)
			return
		}
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

// InspectNetwork reports on given network's details
func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	name := utils.GetName(r)
	options := entities.InspectOptions{}
	reports, errs, err := ic.NetworkInspect(r.Context(), []string{name}, options)
	// If the network cannot be found, we return a 404.
	if len(errs) > 0 {
		utils.Error(w, http.StatusNotFound, define.ErrNoSuchNetwork)
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
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	var netConnect entities.NetworkConnectOptions
	if err := json.NewDecoder(r.Body).Decode(&netConnect); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to decode request JSON payload: %w", err))
		return
	}
	name := utils.GetName(r)

	err := runtime.ConnectContainerToNetwork(netConnect.Container, name, netConnect.PerNetworkOptions)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, netConnect.Container, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// ExistsNetwork check if a network exists
func ExistsNetwork(w http.ResponseWriter, r *http.Request) {
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	name := utils.GetName(r)
	report, err := ic.NetworkExists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, define.ErrNoSuchNetwork)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// Prune removes unused networks
func Prune(w http.ResponseWriter, r *http.Request) {
	if v, err := utils.SupportedVersion(r, ">=4.0.0"); err != nil {
		utils.BadRequest(w, "version", v.String(), err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	pruneOptions := entities.NetworkPruneOptions{
		Filters: *filterMap,
	}
	pruneReports, err := ic.NetworkPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if pruneReports == nil {
		pruneReports = []*entities.NetworkPruneReport{}
	}
	utils.WriteResponse(w, http.StatusOK, pruneReports)
}
