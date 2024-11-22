//go:build !remote

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/filters"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/ps"
	"github.com/containers/podman/v5/pkg/signal"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/docker/docker/api/types"
	dockerBackend "github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	query := struct {
		Force         bool  `schema:"force"`
		Ignore        bool  `schema:"ignore"`
		Depend        bool  `schema:"depend"`
		Link          bool  `schema:"link"`
		Timeout       *uint `schema:"timeout"`
		DockerVolumes bool  `schema:"v"`
		LibpodVolumes bool  `schema:"volumes"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.RmOptions{
		Force:  query.Force,
		Ignore: query.Ignore,
	}
	if utils.IsLibpodRequest(r) {
		options.Volumes = query.LibpodVolumes
		options.Timeout = query.Timeout
		options.Depend = query.Depend
	} else {
		if query.Link {
			utils.Error(w, http.StatusBadRequest, utils.ErrLinkNotSupport)
			return
		}
		options.Volumes = query.DockerVolumes
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	reports, err := containerEngine.ContainerRm(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}
	if len(reports) > 0 && reports[0].Err != nil {
		err = reports[0].Err
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		All   bool `schema:"all"`
		Limit int  `schema:"limit"`
		Size  bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to decode filter parameters for %s: %w", r.URL.String(), err))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	filterFuncs := make([]libpod.ContainerFilter, 0, len(*filterMap))
	all := query.All || query.Limit > 0
	if len(*filterMap) > 0 {
		for k, v := range *filterMap {
			generatedFunc, err := filters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	// Docker thinks that if status is given as an input, then we should override
	// the all setting and always deal with all containers.
	if len((*filterMap)["status"]) > 0 {
		all = true
	}
	if !all {
		runningOnly, err := filters.GenerateContainerFilterFuncs("status", []string{define.ContainerStateRunning.String()}, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, runningOnly)
	}

	containers, err := runtime.GetContainers(false, filterFuncs...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if _, found := r.URL.Query()["limit"]; found && query.Limit > 0 {
		// Sort the libpod containers
		sort.Sort(ps.SortCreateTime{SortContainers: containers})
		// we should perform the lopping before we start getting
		// the expensive information on containers
		if len(containers) > query.Limit {
			containers = containers[:query.Limit]
		}
	}
	list := make([]*handlers.Container, 0, len(containers))
	for _, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, query.Size)
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) {
				// container was removed between the initial fetch of the list and conversion
				logrus.Debugf("Container %s removed between initial fetch and conversion, ignoring in output", ctnr.ID())
				continue
			}
			utils.InternalServerError(w, err)
			return
		}
		list = append(list, api)
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	api, err := LibpodToContainerJSON(ctnr, query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, api)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Signal string `schema:"signal"`
	}{
		Signal: "KILL",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	options := entities.KillOptions{
		Signal: query.Signal,
	}
	report, err := containerEngine.ContainerKill(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrCtrStateInvalid) ||
			errors.Is(err, define.ErrCtrStopped) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		if errors.Is(report[0].Err, define.ErrCtrStateInvalid) ||
			errors.Is(report[0].Err, define.ErrCtrStopped) {
			utils.Error(w, http.StatusConflict, report[0].Err)
			return
		}
		utils.InternalServerError(w, report[0].Err)
		return
	}
	// Docker waits for the container to stop if the signal is 0 or
	// SIGKILL.
	if !utils.IsLibpodRequest(r) {
		sig, err := signal.ParseSignalNameOrNumber(query.Signal)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		if sig == 0 || sig == syscall.SIGKILL {
			opts := entities.WaitOptions{
				Conditions: []string{define.ContainerStateExited.String(), define.ContainerStateStopped.String()},
				Interval:   time.Millisecond * 250,
			}
			if _, err := containerEngine.ContainerWait(r.Context(), []string{name}, opts); err != nil {
				utils.Error(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/wait
	utils.WaitContainerDocker(w, r)
}

func LibpodToContainer(l *libpod.Container, sz bool) (*handlers.Container, error) {
	imageID, imageName := l.Image()

	var (
		err        error
		sizeRootFs int64
		sizeRW     int64
		state      define.ContainerStatus
		status     string
	)

	if state, err = l.State(); err != nil {
		return nil, err
	}
	stateStr := state.String()

	// Some docker states are not the same as ours. This makes sure the state string stays true to the Docker API
	if state == define.ContainerStateCreated {
		stateStr = define.ContainerStateConfigured.String()
	}

	switch state {
	case define.ContainerStateConfigured, define.ContainerStateCreated:
		status = "Created"
	case define.ContainerStateStopped, define.ContainerStateExited:
		exitCode, _, err := l.ExitCode()
		if err != nil {
			return nil, err
		}
		finishedTime, err := l.FinishedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Exited (%d) %s ago", exitCode, units.HumanDuration(time.Since(finishedTime)))
	case define.ContainerStateRunning, define.ContainerStatePaused:
		startedTime, err := l.StartedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Up %s", units.HumanDuration(time.Since(startedTime)))
		if state == define.ContainerStatePaused {
			status += " (Paused)"
		}
	case define.ContainerStateRemoving:
		status = "Removal In Progress"
	case define.ContainerStateStopping:
		status = "Stopping"
	default:
		status = "Unknown"
	}

	if sz {
		if sizeRW, err = l.RWSize(); err != nil {
			return nil, err
		}
		if sizeRootFs, err = l.RootFsSize(); err != nil {
			return nil, err
		}
	}

	portMappings, err := l.PortMappings()
	if err != nil {
		return nil, err
	}

	ports := make([]types.Port, len(portMappings))
	for idx, portMapping := range portMappings {
		ports[idx] = types.Port{
			IP:          portMapping.HostIP,
			PrivatePort: portMapping.ContainerPort,
			PublicPort:  portMapping.HostPort,
			Type:        portMapping.Protocol,
		}
	}
	inspect, err := l.Inspect(false)
	if err != nil {
		return nil, err
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}
	networkSettings := types.SummaryNetworkSettings{}
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []types.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	return &handlers.Container{
		Container: types.Container{
			ID:         l.ID(),
			Names:      []string{fmt.Sprintf("/%s", l.Name())},
			Image:      imageName,
			ImageID:    "sha256:" + imageID,
			Command:    strings.Join(l.Command(), " "),
			Created:    l.CreatedTime().Unix(),
			Ports:      ports,
			SizeRw:     sizeRW,
			SizeRootFs: sizeRootFs,
			Labels:     l.Labels(),
			State:      stateStr,
			Status:     status,
			// FIXME: this seems broken, the field is never shown in the API output.
			HostConfig: struct {
				NetworkMode string            `json:",omitempty"`
				Annotations map[string]string `json:",omitempty"`
			}{
				NetworkMode: "host",
				// TODO: add annotations here for >= v1.46
			},
			NetworkSettings: &networkSettings,
			Mounts:          mounts,
		},
		ContainerCreateConfig: dockerBackend.ContainerCreateConfig{},
	}, nil
}

func convertSecondaryIPPrefixLen(input *define.InspectNetworkSettings, output *types.NetworkSettings) {
	for index, ip := range input.SecondaryIPAddresses {
		output.SecondaryIPAddresses[index].PrefixLen = ip.PrefixLength
	}
	for index, ip := range input.SecondaryIPv6Addresses {
		output.SecondaryIPv6Addresses[index].PrefixLen = ip.PrefixLength
	}
}

func LibpodToContainerJSON(l *libpod.Container, sz bool) (*types.ContainerJSON, error) {
	imageID, imageName := l.Image()
	inspect, err := l.Inspect(sz)
	if err != nil {
		return nil, err
	}
	// Docker uses UTC
	if inspect != nil && inspect.State != nil {
		inspect.State.StartedAt = inspect.State.StartedAt.UTC()
		inspect.State.FinishedAt = inspect.State.FinishedAt.UTC()
	}
	i, err := json.Marshal(inspect.State)
	if err != nil {
		return nil, err
	}
	state := types.ContainerState{}
	if err := json.Unmarshal(i, &state); err != nil {
		return nil, err
	}

	// docker considers paused to be running
	if state.Paused {
		state.Running = true
	}

	// Dockers created state is our configured state
	if state.Status == define.ContainerStateCreated.String() {
		state.Status = define.ContainerStateConfigured.String()
	}

	if l.HasHealthCheck() && state.Status != "created" {
		state.Health = &types.Health{}
		if inspect.State.Health != nil {
			state.Health.Status = inspect.State.Health.Status
			state.Health.FailingStreak = inspect.State.Health.FailingStreak
			log := inspect.State.Health.Log

			for _, item := range log {
				res := &types.HealthcheckResult{}
				s, err := time.Parse(time.RFC3339Nano, item.Start)
				if err != nil {
					return nil, err
				}
				e, err := time.Parse(time.RFC3339Nano, item.End)
				if err != nil {
					return nil, err
				}
				res.Start = s
				res.End = e
				res.ExitCode = item.ExitCode
				res.Output = item.Output
				state.Health.Log = append(state.Health.Log, res)
			}
		}
	}

	formatCapabilities(inspect.HostConfig.CapDrop)
	formatCapabilities(inspect.HostConfig.CapAdd)

	h, err := json.Marshal(inspect.HostConfig)
	if err != nil {
		return nil, err
	}
	hc := container.HostConfig{}
	if err := json.Unmarshal(h, &hc); err != nil {
		return nil, err
	}
	sort.Strings(hc.Binds)

	// k8s-file == json-file
	if hc.LogConfig.Type == define.KubernetesLogging {
		hc.LogConfig.Type = define.JSONLogging
	}
	g, err := json.Marshal(inspect.GraphDriver)
	if err != nil {
		return nil, err
	}
	graphDriver := types.GraphDriverData{}
	if err := json.Unmarshal(g, &graphDriver); err != nil {
		return nil, err
	}

	cb := types.ContainerJSONBase{
		ID:              l.ID(),
		Created:         l.CreatedTime().UTC().Format(time.RFC3339Nano), // Docker uses UTC
		Path:            inspect.Path,
		Args:            inspect.Args,
		State:           &state,
		Image:           "sha256:" + imageID,
		ResolvConfPath:  inspect.ResolvConfPath,
		HostnamePath:    inspect.HostnamePath,
		HostsPath:       inspect.HostsPath,
		LogPath:         l.LogPath(),
		Node:            nil,
		Name:            fmt.Sprintf("/%s", l.Name()),
		RestartCount:    int(inspect.RestartCount),
		Driver:          inspect.Driver,
		Platform:        "linux",
		MountLabel:      inspect.MountLabel,
		ProcessLabel:    inspect.ProcessLabel,
		AppArmorProfile: inspect.AppArmorProfile,
		ExecIDs:         inspect.ExecIDs,
		HostConfig:      &hc,
		GraphDriver:     graphDriver,
		SizeRw:          inspect.SizeRw,
		SizeRootFs:      &inspect.SizeRootFs,
	}

	// set Path and Args
	processArgs := l.Config().Spec.Process.Args
	if len(processArgs) > 0 {
		cb.Path = processArgs[0]
	}
	if len(processArgs) > 1 {
		cb.Args = processArgs[1:]
	}
	stopTimeout := int(l.StopTimeout())

	var healthcheck *container.HealthConfig
	if inspect.Config.Healthcheck != nil {
		healthcheck = &container.HealthConfig{
			Test:        inspect.Config.Healthcheck.Test,
			Interval:    inspect.Config.Healthcheck.Interval,
			Timeout:     inspect.Config.Healthcheck.Timeout,
			StartPeriod: inspect.Config.Healthcheck.StartPeriod,
			Retries:     inspect.Config.Healthcheck.Retries,
		}
	}

	// Apparently the compiler can't convert a map[string]struct{} into a nat.PortSet
	// (Despite a nat.PortSet being that exact struct with some types added)
	var exposedPorts nat.PortSet
	if len(inspect.Config.ExposedPorts) > 0 {
		exposedPorts = make(nat.PortSet)
		for p := range inspect.Config.ExposedPorts {
			exposedPorts[nat.Port(p)] = struct{}{}
		}
	}

	config := container.Config{
		Hostname:        l.Hostname(),
		Domainname:      inspect.Config.DomainName,
		User:            l.User(),
		AttachStdin:     inspect.Config.AttachStdin,
		AttachStdout:    inspect.Config.AttachStdout,
		AttachStderr:    inspect.Config.AttachStderr,
		ExposedPorts:    exposedPorts,
		Tty:             inspect.Config.Tty,
		OpenStdin:       inspect.Config.OpenStdin,
		StdinOnce:       inspect.Config.StdinOnce,
		Env:             inspect.Config.Env,
		Cmd:             l.Command(),
		Healthcheck:     healthcheck,
		ArgsEscaped:     false,
		Image:           imageName,
		Volumes:         nil,
		WorkingDir:      l.WorkingDir(),
		Entrypoint:      l.Entrypoint(),
		NetworkDisabled: false,
		MacAddress:      "",
		OnBuild:         nil,
		Labels:          l.Labels(),
		StopSignal:      strconv.Itoa(int(l.StopSignal())),
		StopTimeout:     &stopTimeout,
		Shell:           nil,
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []types.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	p, err := json.Marshal(inspect.NetworkSettings.Ports)
	if err != nil {
		return nil, err
	}
	ports := nat.PortMap{}
	if err := json.Unmarshal(p, &ports); err != nil {
		return nil, err
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}

	networkSettings := types.NetworkSettings{}
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}

	convertSecondaryIPPrefixLen(inspect.NetworkSettings, &networkSettings)

	// do not report null instead use an empty map
	if networkSettings.Networks == nil {
		networkSettings.Networks = map[string]*network.EndpointSettings{}
	}

	c := types.ContainerJSON{
		ContainerJSONBase: &cb,
		Mounts:            mounts,
		Config:            &config,
		NetworkSettings:   &networkSettings,
	}
	return &c, nil
}

func formatCapabilities(slice []string) {
	for i := range slice {
		slice[i] = strings.TrimPrefix(slice[i], "CAP_")
	}
}

func RenameContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)

	name := utils.GetName(r)
	query := struct {
		Name string `schema:"name"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if _, err := runtime.RenameContainer(r.Context(), ctr, query.Name); err != nil {
		if errors.Is(err, define.ErrPodExists) || errors.Is(err, define.ErrCtrExists) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func UpdateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	options := new(container.UpdateConfig)
	if err := json.NewDecoder(r.Body).Decode(options); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("decoding request body: %w", err))
		return
	}

	// Only handle the bits of update that Docker uses as examples.
	// For example, the update API claims to be able to update devices for
	// existing containers... Which I am very dubious about.
	// Ignore bits like that unless someone asks us for them.

	// We're going to be editing this, so we have to deep-copy to not affect
	// the container's own resources
	resources := new(spec.LinuxResources)
	oldResources := ctr.LinuxResources()
	if oldResources != nil {
		if err := libpod.JSONDeepCopy(oldResources, resources); err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("copying old resource limits: %w", err))
			return
		}
	}

	// CPU limits
	cpu := resources.CPU
	if cpu == nil {
		cpu = new(spec.LinuxCPU)
	}
	useCPU := false
	if options.CPUShares != 0 {
		shares := uint64(options.CPUShares)
		cpu.Shares = &shares
		useCPU = true
	}
	if options.CPUPeriod != 0 {
		period := uint64(options.CPUPeriod)
		cpu.Period = &period
		useCPU = true
	}
	if options.CPUQuota != 0 {
		cpu.Quota = &options.CPUQuota
		useCPU = true
	}
	if options.CPURealtimeRuntime != 0 {
		cpu.RealtimeRuntime = &options.CPURealtimeRuntime
		useCPU = true
	}
	if options.CPURealtimePeriod != 0 {
		period := uint64(options.CPURealtimePeriod)
		cpu.RealtimePeriod = &period
		useCPU = true
	}
	if options.CpusetCpus != "" {
		cpu.Cpus = options.CpusetCpus
		useCPU = true
	}
	if options.CpusetMems != "" {
		cpu.Mems = options.CpusetMems
		useCPU = true
	}
	if useCPU {
		resources.CPU = cpu
	}

	// Memory limits
	mem := resources.Memory
	if mem == nil {
		mem = new(spec.LinuxMemory)
	}
	useMem := false
	if options.Memory != 0 {
		mem.Limit = &options.Memory
		useMem = true
	}
	if options.MemorySwap != 0 {
		mem.Swap = &options.MemorySwap
		useMem = true
	}
	if options.MemoryReservation != 0 {
		mem.Reservation = &options.MemoryReservation
		useMem = true
	}
	if useMem {
		resources.Memory = mem
	}

	// PIDs limit
	if options.PidsLimit != nil {
		if resources.Pids == nil {
			resources.Pids = new(spec.LinuxPids)
		}
		resources.Pids.Limit = *options.PidsLimit
	}

	// Blkio Weight
	if options.BlkioWeight != 0 {
		if resources.BlockIO == nil {
			resources.BlockIO = new(spec.LinuxBlockIO)
		}
		resources.BlockIO.Weight = &options.BlkioWeight
	}

	// Restart policy
	localPolicy := string(options.RestartPolicy.Name)
	restartPolicy := &localPolicy

	var restartRetries *uint
	if options.RestartPolicy.MaximumRetryCount != 0 {
		localRetries := uint(options.RestartPolicy.MaximumRetryCount)
		restartRetries = &localRetries
	}

	if err := ctr.Update(resources, restartPolicy, restartRetries, &define.UpdateHealthCheckConfig{}); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("updating container: %w", err))
		return
	}

	responseStruct := container.ContainerUpdateOKBody{}
	utils.WriteResponse(w, http.StatusOK, responseStruct)
}
