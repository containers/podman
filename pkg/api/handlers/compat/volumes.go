package compat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/filters"
	"github.com/containers/podman/v4/pkg/domain/infra/abi/parse"
	"github.com/containers/podman/v4/pkg/util"
	docker_api_types "github.com/docker/docker/api/types"
	docker_api_types_volume "github.com/docker/docker/api/types/volume"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	filtersMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Reject any libpod specific filters since `GenerateVolumeFilters()` will
	// happily parse them for us.
	for filter := range *filtersMap {
		if filter == "opts" {
			utils.Error(w, http.StatusInternalServerError,
				errors.Errorf("unsupported libpod filters passed to docker endpoint"))
			return
		}
	}
	volumeFilters, err := filters.GenerateVolumeFilters(*filtersMap)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	vols, err := runtime.Volumes(volumeFilters...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volumeConfigs := make([]*docker_api_types.Volume, 0, len(vols))
	for _, v := range vols {
		mp, err := v.MountPoint()
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		config := docker_api_types.Volume{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: mp,
			CreatedAt:  v.CreatedTime().Format(time.RFC3339),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
		}
		volumeConfigs = append(volumeConfigs, &config)
	}
	response := docker_api_types_volume.VolumeListOKBody{
		Volumes:  volumeConfigs,
		Warnings: []string{},
	}
	utils.WriteResponse(w, http.StatusOK, response)
}

func CreateVolume(w http.ResponseWriter, r *http.Request) {
	var (
		volumeOptions []libpod.VolumeCreateOption
		runtime       = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder       = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	/* No query string data*/
	query := struct{}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	// decode params from body
	input := docker_api_types_volume.VolumeCreateBody{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	var (
		existingVolume *libpod.Volume
		err            error
	)
	if len(input.Name) != 0 {
		// See if the volume exists already
		existingVolume, err = runtime.GetVolume(input.Name)
		if err != nil && errors.Cause(err) != define.ErrNoSuchVolume {
			utils.InternalServerError(w, err)
			return
		}
	}

	// if using the compat layer and the volume already exists, we
	// must return a 201 with the same information as create
	if existingVolume != nil && !utils.IsLibpodRequest(r) {
		mp, err := existingVolume.MountPoint()
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		response := docker_api_types.Volume{
			CreatedAt:  existingVolume.CreatedTime().Format(time.RFC3339),
			Driver:     existingVolume.Driver(),
			Labels:     existingVolume.Labels(),
			Mountpoint: mp,
			Name:       existingVolume.Name(),
			Options:    existingVolume.Options(),
			Scope:      existingVolume.Scope(),
		}
		utils.WriteResponse(w, http.StatusCreated, response)
		return
	}

	if len(input.Name) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(input.Name))
	}
	if len(input.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(input.Driver))
	}
	if len(input.Labels) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(input.Labels))
	}
	if len(input.DriverOpts) > 0 {
		parsedOptions, err := parse.VolumeOptions(input.DriverOpts)
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
	mp, err := vol.MountPoint()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := docker_api_types.Volume{
		Name:       config.Name,
		Driver:     config.Driver,
		Mountpoint: mp,
		CreatedAt:  config.CreatedTime.Format(time.RFC3339),
		Labels:     config.Labels,
		Options:    config.Options,
		Scope:      "local",
		// ^^ We don't have volume scoping so we'll just claim it's "local"
		// like we do in the `libpod.Volume.Scope()` method
		//
		// TODO: We don't include the volume `Status` or `UsageData`, but both
		// are nullable in the Docker engine API spec so that's fine for now
	}
	utils.WriteResponse(w, http.StatusCreated, volResponse)
}

func InspectVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	mp, err := vol.MountPoint()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := docker_api_types.Volume{
		Name:       vol.Name(),
		Driver:     vol.Driver(),
		Mountpoint: mp,
		CreatedAt:  vol.CreatedTime().Format(time.RFC3339),
		Labels:     vol.Labels(),
		Options:    vol.Options(),
		Scope:      vol.Scope(),
		// TODO: As above, we don't return `Status` or `UsageData` yet
	}
	utils.WriteResponse(w, http.StatusOK, volResponse)
}

func RemoveVolume(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct {
		Force   bool  `schema:"force"`
		Timeout *uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	/* The implications for `force` differ between Docker and us, so we can't
	 * simply pass the `force` parameter to `runtime.RemoveVolume()`.
	 * Specifically, Docker's behavior seems to be that `force` means "do not
	 * error on missing volume"; ours means "remove any not-running containers
	 * using the volume at the same time".
	 *
	 * With this in mind, we only consider the `force` query parameter when we
	 * hunt for specified volume by name, using it to selectively return a 204
	 * or blow up depending on `force` being truthy or falsey/unset
	 * respectively.
	 */
	name := utils.GetName(r)
	vol, err := runtime.LookupVolume(name)
	if err == nil {
		// As above, we do not pass `force` from the query parameters here
		if err := runtime.RemoveVolume(r.Context(), vol, false, query.Timeout); err != nil {
			if errors.Cause(err) == define.ErrVolumeBeingUsed {
				utils.Error(w, http.StatusConflict, err)
			} else {
				utils.InternalServerError(w, err)
			}
		} else {
			// Success
			utils.WriteResponse(w, http.StatusNoContent, nil)
		}
	} else {
		if !query.Force {
			utils.VolumeNotFound(w, name, err)
		} else {
			// Volume does not exist and `force` is truthy - this emulates what
			// Docker would do when told to `force` removal of a nonexistent
			// volume
			utils.WriteResponse(w, http.StatusNoContent, nil)
		}
	}
}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	f := (url.Values)(*filterMap)
	filterFuncs, err := filters.GeneratePruneVolumeFilters(f)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to parse filters for %s", f.Encode()))
		return
	}

	pruned, err := runtime.PruneVolumes(r.Context(), filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	var errorMsg bytes.Buffer
	var reclaimedSpace uint64
	prunedIds := make([]string, 0, len(pruned))
	for _, v := range pruned {
		if v.Err != nil {
			errorMsg.WriteString(v.Err.Error())
			errorMsg.WriteString("; ")
			continue
		}
		prunedIds = append(prunedIds, v.Id)
		reclaimedSpace += v.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	payload := handlers.VolumesPruneReport{
		VolumesPruneReport: docker_api_types.VolumesPruneReport{
			VolumesDeleted: prunedIds,
			SpaceReclaimed: reclaimedSpace,
		},
	}
	utils.WriteResponse(w, http.StatusOK, payload)
}
