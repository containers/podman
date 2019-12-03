package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/namespaces"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	input := CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	newImage, err := runtime.ImageRuntime().NewFromLocal(input.Image)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}
	cc, err := makeCreateConfig(input, newImage)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "makeCreatConfig()"))
		return
	}

	var pod *libpod.Pod
	ctr, err := shared.CreateContainerFromCreateConfig(runtime, &cc, r.Context(), pod)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "CreateContainerFromCreateConfig()"))
		return
	}

	type ctrCreateResponse struct {
		Id       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	response := ctrCreateResponse{
		Id:       ctr.ID(),
		Warnings: []string{}}

	WriteResponse(w, http.StatusCreated, response)
}

func makeCreateConfig(input CreateContainerConfig, newImage *image2.Image) (createconfig.CreateConfig, error) {
	var err error
	stopSignal := unix.SIGTERM
	if len(input.StopSignal) > 0 {
		stopSignal, err = signal.ParseSignal(input.StopSignal)
		if err != nil {
			return createconfig.CreateConfig{}, err
		}
	}

	workDir := "/"
	if len(input.WorkingDir) > 0 {
		workDir = input.WorkingDir
	}

	stopTimeout := uint(define.CtrRemoveTimeout)
	if input.StopTimeout != nil {
		stopTimeout = uint(*input.StopTimeout)
	}
	c := createconfig.CgroupConfig{
		Cgroups:      "", // podman
		Cgroupns:     "", // podman
		CgroupParent: "", // podman
		CgroupMode:   "", // podman
	}
	security := createconfig.SecurityConfig{
		CapAdd:             input.HostConfig.CapAdd,
		CapDrop:            input.HostConfig.CapDrop,
		LabelOpts:          nil,   // podman
		NoNewPrivs:         false, // podman
		ApparmorProfile:    "",    // podman
		SeccompProfilePath: "",
		SecurityOpts:       input.HostConfig.SecurityOpt,
		Privileged:         input.HostConfig.Privileged,
		ReadOnlyRootfs:     input.HostConfig.ReadonlyRootfs,
		ReadOnlyTmpfs:      false, // podman-only
		Sysctl:             input.HostConfig.Sysctls,
	}

	network := createconfig.NetworkConfig{
		DNSOpt:       input.HostConfig.DNSOptions,
		DNSSearch:    input.HostConfig.DNSSearch,
		DNSServers:   input.HostConfig.DNS,
		ExposedPorts: input.ExposedPorts,
		HTTPProxy:    false, // podman
		IP6Address:   "",
		IPAddress:    "",
		LinkLocalIP:  nil, // docker-only
		MacAddress:   input.MacAddress,
		// NetMode:      nil,
		Network:      input.HostConfig.NetworkMode.NetworkName(),
		NetworkAlias: nil, // docker-only now
		PortBindings: input.HostConfig.PortBindings,
		Publish:      nil, // podmanseccompPath
		PublishAll:   input.HostConfig.PublishAllPorts,
	}

	uts := createconfig.UtsConfig{
		UtsMode:  "",
		NoHosts:  false, // podman
		HostAdd:  input.HostConfig.ExtraHosts,
		Hostname: input.Hostname,
	}

	z := createconfig.UserConfig{
		GroupAdd:   input.HostConfig.GroupAdd,
		IDMappings: &storage.IDMappingOptions{}, // podman //TODO <--- fix this,
		UsernsMode: namespaces.UsernsMode(input.HostConfig.UsernsMode),
		User:       input.User,
	}
	pidConfig := createconfig.PidConfig{PidMode: namespaces.PidMode(input.HostConfig.PidMode)}

	m := createconfig.CreateConfig{
		Annotations:   nil, // podman
		Args:          nil,
		Cgroup:        c,
		CidFile:       "",
		ConmonPidFile: "", // podman
		Command:       input.Cmd,
		UserCommand:   input.Cmd, // podman
		Detach:        false,     //
		// Devices:            input.HostConfig.Devices,
		Entrypoint: input.Entrypoint,
		// Env:                input.Env,
		HealthCheck: nil, //
		// Init:               input.HostConfig.Init,
		InitPath:          "", // tbd
		Image:             input.Image,
		ImageID:           newImage.ID(),
		BuiltinImgVolumes: nil, // podman
		ImageVolumeType:   "",  // podman
		Interactive:       false,
		// IpcMode:           input.HostConfig.IpcMode,
		Labels:    input.Labels,
		LogDriver: input.HostConfig.LogConfig.Type, // is this correct
		// LogDriverOpt:       input.HostConfig.LogConfig.Config,
		Name:    input.Name,
		Network: network,
		// NetMode:            input.HostConfig.NetworkMode,
		// PidMode:            input.HostConfig.PidMode,
		Pod:           "",    // podman
		PodmanPath:    "",    // podman
		Quiet:         false, // front-end only
		Resources:     createconfig.CreateResourceConfig{},
		RestartPolicy: input.HostConfig.RestartPolicy.Name,
		Rm:            input.HostConfig.AutoRemove,
		StopSignal:    stopSignal,
		StopTimeout:   stopTimeout,
		Systemd:       false, // podman
		// Tmpfs:              input.HostConfig.Tmpfs,
		User: z,
		Uts:  uts,
		Tty:  input.Tty,
		// UtsMode:            input.HostConfig.UTSMode,
		Mounts: nil, // we populate
		// MountsFlag:         input.HostConfig.Mounts,
		NamedVolumes: nil, // we populate
		// Volumes:            input.Volumes,
		VolumesFrom: input.HostConfig.VolumesFrom,
		WorkDir:     workDir,
		Rootfs:      "", // podman
		Security:    security,
		Syslog:      false, // podman

		Pid: pidConfig,
	}
	return m, nil
}
