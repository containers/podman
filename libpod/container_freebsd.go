//go:build freebsd
// +build freebsd

package libpod

type containerPlatformState struct {
	// NetworkJail is the name of the container's network VNET
	// jail.  Will only be set if config.CreateNetNS is true, or
	// the container was told to join another container's network
	// namespace.
	NetworkJail string `json:"-"`
}

func networkDisabled(c *Container) (bool, error) {
	if c.config.CreateNetNS {
		return false, nil
	}
	if !c.config.PostConfigureNetNS {
		return c.state.NetworkJail == "", nil
	}
	return false, nil
}
