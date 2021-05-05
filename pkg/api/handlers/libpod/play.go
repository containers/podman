package libpod

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PlayKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Network    string   `schema:"network"`
		TLSVerify  bool     `schema:"tlsVerify"`
		LogDriver  string   `schema:"logDriver"`
		Start      bool     `schema:"start"`
		StaticIPs  []string `schema:"staticIPs"`
		StaticMACs []string `schema:"staticMACs"`
	}{
		TLSVerify: true,
		Start:     true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	staticIPs := make([]net.IP, 0, len(query.StaticIPs))
	for _, ipString := range query.StaticIPs {
		ip := net.ParseIP(ipString)
		if ip == nil {
			utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
				errors.Errorf("Invalid IP address %s", ipString))
			return
		}
		staticIPs = append(staticIPs, ip)
	}

	staticMACs := make([]net.HardwareAddr, 0, len(query.StaticMACs))
	for _, macString := range query.StaticMACs {
		mac, err := net.ParseMAC(macString)
		if err != nil {
			utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
				err)
			return
		}
		staticMACs = append(staticMACs, mac)
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
	authConf, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
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
		Authfile:   authfile,
		Username:   username,
		Password:   password,
		Network:    query.Network,
		Quiet:      true,
		LogDriver:  query.LogDriver,
		StaticIPs:  staticIPs,
		StaticMACs: staticMACs,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["start"]; found {
		options.Start = types.NewOptionalBool(query.Start)
	}

	report, err := containerEngine.PlayKube(r.Context(), tmpfile.Name(), options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "error playing YAML file"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report)
}
