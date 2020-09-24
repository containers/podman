package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	image2 "github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/namespaces"
	"github.com/containers/podman/v2/pkg/signal"
	createconfig "github.com/containers/podman/v2/pkg/spec"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	input := handlers.CreateContainerConfig{}
	query := struct {
		Name string `schema:"name"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if len(input.HostConfig.Links) > 0 {
		utils.Error(w, utils.ErrLinkNotSupport.Error(), http.StatusBadRequest, errors.Wrapf(utils.ErrLinkNotSupport, "bad parameter"))
		return
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(input.Image)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchImage {
			utils.Error(w, "No such image", http.StatusNotFound, err)
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}
	containerConfig, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "GetConfig()"))
		return
	}
	cc, err := makeCreateConfig(r.Context(), containerConfig, input, newImage)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "makeCreatConfig()"))
		return
	}
	cc.Name = query.Name
	utils.CreateContainer(r.Context(), w, runtime, &cc)
}

func makeCreateConfig(ctx context.Context, containerConfig *config.Config, input handlers.CreateContainerConfig, newImage *image2.Image) (createconfig.CreateConfig, error) {
	var (
		err  error
		init bool
	)
	env := make(map[string]string)
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

	// Only use image's Cmd when the user does not set the entrypoint
	if input.Entrypoint == nil && len(input.Cmd) == 0 {
		cmdSlice, err := newImage.Cmd(ctx)
		if err != nil {
			return createconfig.CreateConfig{}, err
		}
		input.Cmd = cmdSlice
	}

	if input.Entrypoint == nil {
		entrypointSlice, err := newImage.Entrypoint(ctx)
		if err != nil {
			return createconfig.CreateConfig{}, err
		}
		input.Entrypoint = entrypointSlice
	}

	stopTimeout := containerConfig.Engine.StopTimeout
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
		UtsMode:  namespaces.UTSMode(input.HostConfig.UTSMode),
		NoHosts:  false, //podman
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
	// TODO: We should check that these binds are all listed in the `Volumes`
	// key since it doesn't make sense to define a `Binds` element for a
	// container path which isn't defined as a volume
	volumes := input.HostConfig.Binds

	// Docker is more flexible about its input where podman throws
	// away incorrectly formatted variables so we cannot reuse the
	// parsing of the env input
	// [Foo Other=one Blank=]
	for _, e := range input.Env {
		splitEnv := strings.Split(e, "=")
		switch len(splitEnv) {
		case 0:
			continue
		case 1:
			env[splitEnv[0]] = ""
		default:
			env[splitEnv[0]] = strings.Join(splitEnv[1:], "=")
		}
	}

	// format the tmpfs mounts into a []string from map
	tmpfs := make([]string, 0, len(input.HostConfig.Tmpfs))
	for k, v := range input.HostConfig.Tmpfs {
		tmpfs = append(tmpfs, fmt.Sprintf("%s:%s", k, v))
	}

	if input.HostConfig.Init != nil && *input.HostConfig.Init {
		init = true
	}

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
		Entrypoint:        input.Entrypoint,
		Env:               env,
		HealthCheck:       nil, //
		Init:              init,
		InitPath:          "", // tbd
		Image:             input.Image,
		ImageID:           newImage.ID(),
		BuiltinImgVolumes: nil, // podman
		ImageVolumeType:   "",  // podman
		Interactive:       input.OpenStdin,
		// IpcMode:           input.HostConfig.IpcMode,
		Labels:    input.Labels,
		LogDriver: input.HostConfig.LogConfig.Type, // is this correct
		// LogDriverOpt:       input.HostConfig.LogConfig.Config,
		Name:          input.Name,
		Network:       network,
		Pod:           "",    // podman
		PodmanPath:    "",    // podman
		Quiet:         false, // front-end only
		Resources:     createconfig.CreateResourceConfig{},
		RestartPolicy: input.HostConfig.RestartPolicy.Name,
		Rm:            input.HostConfig.AutoRemove,
		StopSignal:    stopSignal,
		StopTimeout:   stopTimeout,
		Systemd:       false, // podman
		Tmpfs:         tmpfs,
		User:          z,
		Uts:           uts,
		Tty:           input.Tty,
		Mounts:        nil, // we populate
		// MountsFlag:         input.HostConfig.Mounts,
		NamedVolumes: nil, // we populate
		Volumes:      volumes,
		VolumesFrom:  input.HostConfig.VolumesFrom,
		WorkDir:      workDir,
		Rootfs:       "", // podman
		Security:     security,
		Syslog:       false, // podman

		Pid: pidConfig,
	}

	fullCmd := append(input.Entrypoint, input.Cmd...)
	if len(fullCmd) > 0 {
		m.PodmanPath = fullCmd[0]
		if len(fullCmd) == 1 {
			m.Args = fullCmd
		} else {
			m.Args = fullCmd[1:]
		}
	}

	return m, nil
}
