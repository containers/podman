package specgenutil

import (
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
)

// validate determines if the flags and values given by the user are valid. things checked
// by validate must not need any state information on the flag (i.e. changed)
func validate(c *entities.ContainerCreateOptions) error {
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
		switch {
		case c.IsInfra:
			c.ImageVolume = "bind"
		case c.IsClone: // the image volume type will be deduced later from the container we are cloning
			return nil
		default:
			return errors.Errorf("invalid image-volume type %q. Pick one of bind, tmpfs, or ignore", c.ImageVolume)
		}
	}
	return nil
}
