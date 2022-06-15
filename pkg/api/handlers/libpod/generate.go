package libpod

import (
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func GenerateSystemd(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Name             bool     `schema:"useName"`
		New              bool     `schema:"new"`
		NoHeader         bool     `schema:"noHeader"`
		TemplateUnitFile bool     `schema:"templateUnitFile"`
		RestartPolicy    *string  `schema:"restartPolicy"`
		RestartSec       uint     `schema:"restartSec"`
		StopTimeout      uint     `schema:"stopTimeout"`
		StartTimeout     uint     `schema:"startTimeout"`
		ContainerPrefix  *string  `schema:"containerPrefix"`
		PodPrefix        *string  `schema:"podPrefix"`
		Separator        *string  `schema:"separator"`
		Wants            []string `schema:"wants"`
		After            []string `schema:"after"`
		Requires         []string `schema:"requires"`
	}{
		StartTimeout: 0,
		StopTimeout:  util.DefaultContainerConfig().Engine.StopTimeout,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	ContainerPrefix := "container"
	if query.ContainerPrefix != nil {
		ContainerPrefix = *query.ContainerPrefix
	}

	PodPrefix := "pod"
	if query.PodPrefix != nil {
		PodPrefix = *query.PodPrefix
	}

	Separator := "-"
	if query.Separator != nil {
		Separator = *query.Separator
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateSystemdOptions{
		Name:             query.Name,
		New:              query.New,
		NoHeader:         query.NoHeader,
		TemplateUnitFile: query.TemplateUnitFile,
		RestartPolicy:    query.RestartPolicy,
		StartTimeout:     &query.StartTimeout,
		StopTimeout:      &query.StopTimeout,
		ContainerPrefix:  ContainerPrefix,
		PodPrefix:        PodPrefix,
		Separator:        Separator,
		RestartSec:       &query.RestartSec,
		Wants:            query.Wants,
		After:            query.After,
		Requires:         query.Requires,
	}

	report, err := containerEngine.GenerateSystemd(r.Context(), utils.GetName(r), options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error generating systemd units"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report.Units)
}

func GenerateKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Names   []string `schema:"names"`
		Service bool     `schema:"service"`
	}{
		// Defaults would go here.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateKubeOptions{Service: query.Service}
	report, err := containerEngine.GenerateKube(r.Context(), query.Names, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error generating YAML"))
		return
	}

	// FIXME: Content-Type is being set as application/x-tar NOT text/vnd.yaml
	// https://mailarchive.ietf.org/arch/msg/media-types/e9ZNC0hDXKXeFlAVRWxLCCaG9GI/
	utils.WriteResponse(w, http.StatusOK, report.Reader)
}
