package common

import (
	"github.com/containers/common/pkg/config"
	"github.com/pkg/errors"
)

// validate determines if the flags and values given by the user are valid. things checked
// by validate must not need any state information on the flag (i.e. changed)
func (c *ContainerCLIOpts) validate() error {
	var ()
	if c.Rm && (c.Restart != "" && c.Restart != "no" && c.Restart != "on-failure") {
		return errors.Errorf(`the --rm option conflicts with --restart, when the restartPolicy is not "" and "no"`)
	}

	if _, err := config.ParsePullPolicy(c.Pull); err != nil {
		return err
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
