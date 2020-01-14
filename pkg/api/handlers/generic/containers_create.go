package generic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/namespaces"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage"
	"github.com/docker/docker/pkg/signal"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

	newImage, err := runtime.ImageRuntime().NewFromLocal(input.Image)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}
	cc, err := makeCreateConfig(input, newImage)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "makeCreatConfig()"))
		return
	}

	cc.Name = query.Name
	var pod *libpod.Pod
	ctr, err := shared.CreateContainerFromCreateConfig(runtime, &cc, r.Context(), pod)
	if err != nil {
		if strings.Contains(err.Error(), "invalid log driver") {
			// this does not quite work yet and needs a little more massaging
			w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
			w.WriteHeader(http.StatusInternalServerError)
			msg := fmt.Sprintf("logger: no log driver named '%s' is registered", input.HostConfig.LogConfig.Type)
			if _, err := fmt.Fprintln(w, msg); err != nil {
				log.Errorf("%s: %q", msg, err)
			}
			//s.WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("logger: no log driver named '%s' is registered", input.HostConfig.LogConfig.Type))
			return
		}
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "CreateContainerFromCreateConfig()"))
		return
	}

	response := ContainerCreateResponse{
		Id:       ctr.ID(),
		Warnings: []string{}}

	utils.WriteResponse(w, http.StatusCreated, response)
}

func makeCreateConfig(input handlers.CreateContainerConfig, newImage *image2.Image) (createconfig.CreateConfig, error) {
	var (
		err     error
		init    bool
		tmpfs   []string
		volumes []string
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
	for k := range input.Volumes {
		volumes = append(volumes, k)
	}

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
		Interactive:       false,
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
	return m, nil
}
