package compat

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	nettypes "github.com/containers/podman/v3/libpod/network/types"
	netutil "github.com/containers/podman/v3/libpod/network/util"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	api "github.com/containers/podman/v3/pkg/api/types"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/docker/docker/api/types"

	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// scope is only used to see if the user passes any illegal value, verbose is not used but implemented
	// for compatibility purposes only.
	query := struct {
		scope   string `schema:"scope"`
		verbose bool   `schema:"verbose"`
	}{
		scope: "local",
	}
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.scope != "local" {
		utils.Error(w, "Invalid scope value. Can only be local.", http.StatusBadRequest, define.ErrInvalidArg)
		return
	}
	name := utils.GetName(r)
	net, err := runtime.Network().NetworkInspect(name)
	if err != nil {
		utils.NetworkNotFound(w, name, err)
		return
	}
	report, err := convertLibpodNetworktoDockerNetwork(runtime, net)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func convertLibpodNetworktoDockerNetwork(runtime *libpod.Runtime, network nettypes.Network) (*types.NetworkResource, error) {
	cons, err := runtime.GetAllContainers()
	if err != nil {
		return nil, err
	}
	containerEndpoints := make(map[string]types.EndpointResource, len(cons))
	for _, con := range cons {
		data, err := con.Inspect(false)
		if err != nil {
			return nil, err
		}
		if netData, ok := data.NetworkSettings.Networks[network.Name]; ok {
			containerEndpoint := types.EndpointResource{
				Name:        netData.NetworkID,
				EndpointID:  netData.EndpointID,
				MacAddress:  netData.MacAddress,
				IPv4Address: netData.IPAddress,
				IPv6Address: netData.GlobalIPv6Address,
			}
			containerEndpoints[con.ID()] = containerEndpoint
		}
	}
	ipamConfigs := make([]dockerNetwork.IPAMConfig, 0, len(network.Subnets))
	for _, sub := range network.Subnets {
		ipamConfig := dockerNetwork.IPAMConfig{
			Subnet:  sub.Subnet.String(),
			Gateway: sub.Gateway.String(),
			// TODO add range
		}
		ipamConfigs = append(ipamConfigs, ipamConfig)
	}
	ipamDriver := network.IPAMOptions["driver"]
	if ipamDriver == nettypes.HostLocalIPAMDriver {
		ipamDriver = "default"
	}
	ipam := dockerNetwork.IPAM{
		Driver:  ipamDriver,
		Options: network.IPAMOptions,
		Config:  ipamConfigs,
	}

	report := types.NetworkResource{
		Name:   network.Name,
		ID:     network.ID,
		Driver: network.Driver,
		// TODO add Created: ,
		Internal:   network.Internal,
		EnableIPv6: network.IPv6Enabled,
		Labels:     network.Labels,
		Options:    network.Options,
		IPAM:       ipam,
		Scope:      "local",
		Attachable: false,
		Ingress:    false,
		ConfigFrom: dockerNetwork.ConfigReference{},
		ConfigOnly: false,
		Containers: containerEndpoints,
		Peers:      nil,
		Services:   nil,
	}
	return &report, nil
}

func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.NetworkListOptions{
		Filters: *filterMap,
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	nets, err := ic.NetworkList(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	reports := make([]*types.NetworkResource, 0, len(nets))
	for _, net := range nets {
		report, err := convertLibpodNetworktoDockerNetwork(runtime, net)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		reports = append(reports, report)
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		networkCreate types.NetworkCreateRequest
		network       nettypes.Network
	)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	if err := json.NewDecoder(r.Body).Decode(&networkCreate); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	network.Name = networkCreate.Name
	if networkCreate.Driver == "" {
		networkCreate.Driver = nettypes.DefaultNetworkDriver
	}
	network.Driver = networkCreate.Driver
	network.Labels = networkCreate.Labels
	network.Internal = networkCreate.Internal
	network.IPv6Enabled = networkCreate.EnableIPv6

	// FIXME use docker options and convert them to valid libpod options
	// network.Options = networkCreate.Options

	// dns is only enabled for the bridge driver
	if network.Driver == nettypes.BridgeNetworkDriver {
		network.DNSEnabled = true
	}

	if networkCreate.IPAM != nil && len(networkCreate.IPAM.Config) > 0 {
		for _, conf := range networkCreate.IPAM.Config {
			s := nettypes.Subnet{}
			if len(conf.Subnet) > 0 {
				var err error
				subnet, err := nettypes.ParseCIDR(conf.Subnet)
				if err != nil {
					utils.InternalServerError(w, errors.Wrap(err, "failed to parse subnet"))
					return
				}
				s.Subnet = subnet
			}
			if len(conf.Gateway) > 0 {
				gw := net.ParseIP(conf.Gateway)
				if gw == nil {
					utils.InternalServerError(w, errors.Errorf("failed to parse gateway ip %s", conf.Gateway))
					return
				}
				s.Gateway = gw
			}
			if len(conf.IPRange) > 0 {
				_, net, err := net.ParseCIDR(conf.IPRange)
				if err != nil {
					utils.InternalServerError(w, errors.Wrap(err, "failed to parse ip range"))
					return
				}
				startIP, err := netutil.FirstIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, errors.Wrap(err, "failed to get first ip in range"))
					return
				}
				lastIP, err := netutil.LastIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, errors.Wrap(err, "failed to get last ip in range"))
					return
				}
				s.LeaseRange = &nettypes.LeaseRange{
					StartIP: startIP,
					EndIP:   lastIP,
				}
			}
			network.Subnets = append(network.Subnets, s)
		}
		// FIXME can we use the IPAM driver and options?
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	newNetwork, err := ic.NetworkCreate(r.Context(), network)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	body := struct {
		ID      string `json:"Id"`
		Warning []string
	}{
		ID: newNetwork.ID,
	}
	utils.WriteResponse(w, http.StatusCreated, body)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	query := struct {
		Force bool `schema:"force"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.NetworkRmOptions{
		Force: query.Force,
	}

	name := utils.GetName(r)
	reports, err := ic.NetworkRm(r.Context(), []string{name}, options)
	if err != nil {
		utils.Error(w, "remove Network failed", http.StatusInternalServerError, err)
		return
	}
	if len(reports) == 0 {
		utils.Error(w, "remove Network failed", http.StatusInternalServerError, errors.Errorf("internal error"))
		return
	}
	report := reports[0]
	if report.Err != nil {
		if errors.Cause(report.Err) == define.ErrNoSuchNetwork {
			utils.Error(w, "network not found", http.StatusNotFound, define.ErrNoSuchNetwork)
			return
		}
		utils.InternalServerError(w, report.Err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

// Connect adds a container to a network
func Connect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	var (
		aliases    []string
		netConnect types.NetworkConnect
	)
	if err := json.NewDecoder(r.Body).Decode(&netConnect); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	name := utils.GetName(r)
	if netConnect.EndpointConfig != nil {
		if netConnect.EndpointConfig.Aliases != nil {
			aliases = netConnect.EndpointConfig.Aliases
		}
	}
	err := runtime.ConnectContainerToNetwork(netConnect.Container, name, aliases)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, netConnect.Container, err)
			return
		}
		if errors.Cause(err) == define.ErrNoSuchNetwork {
			utils.Error(w, "network not found", http.StatusNotFound, err)
			return
		}
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Disconnect removes a container from a network
func Disconnect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	var netDisconnect types.NetworkDisconnect
	if err := json.NewDecoder(r.Body).Decode(&netDisconnect); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	name := utils.GetName(r)
	err := runtime.DisconnectContainerFromNetwork(netDisconnect.Container, name, netDisconnect.Force)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.Error(w, "container not found", http.StatusNotFound, err)
			return
		}
		if errors.Cause(err) == define.ErrNoSuchNetwork {
			utils.Error(w, "network not found", http.StatusNotFound, err)
			return
		}
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Prune removes unused networks
func Prune(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	pruneOptions := entities.NetworkPruneOptions{
		Filters: *filterMap,
	}
	pruneReports, err := ic.NetworkPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	type response struct {
		NetworksDeleted []string
	}
	prunedNetworks := []string{}
	for _, pr := range pruneReports {
		if pr.Error != nil {
			logrus.Error(pr.Error)
			continue
		}
		prunedNetworks = append(prunedNetworks, pr.Name)
	}
	utils.WriteResponse(w, http.StatusOK, response{NetworksDeleted: prunedNetworks})
}
