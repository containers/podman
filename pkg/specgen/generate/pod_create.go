package generate

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func buildPauseImage(rt *libpod.Runtime, rtConfig *config.Config) (string, error) {
	version, err := define.GetVersion()
	if err != nil {
		return "", err
	}
	imageName := fmt.Sprintf("localhost/podman-pause:%s-%d", version.Version, version.Built)

	// First check if the image has already been built.
	if _, _, err := rt.LibimageRuntime().LookupImage(imageName, nil); err == nil {
		return imageName, nil
	}

	// Also look into the path as some distributions install catatonit in
	// /usr/bin.
	catatonitPath, err := rtConfig.FindHelperBinary("catatonit", true)
	if err != nil {
		return "", fmt.Errorf("finding pause binary: %w", err)
	}

	buildContent := fmt.Sprintf(`FROM scratch
COPY %s /catatonit
ENTRYPOINT ["/catatonit", "-P"]`, catatonitPath)

	tmpF, err := ioutil.TempFile("", "pause.containerfile")
	if err != nil {
		return "", err
	}
	if _, err := tmpF.WriteString(buildContent); err != nil {
		return "", err
	}
	if err := tmpF.Close(); err != nil {
		return "", err
	}
	defer os.Remove(tmpF.Name())

	buildOptions := buildahDefine.BuildOptions{
		CommonBuildOpts: &buildahDefine.CommonBuildOptions{},
		Output:          imageName,
		Quiet:           true,
		IgnoreFile:      "/dev/null", // makes sure to not read a local .ignorefile (see #13529)
		IIDFile:         "/dev/null", // prevents Buildah from writing the ID on stdout
	}
	if _, _, err := rt.Build(context.Background(), buildOptions, tmpF.Name()); err != nil {
		return "", err
	}

	return imageName, nil
}

func pullOrBuildInfraImage(p *entities.PodSpec, rt *libpod.Runtime) error {
	if p.PodSpecGen.NoInfra {
		return nil
	}

	rtConfig, err := rt.GetConfigNoCopy()
	if err != nil {
		return err
	}

	// NOTE: we need pull down the infra image if it was explicitly set by
	// the user (or containers.conf) to the non-default one.
	imageName := p.PodSpecGen.InfraImage
	if imageName == "" {
		imageName = rtConfig.Engine.InfraImage
	}

	if imageName != "" {
		_, err := rt.LibimageRuntime().Pull(context.Background(), imageName, config.PullPolicyMissing, nil)
		if err != nil {
			return err
		}
	} else {
		name, err := buildPauseImage(rt, rtConfig)
		if err != nil {
			return fmt.Errorf("building local pause image: %w", err)
		}
		imageName = name
	}

	p.PodSpecGen.InfraImage = imageName
	p.PodSpecGen.InfraContainerSpec.RawImageName = imageName

	return nil
}

func MakePod(p *entities.PodSpec, rt *libpod.Runtime) (*libpod.Pod, error) {
	if err := p.PodSpecGen.Validate(); err != nil {
		return nil, err
	}

	if err := pullOrBuildInfraImage(p, rt); err != nil {
		return nil, err
	}

	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		var err error
		p.PodSpecGen.InfraContainerSpec, err = MapSpec(&p.PodSpecGen)
		if err != nil {
			return nil, err
		}
	}

	options, err := createPodOptions(&p.PodSpecGen)
	if err != nil {
		return nil, err
	}
	pod, err := rt.NewPod(context.Background(), p.PodSpecGen, options...)
	if err != nil {
		return nil, err
	}
	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		if p.PodSpecGen.InfraContainerSpec.Name == "" {
			p.PodSpecGen.InfraContainerSpec.Name = pod.ID()[:12] + "-infra"
		}
		_, err = CompleteSpec(context.Background(), rt, p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.InfraContainerSpec.User = "" // infraSpec user will get incorrectly assigned via the container creation process, overwrite here
		rtSpec, spec, opts, err := MakeContainer(context.Background(), rt, p.PodSpecGen.InfraContainerSpec, false, nil)
		if err != nil {
			return nil, err
		}
		spec.Pod = pod.ID()
		opts = append(opts, rt.WithPod(pod))
		spec.CgroupParent = pod.CgroupParent()
		infraCtr, err := ExecuteCreate(context.Background(), rt, rtSpec, spec, true, opts...)
		if err != nil {
			return nil, err
		}
		pod, err = rt.AddInfra(context.Background(), pod, infraCtr)
		if err != nil {
			return nil, err
		}
	} else {
		// SavePod is used to save the pod state and trigger a create event even if infra is not created
		err := rt.SavePod(pod)
		if err != nil {
			return nil, err
		}
	}
	return pod, nil
}

func createPodOptions(p *specgen.PodSpecGenerator) ([]libpod.PodCreateOption, error) {
	var (
		options []libpod.PodCreateOption
	)
	if !p.NoInfra {
		options = append(options, libpod.WithInfraContainer())
		if p.ShareParent == nil || (p.ShareParent != nil && *p.ShareParent) {
			options = append(options, libpod.WithPodParent())
		}
		nsOptions, err := GetNamespaceOptions(p.SharedNamespaces, p.InfraContainerSpec.NetNS.IsHost())
		if err != nil {
			return nil, err
		}
		options = append(options, nsOptions...)
		// Use pod user and infra userns only when --userns is not set to host
		if !p.InfraContainerSpec.UserNS.IsHost() && !p.InfraContainerSpec.UserNS.IsDefault() {
			options = append(options, libpod.WithPodUser())
		}
	}
	if len(p.CgroupParent) > 0 {
		options = append(options, libpod.WithPodCgroupParent(p.CgroupParent))
	}
	if len(p.Labels) > 0 {
		options = append(options, libpod.WithPodLabels(p.Labels))
	}
	if len(p.Name) > 0 {
		options = append(options, libpod.WithPodName(p.Name))
	}
	if p.PodCreateCommand != nil {
		options = append(options, libpod.WithPodCreateCommand(p.PodCreateCommand))
	}

	if len(p.Hostname) > 0 {
		options = append(options, libpod.WithPodHostname(p.Hostname))
	}

	return options, nil
}

// MapSpec modifies the already filled Infra specgenerator,
// replacing necessary values with those specified in pod creation
func MapSpec(p *specgen.PodSpecGenerator) (*specgen.SpecGenerator, error) {
	if len(p.PortMappings) > 0 {
		ports, err := ParsePortMapping(p.PortMappings, nil)
		if err != nil {
			return nil, err
		}
		p.InfraContainerSpec.PortMappings = ports
	}
	switch p.NetNS.NSMode {
	case specgen.Default, "":
		if p.NoInfra {
			logrus.Debugf("No networking because the infra container is missing")
			break
		}
	case specgen.Bridge:
		p.InfraContainerSpec.NetNS.NSMode = specgen.Bridge
		logrus.Debugf("Pod using bridge network mode")
	case specgen.Host:
		logrus.Debugf("Pod will use host networking")
		if len(p.InfraContainerSpec.PortMappings) > 0 ||
			len(p.InfraContainerSpec.Networks) > 0 ||
			p.InfraContainerSpec.NetNS.NSMode == specgen.NoNetwork {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot set host network if network-related configuration is specified")
		}
		p.InfraContainerSpec.NetNS.NSMode = specgen.Host
	case specgen.Slirp:
		logrus.Debugf("Pod will use slirp4netns")
		if p.InfraContainerSpec.NetNS.NSMode != "host" {
			p.InfraContainerSpec.NetworkOptions = p.NetworkOptions
			p.InfraContainerSpec.NetNS.NSMode = specgen.NamespaceMode("slirp4netns")
		}
	case specgen.NoNetwork:
		logrus.Debugf("Pod will not use networking")
		if len(p.InfraContainerSpec.PortMappings) > 0 ||
			len(p.InfraContainerSpec.Networks) > 0 ||
			p.InfraContainerSpec.NetNS.NSMode == "host" {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot disable pod network if network-related configuration is specified")
		}
		p.InfraContainerSpec.NetNS.NSMode = specgen.NoNetwork
	default:
		return nil, errors.Errorf("pods presently do not support network mode %s", p.NetNS.NSMode)
	}

	if len(p.InfraCommand) > 0 {
		p.InfraContainerSpec.Entrypoint = p.InfraCommand
	}

	if len(p.HostAdd) > 0 {
		p.InfraContainerSpec.HostAdd = p.HostAdd
	}
	if len(p.DNSServer) > 0 {
		var dnsServers []net.IP
		dnsServers = append(dnsServers, p.DNSServer...)

		p.InfraContainerSpec.DNSServers = dnsServers
	}
	if len(p.DNSOption) > 0 {
		p.InfraContainerSpec.DNSOptions = p.DNSOption
	}
	if len(p.DNSSearch) > 0 {
		p.InfraContainerSpec.DNSSearch = p.DNSSearch
	}
	if p.NoManageResolvConf {
		p.InfraContainerSpec.UseImageResolvConf = true
	}
	if len(p.Networks) > 0 {
		p.InfraContainerSpec.Networks = p.Networks
	}
	// deprecated cni networks for api users
	if len(p.CNINetworks) > 0 {
		p.InfraContainerSpec.CNINetworks = p.CNINetworks
	}
	if p.NoManageHosts {
		p.InfraContainerSpec.UseImageHosts = p.NoManageHosts
	}

	if len(p.InfraConmonPidFile) > 0 {
		p.InfraContainerSpec.ConmonPidFile = p.InfraConmonPidFile
	}

	p.InfraContainerSpec.Image = p.InfraImage
	return p.InfraContainerSpec, nil
}
