package specgen

import (
	"context"
	"os"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// MakeContainer creates a container based on the SpecGenerator
func (s *SpecGenerator) MakeContainer(rt *libpod.Runtime) (*libpod.Container, error) {
	if err := s.validate(rt); err != nil {
		return nil, errors.Wrap(err, "invalid config provided")
	}
	rtc, err := rt.GetConfig()
	if err != nil {
		return nil, err
	}

	options, err := s.createContainerOptions(rt)
	if err != nil {
		return nil, err
	}

	podmanPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	options = append(options, s.createExitCommandOption(rtc, podmanPath))
	newImage, err := rt.ImageRuntime().NewFromLocal(s.Image)
	if err != nil {
		return nil, err
	}

	options = append(options, libpod.WithRootFSFromImage(newImage.ID(), s.Image, s.RawImageName))

	runtimeSpec, err := s.toOCISpec(rt, newImage)
	if err != nil {
		return nil, err
	}
	return rt.NewContainer(context.Background(), runtimeSpec, options...)
}

func (s *SpecGenerator) createContainerOptions(rt *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
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
	if s.Pod != "" {
		pod, err := rt.LookupPod(s.Pod)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("adding container to pod %s", s.Pod)
		options = append(options, rt.WithPod(pod))
	}
	destinations := []string{}
	//	// Take all mount and named volume destinations.
	for _, mount := range s.Mounts {
		destinations = append(destinations, mount.Destination)
	}
	for _, volume := range s.Volumes {
		destinations = append(destinations, volume.Dest)
	}
	options = append(options, libpod.WithUserVolumes(destinations))

	if len(s.Volumes) != 0 {
		options = append(options, libpod.WithNamedVolumes(s.Volumes))
	}

	if len(s.Command) != 0 {
		options = append(options, libpod.WithCommand(s.Command))
	}

	options = append(options, libpod.WithEntrypoint(s.Entrypoint))
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
	namespaceOptions, err := s.generateNamespaceContainerOpts(rt)
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

func (s *SpecGenerator) createExitCommandOption(config *config.Config, podmanPath string) libpod.CtrCreateOption {
	// We need a cleanup process for containers in the current model.
	// But we can't assume that the caller is Podman - it could be another
	// user of the API.
	// As such, provide a way to specify a path to Podman, so we can
	// still invoke a cleanup process.

	command := []string{podmanPath,
		"--root", config.StorageConfig.GraphRoot,
		"--runroot", config.StorageConfig.RunRoot,
		"--log-level", logrus.GetLevel().String(),
		"--cgroup-manager", config.CgroupManager,
		"--tmpdir", config.TmpDir,
	}
	if config.OCIRuntime != "" {
		command = append(command, []string{"--runtime", config.OCIRuntime}...)
	}
	if config.StorageConfig.GraphDriverName != "" {
		command = append(command, []string{"--storage-driver", config.StorageConfig.GraphDriverName}...)
	}
	for _, opt := range config.StorageConfig.GraphDriverOptions {
		command = append(command, []string{"--storage-opt", opt}...)
	}
	if config.EventsLogger != "" {
		command = append(command, []string{"--events-backend", config.EventsLogger}...)
	}

	// TODO Mheon wants to leave this for now
	//if s.sys {
	//	command = append(command, "--syslog", "true")
	//}
	command = append(command, []string{"container", "cleanup"}...)

	if s.Remove {
		command = append(command, "--rm")
	}
	return libpod.WithExitCommand(command)
}
