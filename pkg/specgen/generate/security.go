package generate

import (
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// setLabelOpts sets the label options of the SecurityConfig according to the
// input.
func setLabelOpts(s *specgen.SpecGenerator, runtime *libpod.Runtime, pidConfig specgen.Namespace, ipcConfig specgen.Namespace) error {
	if !runtime.EnableLabeling() || s.Privileged {
		s.SelinuxOpts = label.DisableSecOpt()
		return nil
	}

	var labelOpts []string
	if pidConfig.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if pidConfig.IsContainer() {
		ctr, err := runtime.LookupContainer(pidConfig.Value)
		if err != nil {
			return errors.Wrapf(err, "container %q not found", pidConfig.Value)
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
	}

	if ipcConfig.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if ipcConfig.IsContainer() {
		ctr, err := runtime.LookupContainer(ipcConfig.Value)
		if err != nil {
			return errors.Wrapf(err, "container %q not found", ipcConfig.Value)
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
	}

	s.SelinuxOpts = append(s.SelinuxOpts, labelOpts...)
	return nil
}

func setupApparmor(s *specgen.SpecGenerator, rtc *config.Config, g *generate.Generator) error {
	hasProfile := len(s.ApparmorProfile) > 0
	if !apparmor.IsEnabled() {
		if hasProfile && s.ApparmorProfile != "unconfined" {
			return errors.Errorf("Apparmor profile %q specified, but Apparmor is not enabled on this system", s.ApparmorProfile)
		}
		return nil
	}
	// If privileged and caller did not specify apparmor profiles return
	if s.Privileged && !hasProfile {
		return nil
	}
	if !hasProfile {
		s.ApparmorProfile = rtc.Containers.ApparmorProfile
	}
	if len(s.ApparmorProfile) > 0 {
		g.SetProcessApparmorProfile(s.ApparmorProfile)
	}

	return nil
}

func securityConfigureGenerator(s *specgen.SpecGenerator, g *generate.Generator, newImage *libimage.Image, rtc *config.Config) error {
	var (
		caplist []string
		err     error
	)
	// HANDLE CAPABILITIES
	// NOTE: Must happen before SECCOMP
	if s.Privileged {
		g.SetupPrivileged(true)
		caplist, err = capabilities.BoundingSet()
		if err != nil {
			return err
		}
	} else {
		mergedCaps, err := capabilities.MergeCapabilities(rtc.Containers.DefaultCapabilities, s.CapAdd, s.CapDrop)
		if err != nil {
			return err
		}
		boundingSet, err := capabilities.BoundingSet()
		if err != nil {
			return err
		}
		boundingCaps := make(map[string]interface{})
		for _, b := range boundingSet {
			boundingCaps[b] = b
		}
		for _, c := range mergedCaps {
			if _, ok := boundingCaps[c]; ok {
				caplist = append(caplist, c)
			}
		}

		privCapsRequired := []string{}

		// If the container image specifies an label with a
		// capabilities.ContainerImageLabel then split the comma separated list
		// of capabilities and record them.  This list indicates the only
		// capabilities, required to run the container.
		var capsRequiredRequested []string
		for key, val := range s.Labels {
			if util.StringInSlice(key, capabilities.ContainerImageLabels) {
				capsRequiredRequested = strings.Split(val, ",")
			}
		}
		if !s.Privileged && len(capsRequiredRequested) > 0 {
			// Pass capRequiredRequested in CapAdd field to normalize capabilities names
			capsRequired, err := capabilities.MergeCapabilities(nil, capsRequiredRequested, nil)
			if err != nil {
				return errors.Wrapf(err, "capabilities requested by user or image are not valid: %q", strings.Join(capsRequired, ","))
			}
			// Verify all capRequired are in the capList
			for _, cap := range capsRequired {
				if !util.StringInSlice(cap, caplist) {
					privCapsRequired = append(privCapsRequired, cap)
				}
			}
			if len(privCapsRequired) == 0 {
				caplist = capsRequired
			} else {
				logrus.Errorf("Capabilities requested by user or image are not allowed by default: %q", strings.Join(privCapsRequired, ","))
			}
		}
	}

	configSpec := g.Config
	configSpec.Process.Capabilities.Ambient = []string{}
	configSpec.Process.Capabilities.Bounding = caplist

	user := strings.Split(s.User, ":")[0]

	if (user == "" && s.UserNS.NSMode != specgen.KeepID) || user == "root" || user == "0" {
		configSpec.Process.Capabilities.Effective = caplist
		configSpec.Process.Capabilities.Permitted = caplist
		configSpec.Process.Capabilities.Inheritable = caplist
	} else {
		mergedCaps, err := capabilities.MergeCapabilities(nil, s.CapAdd, nil)
		if err != nil {
			return errors.Wrapf(err, "capabilities requested by user are not valid: %q", strings.Join(s.CapAdd, ","))
		}
		boundingSet, err := capabilities.BoundingSet()
		if err != nil {
			return err
		}
		boundingCaps := make(map[string]interface{})
		for _, b := range boundingSet {
			boundingCaps[b] = b
		}
		var userCaps []string
		for _, c := range mergedCaps {
			if _, ok := boundingCaps[c]; ok {
				userCaps = append(userCaps, c)
			}
		}
		configSpec.Process.Capabilities.Effective = userCaps
		configSpec.Process.Capabilities.Permitted = userCaps
		configSpec.Process.Capabilities.Inheritable = userCaps

		// Ambient capabilities were added to Linux 4.3.  Set ambient
		// capabilities only when the kernel supports them.
		if supportAmbientCapabilities() {
			configSpec.Process.Capabilities.Ambient = userCaps
		}
	}

	g.SetProcessNoNewPrivileges(s.NoNewPrivileges)

	if err := setupApparmor(s, rtc, g); err != nil {
		return err
	}

	// HANDLE SECCOMP
	if s.SeccompProfilePath != "unconfined" {
		seccompConfig, err := getSeccompConfig(s, configSpec, newImage)
		if err != nil {
			return err
		}
		configSpec.Linux.Seccomp = seccompConfig
	}

	// Clear default Seccomp profile from Generator for unconfined containers
	// and privileged containers which do not specify a seccomp profile.
	if s.SeccompProfilePath == "unconfined" || (s.Privileged && (s.SeccompProfilePath == "" || s.SeccompProfilePath == config.SeccompOverridePath || s.SeccompProfilePath == config.SeccompDefaultPath)) {
		configSpec.Linux.Seccomp = nil
	}

	g.SetRootReadonly(s.ReadOnlyFilesystem)

	noUseIPC := s.IpcNS.NSMode == specgen.FromContainer || s.IpcNS.NSMode == specgen.FromPod || s.IpcNS.NSMode == specgen.Host
	noUseNet := s.NetNS.NSMode == specgen.FromContainer || s.NetNS.NSMode == specgen.FromPod || s.NetNS.NSMode == specgen.Host
	noUseUTS := s.UtsNS.NSMode == specgen.FromContainer || s.UtsNS.NSMode == specgen.FromPod || s.UtsNS.NSMode == specgen.Host

	// Add default sysctls
	defaultSysctls, err := util.ValidateSysctls(rtc.Sysctls())
	if err != nil {
		return err
	}
	for sysctlKey, sysctlVal := range defaultSysctls {
		// Ignore mqueue sysctls if --ipc=host
		if noUseIPC && strings.HasPrefix(sysctlKey, "fs.mqueue.") {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since IPC Namespace set to host", sysctlKey, sysctlVal)

			continue
		}

		// Ignore net sysctls if --net=host
		if noUseNet && strings.HasPrefix(sysctlKey, "net.") {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since Network Namespace set to host", sysctlKey, sysctlVal)
			continue
		}

		// Ignore uts sysctls if --uts=host
		if noUseUTS && (strings.HasPrefix(sysctlKey, "kernel.domainname") || strings.HasPrefix(sysctlKey, "kernel.hostname")) {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since UTS Namespace set to host", sysctlKey, sysctlVal)
			continue
		}

		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	for sysctlKey, sysctlVal := range s.Sysctl {
		if s.IpcNS.IsHost() && strings.HasPrefix(sysctlKey, "fs.mqueue.") {
			return errors.Wrapf(define.ErrInvalidArg, "sysctl %s=%s can't be set since IPC Namespace set to host", sysctlKey, sysctlVal)
		}

		// Ignore net sysctls if --net=host
		if s.NetNS.IsHost() && strings.HasPrefix(sysctlKey, "net.") {
			return errors.Wrapf(define.ErrInvalidArg, "sysctl %s=%s can't be set since Network Namespace set to host", sysctlKey, sysctlVal)
		}

		// Ignore uts sysctls if --uts=host
		if s.UtsNS.IsHost() && (strings.HasPrefix(sysctlKey, "kernel.domainname") || strings.HasPrefix(sysctlKey, "kernel.hostname")) {
			return errors.Wrapf(define.ErrInvalidArg, "sysctl %s=%s can't be set since UTS Namespace set to host", sysctlKey, sysctlVal)
		}

		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	return nil
}
