package serviceapi

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	goRuntime "runtime"
	"strings"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/sysinfo"
	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func (s *APIServer) registerInfoHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/info"), s.serviceHandler(s.info)).Methods("GET")
	return nil
}

func (s *APIServer) info(w http.ResponseWriter, r *http.Request) {
	infoData, err := s.Runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}
	hostInfo := infoData[0].Data
	storeInfo := infoData[1].Data

	configInfo, err := s.Runtime.GetConfig()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain runtime config"))
		return
	}
	versionInfo, err := define.GetVersion()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain podman versions"))
		return
	}
	stateInfo := getContainersState(s.Runtime)
	sysInfo := sysinfo.New(true)

	// FIXME: Need to expose if runtime supports Checkpoint'ing
	// liveRestoreEnabled := criu.CheckForCriu() && configInfo.RuntimeSupportsCheckpoint()

	info := &Info{Info: docker.Info{
		Architecture:       goRuntime.GOARCH,
		BridgeNfIP6tables:  !sysInfo.BridgeNFCallIP6TablesDisabled,
		BridgeNfIptables:   !sysInfo.BridgeNFCallIPTablesDisabled,
		CPUCfsPeriod:       sysInfo.CPUCfsPeriod,
		CPUCfsQuota:        sysInfo.CPUCfsQuota,
		CPUSet:             sysInfo.Cpuset,
		CPUShares:          sysInfo.CPUShares,
		CgroupDriver:       configInfo.CgroupManager,
		ClusterAdvertise:   "",
		ClusterStore:       "",
		ContainerdCommit:   docker.Commit{},
		Containers:         storeInfo["ContainerStore"].(map[string]interface{})["number"].(int),
		ContainersPaused:   stateInfo[define.ContainerStatePaused],
		ContainersRunning:  stateInfo[define.ContainerStateRunning],
		ContainersStopped:  stateInfo[define.ContainerStateStopped] + stateInfo[define.ContainerStateExited],
		Debug:              log.IsLevelEnabled(log.DebugLevel),
		DefaultRuntime:     configInfo.OCIRuntime,
		DockerRootDir:      storeInfo["GraphRoot"].(string),
		Driver:             storeInfo["GraphDriverName"].(string),
		DriverStatus:       getGraphStatus(storeInfo),
		ExperimentalBuild:  true,
		GenericResources:   nil,
		HTTPProxy:          getEnv("http_proxy"),
		HTTPSProxy:         getEnv("https_proxy"),
		ID:                 uuid.New().String(),
		IPv4Forwarding:     !sysInfo.IPv4ForwardingDisabled,
		Images:             storeInfo["ImageStore"].(map[string]interface{})["number"].(int),
		IndexServerAddress: "",
		InitBinary:         "",
		InitCommit:         docker.Commit{},
		Isolation:          "",
		KernelMemory:       sysInfo.KernelMemory,
		KernelMemoryTCP:    false,
		KernelVersion:      hostInfo["kernel"].(string),
		Labels:             nil,
		LiveRestoreEnabled: false,
		LoggingDriver:      "",
		MemTotal:           hostInfo["MemTotal"].(int64),
		MemoryLimit:        sysInfo.MemoryLimit,
		NCPU:               goRuntime.NumCPU(),
		NEventsListener:    0,
		NFd:                getFdCount(),
		NGoroutines:        goRuntime.NumGoroutine(),
		Name:               hostInfo["hostname"].(string),
		NoProxy:            getEnv("no_proxy"),
		OSType:             goRuntime.GOOS,
		OSVersion:          hostInfo["Distribution"].(map[string]interface{})["version"].(string),
		OomKillDisable:     sysInfo.OomKillDisable,
		OperatingSystem:    hostInfo["Distribution"].(map[string]interface{})["distribution"].(string),
		PidsLimit:          sysInfo.PidsLimit,
		Plugins:            docker.PluginsInfo{},
		ProductLicense:     "Apache-2.0",
		RegistryConfig:     nil,
		RuncCommit:         docker.Commit{},
		Runtimes:           getRuntimes(configInfo),
		SecurityOptions:    getSecOpts(sysInfo),
		ServerVersion:      versionInfo.Version,
		SwapLimit:          sysInfo.SwapLimit,
		Swarm: swarm.Info{
			LocalNodeState: swarm.LocalNodeStateInactive,
		},
		SystemStatus: nil,
		SystemTime:   time.Now().Format(time.RFC3339Nano),
		Warnings:     []string{},
	},
		BuildahVersion:     hostInfo["BuildahVersion"].(string),
		CPURealtimePeriod:  sysInfo.CPURealtimePeriod,
		CPURealtimeRuntime: sysInfo.CPURealtimeRuntime,
		CgroupVersion:      hostInfo["CgroupVersion"].(string),
		Rootless:           rootless.IsRootless(),
		SwapFree:           hostInfo["SwapFree"].(int64),
		SwapTotal:          hostInfo["SwapTotal"].(int64),
		Uptime:             hostInfo["uptime"].(string),
	}
	s.WriteResponse(w, http.StatusOK, info)
}

func getGraphStatus(storeInfo map[string]interface{}) [][2]string {
	var graphStatus [][2]string
	for k, v := range storeInfo["GraphStatus"].(map[string]string) {
		graphStatus = append(graphStatus, [2]string{k, v})
	}
	return graphStatus
}

func getSecOpts(sysInfo *sysinfo.SysInfo) []string {
	var secOpts []string
	if sysInfo.AppArmor {
		secOpts = append(secOpts, "name=apparmor")
	}
	if sysInfo.Seccomp {
		// FIXME: get profile name...
		secOpts = append(secOpts, fmt.Sprintf("name=seccomp,profile=%s", "default"))
	}
	return secOpts
}

func getRuntimes(configInfo *config.Config) map[string]docker.Runtime {
	var runtimes = map[string]docker.Runtime{}
	for name, paths := range configInfo.OCIRuntimes {
		runtimes[name] = docker.Runtime{
			Path: paths[0],
			Args: nil,
		}
	}
	return runtimes
}

func getFdCount() (count int) {
	count = -1
	if entries, err := ioutil.ReadDir("/proc/self/fd"); err == nil {
		count = len(entries)
	}
	return
}

// Just ignoring container errors here...
func getContainersState(r *libpod.Runtime) map[define.ContainerStatus]int {
	var states = map[define.ContainerStatus]int{}
	ctnrs, err := r.GetAllContainers()
	if err == nil {
		for _, ctnr := range ctnrs {
			state, err := ctnr.State()
			if err != nil {
				continue
			}
			states[state] += 1
		}
	}
	return states
}

func getEnv(value string) string {
	if v, exists := os.LookupEnv(strings.ToUpper(value)); exists {
		return v
	}
	if v, exists := os.LookupEnv(strings.ToLower(value)); exists {
		return v
	}
	return ""
}
