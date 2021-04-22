// +build linux

package libpod

import (
	"context"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// IDTruncLength is the length of the pod's id that will be used to make the
	// infra container name
	IDTruncLength = 12
)

func (r *Runtime) makeInfraContainer(ctx context.Context, p *Pod, imgName, rawImageName, imgID string, config *v1.ImageConfig) (*Container, error) {
	// Set up generator for infra container defaults
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}

	// Set Pod hostname
	g.Config.Hostname = p.config.Hostname

	var options []CtrCreateOption

	// Command: If user-specified, use that preferentially.
	// If not set and the config file is set, fall back to that.
	var infraCtrCommand []string
	if p.config.InfraContainer.InfraCommand != nil {
		logrus.Debugf("User-specified infra container entrypoint %v", p.config.InfraContainer.InfraCommand)
		infraCtrCommand = p.config.InfraContainer.InfraCommand
	} else if r.config.Engine.InfraCommand != "" {
		logrus.Debugf("Config-specified infra container entrypoint %s", r.config.Engine.InfraCommand)
		infraCtrCommand = []string{r.config.Engine.InfraCommand}
	}
	// Only if set by the user or containers.conf, we set entrypoint for the
	// infra container.
	// This is only used by commit, so it shouldn't matter... But someone
	// may eventually want to commit an infra container?
	// TODO: Should we actually do this if set by containers.conf?
	if infraCtrCommand != nil {
		// Need to duplicate the array - we are going to add Cmd later
		// so the current array will be changed.
		newArr := make([]string, 0, len(infraCtrCommand))
		newArr = append(newArr, infraCtrCommand...)
		options = append(options, WithEntrypoint(newArr))
	}

	isRootless := rootless.IsRootless()

	// I've seen circumstances where config is being passed as nil.
	// Let's err on the side of safety and make sure it's safe to use.
	if config != nil {
		if infraCtrCommand == nil {
			// If we have no entrypoint and command from the image,
			// we can't go on - the infra container has no command.
			if len(config.Entrypoint) == 0 && len(config.Cmd) == 0 {
				return nil, errors.Errorf("infra container has no command")
			}
			if len(config.Entrypoint) > 0 {
				infraCtrCommand = config.Entrypoint
			} else {
				// Use the Docker default "/bin/sh -c"
				// entrypoint, as we're overriding command.
				// If an image doesn't want this, it can
				// override entrypoint too.
				infraCtrCommand = []string{"/bin/sh", "-c"}
			}
		}
		if len(config.Cmd) > 0 {
			infraCtrCommand = append(infraCtrCommand, config.Cmd...)
		}

		if len(config.Env) > 0 {
			for _, nameValPair := range config.Env {
				nameValSlice := strings.Split(nameValPair, "=")
				if len(nameValSlice) < 2 {
					return nil, errors.Errorf("Invalid environment variable structure in pause image")
				}
				g.AddProcessEnv(nameValSlice[0], nameValSlice[1])
			}
		}

		switch {
		case p.config.InfraContainer.HostNetwork:
			if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
				return nil, errors.Wrapf(err, "error removing network namespace from pod %s infra container", p.ID())
			}
		case p.config.InfraContainer.NoNetwork:
			// Do nothing - we have a network namespace by default,
			// but should not configure slirp.
		default:
			// Since user namespace sharing is not implemented, we only need to check if it's rootless
			netmode := "bridge"
			if p.config.InfraContainer.Slirp4netns {
				netmode = "slirp4netns"
				if len(p.config.InfraContainer.NetworkOptions) != 0 {
					options = append(options, WithNetworkOptions(p.config.InfraContainer.NetworkOptions))
				}
			}
			// PostConfigureNetNS should not be set since user namespace sharing is not implemented
			// and rootless networking no longer supports post configuration setup
			options = append(options, WithNetNS(p.config.InfraContainer.PortBindings, false, netmode, p.config.InfraContainer.Networks))
		}

		// For each option in InfraContainerConfig - if set, pass into
		// the infra container we're creating with the appropriate
		// With... option.
		if p.config.InfraContainer.StaticIP != nil {
			options = append(options, WithStaticIP(p.config.InfraContainer.StaticIP))
		}
		if p.config.InfraContainer.StaticMAC != nil {
			options = append(options, WithStaticMAC(p.config.InfraContainer.StaticMAC))
		}
		if p.config.InfraContainer.UseImageResolvConf {
			options = append(options, WithUseImageResolvConf())
		}
		if len(p.config.InfraContainer.DNSServer) > 0 {
			options = append(options, WithDNS(p.config.InfraContainer.DNSServer))
		}
		if len(p.config.InfraContainer.DNSSearch) > 0 {
			options = append(options, WithDNSSearch(p.config.InfraContainer.DNSSearch))
		}
		if len(p.config.InfraContainer.DNSOption) > 0 {
			options = append(options, WithDNSOption(p.config.InfraContainer.DNSOption))
		}
		if p.config.InfraContainer.UseImageHosts {
			options = append(options, WithUseImageHosts())
		}
		if len(p.config.InfraContainer.HostAdd) > 0 {
			options = append(options, WithHosts(p.config.InfraContainer.HostAdd))
		}
		if len(p.config.InfraContainer.ExitCommand) > 0 {
			options = append(options, WithExitCommand(p.config.InfraContainer.ExitCommand))
		}
	}

	g.SetRootReadonly(true)
	g.SetProcessArgs(infraCtrCommand)

	logrus.Debugf("Using %q as infra container command", infraCtrCommand)

	g.RemoveMount("/dev/shm")
	if isRootless {
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"private", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	// Add default sysctls from containers.conf
	defaultSysctls, err := util.ValidateSysctls(r.config.Sysctls())
	if err != nil {
		return nil, err
	}
	for sysctlKey, sysctlVal := range defaultSysctls {
		// Ignore mqueue sysctls if not sharing IPC
		if !p.config.UsePodIPC && strings.HasPrefix(sysctlKey, "fs.mqueue.") {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since IPC Namespace for pod is unused", sysctlKey, sysctlVal)

			continue
		}

		// Ignore net sysctls if host network or not sharing network
		if (p.config.InfraContainer.HostNetwork || !p.config.UsePodNet) && strings.HasPrefix(sysctlKey, "net.") {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since Network Namespace for pod is unused", sysctlKey, sysctlVal)
			continue
		}

		// Ignore uts sysctls if not sharing UTS
		if !p.config.UsePodUTS && (strings.HasPrefix(sysctlKey, "kernel.domainname") || strings.HasPrefix(sysctlKey, "kernel.hostname")) {
			logrus.Infof("Sysctl %s=%s ignored in containers.conf, since UTS Namespace for pod is unused", sysctlKey, sysctlVal)
			continue
		}

		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	containerName := p.ID()[:IDTruncLength] + "-infra"
	options = append(options, r.WithPod(p))
	options = append(options, WithRootFSFromImage(imgID, imgName, rawImageName))
	options = append(options, WithName(containerName))
	options = append(options, withIsInfra())
	if len(p.config.InfraContainer.ConmonPidFile) > 0 {
		options = append(options, WithConmonPidFile(p.config.InfraContainer.ConmonPidFile))
	}

	return r.newContainer(ctx, g.Config, options...)
}

// createInfraContainer wrap creates an infra container for a pod.
// An infra container becomes the basis for kernel namespace sharing between
// containers in the pod.
func (r *Runtime) createInfraContainer(ctx context.Context, p *Pod) (*Container, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	imageName := p.config.InfraContainer.InfraImage
	if imageName == "" {
		imageName = r.config.Engine.InfraImage
	}

	pulledImages, err := r.LibimageRuntime().Pull(ctx, imageName, config.PullPolicyMissing, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error pulling infra-container image")
	}

	newImage := pulledImages[0]
	data, err := newImage.Inspect(ctx, false)
	if err != nil {
		return nil, err
	}

	imageName = "none"
	if len(newImage.Names()) > 0 {
		imageName = newImage.Names()[0]
	}
	imageID := data.ID

	return r.makeInfraContainer(ctx, p, imageName, r.config.Engine.InfraImage, imageID, data.Config)
}
