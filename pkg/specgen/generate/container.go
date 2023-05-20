package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	ann "github.com/containers/podman/v4/pkg/annotations"
	envLib "github.com/containers/podman/v4/pkg/env"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/openshift/imagebuilder"
	"github.com/sirupsen/logrus"
)

func getImageFromSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) (*libimage.Image, string, *libimage.ImageData, error) {
	if s.Image == "" || s.Rootfs != "" {
		return nil, "", nil, nil
	}

	// Image may already have been set in the generator.
	image, resolvedName := s.GetImage()
	if image != nil {
		inspectData, err := image.Inspect(ctx, nil)
		if err != nil {
			return nil, "", nil, err
		}
		return image, resolvedName, inspectData, nil
	}

	// Need to look up image.
	lookupOptions := &libimage.LookupImageOptions{ManifestList: true}
	image, resolvedName, err := r.LibimageRuntime().LookupImage(s.Image, lookupOptions)
	if err != nil {
		return nil, "", nil, err
	}
	manifestList, err := image.ToManifestList()
	// only process if manifest list found otherwise expect it to be regular image
	if err == nil {
		image, err = manifestList.LookupInstance(ctx, s.ImageArch, s.ImageOS, s.ImageVariant)
		if err != nil {
			return nil, "", nil, err
		}
	}
	s.SetImage(image, resolvedName)
	inspectData, err := image.Inspect(ctx, nil)
	if err != nil {
		return nil, "", nil, err
	}
	return image, resolvedName, inspectData, err
}

// Fill any missing parts of the spec generator (e.g. from the image).
// Returns a set of warnings or any fatal error that occurred.
func CompleteSpec(ctx context.Context, r *libpod.Runtime, s *specgen.SpecGenerator) ([]string, error) {
	// Only add image configuration if we have an image
	newImage, _, inspectData, err := getImageFromSpec(ctx, r, s)
	if err != nil {
		return nil, err
	}
	if inspectData != nil {
		inspectData, err = newImage.Inspect(ctx, nil)
		if err != nil {
			return nil, err
		}

		if s.HealthConfig == nil {
			// NOTE: the health check is only set for Docker images
			// but inspect will take care of it.
			s.HealthConfig = inspectData.HealthCheck
			if s.HealthConfig != nil {
				if s.HealthConfig.Timeout == 0 {
					hct, err := time.ParseDuration(define.DefaultHealthCheckTimeout)
					if err != nil {
						return nil, err
					}
					s.HealthConfig.Timeout = hct
				}
				if s.HealthConfig.Interval == 0 {
					hct, err := time.ParseDuration(define.DefaultHealthCheckInterval)
					if err != nil {
						return nil, err
					}
					s.HealthConfig.Interval = hct
				}
			}
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

	rtc, err := r.GetConfigNoCopy()
	if err != nil {
		return nil, err
	}

	// Get Default Environment from containers.conf
	defaultEnvs, err := envLib.ParseSlice(rtc.GetDefaultEnvEx(s.EnvHost, s.HTTPProxy))
	if err != nil {
		return nil, fmt.Errorf("parsing fields in containers.conf: %w", err)
	}
	var envs map[string]string

	// Image Environment defaults
	if inspectData != nil {
		// Image envs from the image if they don't exist
		// already, overriding the default environments
		envs, err = envLib.ParseSlice(inspectData.Config.Env)
		if err != nil {
			return nil, fmt.Errorf("env fields from image failed to parse: %w", err)
		}
		defaultEnvs = envLib.Join(envLib.DefaultEnvVariables(), envLib.Join(defaultEnvs, envs))
	}

	for _, e := range s.EnvMerge {
		processedWord, err := imagebuilder.ProcessWord(e, envLib.Slice(defaultEnvs))
		if err != nil {
			return nil, fmt.Errorf("unable to process variables for --env-merge %s: %w", e, err)
		}
		splitWord := strings.Split(processedWord, "=")
		if _, ok := defaultEnvs[splitWord[0]]; ok {
			defaultEnvs[splitWord[0]] = splitWord[1]
		}
	}

	for _, e := range s.UnsetEnv {
		delete(defaultEnvs, e)
	}

	if s.UnsetEnvAll {
		defaultEnvs = make(map[string]string)
	}
	// First transform the os env into a map. We need it for the labels later in
	// any case.
	osEnv := envLib.Map(os.Environ())

	// Caller Specified defaults
	if s.EnvHost {
		defaultEnvs = envLib.Join(defaultEnvs, osEnv)
	} else if s.HTTPProxy {
		for _, envSpec := range config.ProxyEnv {
			if v, ok := osEnv[envSpec]; ok {
				defaultEnvs[envSpec] = v
			}
		}
	}

	s.Env = envLib.Join(defaultEnvs, s.Env)

	// Labels and Annotations
	if newImage != nil {
		labels, err := newImage.Labels(ctx)
		if err != nil {
			return nil, err
		}

		// labels from the image that don't already exist
		if len(labels) > 0 && s.Labels == nil {
			s.Labels = make(map[string]string)
		}
		for k, v := range labels {
			if _, exists := s.Labels[k]; !exists {
				s.Labels[k] = v
			}
		}

		// Do NOT include image annotations - these can have security
		// implications, we don't want untrusted images setting them.
	}

	// in the event this container is in a pod, and the pod has an infra container
	// we will want to configure it as a type "container" instead defaulting to
	// the behavior of a "sandbox" container
	// In Kata containers:
	// - "sandbox" is the annotation that denotes the container should use its own
	//   VM, which is the default behavior
	// - "container" denotes the container should join the VM of the SandboxID
	//   (the infra container)
	annotations := make(map[string]string)
	if len(s.Pod) > 0 {
		p, err := r.LookupPod(s.Pod)
		if err != nil {
			return nil, err
		}
		sandboxID := p.ID()
		if p.HasInfraContainer() {
			infra, err := p.InfraContainer()
			if err != nil {
				return nil, err
			}
			sandboxID = infra.ID()
		}
		annotations[ann.SandboxID] = sandboxID
		// Check if this is an init-ctr and if so, check if
		// the pod is running.  we do not want to add init-ctrs to
		// a running pod because it creates confusion for us.
		if len(s.InitContainerType) > 0 {
			containerStatuses, err := p.Status()
			if err != nil {
				return nil, err
			}
			// If any one of the containers is running, the pod is considered to be
			// running
			for _, con := range containerStatuses {
				if con == define.ContainerStateRunning {
					return nil, errors.New("cannot add init-ctr to a running pod")
				}
			}
		}
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
	// Unless already set via the CLI, check if we need to disable process
	// labels or set the defaults.
	if len(s.SelinuxOpts) == 0 {
		if err := setLabelOpts(s, r, s.PidNS, s.IpcNS); err != nil {
			return nil, err
		}
	}

	if s.CgroupsMode == "" {
		s.CgroupsMode = rtc.Cgroups()
	}

	// If caller did not specify Pids Limits load default
	s.InitResourceLimits(rtc)

	if s.LogConfiguration == nil {
		s.LogConfiguration = &specgen.LogConfig{}
	}
	// set log-driver from common if not already set
	if len(s.LogConfiguration.Driver) < 1 {
		s.LogConfiguration.Driver = rtc.Containers.LogDriver
	}
	if len(rtc.Containers.LogTag) > 0 {
		if s.LogConfiguration.Driver != define.JSONLogging {
			if s.LogConfiguration.Options == nil {
				s.LogConfiguration.Options = make(map[string]string)
			}

			s.LogConfiguration.Options["tag"] = rtc.Containers.LogTag
		} else {
			logrus.Warnf("log_tag %q is not allowed with %q log_driver", rtc.Containers.LogTag, define.JSONLogging)
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

// ConfigToSpec takes a completed container config and converts it back into a specgenerator for purposes of cloning an existing container
func ConfigToSpec(rt *libpod.Runtime, specg *specgen.SpecGenerator, containerID string) (*libpod.Container, *libpod.InfraInherit, error) {
	c, err := rt.LookupContainer(containerID)
	if err != nil {
		return nil, nil, err
	}
	conf := c.ConfigWithNetworks()
	if conf == nil {
		return nil, nil, fmt.Errorf("failed to get config for container %s", c.ID())
	}

	tmpSystemd := conf.Systemd
	tmpMounts := conf.Mounts

	conf.Systemd = nil
	conf.Mounts = []string{}

	if specg == nil {
		specg = &specgen.SpecGenerator{}
	}

	specg.Pod = conf.Pod

	matching, err := json.Marshal(conf)
	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(matching, specg)
	if err != nil {
		return nil, nil, err
	}

	conf.Systemd = tmpSystemd
	conf.Mounts = tmpMounts

	if conf.Spec != nil {
		if conf.Spec.Linux != nil && conf.Spec.Linux.Resources != nil {
			if specg.ResourceLimits == nil {
				specg.ResourceLimits = conf.Spec.Linux.Resources
			}
		}
		if conf.Spec.Process != nil && conf.Spec.Process.Env != nil {
			env := make(map[string]string)
			for _, entry := range conf.Spec.Process.Env {
				split := strings.SplitN(entry, "=", 2)
				if len(split) == 2 {
					env[split[0]] = split[1]
				}
			}
			specg.Env = env
		}
	}

	nameSpaces := []string{"pid", "net", "cgroup", "ipc", "uts", "user"}
	containers := []string{conf.PIDNsCtr, conf.NetNsCtr, conf.CgroupNsCtr, conf.IPCNsCtr, conf.UTSNsCtr, conf.UserNsCtr}
	place := []*specgen.Namespace{&specg.PidNS, &specg.NetNS, &specg.CgroupNS, &specg.IpcNS, &specg.UtsNS, &specg.UserNS}
	for i, ns := range containers {
		if len(ns) > 0 {
			ns := specgen.Namespace{NSMode: specgen.FromContainer, Value: ns}
			place[i] = &ns
		} else {
			switch nameSpaces[i] {
			case "pid":
				specg.PidNS = specgen.Namespace{NSMode: specgen.Default} // default
			case "net":
				switch {
				case conf.NetMode.IsBridge():
					toExpose := make(map[uint16]string, len(conf.ExposedPorts))
					for _, expose := range []map[uint16][]string{conf.ExposedPorts} {
						for port, proto := range expose {
							toExpose[port] = strings.Join(proto, ",")
						}
					}
					specg.Expose = toExpose
					specg.PortMappings = conf.PortMappings
					specg.NetNS = specgen.Namespace{NSMode: specgen.Bridge}
				case conf.NetMode.IsSlirp4netns():
					toExpose := make(map[uint16]string, len(conf.ExposedPorts))
					for _, expose := range []map[uint16][]string{conf.ExposedPorts} {
						for port, proto := range expose {
							toExpose[port] = strings.Join(proto, ",")
						}
					}
					specg.Expose = toExpose
					specg.PortMappings = conf.PortMappings
					netMode := strings.Split(string(conf.NetMode), ":")
					var val string
					if len(netMode) > 1 {
						val = netMode[1]
					}
					specg.NetNS = specgen.Namespace{NSMode: specgen.Slirp, Value: val}
				case conf.NetMode.IsPrivate():
					specg.NetNS = specgen.Namespace{NSMode: specgen.Private}
				case conf.NetMode.IsDefault():
					specg.NetNS = specgen.Namespace{NSMode: specgen.Default}
				case conf.NetMode.IsUserDefined():
					specg.NetNS = specgen.Namespace{NSMode: specgen.Path, Value: strings.Split(string(conf.NetMode), ":")[1]}
				case conf.NetMode.IsContainer():
					specg.NetNS = specgen.Namespace{NSMode: specgen.FromContainer, Value: strings.Split(string(conf.NetMode), ":")[1]}
				case conf.NetMode.IsPod():
					specg.NetNS = specgen.Namespace{NSMode: specgen.FromPod, Value: strings.Split(string(conf.NetMode), ":")[1]}
				}
			case "cgroup":
				specg.CgroupNS = specgen.Namespace{NSMode: specgen.Default} // default
			case "ipc":
				switch conf.ShmDir {
				case "/dev/shm":
					specg.IpcNS = specgen.Namespace{NSMode: specgen.Host}
				case "":
					specg.IpcNS = specgen.Namespace{NSMode: specgen.None}
				default:
					specg.IpcNS = specgen.Namespace{NSMode: specgen.Default} // default
				}
			case "uts":
				specg.UtsNS = specgen.Namespace{NSMode: specgen.Private} // default
			case "user":
				if conf.AddCurrentUserPasswdEntry {
					specg.UserNS = specgen.Namespace{NSMode: specgen.KeepID}
				} else {
					specg.UserNS = specgen.Namespace{NSMode: specgen.Default} // default
				}
			}
		}
	}

	specg.IDMappings = &conf.IDMappings
	specg.ContainerCreateCommand = conf.CreateCommand
	if len(specg.Rootfs) == 0 {
		specg.Rootfs = conf.Rootfs
	}
	if len(specg.Image) == 0 {
		specg.Image = conf.RootfsImageID
	}
	var named []*specgen.NamedVolume
	if len(conf.NamedVolumes) != 0 {
		for _, v := range conf.NamedVolumes {
			named = append(named, &specgen.NamedVolume{
				Name:    v.Name,
				Dest:    v.Dest,
				Options: v.Options,
			})
		}
	}
	specg.Volumes = named
	var image []*specgen.ImageVolume
	if len(conf.ImageVolumes) != 0 {
		for _, v := range conf.ImageVolumes {
			image = append(image, &specgen.ImageVolume{
				Source:      v.Source,
				Destination: v.Dest,
				ReadWrite:   v.ReadWrite,
			})
		}
	}
	specg.ImageVolumes = image
	var overlay []*specgen.OverlayVolume
	if len(conf.OverlayVolumes) != 0 {
		for _, v := range conf.OverlayVolumes {
			overlay = append(overlay, &specgen.OverlayVolume{
				Source:      v.Source,
				Destination: v.Dest,
				Options:     v.Options,
			})
		}
	}
	specg.OverlayVolumes = overlay
	_, mounts := c.SortUserVolumes(c.ConfigNoCopy().Spec)
	specg.Mounts = mounts
	specg.HostDeviceList = conf.DeviceHostSrc
	specg.Networks = conf.Networks
	specg.ShmSize = &conf.ShmSize
	specg.ShmSizeSystemd = &conf.ShmSizeSystemd

	mapSecurityConfig(conf, specg)

	if c.IsInfra() { // if we are creating this spec for a pod's infra ctr, map the compatible options
		spec, err := json.Marshal(specg)
		if err != nil {
			return nil, nil, err
		}
		infraInherit := &libpod.InfraInherit{}
		err = json.Unmarshal(spec, infraInherit)
		return c, infraInherit, err
	}
	// else just return the container
	return c, nil, nil
}

// mapSecurityConfig takes a libpod.ContainerSecurityConfig and converts it to a specgen.ContinerSecurityConfig
func mapSecurityConfig(c *libpod.ContainerConfig, s *specgen.SpecGenerator) {
	s.Privileged = c.Privileged
	s.SelinuxOpts = append(s.SelinuxOpts, c.LabelOpts...)
	s.User = c.User
	s.Groups = c.Groups
	s.HostUsers = c.HostUsers
}

// Check name looks for existing containers/pods with the same name, and modifies the given string until a new name is found
func CheckName(rt *libpod.Runtime, n string, kind bool) string {
	switch {
	case strings.Contains(n, "-clone"):
		ind := strings.Index(n, "-clone") + 6
		num, err := strconv.Atoi(n[ind:])
		if num == 0 && err != nil { // clone1 is hard to get with this logic, just check for it here.
			if kind {
				_, err = rt.LookupContainer(n + "1")
			} else {
				_, err = rt.LookupPod(n + "1")
			}

			if err != nil {
				n += "1"
				break
			}
		} else {
			n = n[0:ind]
		}
		err = nil
		count := num
		for err == nil {
			count++
			tempN := n + strconv.Itoa(count)
			if kind {
				_, err = rt.LookupContainer(tempN)
			} else {
				_, err = rt.LookupPod(tempN)
			}
		}
		n += strconv.Itoa(count)
	default:
		n += "-clone"
	}
	return n
}
