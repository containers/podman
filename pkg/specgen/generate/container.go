package generate

import (
	"context"
	"os"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/image"
	ann "github.com/containers/podman/v2/pkg/annotations"
	envLib "github.com/containers/podman/v2/pkg/env"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/containers/podman/v2/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Fill any missing parts of the spec generator (e.g. from the image).
// Returns a set of warnings or any fatal error that occurred.
func CompleteSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) ([]string, error) {
	var (
		newImage *image.Image
		err      error
	)

	// Only add image configuration if we have an image
	if s.Image != "" {
		newImage, err = r.ImageRuntime().NewFromLocal(s.Image)
		if err != nil {
			return nil, err
		}

		_, mediaType, err := newImage.Manifest(ctx)
		if err != nil {
			if errors.Cause(err) != image.ErrImageIsBareList {
				return nil, err
			}
			// if err is not runnable image
			// use the local store image with repo@digest matches with the list, if exists
			manifestByte, manifestType, err := newImage.GetManifest(ctx, nil)
			if err != nil {
				return nil, err
			}
			list, err := manifest.ListFromBlob(manifestByte, manifestType)
			if err != nil {
				return nil, err
			}
			images, err := r.ImageRuntime().GetImages()
			if err != nil {
				return nil, err
			}
			findLocal := false
			listDigest, err := list.ChooseInstance(r.SystemContext())
			if err != nil {
				return nil, err
			}
			for _, img := range images {
				for _, imageDigest := range img.Digests() {
					if imageDigest == listDigest {
						newImage = img
						s.Image = img.ID()
						mediaType = manifestType
						findLocal = true
						logrus.Debug("image contains manifest list, using image from local storage")
						break
					}
				}
			}
			if !findLocal {
				return nil, image.ErrImageIsBareList
			}
		}

		if s.HealthConfig == nil && mediaType == manifest.DockerV2Schema2MediaType {
			s.HealthConfig, err = newImage.GetHealthCheck(ctx)
			if err != nil {
				return nil, err
			}
		}

		// Image stop signal
		if s.StopSignal == nil {
			stopSignal, err := newImage.StopSignal(ctx)
			if err != nil {
				return nil, err
			}
			if stopSignal != "" {
				sig, err := signal.ParseSignalNameOrNumber(stopSignal)
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
	// First transform the os env into a map. We need it for the labels later in
	// any case.
	osEnv, err := envLib.ParseSlice(os.Environ())
	if err != nil {
		return nil, errors.Wrap(err, "error parsing host environment variables")
	}

	// Get Default Environment from containers.conf
	defaultEnvs, err := envLib.ParseSlice(rtc.GetDefaultEnv())
	if err != nil {
		return nil, errors.Wrap(err, "error parsing fields in containers.conf")
	}
	if defaultEnvs["container"] == "" {
		defaultEnvs["container"] = "podman"
	}
	var envs map[string]string

	// Image Environment defaults
	if newImage != nil {
		// Image envs from the image if they don't exist
		// already, overriding the default environments
		imageEnvs, err := newImage.Env(ctx)
		if err != nil {
			return nil, err
		}

		envs, err = envLib.ParseSlice(imageEnvs)
		if err != nil {
			return nil, errors.Wrap(err, "Env fields from image failed to parse")
		}
		defaultEnvs = envLib.Join(defaultEnvs, envs)
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

		// labels from the image that dont exist already
		if len(labels) > 0 && s.Labels == nil {
			s.Labels = make(map[string]string)
		}
		for k, v := range labels {
			if _, exists := s.Labels[k]; !exists {
				s.Labels[k] = v
			}
		}

		// Add annotations from the image
		imgAnnotations, err := newImage.Annotations(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range imgAnnotations {
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

	// now pass in the values from client
	for k, v := range s.Annotations {
		annotations[k] = v
	}
	s.Annotations = annotations

	// workdir
	if s.WorkDir == "" {
		if newImage != nil {
			workingDir, err := newImage.WorkingDir(ctx)
			if err != nil {
				return nil, err
			}
			s.WorkDir = workingDir
		}
	}
	if s.WorkDir == "" {
		s.WorkDir = "/"
	}

	if len(s.SeccompProfilePath) < 1 {
		p, err := libpod.DefaultSeccompPath()
		if err != nil {
			return nil, err
		}
		s.SeccompProfilePath = p
	}

	if len(s.User) == 0 && newImage != nil {
		s.User, err = newImage.User(ctx)
		if err != nil {
			return nil, err
		}
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
	if iops := s.ThrottleWriteIOPSDevice; len(iops) > 0 {
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
