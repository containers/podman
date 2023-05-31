package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
)

func KubePlay(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Annotations      map[string]string `schema:"annotations"`
		Network          []string          `schema:"network"`
		TLSVerify        bool              `schema:"tlsVerify"`
		LogDriver        string            `schema:"logDriver"`
		LogOptions       []string          `schema:"logOptions"`
		Start            bool              `schema:"start"`
		StaticIPs        []string          `schema:"staticIPs"`
		StaticMACs       []string          `schema:"staticMACs"`
		NoHosts          bool              `schema:"noHosts"`
		PublishPorts     []string          `schema:"publishPorts"`
		Wait             bool              `schema:"wait"`
		ServiceContainer bool              `schema:"serviceContainer"`
		PrintProgress    bool              `schema:"printProgress`
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
		Annotations:      query.Annotations,
		Authfile:         authfile,
		Username:         username,
		Password:         password,
		Networks:         query.Network,
		NoHosts:          query.NoHosts,
		Quiet:            true,
		LogDriver:        logDriver,
		LogOptions:       query.LogOptions,
		StaticIPs:        staticIPs,
		StaticMACs:       staticMACs,
		IsRemote:         true,
		PublishPorts:     query.PublishPorts,
		Wait:             query.Wait,
		ServiceContainer: query.ServiceContainer,
		PrintProgress:    query.PrintProgress,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["start"]; found {
		options.Start = types.NewOptionalBool(query.Start)
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if query.PrintProgress {
		ctx, cancel = context.WithCancel(r.Context())
		containerEngine.Libpod.SetEventListener(func(eventChan chan *events.Event, errChan chan error, filter string) {

			go func() {
				errChan <- containerEngine.Events(r.Context(), entities.EventsOptions{EventChan: eventChan, Filter: []string{filter}})
			}()
			go func() {
				flush := func() {
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}
				enc := json.NewEncoder(w)
				enc.SetEscapeHTML(true)
				for {
					select {
					case <-ctx.Done():
						return
					default:
						select {
						case event, ok := <-eventChan:
							if !ok {
								return
							}
							enc.Encode(entities.KubePlayReport{Stream: event.ToHumanReadable(true)})
							flush()
							time.Sleep(time.Second)
						case <-errChan:
							return
						default:
							// non-blocking call
						}
					}
				}
			}()
		})
	}
	report, err := containerEngine.PlayKube(r.Context(), r.Body, options)
	if query.PrintProgress {
		cancel()
	}
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
