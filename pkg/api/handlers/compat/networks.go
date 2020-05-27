package compat

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/containers/libpod/pkg/network"
	"github.com/docker/docker/api/types"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

type CompatInspectNetwork struct {
	types.NetworkResource
}

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// FYI scope and version are currently unused but are described by the API
	// Leaving this for if/when we have to enable these
	//query := struct {
	//	scope   string
	//	verbose bool
	//}{
	//	// override any golang type defaults
	//}
	//decoder := r.Context().Value("decoder").(*schema.Decoder)
	//if err := decoder.Decode(&query, r.URL.Query()); err != nil {
	//	utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
	//	return
	//}
	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	name := utils.GetName(r)
	_, err = network.InspectNetwork(config, name)
	if err != nil {
		// TODO our network package does not distinguish between not finding a
		// specific network vs not being able to read it
		utils.InternalServerError(w, err)
		return
	}
	report, err := getNetworkResourceByName(name, runtime)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func getNetworkResourceByName(name string, runtime *libpod.Runtime) (*types.NetworkResource, error) {
	var (
		ipamConfigs []dockerNetwork.IPAMConfig
	)
	config, err := runtime.GetConfig()
	if err != nil {
		return nil, err
	}
	containerEndpoints := map[string]types.EndpointResource{}
	// Get the network path so we can get created time
	networkConfigPath, err := network.GetCNIConfigPathByName(config, name)
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
		if netData, ok := data.NetworkSettings.Networks[name]; ok {
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
	report := types.NetworkResource{
		Name:       name,
		ID:         "",
		Created:    time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)), // nolint: unconvert
		Scope:      "",
		Driver:     network.DefaultNetworkDriver,
		EnableIPv6: false,
		IPAM: dockerNetwork.IPAM{
			Driver:  "default",
			Options: nil,
			Config:  ipamConfigs,
		},
		Internal:   false,
		Attachable: false,
		Ingress:    false,
		ConfigFrom: dockerNetwork.ConfigReference{},
		ConfigOnly: false,
		Containers: containerEndpoints,
		Options:    nil,
		Labels:     nil,
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
	var (
		reports []*types.NetworkResource
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Filters map[string][]string `schema:"filters"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// TODO remove when filters are implemented
	if len(query.Filters) > 0 {
		utils.InternalServerError(w, errors.New("filters for listing networks is not implemented"))
		return
	}
	netNames, err := network.GetNetworkNamesFromFileSystem(config)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, name := range netNames {
		report, err := getNetworkResourceByName(name, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
		}
		reports = append(reports, report)
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
	// At present I think we should just suport the bridge driver
	// and allow demand to make us consider more
	if networkCreate.Driver != network.DefaultNetworkDriver {
		utils.InternalServerError(w, errors.New("network create only supports the bridge driver"))
		return
	}
	ncOptions := entities.NetworkCreateOptions{
		Driver:   network.DefaultNetworkDriver,
		Internal: networkCreate.Internal,
	}
	if networkCreate.IPAM != nil && networkCreate.IPAM.Config != nil {
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
	_, err := ce.NetworkCreate(r.Context(), name, ncOptions)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	report := types.NetworkCreate{
		CheckDuplicate: networkCreate.CheckDuplicate,
		Driver:         networkCreate.Driver,
		Scope:          networkCreate.Scope,
		EnableIPv6:     networkCreate.EnableIPv6,
		IPAM:           networkCreate.IPAM,
		Internal:       networkCreate.Internal,
		Attachable:     networkCreate.Attachable,
		Ingress:        networkCreate.Ingress,
		ConfigOnly:     networkCreate.ConfigOnly,
		ConfigFrom:     networkCreate.ConfigFrom,
		Options:        networkCreate.Options,
		Labels:         networkCreate.Labels,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	config, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	name := utils.GetName(r)
	exists, err := network.Exists(config, name)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if !exists {
		utils.Error(w, "network not found", http.StatusNotFound, err)
		return
	}
	if err := network.RemoveNetwork(config, name); err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
