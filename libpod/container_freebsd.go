//go:build freebsd
// +build freebsd

package libpod

type containerPlatformState struct {
	// NetNS is the name of the container's network VNET
	// jail.  Will only be set if config.CreateNetNS is true, or
	// the container was told to join another container's network
	// namespace.
	NetNS *jailNetNS `json:"-"`
}

type jailNetNS struct {
	Name string `json:"-"`
}

func (ns *jailNetNS) Path() string {
	// The jail name approximately corresponds to the Linux netns path
	return ns.Name
}

func networkDisabled(c *Container) (bool, error) {
	if c.config.CreateNetNS {
		return false, nil
	}
	if !c.config.PostConfigureNetNS {
		return c.state.NetNS != nil, nil
	}
	return false, nil
}
