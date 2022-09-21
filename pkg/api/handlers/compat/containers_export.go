package compat

import (
	"fmt"
	"net/http"
	"os"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
)

func ExportContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	tmpfile, err := os.CreateTemp("", "api.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}
	defer os.Remove(tmpfile.Name())
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to close tempfile: %w", err))
		return
	}
	if err := con.Export(tmpfile.Name()); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to save image: %w", err))
		return
	}
	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to read the exported tarfile: %w", err))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}
