package generate

import (
	"context"
	"net"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func MakePod(p *entities.PodSpec, rt *libpod.Runtime) (*libpod.Pod, error) {
	if err := p.PodSpecGen.Validate(); err != nil {
		return nil, err
	}
	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		var err error
		p.PodSpecGen.InfraContainerSpec, err = MapSpec(&p.PodSpecGen)
		if err != nil {
			return nil, err
		}
	}

	options, err := createPodOptions(&p.PodSpecGen, rt, p.PodSpecGen.InfraContainerSpec)
	if err != nil {
		return nil, err
	}
	pod, err := rt.NewPod(context.Background(), p.PodSpecGen, options...)
	if err != nil {
		return nil, err
	}
	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		p.PodSpecGen.InfraContainerSpec.ContainerCreateCommand = []string{} // we do NOT want os.Args as the command, will display the pod create cmd
		if p.PodSpecGen.InfraContainerSpec.Name == "" {
			p.PodSpecGen.InfraContainerSpec.Name = pod.ID()[:12] + "-infra"
		}
		_, err = CompleteSpec(context.Background(), rt, p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.InfraContainerSpec.User = "" // infraSpec user will get incorrectly assigned via the container creation process, overwrite here
		rtSpec, spec, opts, err := MakeContainer(context.Background(), rt, p.PodSpecGen.InfraContainerSpec)
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
	}
	return pod, nil
}

func createPodOptions(p *specgen.PodSpecGenerator, rt *libpod.Runtime, infraSpec *specgen.SpecGenerator) ([]libpod.PodCreateOption, error) {
	var (
		options []libpod.PodCreateOption
	)
	if !p.NoInfra { //&& infraSpec != nil {
		options = append(options, libpod.WithInfraContainer())
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
		ports, _, _, err := ParsePortMapping(p.PortMappings)
		if err != nil {
			return nil, err
		}
		p.InfraContainerSpec.PortMappings = libpod.WithInfraContainerPorts(ports, p.InfraContainerSpec)
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
			p.InfraContainerSpec.StaticIP != nil ||
			p.InfraContainerSpec.StaticMAC != nil ||
			len(p.InfraContainerSpec.CNINetworks) > 0 ||
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
			p.InfraContainerSpec.StaticIP != nil ||
			p.InfraContainerSpec.StaticMAC != nil ||
			len(p.InfraContainerSpec.CNINetworks) > 0 ||
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
	if p.StaticIP != nil {
		p.InfraContainerSpec.StaticIP = p.StaticIP
	}
	if p.StaticMAC != nil {
		p.InfraContainerSpec.StaticMAC = p.StaticMAC
	}
	if p.NoManageResolvConf {
		p.InfraContainerSpec.UseImageResolvConf = true
	}
	if len(p.CNINetworks) > 0 {
		p.InfraContainerSpec.CNINetworks = p.CNINetworks
	}
	if p.NoManageHosts {
		p.InfraContainerSpec.UseImageHosts = p.NoManageHosts
	}

	if len(p.InfraConmonPidFile) > 0 {
		p.InfraContainerSpec.ConmonPidFile = p.InfraConmonPidFile
	}

	if p.InfraImage != config.DefaultInfraImage {
		p.InfraContainerSpec.Image = p.InfraImage
	}
	return p.InfraContainerSpec, nil
}
