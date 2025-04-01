//go:build !remote

package generate

import (
	"context"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/libpod"
)

// PullInfraImage pulls down the specified image or the one set in
// containers.conf. If none is set, it returns an empty string. In this
// case, the rootfs-based pause image is used by libpod.
func PullInfraImage(rt *libpod.Runtime, imageName string) (string, error) {
	rtConfig, err := rt.GetConfigNoCopy()
	if err != nil {
		return "", err
	}

	if imageName == "" {
		imageName = rtConfig.Engine.InfraImage
	}

	if imageName != "" {
		_, err := rt.LibimageRuntime().Pull(context.Background(), imageName, config.PullPolicyMissing, nil)
		if err != nil {
			return "", err
		}
		return imageName, nil
	}

	return "", nil
}
