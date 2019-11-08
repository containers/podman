package createconfig

import (
	"fmt"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/docker/docker/oci/caps"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
)

func (c *SecurityConfig) ToCreateOptions() ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	options = append(options, libpod.WithSecLabels(c.LabelOpts))
	options = append(options, libpod.WithPrivileged(c.Privileged))
	return options, nil
}

func (c *SecurityConfig) SetLabelOpts(runtime *libpod.Runtime, pidConfig *PidConfig, ipcConfig *IpcConfig) error {
	if c.Privileged {
		c.LabelOpts = label.DisableSecOpt()
		return nil
	}

	var labelOpts []string
	if pidConfig.PidMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if pidConfig.PidMode.IsContainer() {
		ctr, err := runtime.LookupContainer(pidConfig.PidMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", pidConfig.PidMode.Container())
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
	}

	if ipcConfig.IpcMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if ipcConfig.IpcMode.IsContainer() {
		ctr, err := runtime.LookupContainer(ipcConfig.IpcMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", ipcConfig.IpcMode.Container())
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
	}

	c.LabelOpts = append(c.LabelOpts, labelOpts...)
	return nil
}

func (c *SecurityConfig) SetSecurityOpts(runtime *libpod.Runtime, securityOpts []string) error {
	for _, opt := range securityOpts {
		if opt == "no-new-privileges" {
			c.NoNewPrivs = true
		} else {
			con := strings.SplitN(opt, "=", 2)
			if len(con) != 2 {
				return fmt.Errorf("invalid --security-opt 1: %q", opt)
			}

			switch con[0] {
			case "label":
				c.LabelOpts = append(c.LabelOpts, con[1])
			case "apparmor":
				c.ApparmorProfile = con[1]
			case "seccomp":
				c.SeccompProfilePath = con[1]
			default:
				return fmt.Errorf("invalid --security-opt 2: %q", opt)
			}
		}
	}

	if c.SeccompProfilePath == "" {
		var err error
		c.SeccompProfilePath, err = libpod.DefaultSeccompPath()
		if err != nil {
			return err
		}
	}
	c.SecurityOpts = securityOpts
	return nil
}

func (c *SecurityConfig) ConfigureGenerator(g *generate.Generator, user *UserConfig) error {
	// HANDLE CAPABILITIES
	// NOTE: Must happen before SECCOMP
	if c.Privileged {
		g.SetupPrivileged(true)
	}

	useNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	configSpec := g.Config
	var err error
	var caplist []string
	bounding := configSpec.Process.Capabilities.Bounding
	if useNotRoot(user.User) {
		configSpec.Process.Capabilities.Bounding = caplist
	}
	caplist, err = caps.TweakCapabilities(configSpec.Process.Capabilities.Bounding, c.CapAdd, c.CapDrop, nil, false)
	if err != nil {
		return err
	}

	configSpec.Process.Capabilities.Bounding = caplist
	configSpec.Process.Capabilities.Permitted = caplist
	configSpec.Process.Capabilities.Inheritable = caplist
	configSpec.Process.Capabilities.Effective = caplist
	configSpec.Process.Capabilities.Ambient = caplist
	if useNotRoot(user.User) {
		caplist, err = caps.TweakCapabilities(bounding, c.CapAdd, c.CapDrop, nil, false)
		if err != nil {
			return err
		}
	}
	configSpec.Process.Capabilities.Bounding = caplist

	// HANDLE SECCOMP
	if c.SeccompProfilePath != "unconfined" {
		seccompConfig, err := getSeccompConfig(c, configSpec)
		if err != nil {
			return err
		}
		configSpec.Linux.Seccomp = seccompConfig
	}

	// Clear default Seccomp profile from Generator for privileged containers
	if c.SeccompProfilePath == "unconfined" || c.Privileged {
		configSpec.Linux.Seccomp = nil
	}

	for _, opt := range c.SecurityOpts {
		// Split on both : and =
		splitOpt := strings.Split(opt, "=")
		if len(splitOpt) == 1 {
			splitOpt = strings.Split(opt, ":")
		}
		if len(splitOpt) < 2 {
			continue
		}
		switch splitOpt[0] {
		case "label":
			configSpec.Annotations[libpod.InspectAnnotationLabel] = splitOpt[1]
		case "seccomp":
			configSpec.Annotations[libpod.InspectAnnotationSeccomp] = splitOpt[1]
		case "apparmor":
			configSpec.Annotations[libpod.InspectAnnotationApparmor] = splitOpt[1]
		}
	}

	g.SetRootReadonly(c.ReadOnlyRootfs)
	for sysctlKey, sysctlVal := range c.Sysctl {
		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	return nil
}
