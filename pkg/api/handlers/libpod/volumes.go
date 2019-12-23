package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func CreateVolume(w http.ResponseWriter, r *http.Request) {
	var (
		volumeOptions []libpod.VolumeCreateOption
		runtime       = r.Context().Value("runtime").(*libpod.Runtime)
		decoder       = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
	}{
		// override any golang type defaults
	}
	input := handlers.VolumeCreateConfig{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	// decode params from body
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	if len(input.Name) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(input.Name))
	}
	if len(input.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(input.Driver))
	}
	if len(input.Label) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(input.Label))
	}
	if len(input.Opts) > 0 {
		parsedOptions, err := shared.ParseVolumeOptions(input.Opts)
		if err != nil {
			utils.InternalServerError(w, err)
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}
	vol, err := runtime.NewVolume(r.Context(), volumeOptions...)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, vol.Name())
}

func InspectVolume(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := mux.Vars(r)["name"]
	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
	}
	inspect, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	//var (
	//	runtime = r.Context().Value("runtime").(*libpod.Runtime)
	//	decoder = r.Context().Value("decoder").(*schema.Decoder)
	//)
	//query := struct {
	//	Filter string `json:"filter"`
	//}{
	//	// override any golang type defaults
	//}
	//
	//if err := decoder.Decode(&query, r.URL.Query()); err != nil {
	//	utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
	//		errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
	//	return
	//}
	/*
		This is all in main in cmd and needs to be extracted from there first.
	*/

}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	pruned, errs := runtime.PruneVolumes(r.Context())
	if errs != nil {
		if len(errs) > 1 {
			for _, err := range errs {
				log.Infof("Request Failed(%s): %s", http.StatusText(http.StatusInternalServerError), err.Error())
			}
		}
		utils.InternalServerError(w, errs[len(errs)-1])
	}
	utils.WriteResponse(w, http.StatusOK, pruned)
}

func RemoveVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
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
	name := mux.Vars(r)["name"]
	vol, err := runtime.LookupVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
	}
	if err := runtime.RemoveVolume(r.Context(), vol, query.Force); err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, "")
}
