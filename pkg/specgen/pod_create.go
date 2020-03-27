package specgen

import (
	"context"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/sirupsen/logrus"
)

func (p *PodSpecGenerator) MakePod(rt *libpod.Runtime) (*libpod.Pod, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	options, err := p.createPodOptions()
	if err != nil {
		return nil, err
	}
	return rt.NewPod(context.Background(), options...)
}

func (p *PodSpecGenerator) createPodOptions() ([]libpod.PodCreateOption, error) {
	var (
		options []libpod.PodCreateOption
	)
	if !p.NoInfra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := shared.GetNamespaceOptions(p.SharedNamespaces)
		if err != nil {
			return nil, err
		}
		options = append(options, nsOptions...)
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
	switch p.NetNS.NSMode {
	case Bridge:
		logrus.Debugf("Pod using default network mode")
	case Host:
		logrus.Debugf("Pod will use host networking")
		options = append(options, libpod.WithPodHostNetwork())
	default:
		logrus.Debugf("Pod joining CNI networks: %v", p.CNINetworks)
		options = append(options, libpod.WithPodNetworks(p.CNINetworks))
	}

	if p.NoManageHosts {
		options = append(options, libpod.WithPodUseImageHosts())
	}
	if len(p.PortMappings) > 0 {
		options = append(options, libpod.WithInfraContainerPorts(p.PortMappings))
	}
	options = append(options, libpod.WithPodCgroups())
	return options, nil
}
