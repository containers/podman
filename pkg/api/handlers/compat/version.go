package compat

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/types"
	"github.com/containers/podman/v4/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	running, err := define.GetVersion()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	info, err := runtime.Info()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to obtain system memory info"))
		return
	}

	components := []types.ComponentVersion{{
		Name:    "Podman Engine",
		Version: running.Version,
		Details: map[string]string{
			"APIVersion":    version.APIVersion[version.Libpod][version.CurrentAPI].String(),
			"Arch":          goRuntime.GOARCH,
			"BuildTime":     time.Unix(running.Built, 0).Format(time.RFC3339),
			"Experimental":  "false",
			"GitCommit":     running.GitCommit,
			"GoVersion":     running.GoVersion,
			"KernelVersion": info.Host.Kernel,
			"MinAPIVersion": version.APIVersion[version.Libpod][version.MinimalAPI].String(),
			"Os":            goRuntime.GOOS,
		},
	}}

	if conmon, oci, err := runtime.DefaultOCIRuntime().RuntimeInfo(); err != nil {
		logrus.Warnf("Failed to retrieve Conmon and OCI Information: %q", err.Error())
	} else {
		additional := []types.ComponentVersion{
			{
				Name:    "Conmon",
				Version: conmon.Version,
				Details: map[string]string{
					"Package": conmon.Package,
				}},
			{
				Name:    fmt.Sprintf("OCI Runtime (%s)", oci.Name),
				Version: oci.Version,
				Details: map[string]string{
					"Package": oci.Package,
				}},
		}
		components = append(components, additional...)
	}

	apiVersion := version.APIVersion[version.Compat][version.CurrentAPI]
	minVersion := version.APIVersion[version.Compat][version.MinimalAPI]

	utils.WriteResponse(w, http.StatusOK, entities.ComponentVersion{
		Version: types.Version{
			Platform: struct {
				Name string
			}{
				Name: fmt.Sprintf("%s/%s/%s-%s", goRuntime.GOOS, goRuntime.GOARCH, info.Host.Distribution.Distribution, info.Host.Distribution.Version),
			},
			APIVersion:    fmt.Sprintf("%d.%d", apiVersion.Major, apiVersion.Minor),
			Arch:          components[0].Details["Arch"],
			BuildTime:     components[0].Details["BuildTime"],
			Components:    components,
			Experimental:  false,
			GitCommit:     components[0].Details["GitCommit"],
			GoVersion:     components[0].Details["GoVersion"],
			KernelVersion: components[0].Details["KernelVersion"],
			MinAPIVersion: fmt.Sprintf("%d.%d", minVersion.Major, minVersion.Minor),
			Os:            components[0].Details["Os"],
			Version:       components[0].Version,
		}})
}
