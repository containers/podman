//go:build !amd64 && !arm64

package localapi

import (
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

func WarnIfMachineVolumesUnavailable(_ *entities.PodmanConfig, _ []string) {
	logrus.Debug("skipping machine volume check: podman machine mode not supported")
}
