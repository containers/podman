package serviceapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	createconfig "github.com/containers/libpod/pkg/spec"
)

func createContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	ctx := context.Background()
	input := CreateContainer{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	cc, err := makeCreateConfig(input)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	var pod *libpod.Pod
	_, err = shared.CreateContainerFromCreateConfig(runtime, &cc, ctx, pod)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	return
}

func makeCreateConfig(input CreateContainer) (createconfig.CreateConfig, error) {
	m := createconfig.CreateConfig{
		Annotations:   nil,
		Args:          nil,
		CapAdd:        input.HostConfig.CapAdd,
		CapDrop:       input.HostConfig.CapDrop,
		CidFile:       "",
		ConmonPidFile: "", //podman
		Cgroupns:      "", //podman
		Cgroups:       "", //podman
		CgroupParent:  input.HostConfig.CgroupParent,
		Command:       input.Cmd,
		UserCommand:   nil,   //podman
		Detach:        false, //
		//Devices:            input.HostConfig.Devices,
		DNSOpt:     input.HostConfig.DNSOptions,
		DNSSearch:  input.HostConfig.DNSSearch,
		DNSServers: input.HostConfig.DNS,
		Entrypoint: input.Entrypoint,
		//Env:                input.Env,
		ExposedPorts: input.ExposedPorts,
		GroupAdd:     input.HostConfig.GroupAdd,
		HealthCheck:  nil,   //
		NoHosts:      false, //podman
		HostAdd:      input.HostConfig.ExtraHosts,
		Hostname:     input.Hostname,
		HTTPProxy:    false, //podman
		//Init:               input.HostConfig.Init,
		InitPath:          "", // tbd
		Image:             input.Image,
		ImageID:           "",  // added later
		BuiltinImgVolumes: nil, //podman
		IDMappings:        nil, //podman
		ImageVolumeType:   "",  //podman
		Interactive:       false,
		//IpcMode:           input.HostConfig.IpcMode,
		IP6Address:  "",
		IPAddress:   "",
		Labels:      input.Labels,
		LinkLocalIP: nil,                             //docker-only
		LogDriver:   input.HostConfig.LogConfig.Type, // is this correct
		//LogDriverOpt:       input.HostConfig.LogConfig.Config,
		MacAddress: input.MacAddress,
		Name:       input.Name,
		//NetMode:            input.HostConfig.NetworkMode,
		Network:      input.HostConfig.NetworkMode.NetworkName(),
		NetworkAlias: nil, // dockeronly ?
		//PidMode:            input.HostConfig.PidMode,
		Pod:            "", //podman
		PodmanPath:     "", //podman
		CgroupMode:     "", //podman
		PortBindings:   input.HostConfig.PortBindings,
		Privileged:     input.HostConfig.Privileged,
		Publish:        nil, //podman
		PublishAll:     input.HostConfig.PublishAllPorts,
		Quiet:          false, //front-end only
		ReadOnlyRootfs: input.HostConfig.ReadonlyRootfs,
		ReadOnlyTmpfs:  false, //podman
		Resources:      createconfig.CreateResourceConfig{},
		//RestartPolicy:      input.HostConfig.RestartPolicy,
		Rm: input.HostConfig.AutoRemove,
		//StopSignal:         input.StopSignal,
		//StopTimeout:        input.StopTimeout,
		Sysctl:  input.HostConfig.Sysctls,
		Systemd: false, //podman
		//Tmpfs:              input.HostConfig.Tmpfs,
		Tty: input.Tty,
		//UsernsMode:         input.HostConfig.UsernsMode,
		User: input.User,
		//UtsMode:            input.HostConfig.UTSMode,
		Mounts: nil, //we populate
		//MountsFlag:         input.HostConfig.Mounts,
		NamedVolumes: nil, // we populate
		//Volumes:            input.Volumes,
		VolumesFrom:        input.HostConfig.VolumesFrom,
		WorkDir:            input.WorkingDir,
		LabelOpts:          nil,   // we populate
		NoNewPrivs:         false, // we populate
		ApparmorProfile:    "",    // we populate
		SeccompProfilePath: "",    //we populate
		SecurityOpts:       input.HostConfig.SecurityOpt,
		Rootfs:             "",    //podman
		Syslog:             false, //podman
	}

	return m, nil
}
