package machine

import (
	"os"
	"strings"
	"sync"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

type Marker struct {
	Enabled bool
	Type    string
}

const (
	markerFile = "/etc/containers/podman-machine"
	Wsl        = "wsl"
	Qemu       = "qemu"
	AppleHV    = "applehv"
	HyperV     = "hyperv"
)

var (
	markerSync sync.Once
	marker     *Marker
)

func loadMachineMarker(file string) {
	var kind string

	// Support deprecated config value for compatibility
	enabled := isLegacyConfigSet()

	if content, err := os.ReadFile(file); err == nil {
		enabled = true
		kind = strings.TrimSpace(string(content))
	}

	marker = &Marker{enabled, kind}
}

func isLegacyConfigSet() bool {
	config, err := config.Default()
	if err != nil {
		logrus.Warnf("could not obtain container configuration")
		return false
	}

	//nolint:staticcheck //lint:ignore SA1019 deprecated call
	return config.Engine.MachineEnabled
}

func IsPodmanMachine() bool {
	return GetMachineMarker().Enabled
}

func HostType() string {
	return GetMachineMarker().Type
}

func IsGvProxyBased() bool {
	return IsPodmanMachine() && HostType() != Wsl
}

func GetMachineMarker() *Marker {
	markerSync.Do(func() {
		loadMachineMarker(markerFile)
	})
	return marker
}
