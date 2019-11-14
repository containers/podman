package serviceapi

import (
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/libpod/define"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/storage"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	createconfig "github.com/containers/libpod/pkg/spec"
)

func (s *APIServer) createContainer(w http.ResponseWriter, r *http.Request) {
	input := CreateContainer{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	newImage, err := s.Runtime.ImageRuntime().NewFromLocal(input.Image)
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
	ctr, err := shared.CreateContainerFromCreateConfig(s.Runtime, &cc, s.Context, pod)
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

	s.WriteResponse(w, http.StatusCreated, response)
	return
}

func makeCreateConfig(input CreateContainer, newImage *image2.Image) (createconfig.CreateConfig, error) {
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

	m := createconfig.CreateConfig{
		Annotations:   nil, // podman
		Args:          nil,
		CapAdd:        input.HostConfig.CapAdd,
		CapDrop:       input.HostConfig.CapDrop,
		CidFile:       "",
		ConmonPidFile: "", // podman
		Cgroupns:      "", // podman
		Cgroups:       "", // podman
		CgroupParent:  input.HostConfig.CgroupParent,
		Command:       input.Cmd,
		UserCommand:   nil,   // podman
		Detach:        false, //
		// Devices:            input.HostConfig.Devices,
		DNSOpt:     input.HostConfig.DNSOptions,
		DNSSearch:  input.HostConfig.DNSSearch,
		DNSServers: input.HostConfig.DNS,
		Entrypoint: input.Entrypoint,
		// Env:                input.Env,
		ExposedPorts: input.ExposedPorts,
		GroupAdd:     input.HostConfig.GroupAdd,
		HealthCheck:  nil,   //
		NoHosts:      false, // podman
		HostAdd:      input.HostConfig.ExtraHosts,
		Hostname:     input.Hostname,
		HTTPProxy:    false, // podman
		// Init:               input.HostConfig.Init,
		InitPath:          "", // tbd
		Image:             input.Image,
		ImageID:           newImage.ID(),
		BuiltinImgVolumes: nil,                         // podman
		IDMappings:        &storage.IDMappingOptions{}, // podman //TODO <--- fix this
		ImageVolumeType:   "",                          // podman
		Interactive:       false,
		// IpcMode:           input.HostConfig.IpcMode,
		IP6Address:  "",
		IPAddress:   "",
		Labels:      input.Labels,
		LinkLocalIP: nil,                             // docker-only
		LogDriver:   input.HostConfig.LogConfig.Type, // is this correct
		// LogDriverOpt:       input.HostConfig.LogConfig.Config,
		MacAddress: input.MacAddress,
		Name:       input.Name,
		// NetMode:            input.HostConfig.NetworkMode,
		Network:      input.HostConfig.NetworkMode.NetworkName(),
		NetworkAlias: nil, // dockeronly ?
		// PidMode:            input.HostConfig.PidMode,
		Pod:            "", // podman
		PodmanPath:     "", // podman
		CgroupMode:     "", // podman
		PortBindings:   input.HostConfig.PortBindings,
		Privileged:     input.HostConfig.Privileged,
		Publish:        nil, // podmanseccompPath
		PublishAll:     input.HostConfig.PublishAllPorts,
		Quiet:          false, // front-end only
		ReadOnlyRootfs: input.HostConfig.ReadonlyRootfs,
		ReadOnlyTmpfs:  false, // podman
		Resources:      createconfig.CreateResourceConfig{},
		RestartPolicy:  input.HostConfig.RestartPolicy.Name,
		Rm:             input.HostConfig.AutoRemove,
		StopSignal:     stopSignal,
		StopTimeout:    stopTimeout,
		Sysctl:         input.HostConfig.Sysctls,
		Systemd:        false, // podman
		// Tmpfs:              input.HostConfig.Tmpfs,
		Tty:        input.Tty,
		UsernsMode: namespaces.UsernsMode(input.HostConfig.UsernsMode),
		User:       input.User,
		// UtsMode:            input.HostConfig.UTSMode,
		Mounts: nil, // we populate
		// MountsFlag:         input.HostConfig.Mounts,
		NamedVolumes: nil, // we populate
		// Volumes:            input.Volumes,
		VolumesFrom:        input.HostConfig.VolumesFrom,
		WorkDir:            workDir,
		LabelOpts:          nil,          // we populate
		NoNewPrivs:         false,        // we populate
		ApparmorProfile:    "",           // we populate
		SeccompProfilePath: "unconfined", // we populate
		SecurityOpts:       input.HostConfig.SecurityOpt,
		Rootfs:             "",    // podman
		Syslog:             false, // podman
	}
	return m, nil
}
