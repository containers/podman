//go:build !remote

package generate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/specgen"
)

// PullInfraImage pulls down the specified image or the one set in
// containers.conf.  If none is set, it builds a local pause image.
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

func PrepareInfraSpec(rtConfig *config.Config, infraCommand []string, infraContainerSpec *specgen.SpecGenerator) error {
	// Also look into the path as some distributions install catatonit in
	// /usr/bin.
	catatonitPath, err := rtConfig.FindInitBinary()
	if err != nil {
		return fmt.Errorf("finding pause binary: %w", err)
	}
	catatonitPath, err = filepath.EvalSymlinks(catatonitPath)
	if err != nil {
		return fmt.Errorf("follow symlink to pause binary: %w", err)
	}

	infraContainerSpec.Rootfs = filepath.Dir(catatonitPath)
	overlay := true
	infraContainerSpec.RootfsOverlay = &overlay
	if len(infraCommand) > 0 {
		infraContainerSpec.Entrypoint = infraCommand
	} else {
		infraContainerSpec.Entrypoint = []string{"/" + filepath.Base(catatonitPath), "-P"}
	}
	return nil
}
