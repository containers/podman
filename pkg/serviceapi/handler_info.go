package serviceapi

import (
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/rootless"
	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func registerInfoHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/info"), serviceHandler(info)).Methods("GET")
	return nil
}

func info(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	infoData, err := runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}
	hostInfo := infoData[0].Data
	storeInfo := infoData[1].Data

	configInfo, err := runtime.GetConfig()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain runtime config"))
		return
	}
	versionInfo, err := define.GetVersion()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain podman versions"))
		return
	}
	var graphStatus [][2]string
	for k, v := range storeInfo["GraphStatus"].(map[string]string) {
		graphStatus = append(graphStatus, [2]string{k, v})
	}

	info := &Info{Info: docker.Info{
		Architecture:       goRuntime.GOARCH,
		BridgeNfIP6tables:  false,
		BridgeNfIptables:   false,
		CPUCfsPeriod:       false,
		CPUCfsQuota:        false,
		CPUSet:             false,
		CPUShares:          false,
		CgroupDriver:       configInfo.CgroupManager,
		ClusterAdvertise:   "",
		ClusterStore:       "",
		ContainerdCommit:   docker.Commit{},
		Containers:         storeInfo["ContainerStore"].(map[string]interface{})["number"].(int),
		ContainersPaused:   0,
		ContainersRunning:  0,
		ContainersStopped:  0,
		Debug:              false,
		DefaultRuntime:     "",
		DockerRootDir:      storeInfo["RunRoot"].(string),
		Driver:             storeInfo["GraphDriverName"].(string),
		DriverStatus:       graphStatus,
		ExperimentalBuild:  false,
		GenericResources:   nil,
		HTTPProxy:          "",
		HTTPSProxy:         "",
		ID:                 "podman",
		IPv4Forwarding:     false,
		Images:             storeInfo["ImageStore"].(map[string]interface{})["number"].(int),
		IndexServerAddress: "",
		InitBinary:         "",
		InitCommit:         docker.Commit{},
		Isolation:          "",
		KernelMemory:       false,
		KernelMemoryTCP:    false,
		KernelVersion:      hostInfo["kernel"].(string),
		Labels:             nil,
		LiveRestoreEnabled: false,
		LoggingDriver:      "",
		MemTotal:           hostInfo["MemTotal"].(int64),
		MemoryLimit:        false,
		NCPU:               goRuntime.NumCPU(),
		NEventsListener:    0,
		NFd:                0,
		NGoroutines:        0,
		Name:               hostInfo["hostname"].(string),
		NoProxy:            "",
		OSType:             hostInfo["Distribution"].(map[string]interface{})["distribution"].(string),
		OSVersion:          hostInfo["Distribution"].(map[string]interface{})["version"].(string),
		OomKillDisable:     false,
		OperatingSystem:    goRuntime.GOOS,
		PidsLimit:          false,
		Plugins:            docker.PluginsInfo{},
		ProductLicense:     "",
		RegistryConfig:     nil,
		RuncCommit:         docker.Commit{},
		Runtimes:           nil,
		SecurityOptions:    nil,
		ServerVersion:      versionInfo.Version,
		SwapLimit:          false,
		Swarm:              swarm.Info{},
		SystemStatus:       nil,
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		Warnings:           nil,
	},
		Rootless:       rootless.IsRootless(),
		BuildahVersion: hostInfo["BuildahVersion"].(string),
		CgroupVersion:  hostInfo["CgroupVersion"].(string),
		SwapTotal:      hostInfo["SwapTotal"].(int64),
		SwapFree:       hostInfo["SwapFree"].(int64),
		Uptime:         hostInfo["uptime"].(string),
	}

	w.(ServiceWriter).WriteJSON(http.StatusOK, info)
}
