package generate

import (
	"context"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// MakeContainer creates a container based on the SpecGenerator
func MakeContainer(ctx context.Context, rt *libpod.Runtime, s *specgen.SpecGenerator) (*libpod.Container, error) {
	rtc, err := rt.GetConfig()
	if err != nil {
		return nil, err
	}

	// If joining a pod, retrieve the pod for use.
	var pod *libpod.Pod
	if s.Pod != "" {
		pod, err = rt.LookupPod(s.Pod)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving pod %s", s.Pod)
		}
	}

	// Set defaults for unset namespaces
	if s.PidNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("pid", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.PidNS = defaultNS
	}
	if s.IpcNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("ipc", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.IpcNS = defaultNS
	}
	if s.UtsNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("uts", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.UtsNS = defaultNS
	}
	if s.UserNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("user", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.UserNS = defaultNS
	}
	if s.NetNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("net", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.NetNS = defaultNS
	}
	if s.CgroupNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("cgroup", rtc, pod)
		if err != nil {
			return nil, err
		}
		s.CgroupNS = defaultNS
	}

	options := []libpod.CtrCreateOption{}
	options = append(options, libpod.WithCreateCommand())

	var newImage *image.Image
	if s.Rootfs != "" {
		options = append(options, libpod.WithRootFS(s.Rootfs))
	} else {
		newImage, err = rt.ImageRuntime().NewFromLocal(s.Image)
		if err != nil {
			return nil, err
		}
		imgName := s.Image
		names := newImage.Names()
		if len(names) > 0 {
			imgName = names[0]
		}
		options = append(options, libpod.WithRootFSFromImage(newImage.ID(), imgName, s.Image))
	}
	if err := s.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config provided")
	}

	finalMounts, finalVolumes, err := finalizeMounts(ctx, s, rt, rtc, newImage)
	if err != nil {
		return nil, err
	}

	opts, err := createContainerOptions(ctx, rt, s, pod, finalVolumes, newImage)
	if err != nil {
		return nil, err
	}
	options = append(options, opts...)

	podmanPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	// TODO: Enable syslog support - we'll need to put this in SpecGen.
	options = append(options, libpod.WithExitCommand(CreateExitCommandArgs(rt.StorageConfig(), rtc, podmanPath, false, s.Remove, false)))

	runtimeSpec, err := SpecGenToOCI(ctx, s, rt, rtc, newImage, finalMounts)
	if err != nil {
		return nil, err
	}
	return rt.NewContainer(ctx, runtimeSpec, options...)
}

func createContainerOptions(ctx context.Context, rt *libpod.Runtime, s *specgen.SpecGenerator, pod *libpod.Pod, volumes []*specgen.NamedVolume, img *image.Image) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var err error

	if s.Stdin {
		options = append(options, libpod.WithStdin())
	}
	if len(s.Systemd) > 0 {
		options = append(options, libpod.WithSystemd())
	}
	if len(s.Name) > 0 {
		logrus.Debugf("setting container name %s", s.Name)
		options = append(options, libpod.WithName(s.Name))
	}
	if pod != nil {
		logrus.Debugf("adding container to pod %s", pod.Name())
		options = append(options, rt.WithPod(pod))
	}
	destinations := []string{}
	// Take all mount and named volume destinations.
	for _, mount := range s.Mounts {
		destinations = append(destinations, mount.Destination)
	}
	for _, volume := range volumes {
		destinations = append(destinations, volume.Dest)
	}
	options = append(options, libpod.WithUserVolumes(destinations))

	if len(volumes) != 0 {
		var vols []*libpod.ContainerNamedVolume
		for _, v := range volumes {
			vols = append(vols, &libpod.ContainerNamedVolume{
				Name:    v.Name,
				Dest:    v.Dest,
				Options: v.Options,
			})
		}
		options = append(options, libpod.WithNamedVolumes(vols))
	}

	if s.Command != nil {
		options = append(options, libpod.WithCommand(s.Command))
	}
	if s.Entrypoint != nil {
		options = append(options, libpod.WithEntrypoint(s.Entrypoint))
	}
	if s.StopSignal != nil {
		options = append(options, libpod.WithStopSignal(*s.StopSignal))
	}
	if s.StopTimeout != nil {
		options = append(options, libpod.WithStopTimeout(*s.StopTimeout))
	}
	if s.LogConfiguration != nil {
		if len(s.LogConfiguration.Path) > 0 {
			options = append(options, libpod.WithLogPath(s.LogConfiguration.Path))
		}
		if len(s.LogConfiguration.Options) > 0 && s.LogConfiguration.Options["tag"] != "" {
			// Note: I'm really guessing here.
			options = append(options, libpod.WithLogTag(s.LogConfiguration.Options["tag"]))
		}

		if len(s.LogConfiguration.Driver) > 0 {
			options = append(options, libpod.WithLogDriver(s.LogConfiguration.Driver))
		}
	}

	// Security options
	if len(s.SelinuxOpts) > 0 {
		options = append(options, libpod.WithSecLabels(s.SelinuxOpts))
	}
	options = append(options, libpod.WithPrivileged(s.Privileged))

	// Get namespace related options
	namespaceOptions, err := GenerateNamespaceOptions(ctx, s, rt, pod, img)
	if err != nil {
		return nil, err
	}
	options = append(options, namespaceOptions...)

	if len(s.ConmonPidFile) > 0 {
		options = append(options, libpod.WithConmonPidFile(s.ConmonPidFile))
	}
	options = append(options, libpod.WithLabels(s.Labels))
	if s.ShmSize != nil {
		options = append(options, libpod.WithShmSize(*s.ShmSize))
	}
	if s.Rootfs != "" {
		options = append(options, libpod.WithRootFS(s.Rootfs))
	}
	// Default used if not overridden on command line

	if s.RestartPolicy != "" {
		if s.RestartPolicy == "unless-stopped" {
			return nil, errors.Wrapf(define.ErrInvalidArg, "the unless-stopped restart policy is not supported")
		}
		if s.RestartRetries != nil {
			options = append(options, libpod.WithRestartRetries(*s.RestartRetries))
		}
		options = append(options, libpod.WithRestartPolicy(s.RestartPolicy))
	}

	if s.ContainerHealthCheckConfig.HealthConfig != nil {
		options = append(options, libpod.WithHealthCheck(s.ContainerHealthCheckConfig.HealthConfig))
		logrus.Debugf("New container has a health check")
	}
	return options, nil
}

func CreateExitCommandArgs(storageConfig storage.StoreOptions, config *config.Config, podmanPath string, syslog, rm bool, exec bool) []string {
	// We need a cleanup process for containers in the current model.
	// But we can't assume that the caller is Podman - it could be another
	// user of the API.
	// As such, provide a way to specify a path to Podman, so we can
	// still invoke a cleanup process.

	command := []string{podmanPath,
		"--root", storageConfig.GraphRoot,
		"--runroot", storageConfig.RunRoot,
		"--log-level", logrus.GetLevel().String(),
		"--cgroup-manager", config.Engine.CgroupManager,
		"--tmpdir", config.Engine.TmpDir,
	}
	if config.Engine.OCIRuntime != "" {
		command = append(command, []string{"--runtime", config.Engine.OCIRuntime}...)
	}
	if storageConfig.GraphDriverName != "" {
		command = append(command, []string{"--storage-driver", storageConfig.GraphDriverName}...)
	}
	for _, opt := range storageConfig.GraphDriverOptions {
		command = append(command, []string{"--storage-opt", opt}...)
	}
	if config.Engine.EventsLogger != "" {
		command = append(command, []string{"--events-backend", config.Engine.EventsLogger}...)
	}

	if syslog {
		command = append(command, "--syslog", "true")
	}
	command = append(command, []string{"container", "cleanup"}...)

	if rm {
		command = append(command, "--rm")
	}

	if exec {
		command = append(command, "--exec")
	}

	return command
}
