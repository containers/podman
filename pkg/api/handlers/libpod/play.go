package libpod

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/auth"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PlayKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Network   string `schema:"reference"`
		TLSVerify bool   `schema:"tlsVerify"`
	}{
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Fetch the K8s YAML file from the body, and copy it to a temp file.
	tmpfile, err := ioutil.TempFile("", "libpod-play-kube.yml")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer os.Remove(tmpfile.Name())
	if _, err := io.Copy(tmpfile, r.Body); err != nil && err != io.EOF {
		tmpfile.Close()
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "error closing temporary file"))
		return
	}
	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %q header for %s", auth.XRegistryAuthHeader, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authConf != nil {
		username = authConf.Username
		password = authConf.Password
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.PlayKubeOptions{
		Authfile: authfile,
		Username: username,
		Password: password,
		Network:  query.Network,
		Quiet:    true,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	report, err := containerEngine.PlayKube(r.Context(), tmpfile.Name(), options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "error playing YAML file"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report)
}
