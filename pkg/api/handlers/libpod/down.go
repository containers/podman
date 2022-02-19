package libpod

import (
	"io"
	"net/http"
	"os"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func DownKube(w http.ResponseWriter, r *http.Request) {
	PlayKubeDown(w, r)
}

func PlayKubeDown(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	tmpfile, err := os.CreateTemp("", "libpod-play-kube.yml")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			logrus.Warn(err)
		}
	}()
	if _, err := io.Copy(tmpfile, r.Body); err != nil && err != io.EOF {
		if err := tmpfile.Close(); err != nil {
			logrus.Warn(err)
		}
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error closing temporary file"))
		return
	}
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := new(entities.PlayKubeDownOptions)
	report, err := containerEngine.PlayKubeDown(r.Context(), tmpfile.Name(), *options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error tearing down YAML file"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
