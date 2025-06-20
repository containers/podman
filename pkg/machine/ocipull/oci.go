package ocipull

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/containers/podman/v5/version"
)

type OSVersion struct {
	*semver.Version
}

type Disker interface {
	Get() error
}

func getVersion() *OSVersion {
	v := version.Version
	return &OSVersion{&v}
}

func (o *OSVersion) majorMinor() string {
	return fmt.Sprintf("%d.%d", o.Major, o.Minor)
}
