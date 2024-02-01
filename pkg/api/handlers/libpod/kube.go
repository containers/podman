package libpod

import (
	"fmt"
	"net"
	"net/http"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
)

func KubePlay(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Annotations      map[string]string `schema:"annotations"`
		LogDriver        string            `schema:"logDriver"`
		LogOptions       []string          `schema:"logOptions"`
		Network          []string          `schema:"network"`
		NoHosts          bool              `schema:"noHosts"`
		NoTrunc          bool              `schema:"noTrunc"`
		Replace          bool              `schema:"replace"`
		PublishPorts     []string          `schema:"publishPorts"`
		PublishAllPorts  bool              `schema:"publishAllPorts"`
		ServiceContainer bool              `schema:"serviceContainer"`
		Start            bool              `schema:"start"`
		StaticIPs        []string          `schema:"staticIPs"`
		StaticMACs       []string          `schema:"staticMACs"`
		TLSVerify        bool              `schema:"tlsVerify"`
		Userns           string            `schema:"userns"`
		Wait             bool              `schema:"wait"`
	}{
		TLSVerify: true,
		Start:     true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	staticIPs := make([]net.IP, 0, len(query.StaticIPs))
	for _, ipString := range query.StaticIPs {
		ip := net.ParseIP(ipString)
		if ip == nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid IP address %s", ipString))
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
		Annotations:        query.Annotations,
		Authfile:           authfile,
		IsRemote:           true,
		LogDriver:          logDriver,
		LogOptions:         query.LogOptions,
		Networks:           query.Network,
		NoHosts:            query.NoHosts,
		Password:           password,
		PublishPorts:       query.PublishPorts,
		PublishAllPorts:    query.PublishAllPorts,
		Quiet:              true,
		Replace:            query.Replace,
		ServiceContainer:   query.ServiceContainer,
		StaticIPs:          staticIPs,
		StaticMACs:         staticMACs,
		UseLongAnnotations: query.NoTrunc,
		Username:           username,
		Userns:             query.Userns,
		Wait:               query.Wait,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["start"]; found {
		options.Start = types.NewOptionalBool(query.Start)
	}
	report, err := containerEngine.PlayKube(r.Context(), r.Body, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("playing YAML file: %w", err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func KubePlayDown(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		Force: false,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	report, err := containerEngine.PlayKubeDown(r.Context(), r.Body, entities.PlayKubeDownOptions{Force: query.Force})
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("tearing down YAML file: %w", err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func KubeGenerate(w http.ResponseWriter, r *http.Request) {
	GenerateKube(w, r)
}

func KubeApply(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		CACertFile string `schema:"caCertFile"`
		Kubeconfig string `schema:"kubeconfig"`
		Namespace  string `schema:"namespace"`
	}{
		// Defaults would go here.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.ApplyOptions{CACertFile: query.CACertFile, Kubeconfig: query.Kubeconfig, Namespace: query.Namespace}
	if err := containerEngine.KubeApply(r.Context(), r.Body, options); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error applying YAML to k8s cluster: %w", err))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "Deployed!")
}
