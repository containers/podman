package libpod

import (
	"github.com/containers/podman/v2/libpod/define"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Validate that the configuration of a container is valid.
func (c *Container) validate() error {
	imageIDSet := c.config.RootfsImageID != ""
	imageNameSet := c.config.RootfsImageName != ""
	rootfsSet := c.config.Rootfs != ""

	// If one of RootfsImageIDor RootfsImageName are set, both must be set.
	if (imageIDSet || imageNameSet) && !(imageIDSet && imageNameSet) {
		return errors.Wrapf(define.ErrInvalidArg, "both RootfsImageName and RootfsImageID must be set if either is set")
	}

	// Cannot set RootfsImageID and Rootfs at the same time
	if imageIDSet && rootfsSet {
		return errors.Wrapf(define.ErrInvalidArg, "cannot set both an image ID and rootfs for a container")
	}

	// Must set at least one of RootfsImageID or Rootfs
	if !(imageIDSet || rootfsSet) {
		return errors.Wrapf(define.ErrInvalidArg, "must set root filesystem source to either image or rootfs")
	}

	// Cannot make a network namespace if we are joining another container's
	// network namespace
	if c.config.CreateNetNS && c.config.NetNsCtr != "" {
		return errors.Wrapf(define.ErrInvalidArg, "cannot both create a network namespace and join another container's network namespace")
	}

	if c.config.CgroupsMode == cgroupSplit && c.config.CgroupParent != "" {
		return errors.Wrapf(define.ErrInvalidArg, "cannot specify --cgroup-mode=split with a cgroup-parent")
	}

	// Not creating cgroups has a number of requirements, mostly related to
	// the PID namespace.
	if c.config.NoCgroups || c.config.CgroupsMode == "disabled" {
		if c.config.PIDNsCtr != "" {
			return errors.Wrapf(define.ErrInvalidArg, "cannot join another container's PID namespace if not creating cgroups")
		}

		if c.config.CgroupParent != "" {
			return errors.Wrapf(define.ErrInvalidArg, "cannot set cgroup parent if not creating cgroups")
		}

		// Ensure we have a PID namespace
		if c.config.Spec.Linux == nil {
			return errors.Wrapf(define.ErrInvalidArg, "must provide Linux namespace configuration in OCI spec when using NoCgroups")
		}
		foundPid := false
		for _, ns := range c.config.Spec.Linux.Namespaces {
			if ns.Type == spec.PIDNamespace {
				foundPid = true
				if ns.Path != "" {
					return errors.Wrapf(define.ErrInvalidArg, "containers not creating CGroups must create a private PID namespace - cannot use another")
				}
				break
			}
		}
		if !foundPid {
			return errors.Wrapf(define.ErrInvalidArg, "containers not creating CGroups must create a private PID namespace")
		}
	}

	// Can only set static IP or MAC is creating a network namespace.
	if !c.config.CreateNetNS && (c.config.StaticIP != nil || c.config.StaticMAC != nil) {
		return errors.Wrapf(define.ErrInvalidArg, "cannot set static IP or MAC address if not creating a network namespace")
	}

	// Cannot set static IP or MAC if joining >1 CNI network.
	if len(c.config.Networks) > 1 && (c.config.StaticIP != nil || c.config.StaticMAC != nil) {
		return errors.Wrapf(define.ErrInvalidArg, "cannot set static IP or MAC address if joining more than one CNI network")
	}

	// Using image resolv.conf conflicts with various DNS settings.
	if c.config.UseImageResolvConf &&
		(len(c.config.DNSSearch) > 0 || len(c.config.DNSServer) > 0 ||
			len(c.config.DNSOption) > 0) {
		return errors.Wrapf(define.ErrInvalidArg, "cannot configure DNS options if using image's resolv.conf")
	}

	if c.config.UseImageHosts && len(c.config.HostAdd) > 0 {
		return errors.Wrapf(define.ErrInvalidArg, "cannot add to /etc/hosts if using image's /etc/hosts")
	}

	// Check named volume, overlay volume and image volume destination conflits
	destinations := make(map[string]bool)
	for _, vol := range c.config.NamedVolumes {
		// Don't check if they already exist.
		// If they don't we will automatically create them.
		if _, ok := destinations[vol.Dest]; ok {
			return errors.Wrapf(define.ErrInvalidArg, "two volumes found with destination %s", vol.Dest)
		}
		destinations[vol.Dest] = true
	}
	for _, vol := range c.config.OverlayVolumes {
		// Don't check if they already exist.
		// If they don't we will automatically create them.
		if _, ok := destinations[vol.Dest]; ok {
			return errors.Wrapf(define.ErrInvalidArg, "two volumes found with destination %s", vol.Dest)
		}
		destinations[vol.Dest] = true
	}
	for _, vol := range c.config.ImageVolumes {
		// Don't check if they already exist.
		// If they don't we will automatically create them.
		if _, ok := destinations[vol.Dest]; ok {
			return errors.Wrapf(define.ErrInvalidArg, "two volumes found with destination %s", vol.Dest)
		}
		destinations[vol.Dest] = true
	}

	// Check that networks and network aliases match up.
	ctrNets := make(map[string]bool)
	for _, net := range c.config.Networks {
		ctrNets[net] = true
	}
	for net := range c.config.NetworkAliases {
		if _, ok := ctrNets[net]; !ok {
			return errors.Wrapf(define.ErrNoSuchNetwork, "container tried to set network aliases for network %s but is not connected to the network", net)
		}
	}

	return nil
}
