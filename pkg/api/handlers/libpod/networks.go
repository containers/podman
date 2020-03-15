package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/network"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func CreateNetwork(w http.ResponseWriter, r *http.Request) {}
func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	configDir := config.CNIConfigDir
	if len(configDir) < 1 {
		configDir = network.CNIConfigDir
	}
	networks, err := network.LoadCNIConfsFromDir(configDir)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, networks)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 404 no such
	// 500 internal
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
	if err := network.RemoveNetwork(name); err != nil {
		// If the network cannot be found, we return a 404.
		if errors.Cause(err) == network.ErrNetworkNotFound {
			utils.Error(w, "Something went wrong", http.StatusNotFound, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
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
	n, err := network.InspectNetwork(name)
	if err != nil {
		// If the network cannot be found, we return a 404.
		if errors.Cause(err) == network.ErrNetworkNotFound {
			utils.Error(w, "Something went wrong", http.StatusNotFound, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, n)
}
