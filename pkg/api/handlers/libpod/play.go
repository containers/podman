package libpod

import (
	"net"
	"net/http"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PlayKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Annotations map[string]string `schema:"annotations"`
		Network     []string          `schema:"network"`
		TLSVerify   bool              `schema:"tlsVerify"`
		LogDriver   string            `schema:"logDriver"`
		LogOptions  []string          `schema:"logOptions"`
		Start       bool              `schema:"start"`
		StaticIPs   []string          `schema:"staticIPs"`
		StaticMACs  []string          `schema:"staticMACs"`
		NoHosts     bool              `schema:"noHosts"`
	}{
		TLSVerify: true,
		Start:     true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	staticIPs := make([]net.IP, 0, len(query.StaticIPs))
	for _, ipString := range query.StaticIPs {
		ip := net.ParseIP(ipString)
		if ip == nil {
			utils.Error(w, http.StatusBadRequest, errors.Errorf("Invalid IP address %s", ipString))
			return
		}
		staticIPs = append(staticIPs, ip)
	}

	staticMACs := make([]net.HardwareAddr, 0, len(query.StaticMACs))
	for _, macString := range query.StaticMACs {
		mac, err := net.ParseMAC(macString)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		staticMACs = append(staticMACs, mac)
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authConf != nil {
		username = authConf.Username
		password = authConf.Password
	}

	logDriver := query.LogDriver
	if logDriver == "" {
		config, err := runtime.GetConfig()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		logDriver = config.Containers.LogDriver
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.PlayKubeOptions{
		Annotations: query.Annotations,
		Authfile:    authfile,
		Username:    username,
		Password:    password,
		Networks:    query.Network,
		NoHosts:     query.NoHosts,
		Quiet:       true,
		LogDriver:   logDriver,
		LogOptions:  query.LogOptions,
		StaticIPs:   staticIPs,
		StaticMACs:  staticMACs,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["start"]; found {
		options.Start = types.NewOptionalBool(query.Start)
	}
	report, err := containerEngine.PlayKube(r.Context(), r.Body, options)
	_ = r.Body.Close()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error playing YAML file"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PlayKubeDown(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := new(entities.PlayKubeDownOptions)
	report, err := containerEngine.PlayKubeDown(r.Context(), r.Body, *options)
	_ = r.Body.Close()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error tearing down YAML file"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
