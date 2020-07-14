package specgen

import (
	"strings"

	"github.com/containers/libpod/v2/pkg/rootless"
	"github.com/containers/libpod/v2/pkg/util"
	"github.com/pkg/errors"
)

var (
	// ErrInvalidSpecConfig describes an error that the given SpecGenerator is invalid
	ErrInvalidSpecConfig = errors.New("invalid configuration")
	// SystemDValues describes the only values that SystemD can be
	SystemDValues = []string{"true", "false", "always"}
	// ImageVolumeModeValues describes the only values that ImageVolumeMode can be
	ImageVolumeModeValues = []string{"ignore", "tmpfs", "anonymous"}
)

func exclusiveOptions(opt1, opt2 string) error {
	return errors.Errorf("%s and %s are mutually exclusive options", opt1, opt2)
}

// Validate verifies that the given SpecGenerator is valid and satisfies required
// input for creating a container.
func (s *SpecGenerator) Validate() error {

	if rootless.IsRootless() {
		if s.StaticIP != nil || s.StaticIPv6 != nil {
			return ErrNoStaticIPRootless
		}
		if s.StaticMAC != nil {
			return ErrNoStaticMACRootless
		}
	}

	//
	// ContainerBasicConfig
	//
	// Rootfs and Image cannot both populated
	if len(s.ContainerStorageConfig.Image) > 0 && len(s.ContainerStorageConfig.Rootfs) > 0 {
		return errors.Wrap(ErrInvalidSpecConfig, "both image and rootfs cannot be simultaneously")
	}
	// Cannot set hostname and utsns
	if len(s.ContainerBasicConfig.Hostname) > 0 && !s.ContainerBasicConfig.UtsNS.IsPrivate() {
		return errors.Wrap(ErrInvalidSpecConfig, "cannot set hostname when running in the host UTS namespace")
	}
	// systemd values must be true, false, or always
	if len(s.ContainerBasicConfig.Systemd) > 0 && !util.StringInSlice(strings.ToLower(s.ContainerBasicConfig.Systemd), SystemDValues) {
		return errors.Wrapf(ErrInvalidSpecConfig, "--systemd values must be one of %q", strings.Join(SystemDValues, ", "))
	}

	//
	// ContainerStorageConfig
	//
	// rootfs and image cannot both be set
	if len(s.ContainerStorageConfig.Image) > 0 && len(s.ContainerStorageConfig.Rootfs) > 0 {
		return exclusiveOptions("rootfs", "image")
	}
	// imagevolumemode must be one of ignore, tmpfs, or anonymous if given
	if len(s.ContainerStorageConfig.ImageVolumeMode) > 0 && !util.StringInSlice(strings.ToLower(s.ContainerStorageConfig.ImageVolumeMode), ImageVolumeModeValues) {
		return errors.Errorf("invalid ImageVolumeMode %q, value must be one of %s",
			s.ContainerStorageConfig.ImageVolumeMode, strings.Join(ImageVolumeModeValues, ","))
	}
	// shmsize conflicts with IPC namespace
	if s.ContainerStorageConfig.ShmSize != nil && !s.ContainerStorageConfig.IpcNS.IsPrivate() {
		return errors.New("cannot set shmsize when running in the host IPC Namespace")
	}

	//
	// ContainerSecurityConfig
	//
	// capadd and privileged are exclusive
	if len(s.CapAdd) > 0 && s.Privileged {
		return exclusiveOptions("CapAdd", "privileged")
	}
	// userns and idmappings conflict
	if s.UserNS.IsPrivate() && s.IDMappings == nil {
		return errors.Wrap(ErrInvalidSpecConfig, "IDMappings are required when not creating a User namespace")
	}

	//
	// ContainerCgroupConfig
	//
	//
	// None for now

	//
	// ContainerNetworkConfig
	//
	// useimageresolveconf conflicts with dnsserver, dnssearch, dnsoption
	if s.UseImageResolvConf {
		if len(s.DNSServers) > 0 {
			return exclusiveOptions("UseImageResolvConf", "DNSServer")
		}
		if len(s.DNSSearch) > 0 {
			return exclusiveOptions("UseImageResolvConf", "DNSSearch")
		}
		if len(s.DNSOptions) > 0 {
			return exclusiveOptions("UseImageResolvConf", "DNSOption")
		}
	}
	// UseImageHosts and HostAdd are exclusive
	if s.UseImageHosts && len(s.HostAdd) > 0 {
		return exclusiveOptions("UseImageHosts", "HostAdd")
	}

	// TODO the specgen does not appear to handle this?  Should it
	//switch config.Cgroup.Cgroups {
	//case "disabled":
	//	if addedResources {
	//		return errors.New("cannot specify resource limits when cgroups are disabled is specified")
	//	}
	//	configSpec.Linux.Resources = &spec.LinuxResources{}
	//case "enabled", "no-conmon", "":
	//	// Do nothing
	//default:
	//	return errors.New("unrecognized option for cgroups; supported are 'default', 'disabled', 'no-conmon'")
	//}

	// Namespaces
	if err := s.UtsNS.validate(); err != nil {
		return err
	}
	if err := s.IpcNS.validate(); err != nil {
		return err
	}
	if err := s.PidNS.validate(); err != nil {
		return err
	}
	if err := s.CgroupNS.validate(); err != nil {
		return err
	}
	if err := validateUserNS(&s.UserNS); err != nil {
		return err
	}

	// The following are defaults as needed by container creation
	if len(s.WorkDir) < 1 {
		s.WorkDir = "/"
	}

	// Set defaults if network info is not provided
	if s.NetNS.NSMode == "" {
		s.NetNS.NSMode = Bridge
		if rootless.IsRootless() {
			s.NetNS.NSMode = Slirp
		}
	}
	if err := validateNetNS(&s.NetNS); err != nil {
		return err
	}
	return nil
}
