package serviceapi

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/libpod/libpod/define"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/version"), s.serviceHandler(s.versionHandler))
	r.Handle("/version", s.serviceHandler(s.versionHandler))
	return nil
}

func (s *APIServer) versionHandler(w http.ResponseWriter, r *http.Request) {
	versionInfo, err := define.GetVersion()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	infoData, err := s.Runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}
	hostInfo := infoData[0].Data

	components := []docker.ComponentVersion{{
		Name:    "Podman Engine",
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

	s.WriteResponse(w, http.StatusOK, Version{docker.Version{
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
