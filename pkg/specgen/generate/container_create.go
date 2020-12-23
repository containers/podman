package generate

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// MakeContainer creates a container based on the SpecGenerator.
// Returns the created, container and any warnings resulting from creating the
// container, or an error.
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
	if s.ContainerCreateCommand != nil {
		options = append(options, libpod.WithCreateCommand(s.ContainerCreateCommand))
	}

	var newImage *image.Image
	if s.Rootfs != "" {
		options = append(options, libpod.WithRootFS(s.Rootfs))
	} else {
		newImage, err = rt.ImageRuntime().NewFromLocal(s.Image)
		if err != nil {
			return nil, err
		}
		// If the input name changed, we could properly resolve the
		// image. Otherwise, it must have been an ID where we're
		// defaulting to the first name or an empty one if no names are
		// present.
		imgName := newImage.InputName
		if s.Image == newImage.InputName && strings.HasPrefix(newImage.ID(), s.Image) {
			names := newImage.Names()
			if len(names) > 0 {
				imgName = names[0]
			}
		}

		options = append(options, libpod.WithRootFSFromImage(newImage.ID(), imgName, s.RawImageName))
	}
	if err := s.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config provided")
	}

	finalMounts, finalVolumes, finalOverlays, err := finalizeMounts(ctx, s, rt, rtc, newImage)
	if err != nil {
		return nil, err
	}

	command, err := makeCommand(ctx, s, newImage, rtc)
	if err != nil {
		return nil, err
	}

	opts, err := createContainerOptions(ctx, rt, s, pod, finalVolumes, finalOverlays, newImage, command)
	if err != nil {
		return nil, err
	}
	options = append(options, opts...)

	exitCommandArgs, err := CreateExitCommandArgs(rt.StorageConfig(), rtc, logrus.IsLevelEnabled(logrus.DebugLevel), s.Remove, false)
	if err != nil {
		return nil, err
	}
	options = append(options, libpod.WithExitCommand(exitCommandArgs))

	if len(s.Aliases) > 0 {
		options = append(options, libpod.WithNetworkAliases(s.Aliases))
	}

	runtimeSpec, err := SpecGenToOCI(ctx, s, rt, rtc, newImage, finalMounts, pod, command)
	if err != nil {
		return nil, err
	}
	return rt.NewContainer(ctx, runtimeSpec, options...)
}

func createContainerOptions(ctx context.Context, rt *libpod.Runtime, s *specgen.SpecGenerator, pod *libpod.Pod, volumes []*specgen.NamedVolume, overlays []*specgen.OverlayVolume, img *image.Image, command []string) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var err error

	if s.PreserveFDs > 0 {
		options = append(options, libpod.WithPreserveFDs(s.PreserveFDs))
	}

	if s.Stdin {
		options = append(options, libpod.WithStdin())
	}

	if s.Timezone != "" {
		options = append(options, libpod.WithTimezone(s.Timezone))
	}
	if s.Umask != "" {
		options = append(options, libpod.WithUmask(s.Umask))
	}

	useSystemd := false
	switch s.Systemd {
	case "always":
		useSystemd = true
	case "false":
		break
	case "", "true":
		if len(command) == 0 {
			command, err = img.Cmd(ctx)
			if err != nil {
				return nil, err
			}
		}

		if len(command) > 0 {
			useSystemdCommands := map[string]bool{
				"/sbin/init":           true,
				"/usr/sbin/init":       true,
				"/usr/local/sbin/init": true,
			}
			if useSystemdCommands[command[0]] || (filepath.Base(command[0]) == "systemd") {
				useSystemd = true
			}
		}
	default:
		return nil, errors.Wrapf(err, "invalid value %q systemd option requires 'true, false, always'", s.Systemd)
	}
	logrus.Debugf("using systemd mode: %t", useSystemd)
	if useSystemd {
		// is StopSignal was not set by the user then set it to systemd
		// expected StopSigal
		if s.StopSignal == nil {
			stopSignal, err := util.ParseSignal("RTMIN+3")
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing systemd signal")
			}
			s.StopSignal = &stopSignal
		}

		options = append(options, libpod.WithSystemd())
	}
	if len(s.SdNotifyMode) > 0 {
		options = append(options, libpod.WithSdNotifyMode(s.SdNotifyMode))
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
	for _, overlayVolume := range overlays {
		destinations = append(destinations, overlayVolume.Destination)
	}
	for _, imageVolume := range s.ImageVolumes {
		destinations = append(destinations, imageVolume.Destination)
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

	if len(overlays) != 0 {
		var vols []*libpod.ContainerOverlayVolume
		for _, v := range overlays {
			vols = append(vols, &libpod.ContainerOverlayVolume{
				Dest:   v.Destination,
				Source: v.Source,
			})
		}
		options = append(options, libpod.WithOverlayVolumes(vols))
	}

	if len(s.ImageVolumes) != 0 {
		var vols []*libpod.ContainerImageVolume
		for _, v := range s.ImageVolumes {
			vols = append(vols, &libpod.ContainerImageVolume{
				Dest:      v.Destination,
				Source:    v.Source,
				ReadWrite: v.ReadWrite,
			})
		}
		options = append(options, libpod.WithImageVolumes(vols))
	}

	if s.Command != nil {
		options = append(options, libpod.WithCommand(s.Command))
	}
	if s.Entrypoint != nil {
		options = append(options, libpod.WithEntrypoint(s.Entrypoint))
	}
	// If the user did not set an workdir but the image did, ensure it is
	// created.
	if s.WorkDir == "" && img != nil {
		options = append(options, libpod.WithCreateWorkingDir())
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
		if s.LogConfiguration.Size > 0 {
			options = append(options, libpod.WithMaxLogSize(s.LogConfiguration.Size))
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
	} else {
		if pod != nil {
			// duplicate the security options from the pod
			processLabel, err := pod.ProcessLabel()
			if err != nil {
				return nil, err
			}
			if processLabel != "" {
				selinuxOpts, err := label.DupSecOpt(processLabel)
				if err != nil {
					return nil, err
				}
				options = append(options, libpod.WithSecLabels(selinuxOpts))
			}
		}
	}
	options = append(options, libpod.WithPrivileged(s.Privileged))

	// Get namespace related options
	namespaceOptions, err := namespaceOptions(ctx, s, rt, pod, img)
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

func CreateExitCommandArgs(storageConfig storage.StoreOptions, config *config.Config, syslog, rm, exec bool) ([]string, error) {
	// We need a cleanup process for containers in the current model.
	// But we can't assume that the caller is Podman - it could be another
	// user of the API.
	// As such, provide a way to specify a path to Podman, so we can
	// still invoke a cleanup process.

	podmanPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

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
		command = append(command, "--syslog")
	}
	command = append(command, []string{"container", "cleanup"}...)

	if rm {
		command = append(command, "--rm")
	}

	// This has to be absolutely last, to ensure that the exec session ID
	// will be added after it by Libpod.
	if exec {
		command = append(command, "--exec")
	}

	return command, nil
}
