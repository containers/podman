package generate

import (
	"context"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func MakePod(p *specgen.PodSpecGenerator, rt *libpod.Runtime) (*libpod.Pod, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	options, err := createPodOptions(p, rt)
	if err != nil {
		return nil, err
	}
	return rt.NewPod(context.Background(), options...)
}

func createPodOptions(p *specgen.PodSpecGenerator, rt *libpod.Runtime) ([]libpod.PodCreateOption, error) {
	var (
		options []libpod.PodCreateOption
	)
	if !p.NoInfra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := GetNamespaceOptions(p.SharedNamespaces)
		if err != nil {
			return nil, err
		}
		options = append(options, nsOptions...)

		// Make our exit command
		storageConfig := rt.StorageConfig()
		runtimeConfig, err := rt.GetConfig()
		if err != nil {
			return nil, err
		}
		exitCommand, err := CreateExitCommandArgs(storageConfig, runtimeConfig, logrus.IsLevelEnabled(logrus.DebugLevel), false, false)
		if err != nil {
			return nil, errors.Wrapf(err, "error creating infra container exit command")
		}
		options = append(options, libpod.WithPodInfraExitCommand(exitCommand))
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
	if len(p.Hostname) > 0 {
		options = append(options, libpod.WithPodHostname(p.Hostname))
	}
	if len(p.HostAdd) > 0 {
		options = append(options, libpod.WithPodHosts(p.HostAdd))
	}
	if len(p.DNSServer) > 0 {
		var dnsServers []string
		for _, d := range p.DNSServer {
			dnsServers = append(dnsServers, d.String())
		}
		options = append(options, libpod.WithPodDNS(dnsServers))
	}
	if len(p.DNSOption) > 0 {
		options = append(options, libpod.WithPodDNSOption(p.DNSOption))
	}
	if len(p.DNSSearch) > 0 {
		options = append(options, libpod.WithPodDNSSearch(p.DNSSearch))
	}
	if p.StaticIP != nil {
		options = append(options, libpod.WithPodStaticIP(*p.StaticIP))
	}
	if p.StaticMAC != nil {
		options = append(options, libpod.WithPodStaticMAC(*p.StaticMAC))
	}
	if p.NoManageResolvConf {
		options = append(options, libpod.WithPodUseImageResolvConf())
	}
	if len(p.CNINetworks) > 0 {
		options = append(options, libpod.WithPodNetworks(p.CNINetworks))
	}

	if len(p.InfraImage) > 0 {
		options = append(options, libpod.WithInfraImage(p.InfraImage))
	}

	if len(p.InfraCommand) > 0 {
		options = append(options, libpod.WithInfraCommand(p.InfraCommand))
	}

	switch p.NetNS.NSMode {
	case specgen.Bridge, specgen.Default, "":
		logrus.Debugf("Pod using default network mode")
	case specgen.Host:
		logrus.Debugf("Pod will use host networking")
		options = append(options, libpod.WithPodHostNetwork())
	case specgen.Slirp:
		logrus.Debugf("Pod will use slirp4netns")
		options = append(options, libpod.WithPodSlirp4netns(p.NetworkOptions))
	default:
		return nil, errors.Errorf("pods presently do not support network mode %s", p.NetNS.NSMode)
	}

	if p.NoManageHosts {
		options = append(options, libpod.WithPodUseImageHosts())
	}
	if len(p.PortMappings) > 0 {
		ports, _, _, err := parsePortMapping(p.PortMappings)
		if err != nil {
			return nil, err
		}
		options = append(options, libpod.WithInfraContainerPorts(ports))
	}
	options = append(options, libpod.WithPodCgroups())
	if p.PodCreateCommand != nil {
		options = append(options, libpod.WithPodCreateCommand(p.PodCreateCommand))
	}
	if len(p.InfraConmonPidFile) > 0 {
		options = append(options, libpod.WithInfraConmonPidFile(p.InfraConmonPidFile))
	}
	return options, nil
}
