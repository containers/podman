// +build linux

package libpod

import (
	"context"
	"strings"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
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

	isRootless := rootless.IsRootless()

	entrypointSet := len(p.config.InfraContainer.InfraCommand) > 0
	entryPoint := p.config.InfraContainer.InfraCommand
	entryCmd := []string{}
	var options []CtrCreateOption
	// I've seen circumstances where config is being passed as nil.
	// Let's err on the side of safety and make sure it's safe to use.
	if config != nil {
		// default to entrypoint in image if there is one
		if !entrypointSet {
			if len(config.Entrypoint) > 0 {
				entrypointSet = true
				entryPoint = config.Entrypoint
				entryCmd = config.Entrypoint
			}
		} else { // so use the InfraCommand
			entrypointSet = true
			entryCmd = entryPoint
		}

		if len(config.Cmd) > 0 {
			// We can't use the default pause command, since we're
			// sourcing from the image. If we didn't already set an
			// entrypoint, set one now.
			if !entrypointSet {
				// Use the Docker default "/bin/sh -c"
				// entrypoint, as we're overriding command.
				// If an image doesn't want this, it can
				// override entrypoint too.
				entryCmd = []string{"/bin/sh", "-c"}
			}
			entryCmd = append(entryCmd, config.Cmd...)
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

		// Since user namespace sharing is not implemented, we only need to check if it's rootless
		if !p.config.InfraContainer.HostNetwork {
			netmode := "bridge"
			if isRootless || p.config.InfraContainer.Slirp4netns {
				netmode = "slirp4netns"
				if len(p.config.InfraContainer.NetworkOptions) != 0 {
					options = append(options, WithNetworkOptions(p.config.InfraContainer.NetworkOptions))
				}
			}
			// PostConfigureNetNS should not be set since user namespace sharing is not implemented
			// and rootless networking no longer supports post configuration setup
			options = append(options, WithNetNS(p.config.InfraContainer.PortBindings, false, netmode, p.config.InfraContainer.Networks))
		} else if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
			return nil, errors.Wrapf(err, "error removing network namespace from pod %s infra container", p.ID())
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
	g.SetProcessArgs(entryCmd)

	logrus.Debugf("Using %q as infra container entrypoint", entryCmd)

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

	containerName := p.ID()[:IDTruncLength] + "-infra"
	options = append(options, r.WithPod(p))
	options = append(options, WithRootFSFromImage(imgID, imgName, rawImageName))
	options = append(options, WithName(containerName))
	options = append(options, withIsInfra())
	if entrypointSet {
		options = append(options, WithEntrypoint(entryPoint))
	}
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

	img := p.config.InfraContainer.InfraImage
	if img == "" {
		img = r.config.Engine.InfraImage
	}
	newImage, err := r.ImageRuntime().New(ctx, img, "", "", nil, nil, image.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		return nil, err
	}

	data, err := newImage.InspectNoSize(ctx)
	if err != nil {
		return nil, err
	}
	imageName := newImage.Names()[0]
	imageID := data.ID

	return r.makeInfraContainer(ctx, p, imageName, r.config.Engine.InfraImage, imageID, data.Config)
}
