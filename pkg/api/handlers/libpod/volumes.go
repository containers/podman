package libpod

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
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
		Name:        config.Name,
		Labels:      config.Labels,
		Driver:      config.Driver,
		MountPoint:  config.MountPoint,
		CreatedTime: config.CreatedTime,
		Options:     config.Options,
		UID:         config.UID,
		GID:         config.GID,
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
	}
	inspect, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		decoder       = r.Context().Value("decoder").(*schema.Decoder)
		err           error
		runtime       = r.Context().Value("runtime").(*libpod.Runtime)
		volumeConfigs []*libpod.VolumeConfig
		volumeFilters []libpod.VolumeFilter
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

	if len(query.Filters) > 0 {
		volumeFilters, err = generateVolumeFilters(query.Filters)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}
	vols, err := runtime.Volumes(volumeFilters...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, v := range vols {
		config, err := v.Config()
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		volumeConfigs = append(volumeConfigs, config)
	}
	utils.WriteResponse(w, http.StatusOK, volumeConfigs)
}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
	)
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

func generateVolumeFilters(filters map[string][]string) ([]libpod.VolumeFilter, error) {
	var vf []libpod.VolumeFilter
	for filter, v := range filters {
		for _, val := range v {
			switch filter {
			case "name":
				nameVal := val
				vf = append(vf, func(v *libpod.Volume) bool {
					return nameVal == v.Name()
				})
			case "driver":
				driverVal := val
				vf = append(vf, func(v *libpod.Volume) bool {
					return v.Driver() == driverVal
				})
			case "scope":
				scopeVal := val
				vf = append(vf, func(v *libpod.Volume) bool {
					return v.Scope() == scopeVal
				})
			case "label":
				filterArray := strings.SplitN(val, "=", 2)
				filterKey := filterArray[0]
				var filterVal string
				if len(filterArray) > 1 {
					filterVal = filterArray[1]
				} else {
					filterVal = ""
				}
				vf = append(vf, func(v *libpod.Volume) bool {
					for labelKey, labelValue := range v.Labels() {
						if labelKey == filterKey && ("" == filterVal || labelValue == filterVal) {
							return true
						}
					}
					return false
				})
			case "opt":
				filterArray := strings.SplitN(val, "=", 2)
				filterKey := filterArray[0]
				var filterVal string
				if len(filterArray) > 1 {
					filterVal = filterArray[1]
				} else {
					filterVal = ""
				}
				vf = append(vf, func(v *libpod.Volume) bool {
					for labelKey, labelValue := range v.Options() {
						if labelKey == filterKey && ("" == filterVal || labelValue == filterVal) {
							return true
						}
					}
					return false
				})
			default:
				return nil, errors.Errorf("%q is in an invalid volume filter", filter)
			}
		}
	}
	return vf, nil
}
