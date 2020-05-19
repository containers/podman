package compat

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	docker "github.com/docker/docker/api/types"
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
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}
	components := []docker.ComponentVersion{{
		Name:    "Podman Engine",
		Version: versionInfo.Version,
		Details: map[string]string{
			"APIVersion":    utils.ApiVersion[utils.LibpodTree][utils.CurrentApiVersion].String(),
			"Arch":          goRuntime.GOARCH,
			"BuildTime":     time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
			"Experimental":  "true",
			"GitCommit":     versionInfo.GitCommit,
			"GoVersion":     versionInfo.GoVersion,
			"KernelVersion": infoData.Host.Kernel,
			"MinAPIVersion": utils.ApiVersion[utils.LibpodTree][utils.MinimalApiVersion].String(),
			"Os":            goRuntime.GOOS,
		},
	}}

	utils.WriteResponse(w, http.StatusOK, entities.ComponentVersion{
		Version: docker.Version{
			Platform: struct {
				Name string
			}{
				Name: fmt.Sprintf("%s/%s/%s-%s", goRuntime.GOOS, goRuntime.GOARCH, infoData.Host.Distribution.Distribution, infoData.Host.Distribution.Version),
			},
			APIVersion:    components[0].Details["APIVersion"],
			Arch:          components[0].Details["Arch"],
			BuildTime:     components[0].Details["BuildTime"],
			Components:    components,
			Experimental:  true,
			GitCommit:     components[0].Details["GitCommit"],
			GoVersion:     components[0].Details["GoVersion"],
			KernelVersion: components[0].Details["KernelVersion"],
			MinAPIVersion: components[0].Details["MinAPIVersion"],
			Os:            components[0].Details["Os"],
			Version:       components[0].Version,
		}})
}
