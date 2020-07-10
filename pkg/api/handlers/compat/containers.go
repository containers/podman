package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
		Link  bool `schema:"link"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.Link && !utils.IsLibpodRequest(r) {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			utils.ErrLinkNotSupport)
		return
	}

	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if err := runtime.RemoveContainer(r.Context(), con, query.Force, query.Vols); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	var (
		containers []*libpod.Container
		err        error
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All     bool                `schema:"all"`
		Limit   int                 `schema:"limit"`
		Size    bool                `schema:"size"`
		Filters map[string][]string `schema:"filters"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if query.All {
		containers, err = runtime.GetAllContainers()
	} else {
		containers, err = runtime.GetRunningContainers()
	}
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if _, found := r.URL.Query()["limit"]; found && query.Limit != -1 {
		last := query.Limit
		if len(containers) > last {
			containers = containers[len(containers)-last:]
		}
	}
	// TODO filters still need to be applied
	var list = make([]*handlers.Container, len(containers))
	for i, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, query.Size)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		list[i] = api
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Signal string `schema:"signal"`
	}{
		Signal: "KILL",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	sig, err := signal.ParseSignalNameOrNumber(query.Signal)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill Container %s, it is not running", name)))
		return
	}

	err = con.Kill(uint(sig))
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "unable to kill Container %s", name))
	}

	if utils.IsLibpodRequest(r) {
		// the kill behavior for docker differs from podman in that they appear to wait
		// for the Container to croak so the exit code is accurate immediately after the
		// kill is sent.  libpod does not.  but we can add a wait here only for the docker
		// side of things and mimic that behavior
		if _, err = con.Wait(); err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to wait for Container %s", con.ID()))
			return
		}
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	var msg string
	// /{version}/containers/(name)/wait
	exitCode, err := utils.WaitContainer(w, r)
	if err != nil {
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func LibpodToContainer(l *libpod.Container, sz bool) (*handlers.Container, error) {
	imageID, imageName := l.Image()

	var (
		err        error
		sizeRootFs int64
		sizeRW     int64
		state      define.ContainerStatus
	)

	if state, err = l.State(); err != nil {
		return nil, err
	}
	stateStr := state.String()
	if stateStr == "configured" {
		stateStr = "created"
	}

	if sz {
		if sizeRW, err = l.RWSize(); err != nil {
			return nil, err
		}
		if sizeRootFs, err = l.RootFsSize(); err != nil {
			return nil, err
		}
	}

	return &handlers.Container{Container: types.Container{
		ID:         l.ID(),
		Names:      []string{fmt.Sprintf("/%s", l.Name())},
		Image:      imageName,
		ImageID:    imageID,
		Command:    strings.Join(l.Command(), " "),
		Created:    l.CreatedTime().Unix(),
		Ports:      nil,
		SizeRw:     sizeRW,
		SizeRootFs: sizeRootFs,
		Labels:     l.Labels(),
		State:      stateStr,
		Status:     "",
		HostConfig: struct {
			NetworkMode string `json:",omitempty"`
		}{
			"host"},
		NetworkSettings: nil,
		Mounts:          nil,
	},
		ContainerCreateConfig: types.ContainerCreateConfig{},
	}, nil
}

func LibpodToContainerJSON(l *libpod.Container, sz bool) (*types.ContainerJSON, error) {
	_, imageName := l.Image()
	inspect, err := l.Inspect(sz)
	if err != nil {
		return nil, err
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

	h, err := json.Marshal(inspect.HostConfig)
	if err != nil {
		return nil, err
	}
	hc := container.HostConfig{}
	if err := json.Unmarshal(h, &hc); err != nil {
		return nil, err
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
		Created:         l.CreatedTime().String(),
		Path:            "",
		Args:            nil,
		State:           &state,
		Image:           imageName,
		ResolvConfPath:  inspect.ResolvConfPath,
		HostnamePath:    inspect.HostnamePath,
		HostsPath:       inspect.HostsPath,
		LogPath:         l.LogPath(),
		Node:            nil,
		Name:            fmt.Sprintf("/%s", l.Name()),
		RestartCount:    0,
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

	stopTimeout := int(l.StopTimeout())

	ports := make(nat.PortSet)
	for p := range inspect.HostConfig.PortBindings {
		splitp := strings.SplitN(p, "/", 2)
		if len(splitp) != 2 {
			return nil, errors.Errorf("PORT/PROTOCOL Format required for %q", p)
		}
		port, err := nat.NewPort(splitp[1], splitp[0])
		if err != nil {
			return nil, err
		}
		ports[port] = struct{}{}
	}

	config := container.Config{
		Hostname:        l.Hostname(),
		Domainname:      inspect.Config.DomainName,
		User:            l.User(),
		AttachStdin:     inspect.Config.AttachStdin,
		AttachStdout:    inspect.Config.AttachStdout,
		AttachStderr:    inspect.Config.AttachStderr,
		ExposedPorts:    ports,
		Tty:             inspect.Config.Tty,
		OpenStdin:       inspect.Config.OpenStdin,
		StdinOnce:       inspect.Config.StdinOnce,
		Env:             inspect.Config.Env,
		Cmd:             inspect.Config.Cmd,
		Healthcheck:     nil,
		ArgsEscaped:     false,
		Image:           imageName,
		Volumes:         nil,
		WorkingDir:      l.WorkingDir(),
		Entrypoint:      l.Entrypoint(),
		NetworkDisabled: false,
		MacAddress:      "",
		OnBuild:         nil,
		Labels:          l.Labels(),
		StopSignal:      string(l.StopSignal()),
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

	networkSettingsDefault := types.DefaultNetworkSettings{
		EndpointID:          "",
		Gateway:             "",
		GlobalIPv6Address:   "",
		GlobalIPv6PrefixLen: 0,
		IPAddress:           "",
		IPPrefixLen:         0,
		IPv6Gateway:         "",
		MacAddress:          l.Config().StaticMAC.String(),
	}

	networkSettings := types.NetworkSettings{
		NetworkSettingsBase:    types.NetworkSettingsBase{},
		DefaultNetworkSettings: networkSettingsDefault,
		Networks:               nil,
	}

	c := types.ContainerJSON{
		ContainerJSONBase: &cb,
		Mounts:            mounts,
		Config:            &config,
		NetworkSettings:   &networkSettings,
	}
	return &c, nil
}
