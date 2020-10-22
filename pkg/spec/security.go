package createconfig

import (
	"fmt"
	"strings"

	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ToCreateOptions convert the SecurityConfig to a slice of container create
// options.
func (c *SecurityConfig) ToCreateOptions() ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	options = append(options, libpod.WithSecLabels(c.LabelOpts))
	options = append(options, libpod.WithPrivileged(c.Privileged))
	return options, nil
}

// SetLabelOpts sets the label options of the SecurityConfig according to the
// input.
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

// SetSecurityOpts the the security options (labels, apparmor, seccomp, etc.).
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
			case "proc-opts":
				c.ProcOpts = strings.Split(con[1], ",")
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

// ConfigureGenerator configures the generator according to the input.
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
	var defaultCaplist []string
	bounding := configSpec.Process.Capabilities.Bounding
	if useNotRoot(user.User) {
		configSpec.Process.Capabilities.Bounding = defaultCaplist
	}
	defaultCaplist, err = capabilities.MergeCapabilities(configSpec.Process.Capabilities.Bounding, c.CapAdd, c.CapDrop)
	if err != nil {
		return err
	}

	privCapRequired := []string{}

	if !c.Privileged && len(c.CapRequired) > 0 {
		// Pass CapRequired in CapAdd field to normalize capabilities names
		capRequired, err := capabilities.MergeCapabilities(nil, c.CapRequired, nil)
		if err != nil {
			logrus.Errorf("capabilities requested by user or image are not valid: %q", strings.Join(c.CapRequired, ","))
		} else {
			// Verify all capRequiered are in the defaultCapList
			for _, cap := range capRequired {
				if !util.StringInSlice(cap, defaultCaplist) {
					privCapRequired = append(privCapRequired, cap)
				}
			}
		}
		if len(privCapRequired) == 0 {
			defaultCaplist = capRequired
		} else {
			logrus.Errorf("capabilities requested by user or image are not allowed by default: %q", strings.Join(privCapRequired, ","))
		}
	}
	configSpec.Process.Capabilities.Bounding = defaultCaplist
	configSpec.Process.Capabilities.Permitted = defaultCaplist
	configSpec.Process.Capabilities.Inheritable = defaultCaplist
	configSpec.Process.Capabilities.Effective = defaultCaplist
	configSpec.Process.Capabilities.Ambient = defaultCaplist
	if useNotRoot(user.User) {
		defaultCaplist, err = capabilities.MergeCapabilities(bounding, c.CapAdd, c.CapDrop)
		if err != nil {
			return err
		}
	}
	configSpec.Process.Capabilities.Bounding = defaultCaplist

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
		splitOpt := strings.SplitN(opt, "=", 2)
		if len(splitOpt) == 1 {
			splitOpt = strings.Split(opt, ":")
		}
		if len(splitOpt) < 2 {
			continue
		}
		switch splitOpt[0] {
		case "label":
			configSpec.Annotations[define.InspectAnnotationLabel] = splitOpt[1]
		case "seccomp":
			configSpec.Annotations[define.InspectAnnotationSeccomp] = splitOpt[1]
		case "apparmor":
			configSpec.Annotations[define.InspectAnnotationApparmor] = splitOpt[1]
		}
	}

	g.SetRootReadonly(c.ReadOnlyRootfs)
	for sysctlKey, sysctlVal := range c.Sysctl {
		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	return nil
}
