package machine

import (
	"os"
	"strings"
	"sync"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

// TODO: change name to MachineMarker since package is already called machine
//
//nolint:revive
type MachineMarker struct {
	Enabled bool
	Type    string
}

const (
	markerFile = "/etc/containers/podman-machine"
	Wsl        = "wsl"
	Qemu       = "qemu"
)

var (
	markerSync    sync.Once
	machineMarker *MachineMarker
)

func loadMachineMarker(file string) {
	var kind string

	// Support deprecated config value for compatibility
	enabled := isLegacyConfigSet()

	if content, err := os.ReadFile(file); err == nil {
		enabled = true
		kind = strings.TrimSpace(string(content))
	}

	machineMarker = &MachineMarker{enabled, kind}
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

// TODO: change name to HostType since package is already called machine
//
//nolint:revive
func MachineHostType() string {
	return GetMachineMarker().Type
}

func IsGvProxyBased() bool {
	return IsPodmanMachine() && MachineHostType() != Wsl
}

func GetMachineMarker() *MachineMarker {
	markerSync.Do(func() {
		loadMachineMarker(markerFile)
	})
	return machineMarker
}
