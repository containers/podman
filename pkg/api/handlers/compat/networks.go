//go:build !remote

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	nettypes "github.com/containers/common/libnetwork/types"
	netutil "github.com/containers/common/libnetwork/util"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
	"golang.org/x/exp/maps"

	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/sirupsen/logrus"
)

func normalizeNetworkName(rt *libpod.Runtime, name string) (string, bool) {
	if name == nettypes.BridgeNetworkDriver {
		return rt.Network().DefaultNetworkName(), true
	}
	return name, false
}

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
	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.scope != "local" {
		utils.Error(w, http.StatusBadRequest, define.ErrInvalidArg)
		return
	}
	name, changed := normalizeNetworkName(runtime, utils.GetName(r))
	net, err := runtime.Network().NetworkInspect(name)
	if err != nil {
		utils.NetworkNotFound(w, name, err)
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	statuses, err := ic.GetContainerNetStatuses()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	report := convertLibpodNetworktoDockerNetwork(runtime, statuses, &net, changed)
	utils.WriteResponse(w, http.StatusOK, report)
}

func convertLibpodNetworktoDockerNetwork(runtime *libpod.Runtime, statuses []abi.ContainerNetStatus, network *nettypes.Network, changeDefaultName bool) *dockerNetwork.Inspect {
	containerEndpoints := make(map[string]dockerNetwork.EndpointResource, len(statuses))
	for _, st := range statuses {
		if netData, ok := st.Status[network.Name]; ok {
			ipv4Address := ""
			ipv6Address := ""
			macAddr := ""
			for _, dev := range netData.Interfaces {
				for _, subnet := range dev.Subnets {
					// Note the docker API really wants the full CIDR subnet not just a single ip.
					// https://github.com/containers/podman/pull/12328
					if netutil.IsIPv4(subnet.IPNet.IP) {
						ipv4Address = subnet.IPNet.String()
					} else {
						ipv6Address = subnet.IPNet.String()
					}
				}
				macAddr = dev.MacAddress.String()
				break
			}
			containerEndpoint := dockerNetwork.EndpointResource{
				Name:        st.Name,
				MacAddress:  macAddr,
				IPv4Address: ipv4Address,
				IPv6Address: ipv6Address,
			}
			containerEndpoints[st.ID] = containerEndpoint
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

	name := network.Name
	if changeDefaultName && name == runtime.Network().DefaultNetworkName() {
		name = nettypes.BridgeNetworkDriver
	}
	// Make sure to clone the map as we have access to the map stored in
	// the network backend and will overwrite it which is not good.
	options := maps.Clone(network.Options)
	// bridge always has isolate set in the compat API but we should not return it to not confuse callers
	// https://github.com/containers/podman/issues/15580
	delete(options, nettypes.IsolateOption)

	report := dockerNetwork.Inspect{
		Name:       name,
		ID:         network.ID,
		Driver:     network.Driver,
		Created:    network.Created,
		Internal:   network.Internal,
		EnableIPv6: network.IPv6Enabled,
		Labels:     network.Labels,
		Options:    options,
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
	return &report
}

func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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
	statuses, err := ic.GetContainerNetStatuses()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	reports := make([]*dockerNetwork.Summary, 0, len(nets))
	for _, net := range nets {
		report := convertLibpodNetworktoDockerNetwork(runtime, statuses, &net, true)
		reports = append(reports, report)
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		networkCreate   dockerNetwork.CreateRequest
		network         nettypes.Network
		responseWarning string
	)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	if err := json.NewDecoder(r.Body).Decode(&networkCreate); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	network.Name = networkCreate.Name
	if networkCreate.Driver == "" {
		networkCreate.Driver = nettypes.DefaultNetworkDriver
	}
	network.Driver = networkCreate.Driver
	network.Labels = networkCreate.Labels
	network.Internal = networkCreate.Internal
	if networkCreate.EnableIPv6 != nil {
		network.IPv6Enabled = *networkCreate.EnableIPv6
	}

	network.Options = make(map[string]string)

	// dockers bridge networks are always isolated from each other
	if network.Driver == nettypes.BridgeNetworkDriver {
		network.Options[nettypes.IsolateOption] = "true"
	}

	for opt, optVal := range networkCreate.Options {
		switch opt {
		case nettypes.MTUOption:
			fallthrough
		case "com.docker.network.driver.mtu":
			network.Options[nettypes.MTUOption] = optVal
		case "com.docker.network.bridge.name":
			if network.Driver == nettypes.BridgeNetworkDriver {
				network.NetworkInterface = optVal
			}
		case nettypes.ModeOption:
			if network.Driver == nettypes.MacVLANNetworkDriver || network.Driver == nettypes.IPVLANNetworkDriver {
				network.Options[opt] = optVal
			}
		case "parent":
			if network.Driver == nettypes.MacVLANNetworkDriver || network.Driver == nettypes.IPVLANNetworkDriver {
				network.NetworkInterface = optVal
			}
		default:
			responseWarning = "\"" + opt + ": " + optVal + "\" is not a recognized option"
		}
	}

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
					utils.InternalServerError(w, fmt.Errorf("failed to parse subnet: %w", err))
					return
				}
				s.Subnet = subnet
			}
			if len(conf.Gateway) > 0 {
				gw := net.ParseIP(conf.Gateway)
				if gw == nil {
					utils.InternalServerError(w, fmt.Errorf("failed to parse gateway ip %s", conf.Gateway))
					return
				}
				s.Gateway = gw
			}
			if len(conf.IPRange) > 0 {
				_, net, err := net.ParseCIDR(conf.IPRange)
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to parse ip range: %w", err))
					return
				}
				startIP, err := netutil.FirstIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to get first ip in range: %w", err))
					return
				}
				lastIP, err := netutil.LastIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to get last ip in range: %w", err))
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

	opts := nettypes.NetworkCreateOptions{
		// networkCreate.CheckDuplicate is deprecated since API v1.44,
		// but it defaults to true when sent by the client package to
		// older daemons.
		IgnoreIfExists: false,
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	newNetwork, err := ic.NetworkCreate(r.Context(), network, &opts)
	if err != nil {
		if errors.Is(err, nettypes.ErrNetworkExists) {
			utils.Error(w, http.StatusConflict, err)
		} else {
			utils.InternalServerError(w, err)
		}
		return
	}

	body := struct {
		ID      string `json:"Id"`
		Warning string `json:"Warning"`
	}{
		ID:      newNetwork.ID,
		Warning: responseWarning,
	}
	utils.WriteResponse(w, http.StatusCreated, body)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	query := struct {
		Force   bool  `schema:"force"`
		Timeout *uint `schema:"timeout"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.NetworkRmOptions{
		Force:   query.Force,
		Timeout: query.Timeout,
	}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	reports, err := ic.NetworkRm(r.Context(), []string{name}, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(reports) == 0 {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("internal error"))
		return
	}
	report := reports[0]
	if report.Err != nil {
		if errors.Is(report.Err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, define.ErrNoSuchNetwork)
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

	var netConnect dockerNetwork.ConnectOptions
	if err := json.NewDecoder(r.Body).Decode(&netConnect); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	netOpts := nettypes.PerNetworkOptions{}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	if netConnect.EndpointConfig != nil {
		if netConnect.EndpointConfig.Aliases != nil {
			netOpts.Aliases = netConnect.EndpointConfig.Aliases
		}

		// if IP address is provided
		if len(netConnect.EndpointConfig.IPAddress) > 0 {
			staticIP := net.ParseIP(netConnect.EndpointConfig.IPAddress)
			if staticIP == nil {
				utils.Error(w, http.StatusInternalServerError,
					fmt.Errorf("failed to parse the ip address %q", netConnect.EndpointConfig.IPAddress))
				return
			}
			netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
		}

		if netConnect.EndpointConfig.IPAMConfig != nil {
			// if IPAMConfig.IPv4Address is provided
			if len(netConnect.EndpointConfig.IPAMConfig.IPv4Address) > 0 {
				staticIP := net.ParseIP(netConnect.EndpointConfig.IPAMConfig.IPv4Address)
				if staticIP == nil {
					utils.Error(w, http.StatusInternalServerError,
						fmt.Errorf("failed to parse the ipv4 address %q", netConnect.EndpointConfig.IPAMConfig.IPv4Address))
					return
				}
				netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
			}
			// if IPAMConfig.IPv6Address is provided
			if len(netConnect.EndpointConfig.IPAMConfig.IPv6Address) > 0 {
				staticIP := net.ParseIP(netConnect.EndpointConfig.IPAMConfig.IPv6Address)
				if staticIP == nil {
					utils.Error(w, http.StatusInternalServerError,
						fmt.Errorf("failed to parse the ipv6 address %q", netConnect.EndpointConfig.IPAMConfig.IPv6Address))
					return
				}
				netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
			}
		}
		// If MAC address is provided
		if len(netConnect.EndpointConfig.MacAddress) > 0 {
			staticMac, err := net.ParseMAC(netConnect.EndpointConfig.MacAddress)
			if err != nil {
				utils.Error(w, http.StatusInternalServerError,
					fmt.Errorf("failed to parse the mac address %q", netConnect.EndpointConfig.IPAMConfig.IPv6Address))
				return
			}
			netOpts.StaticMAC = nettypes.HardwareAddr(staticMac)
		}
	}
	err := runtime.ConnectContainerToNetwork(netConnect.Container, name, netOpts)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, netConnect.Container, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, define.ErrNetworkConnected) {
			utils.Error(w, http.StatusForbidden, err)
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Disconnect removes a container from a network
func Disconnect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	var netDisconnect dockerNetwork.DisconnectOptions
	if err := json.NewDecoder(r.Body).Decode(&netDisconnect); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	err := runtime.DisconnectContainerFromNetwork(netDisconnect.Container, name, netDisconnect.Force)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Prune removes unused networks
func Prune(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	pruneOptions := entities.NetworkPruneOptions{
		Filters: *filterMap,
	}
	pruneReports, err := ic.NetworkPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
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
