package common

import (
	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

// validate determines if the flags and values given by the user are valid. things checked
// by validate must not need any state information on the flag (i.e. changed)
func (c *ContainerCLIOpts) validate() error {
	var ()
	if c.Rm && c.Restart != "" && c.Restart != "no" {
		return errors.Errorf("the --rm option conflicts with --restart")
	}

	if _, err := util.ValidatePullType(c.Pull); err != nil {
		return err
	}
	// Verify the additional hosts are in correct format
	for _, host := range c.Net.AddHosts {
		if _, err := parse.ValidateExtraHost(host); err != nil {
			return err
		}
	}

	if dnsSearches := c.Net.DNSSearch; len(dnsSearches) > 0 {
		// Validate domains are good
		for _, dom := range dnsSearches {
			if dom == "." {
				if len(dnsSearches) > 1 {
					return errors.Errorf("cannot pass additional search domains when also specifying '.'")
				}
				continue
			}
			if _, err := parse.ValidateDomain(dom); err != nil {
				return err
			}
		}
	}
	var imageVolType = map[string]string{
		"bind":   "",
		"tmpfs":  "",
		"ignore": "",
	}
	if _, ok := imageVolType[c.ImageVolume]; !ok {
		return errors.Errorf("invalid image-volume type %q. Pick one of bind, tmpfs, or ignore", c.ImageVolume)
	}
	return nil

}
