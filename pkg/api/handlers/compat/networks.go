package compat

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	networkid "github.com/containers/podman/v3/pkg/network"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/docker/docker/api/types"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// scope is only used to see if the user passes any illegal value, verbose is not used but implemented
	// for compatibility purposes only.
	query := struct {
		scope   string `schema:"scope"`
		verbose bool   `schema:"verbose"`
	}{
		scope: "local",
	}
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.scope != "local" {
		utils.Error(w, "Invalid scope value. Can only be local.", http.StatusBadRequest, define.ErrInvalidArg)
		return
	}
	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	name := utils.GetName(r)
	_, err = network.InspectNetwork(config, name)
	if err != nil {
		utils.NetworkNotFound(w, name, err)
		return
	}
	report, err := getNetworkResourceByNameOrID(name, runtime, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func getNetworkResourceByNameOrID(nameOrID string, runtime *libpod.Runtime, filters map[string][]string) (*types.NetworkResource, error) {
	var (
		ipamConfigs []dockerNetwork.IPAMConfig
	)
	config, err := runtime.GetConfig()
	if err != nil {
		return nil, err
	}
	containerEndpoints := map[string]types.EndpointResource{}
	// Get the network path so we can get created time
	networkConfigPath, err := network.GetCNIConfigPathByNameOrID(config, nameOrID)
	if err != nil {
		return nil, err
	}
	f, err := os.Stat(networkConfigPath)
	if err != nil {
		return nil, err
	}
	stat := f.Sys().(*syscall.Stat_t)
	cons, err := runtime.GetAllContainers()
	if err != nil {
		return nil, err
	}
	conf, err := libcni.ConfListFromFile(networkConfigPath)
	if err != nil {
		return nil, err
	}
	if len(filters) > 0 {
		ok, err := network.IfPassesFilter(conf, filters)
		if err != nil {
			return nil, err
		}
		if !ok {
			// do not return the config if we did not match the filter
			return nil, nil
		}
	}

	// No Bridge plugin means we bail
	bridge, err := genericPluginsToBridge(conf.Plugins, network.DefaultNetworkDriver)
	if err != nil {
		return nil, err
	}
	for _, outer := range bridge.IPAM.Ranges {
		for _, n := range outer {
			ipamConfig := dockerNetwork.IPAMConfig{
				Subnet:  n.Subnet,
				Gateway: n.Gateway,
			}
			ipamConfigs = append(ipamConfigs, ipamConfig)
		}
	}

	for _, con := range cons {
		data, err := con.Inspect(false)
		if err != nil {
			return nil, err
		}
		if netData, ok := data.NetworkSettings.Networks[conf.Name]; ok {
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

	labels := network.GetNetworkLabels(conf)
	if labels == nil {
		labels = map[string]string{}
	}

	report := types.NetworkResource{
		Name:       conf.Name,
		ID:         networkid.GetNetworkID(conf.Name),
		Created:    time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)), // nolint: unconvert
		Scope:      "local",
		Driver:     network.DefaultNetworkDriver,
		EnableIPv6: false,
		IPAM: dockerNetwork.IPAM{
			Driver:  "default",
			Options: map[string]string{},
			Config:  ipamConfigs,
		},
		Internal:   !bridge.IsGW,
		Attachable: false,
		Ingress:    false,
		ConfigFrom: dockerNetwork.ConfigReference{},
		ConfigOnly: false,
		Containers: containerEndpoints,
		Options:    map[string]string{},
		Labels:     labels,
		Peers:      nil,
		Services:   nil,
	}
	return &report, nil
}

func genericPluginsToBridge(plugins []*libcni.NetworkConfig, pluginType string) (network.HostLocalBridge, error) {
	var bridge network.HostLocalBridge
	generic, err := findPluginByName(plugins, pluginType)
	if err != nil {
		return bridge, err
	}
	err = json.Unmarshal(generic, &bridge)
	return bridge, err
}

func findPluginByName(plugins []*libcni.NetworkConfig, pluginType string) ([]byte, error) {
	for _, p := range plugins {
		if pluginType == p.Network.Type {
			return p.Bytes, nil
		}
	}
	return nil, errors.New("unable to find bridge plugin")
}

func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	netNames, err := network.GetNetworkNamesFromFileSystem(config)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	reports := []*types.NetworkResource{}
	logrus.Debugf("netNames: %q", strings.Join(netNames, ", "))
	for _, name := range netNames {
		report, err := getNetworkResourceByNameOrID(name, runtime, *filterMap)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		if report != nil {
			reports = append(reports, report)
		}
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		name          string
		networkCreate types.NetworkCreateRequest
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	if err := json.NewDecoder(r.Body).Decode(&networkCreate); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	if len(networkCreate.Name) > 0 {
		name = networkCreate.Name
	}
	if len(networkCreate.Driver) < 1 {
		networkCreate.Driver = network.DefaultNetworkDriver
	}
	// At present I think we should just support the bridge driver
	// and allow demand to make us consider more
	if networkCreate.Driver != network.DefaultNetworkDriver {
		utils.InternalServerError(w, errors.New("network create only supports the bridge driver"))
		return
	}
	ncOptions := entities.NetworkCreateOptions{
		Driver:   network.DefaultNetworkDriver,
		Internal: networkCreate.Internal,
		Labels:   networkCreate.Labels,
	}
	if networkCreate.IPAM != nil && len(networkCreate.IPAM.Config) > 0 {
		if len(networkCreate.IPAM.Config) > 1 {
			utils.InternalServerError(w, errors.New("compat network create can only support one IPAM config"))
			return
		}

		if len(networkCreate.IPAM.Config[0].Subnet) > 0 {
			_, subnet, err := net.ParseCIDR(networkCreate.IPAM.Config[0].Subnet)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			ncOptions.Subnet = *subnet
		}
		if len(networkCreate.IPAM.Config[0].Gateway) > 0 {
			ncOptions.Gateway = net.ParseIP(networkCreate.IPAM.Config[0].Gateway)
		}
		if len(networkCreate.IPAM.Config[0].IPRange) > 0 {
			_, IPRange, err := net.ParseCIDR(networkCreate.IPAM.Config[0].IPRange)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			ncOptions.Range = *IPRange
		}
	}
	ce := abi.ContainerEngine{Libpod: runtime}
	if _, err := ce.NetworkCreate(r.Context(), name, ncOptions); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	net, err := getNetworkResourceByNameOrID(name, runtime, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	body := struct {
		ID      string `json:"Id"`
		Warning []string
	}{
		ID: net.ID,
	}
	utils.WriteResponse(w, http.StatusCreated, body)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	query := struct {
		Force bool `schema:"force"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	decoder := r.Context().Value("decoder").(*schema.Decoder)
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
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
