package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/filters"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
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
	input := entities.VolumeCreateOptions{}
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
	if len(input.Options) > 0 {
		parsedOptions, err := shared.ParseVolumeOptions(input.Options)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}
	vol, err := runtime.NewVolume(r.Context(), volumeOptions...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	config, err := vol.Config()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		Name:       config.Name,
		Driver:     config.Driver,
		Mountpoint: config.MountPoint,
		CreatedAt:  config.CreatedTime,
		Labels:     config.Labels,
		Options:    config.Options,
		UID:        config.UID,
		GID:        config.GID,
	}
	utils.WriteResponse(w, http.StatusOK, volResponse)
}

func InspectVolume(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
	)
	name := utils.GetName(r)
	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		Name:       vol.Name(),
		Driver:     vol.Driver(),
		Mountpoint: vol.MountPoint(),
		CreatedAt:  vol.CreatedTime(),
		Labels:     vol.Labels(),
		Scope:      vol.Scope(),
		Options:    vol.Options(),
		UID:        vol.UID(),
		GID:        vol.GID(),
	}
	utils.WriteResponse(w, http.StatusOK, volResponse)
}

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		decoder       = r.Context().Value("decoder").(*schema.Decoder)
		runtime       = r.Context().Value("runtime").(*libpod.Runtime)
		volumeConfigs []*entities.VolumeListReport
	)
	query := struct {
		Filters map[string][]string `schema:"filters"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	volumeFilters, err := filters.GenerateVolumeFilters(query.Filters)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	vols, err := runtime.Volumes(volumeFilters...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, v := range vols {
		config := entities.VolumeConfigResponse{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: v.MountPoint(),
			CreatedAt:  v.CreatedTime(),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
			UID:        v.UID(),
			GID:        v.GID(),
		}
		volumeConfigs = append(volumeConfigs, &entities.VolumeListReport{VolumeConfigResponse: config})
	}
	utils.WriteResponse(w, http.StatusOK, volumeConfigs)
}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		reports []*entities.VolumePruneReport
	)
	pruned, err := runtime.PruneVolumes(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for k, v := range pruned {
		reports = append(reports, &entities.VolumePruneReport{
			Err: v,
			Id:  k,
		})
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func RemoveVolume(w http.ResponseWriter, r *http.Request) {
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
	name := utils.GetName(r)
	vol, err := runtime.LookupVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	if err := runtime.RemoveVolume(r.Context(), vol, query.Force); err != nil {
		if errors.Cause(err) == define.ErrVolumeBeingUsed {
			utils.Error(w, "volumes being used", http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
