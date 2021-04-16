package compat

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/entities/types"
	"github.com/containers/podman/v3/version"
	"github.com/pkg/errors"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 internal
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	versionInfo, err := define.GetVersion()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	infoData, err := runtime.Info()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to obtain system memory info"))
		return
	}

	components := []types.ComponentVersion{{
		Name:    "Podman Engine",
		Version: versionInfo.Version,
		Details: map[string]string{
			"APIVersion":    version.APIVersion[version.Libpod][version.CurrentAPI].String(),
			"Arch":          goRuntime.GOARCH,
			"BuildTime":     time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
			"Experimental":  "true",
			"GitCommit":     versionInfo.GitCommit,
			"GoVersion":     versionInfo.GoVersion,
			"KernelVersion": infoData.Host.Kernel,
			"MinAPIVersion": version.APIVersion[version.Libpod][version.MinimalAPI].String(),
			"Os":            goRuntime.GOOS,
		},
	}}

	apiVersion := version.APIVersion[version.Compat][version.CurrentAPI]
	minVersion := version.APIVersion[version.Compat][version.MinimalAPI]

	utils.WriteResponse(w, http.StatusOK, entities.ComponentVersion{
		Version: types.Version{
			Platform: struct {
				Name string
			}{
				Name: fmt.Sprintf("%s/%s/%s-%s", goRuntime.GOOS, goRuntime.GOARCH, infoData.Host.Distribution.Distribution, infoData.Host.Distribution.Version),
			},
			APIVersion:    fmt.Sprintf("%d.%d", apiVersion.Major, apiVersion.Minor),
			Arch:          components[0].Details["Arch"],
			BuildTime:     components[0].Details["BuildTime"],
			Components:    components,
			Experimental:  true,
			GitCommit:     components[0].Details["GitCommit"],
			GoVersion:     components[0].Details["GoVersion"],
			KernelVersion: components[0].Details["KernelVersion"],
			MinAPIVersion: fmt.Sprintf("%d.%d", minVersion.Major, minVersion.Minor),
			Os:            components[0].Details["Os"],
			Version:       components[0].Version,
		}})
}
