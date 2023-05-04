//go:build freebsd
// +build freebsd

package libpod

func networkDisabled(c *Container) (bool, error) {
	if c.config.CreateNetNS {
		return false, nil
	}
	return c.state.NetNS != "", nil
}
