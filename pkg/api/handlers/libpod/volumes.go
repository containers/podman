//go:build !remote

package libpod

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/entities/reports"
	"github.com/containers/podman/v5/pkg/domain/filters"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/domain/infra/abi/parse"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/gorilla/schema"
)

func CreateVolume(w http.ResponseWriter, r *http.Request) {
	var (
		volumeOptions []libpod.VolumeCreateOption
		runtime       = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder       = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct{}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	input := entities.VolumeCreateOptions{}
	// decode params from body
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	if len(input.Name) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(input.Name))
	}
	if len(input.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(input.Driver))
	}

	// Label provided for compatibility.
	labels := make(map[string]string, len(input.Label)+len(input.Labels))
	maps.Copy(labels, input.Label)
	maps.Copy(labels, input.Labels)
	if len(labels) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(labels))
	}

	if len(input.Options) > 0 {
		parsedOptions, err := parse.VolumeOptions(input.Options)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}

	if input.IgnoreIfExists {
		volumeOptions = append(volumeOptions, libpod.WithVolumeIgnoreIfExist())
	}

	if input.UID != nil {
		volumeOptions = append(volumeOptions, libpod.WithVolumeUID(*input.UID), libpod.WithVolumeNoChown())
	}
	if input.GID != nil {
		volumeOptions = append(volumeOptions, libpod.WithVolumeGID(*input.GID), libpod.WithVolumeNoChown())
	}

	if input.Pinned {
		volumeOptions = append(volumeOptions, libpod.WithVolumePinned())
	}

	vol, err := runtime.NewVolume(r.Context(), volumeOptions...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	inspectOut, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		InspectVolumeData: *inspectOut,
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
	inspectOut, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		InspectVolumeData: *inspectOut,
	}
	utils.WriteResponse(w, http.StatusOK, volResponse)
}

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	volumeConfigs, err := ic.VolumeList(r.Context(), entities.VolumeListOptions{Filter: *filterMap})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, volumeConfigs)
}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	reports, err := pruneVolumesHelper(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func pruneVolumesHelper(r *http.Request) ([]*reports.PruneReport, error) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		return nil, err
	}

	f := (url.Values)(*filterMap)
	filterFuncs := []libpod.VolumeFilter{}
	for filter, filterValues := range f {
		filterFunc, err := filters.GeneratePruneVolumeFilters(filter, filterValues, runtime)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}

	// Check for includePinned parameter
	includePinned := false
	if includeParam := r.URL.Query().Get("includePinned"); includeParam == "true" {
		includePinned = true
	}

	reports, err := runtime.PruneVolumesWithOptions(r.Context(), filterFuncs, includePinned)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func RemoveVolume(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct {
		Force         bool  `schema:"force"`
		Timeout       *uint `schema:"timeout"`
		IncludePinned bool  `schema:"includePinned"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	vol, err := runtime.LookupVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	// Check if volume is pinned and --include-pinned flag is not set
	if vol.Pinned() && !query.IncludePinned {
		utils.Error(w, http.StatusBadRequest, 
			fmt.Errorf("volume %s is pinned and cannot be removed without includePinned=true parameter", vol.Name()))
		return
	}
	
	if err := runtime.RemoveVolume(r.Context(), vol, query.Force, query.Timeout); err != nil {
		if errors.Is(err, define.ErrVolumeBeingUsed) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// ExistsVolume check if a volume exists
func ExistsVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.VolumeExists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, define.ErrNoSuchVolume)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// ExportVolume exports a volume
func ExportVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}

	contents, err := vol.Export()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, contents)
}

// ImportVolume imports a volume
func ImportVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}

	if r.Body == nil {
		utils.Error(w, http.StatusInternalServerError, errors.New("must provide tar file to import in request body"))
		return
	}
	defer r.Body.Close()

	if err := vol.Import(r.Body); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, "")
}
