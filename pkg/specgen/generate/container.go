package generate

import (
	"context"

	"github.com/containers/libpod/libpod"
	ann "github.com/containers/libpod/pkg/annotations"
	envLib "github.com/containers/libpod/pkg/env"
	"github.com/containers/libpod/pkg/signal"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func CompleteSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) error {

	newImage, err := r.ImageRuntime().NewFromLocal(s.Image)
	if err != nil {
		return err
	}

	// Image stop signal
	if s.StopSignal == nil && newImage.Config != nil {
		sig, err := signal.ParseSignalNameOrNumber(newImage.Config.StopSignal)
		if err != nil {
			return err
		}
		s.StopSignal = &sig
	}
	// Image envs from the image if they don't exist
	// already
	if newImage.Config != nil && len(newImage.Config.Env) > 0 {
		envs, err := envLib.ParseSlice(newImage.Config.Env)
		if err != nil {
			return err
		}
		for k, v := range envs {
			if _, exists := s.Env[k]; !exists {
				s.Env[v] = k
			}
		}
	}

	// labels from the image that dont exist already
	if config := newImage.Config; config != nil {
		for k, v := range config.Labels {
			if _, exists := s.Labels[k]; !exists {
				s.Labels[k] = v
			}
		}
	}

	// annotations
	// in the event this container is in a pod, and the pod has an infra container
	// we will want to configure it as a type "container" instead defaulting to
	// the behavior of a "sandbox" container
	// In Kata containers:
	// - "sandbox" is the annotation that denotes the container should use its own
	//   VM, which is the default behavior
	// - "container" denotes the container should join the VM of the SandboxID
	//   (the infra container)
	s.Annotations = make(map[string]string)
	if len(s.Pod) > 0 {
		s.Annotations[ann.SandboxID] = s.Pod
		s.Annotations[ann.ContainerType] = ann.ContainerTypeContainer
	}
	//
	// Next, add annotations from the image
	annotations, err := newImage.Annotations(ctx)
	if err != nil {
		return err
	}
	for k, v := range annotations {
		annotations[k] = v
	}

	// entrypoint
	if config := newImage.Config; config != nil {
		if len(s.Entrypoint) < 1 && len(config.Entrypoint) > 0 {
			s.Entrypoint = config.Entrypoint
		}
		if len(s.Command) < 1 && len(config.Cmd) > 0 {
			s.Command = config.Cmd
		}
		if len(s.Command) < 1 && len(s.Entrypoint) < 1 {
			return errors.Errorf("No command provided or as CMD or ENTRYPOINT in this image")
		}
		// workdir
		if len(s.WorkDir) < 1 && len(config.WorkingDir) > 1 {
			s.WorkDir = config.WorkingDir
		}
	}

	if len(s.SeccompProfilePath) < 1 {
		p, err := libpod.DefaultSeccompPath()
		if err != nil {
			return err
		}
		s.SeccompProfilePath = p
	}

	if user := s.User; len(user) == 0 {
		switch {
		// TODO This should be enabled when namespaces actually work
		//case usernsMode.IsKeepID():
		//	user = fmt.Sprintf("%d:%d", rootless.GetRootlessUID(), rootless.GetRootlessGID())
		case newImage.Config == nil || (newImage.Config != nil && len(newImage.Config.User) == 0):
			s.User = "0"
		default:
			s.User = newImage.Config.User
		}
	}
	if err := finishThrottleDevices(s); err != nil {
		return err
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
