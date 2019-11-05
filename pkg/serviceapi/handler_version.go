package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
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

	buffer, err := json.Marshal(Version{docker.Version{
		Platform: struct {
			Name string
		}{
			Name: fmt.Sprintf("%s/%s/%s", goRuntime.GOOS, goRuntime.GOARCH, hostInfo["Distribution"].(map[string]interface{})["distribution"].(string)),
		},
		Components:    nil,
		Version:       versionInfo.Version,
		APIVersion:    DefaultApiVersion,
		MinAPIVersion: MinimalApiVersion,
		GitCommit:     versionInfo.GitCommit,
		GoVersion:     versionInfo.GoVersion,
		Os:            goRuntime.GOOS,
		Arch:          goRuntime.GOARCH,
		KernelVersion: hostInfo["kernel"].(string),
		Experimental:  true,
		BuildTime:     time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
	}})
	if err != nil {
		Error(w, "server error", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(buffer))
}
