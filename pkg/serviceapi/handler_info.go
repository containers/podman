package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	goRuntime "runtime"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/gorilla/mux"
)

func registerInfoHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/info"), serviceHandler(info))
	return nil
}

func info(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	infoData, err := runtime.Info()
	if err != nil {
		apiError(w, fmt.Sprintf("Failed to obtain system memory info: %s", err), http.StatusInternalServerError)
		return
	}
	configInfo, err := runtime.GetConfig()
	if err != nil {
		apiError(w, fmt.Sprintf("Failed to obtain runtime config: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	hostInfo := infoData[0].Data
	storeInfo := infoData[1].Data

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
		ServerVersion:      "",
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

	buffer, err := json.Marshal(info)
	if err != nil {
		apiError(w,
			fmt.Sprintf("Failed to convert API images to json: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(buffer))
}
