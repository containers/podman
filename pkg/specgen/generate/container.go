package generate

import (
	"context"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	ann "github.com/containers/libpod/pkg/annotations"
	envLib "github.com/containers/libpod/pkg/env"
	"github.com/containers/libpod/pkg/signal"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func CompleteSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) error {
	// If a rootfs is used, then there is no image data
	if s.ContainerStorageConfig.Rootfs != "" {
		return nil
	}

	newImage, err := r.ImageRuntime().NewFromLocal(s.Image)
	if err != nil {
		return err
	}

	_, mediaType, err := newImage.Manifest(ctx)
	if err != nil {
		return err
	}

	if s.HealthConfig == nil && mediaType == manifest.DockerV2Schema2MediaType {
		s.HealthConfig, err = newImage.GetHealthCheck(ctx)
		if err != nil {
			return err
		}
	}

	// Image stop signal
	if s.StopSignal == nil {
		stopSignal, err := newImage.StopSignal(ctx)
		if err != nil {
			return err
		}
		if stopSignal != "" {
			sig, err := signal.ParseSignalNameOrNumber(stopSignal)
			if err != nil {
				return err
			}
			s.StopSignal = &sig
		}
	}

	rtc, err := r.GetConfig()
	if err != nil {
		return err
	}
	// Get Default Environment
	defaultEnvs, err := envLib.ParseSlice(rtc.Containers.Env)
	if err != nil {
		return errors.Wrap(err, "Env fields in containers.conf failed to parse")
	}

	// Image envs from the image if they don't exist
	// already, overriding the default environments
	imageEnvs, err := newImage.Env(ctx)
	if err != nil {
		return err
	}

	envs, err := envLib.ParseSlice(imageEnvs)
	if err != nil {
		return errors.Wrap(err, "Env fields from image failed to parse")
	}
	s.Env = envLib.Join(envLib.Join(defaultEnvs, envs), s.Env)

	labels, err := newImage.Labels(ctx)
	if err != nil {
		return err
	}

	// labels from the image that dont exist already
	if len(labels) > 0 && s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	for k, v := range labels {
		if _, exists := s.Labels[k]; !exists {
			s.Labels[k] = v
		}
	}

	// annotations

	// Add annotations from the image
	annotations, err := newImage.Annotations(ctx)
	if err != nil {
		return err
	}
	for k, v := range annotations {
		annotations[k] = v
	}

	// in the event this container is in a pod, and the pod has an infra container
	// we will want to configure it as a type "container" instead defaulting to
	// the behavior of a "sandbox" container
	// In Kata containers:
	// - "sandbox" is the annotation that denotes the container should use its own
	//   VM, which is the default behavior
	// - "container" denotes the container should join the VM of the SandboxID
	//   (the infra container)

	if len(s.Pod) > 0 {
		annotations[ann.SandboxID] = s.Pod
		annotations[ann.ContainerType] = ann.ContainerTypeContainer
	}

	// now pass in the values from client
	for k, v := range s.Annotations {
		annotations[k] = v
	}
	s.Annotations = annotations

	// workdir
	workingDir, err := newImage.WorkingDir(ctx)
	if err != nil {
		return err
	}
	if len(s.WorkDir) < 1 && len(workingDir) > 1 {
		s.WorkDir = workingDir
	}

	if len(s.SeccompProfilePath) < 1 {
		p, err := libpod.DefaultSeccompPath()
		if err != nil {
			return err
		}
		s.SeccompProfilePath = p
	}

	if len(s.User) == 0 {
		s.User, err = newImage.User(ctx)
		if err != nil {
			return err
		}
	}
	if err := finishThrottleDevices(s); err != nil {
		return err
	}
	// Unless already set via the CLI, check if we need to disable process
	// labels or set the defaults.
	if len(s.SelinuxOpts) == 0 {
		if err := setLabelOpts(s, r, s.PidNS, s.IpcNS); err != nil {
			return err
		}
	}

	return nil
}

// finishThrottleDevices takes the temporary representation of the throttle
// devices in the specgen and looks up the major and major minors. it then
// sets the throttle devices proper in the specgen
func finishThrottleDevices(s *specgen.SpecGenerator) error {
	if bps := s.ThrottleReadBpsDevice; len(bps) > 0 {
		for k, v := range bps {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(statT.Rdev)))
			v.Minor = (int64(unix.Minor(statT.Rdev)))
			s.ResourceLimits.BlockIO.ThrottleReadBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleReadBpsDevice, v)
		}
	}
	if bps := s.ThrottleWriteBpsDevice; len(bps) > 0 {
		for k, v := range bps {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(statT.Rdev)))
			v.Minor = (int64(unix.Minor(statT.Rdev)))
			s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice, v)
		}
	}
	if iops := s.ThrottleReadIOPSDevice; len(iops) > 0 {
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(statT.Rdev)))
			v.Minor = (int64(unix.Minor(statT.Rdev)))
			s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice, v)
		}
	}
	if iops := s.ThrottleWriteBpsDevice; len(iops) > 0 {
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(statT.Rdev)))
			v.Minor = (int64(unix.Minor(statT.Rdev)))
			s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice, v)
		}
	}
	return nil
}
