package generate

import (
	"context"
	"fmt"
	"os"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
)

// PullOrBuildInfraImage pulls down the specified image or the one set in
// containers.conf.  If none is set, it builds a local pause image.
func PullOrBuildInfraImage(rt *libpod.Runtime, imageName string) (string, error) {
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

	name, err := buildPauseImage(rt, rtConfig)
	if err != nil {
		return "", fmt.Errorf("building local pause image: %w", err)
	}
	return name, nil
}

func buildPauseImage(rt *libpod.Runtime, rtConfig *config.Config) (string, error) {
	version, err := define.GetVersion()
	if err != nil {
		return "", err
	}
	imageName := fmt.Sprintf("localhost/podman-pause:%s-%d", version.Version, version.Built)

	// First check if the image has already been built.
	if _, _, err := rt.LibimageRuntime().LookupImage(imageName, nil); err == nil {
		return imageName, nil
	}

	// Also look into the path as some distributions install catatonit in
	// /usr/bin.
	catatonitPath, err := rtConfig.FindHelperBinary("catatonit", true)
	if err != nil {
		return "", fmt.Errorf("finding pause binary: %w", err)
	}

	buildContent := fmt.Sprintf(`FROM scratch
COPY %s /catatonit
ENTRYPOINT ["/catatonit", "-P"]`, catatonitPath)

	tmpF, err := os.CreateTemp("", "pause.containerfile")
	if err != nil {
		return "", err
	}
	if _, err := tmpF.WriteString(buildContent); err != nil {
		return "", err
	}
	if err := tmpF.Close(); err != nil {
		return "", err
	}
	defer os.Remove(tmpF.Name())

	buildOptions := buildahDefine.BuildOptions{
		CommonBuildOpts: &buildahDefine.CommonBuildOptions{},
		Output:          imageName,
		Quiet:           true,
		IgnoreFile:      "/dev/null", // makes sure to not read a local .ignorefile (see #13529)
		IIDFile:         "/dev/null", // prevents Buildah from writing the ID on stdout
		IDMappingOptions: &buildahDefine.IDMappingOptions{
			// Use the host UID/GID mappings for the build to avoid issues when
			// running with a custom mapping (BZ #2083997).
			HostUIDMapping: true,
			HostGIDMapping: true,
		},
	}
	if _, _, err := rt.Build(context.Background(), buildOptions, tmpF.Name()); err != nil {
		return "", err
	}

	return imageName, nil
}
