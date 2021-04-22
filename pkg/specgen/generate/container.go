package generate

import (
	"context"
	"os"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v3/libpod"
	ann "github.com/containers/podman/v3/pkg/annotations"
	envLib "github.com/containers/podman/v3/pkg/env"
	"github.com/containers/podman/v3/pkg/signal"
	"github.com/containers/podman/v3/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Fill any missing parts of the spec generator (e.g. from the image).
// Returns a set of warnings or any fatal error that occurred.
func CompleteSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) ([]string, error) {
	// Only add image configuration if we have an image
	var newImage *libimage.Image
	var inspectData *libimage.ImageData
	var err error
	if s.Image != "" {
		newImage, _, err = r.LibimageRuntime().LookupImage(s.Image, nil)
		if err != nil {
			return nil, err
		}

		inspectData, err = newImage.Inspect(ctx, false)
		if err != nil {
			return nil, err
		}

		if s.HealthConfig == nil {
			// NOTE: the health check is only set for Docker images
			// but inspect will take care of it.
			s.HealthConfig = inspectData.HealthCheck
		}

		// Image stop signal
		if s.StopSignal == nil {
			if inspectData.Config.StopSignal != "" {
				sig, err := signal.ParseSignalNameOrNumber(inspectData.Config.StopSignal)
				if err != nil {
					return nil, err
				}
				s.StopSignal = &sig
			}
		}
	}

	rtc, err := r.GetConfig()
	if err != nil {
		return nil, err
	}

	// Get Default Environment from containers.conf
	defaultEnvs, err := envLib.ParseSlice(rtc.GetDefaultEnvEx(s.EnvHost, s.HTTPProxy))
	if err != nil {
		return nil, errors.Wrap(err, "error parsing fields in containers.conf")
	}
	if defaultEnvs["container"] == "" {
		defaultEnvs["container"] = "podman"
	}
	var envs map[string]string

	// Image Environment defaults
	if inspectData != nil {
		// Image envs from the image if they don't exist
		// already, overriding the default environments
		envs, err = envLib.ParseSlice(inspectData.Config.Env)
		if err != nil {
			return nil, errors.Wrap(err, "Env fields from image failed to parse")
		}
		defaultEnvs = envLib.Join(defaultEnvs, envs)
	}

	// First transform the os env into a map. We need it for the labels later in
	// any case.
	osEnv, err := envLib.ParseSlice(os.Environ())
	if err != nil {
		return nil, errors.Wrap(err, "error parsing host environment variables")
	}
	// Caller Specified defaults
	if s.EnvHost {
		defaultEnvs = envLib.Join(defaultEnvs, osEnv)
	} else if s.HTTPProxy {
		for _, envSpec := range []string{
			"http_proxy",
			"HTTP_PROXY",
			"https_proxy",
			"HTTPS_PROXY",
			"ftp_proxy",
			"FTP_PROXY",
			"no_proxy",
			"NO_PROXY",
		} {
			if v, ok := osEnv[envSpec]; ok {
				defaultEnvs[envSpec] = v
			}
		}
	}

	s.Env = envLib.Join(defaultEnvs, s.Env)

	// Labels and Annotations
	annotations := make(map[string]string)
	if newImage != nil {
		labels, err := newImage.Labels(ctx)
		if err != nil {
			return nil, err
		}

		// labels from the image that don't exist already
		if len(labels) > 0 && s.Labels == nil {
			s.Labels = make(map[string]string)
		}
		for k, v := range labels {
			if _, exists := s.Labels[k]; !exists {
				s.Labels[k] = v
			}
		}

		// Add annotations from the image
		for k, v := range inspectData.Annotations {
			annotations[k] = v
		}
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

	for _, v := range rtc.Containers.Annotations {
		split := strings.SplitN(v, "=", 2)
		k := split[0]
		v := ""
		if len(split) == 2 {
			v = split[1]
		}
		annotations[k] = v
	}
	// now pass in the values from client
	for k, v := range s.Annotations {
		annotations[k] = v
	}
	s.Annotations = annotations

	if len(s.SeccompProfilePath) < 1 {
		p, err := libpod.DefaultSeccompPath()
		if err != nil {
			return nil, err
		}
		s.SeccompProfilePath = p
	}

	if len(s.User) == 0 && inspectData != nil {
		s.User = inspectData.Config.User
	}
	if err := finishThrottleDevices(s); err != nil {
		return nil, err
	}
	// Unless already set via the CLI, check if we need to disable process
	// labels or set the defaults.
	if len(s.SelinuxOpts) == 0 {
		if err := setLabelOpts(s, r, s.PidNS, s.IpcNS); err != nil {
			return nil, err
		}
	}

	// If caller did not specify Pids Limits load default
	if s.ResourceLimits == nil || s.ResourceLimits.Pids == nil {
		if s.CgroupsMode != "disabled" {
			limit := rtc.PidsLimit()
			if limit != 0 {
				if s.ResourceLimits == nil {
					s.ResourceLimits = &spec.LinuxResources{}
				}
				s.ResourceLimits.Pids = &spec.LinuxPids{
					Limit: limit,
				}
			}
		}
	}

	if s.LogConfiguration == nil {
		s.LogConfiguration = &specgen.LogConfig{}
	}
	// set log-driver from common if not already set
	if len(s.LogConfiguration.Driver) < 1 {
		s.LogConfiguration.Driver = rtc.Containers.LogDriver
	}

	warnings, err := verifyContainerResources(s)
	if err != nil {
		return warnings, err
	}

	// Warn on net=host/container/pod/none and port mappings.
	if (s.NetNS.NSMode == specgen.Host || s.NetNS.NSMode == specgen.FromContainer ||
		s.NetNS.NSMode == specgen.FromPod || s.NetNS.NSMode == specgen.NoNetwork) &&
		len(s.PortMappings) > 0 {
		warnings = append(warnings, "Port mappings have been discarded as one of the Host, Container, Pod, and None network modes are in use")
	}

	return warnings, nil
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
			v.Major = (int64(unix.Major(uint64(statT.Rdev))))
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev))))
			s.ResourceLimits.BlockIO.ThrottleReadBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleReadBpsDevice, v)
		}
	}
	if bps := s.ThrottleWriteBpsDevice; len(bps) > 0 {
		for k, v := range bps {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev))))
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev))))
			s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice, v)
		}
	}
	if iops := s.ThrottleReadIOPSDevice; len(iops) > 0 {
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev))))
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev))))
			s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice, v)
		}
	}
	if iops := s.ThrottleWriteIOPSDevice; len(iops) > 0 {
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return err
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev))))
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev))))
			s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice, v)
		}
	}
	return nil
}
