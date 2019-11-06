package serviceapi

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func registerVersionHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/version"), serviceHandler(versionHandler))
	r.Handle("/version", serviceHandler(versionHandler))
	return nil
}

func versionHandler(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	versionInfo, err := define.GetVersion()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	infoData, err := runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}
	hostInfo := infoData[0].Data

	components := []docker.ComponentVersion{{
		Name:    "Engine",
		Version: versionInfo.Version,
		Details: map[string]string{
			"APIVersion":    DefaultApiVersion,
			"Arch":          goRuntime.GOARCH,
			"BuildTime":     time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
			"Experimental":  "true",
			"GitCommit":     versionInfo.GitCommit,
			"GoVersion":     versionInfo.GoVersion,
			"KernelVersion": hostInfo["kernel"].(string),
			"MinAPIVersion": MinimalApiVersion,
			"Os":            goRuntime.GOOS,
		},
	}}

	w.(ServiceWriter).WriteJSON(http.StatusOK, Version{docker.Version{
		Platform: struct {
			Name string
		}{
			Name: fmt.Sprintf("%s/%s/%s", goRuntime.GOOS, goRuntime.GOARCH, hostInfo["Distribution"].(map[string]interface{})["distribution"].(string)),
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
