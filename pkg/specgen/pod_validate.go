package specgen

import (
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
)

var (
	// ErrInvalidPodSpecConfig describes an error given when the podspecgenerator is invalid
	ErrInvalidPodSpecConfig = errors.New("invalid pod spec")
	// containerConfig has the default configurations defined in containers.conf
	containerConfig = util.DefaultContainerConfig()
)

func exclusivePodOptions(opt1, opt2 string) error {
	return errors.Wrapf(ErrInvalidPodSpecConfig, "%s and %s are mutually exclusive pod options", opt1, opt2)
}

// Validate verifies the input is valid
func (p *PodSpecGenerator) Validate() error {
	if rootless.IsRootless() && len(p.CNINetworks) == 0 {
		if p.StaticIP != nil {
			return ErrNoStaticIPRootless
		}
		if p.StaticMAC != nil {
			return ErrNoStaticMACRootless
		}
	}

	// PodBasicConfig
	if p.NoInfra {
		if len(p.InfraCommand) > 0 {
			return exclusivePodOptions("NoInfra", "InfraCommand")
		}
		if len(p.InfraImage) > 0 {
			return exclusivePodOptions("NoInfra", "InfraImage")
		}
		if len(p.SharedNamespaces) > 0 {
			return exclusivePodOptions("NoInfra", "SharedNamespaces")
		}
	}

	// PodNetworkConfig
	if err := validateNetNS(&p.NetNS); err != nil {
		return err
	}
	if p.NoInfra {
		if p.NetNS.NSMode != Default && p.NetNS.NSMode != "" {
			return errors.New("NoInfra and network modes cannot be used together")
		}
		if p.StaticIP != nil {
			return exclusivePodOptions("NoInfra", "StaticIP")
		}
		if p.StaticMAC != nil {
			return exclusivePodOptions("NoInfra", "StaticMAC")
		}
		if len(p.DNSOption) > 0 {
			return exclusivePodOptions("NoInfra", "DNSOption")
		}
		if len(p.DNSSearch) > 0 {
			return exclusivePodOptions("NoInfo", "DNSSearch")
		}
		if len(p.DNSServer) > 0 {
			return exclusivePodOptions("NoInfra", "DNSServer")
		}
		if len(p.HostAdd) > 0 {
			return exclusivePodOptions("NoInfra", "HostAdd")
		}
		if p.NoManageResolvConf {
			return exclusivePodOptions("NoInfra", "NoManageResolvConf")
		}
	}
	if p.NetNS.NSMode != "" && p.NetNS.NSMode != Bridge && p.NetNS.NSMode != Slirp && p.NetNS.NSMode != Default {
		if len(p.PortMappings) > 0 {
			return errors.New("PortMappings can only be used with Bridge or slirp4netns networking")
		}
		if len(p.CNINetworks) > 0 {
			return errors.New("CNINetworks can only be used with Bridge mode networking")
		}
	}
	if p.NoManageResolvConf {
		if len(p.DNSServer) > 0 {
			return exclusivePodOptions("NoManageResolvConf", "DNSServer")
		}
		if len(p.DNSSearch) > 0 {
			return exclusivePodOptions("NoManageResolvConf", "DNSSearch")
		}
		if len(p.DNSOption) > 0 {
			return exclusivePodOptions("NoManageResolvConf", "DNSOption")
		}
	}
	if p.NoManageHosts && len(p.HostAdd) > 0 {
		return exclusivePodOptions("NoManageHosts", "HostAdd")
	}

	return nil
}
