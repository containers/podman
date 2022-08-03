// +build darwin

package buildah

import (
	"github.com/containers/buildah/define"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// ContainerDevices is an alias for a slice of github.com/opencontainers/runc/libcontainer/configs.Device structures.
type ContainerDevices define.ContainerDevices

func setChildProcess() error {
	return errors.New("function not supported on non-linux systems")
}

func runUsingRuntimeMain() {}

func (b *Builder) Run(command []string, options RunOptions) error {
	return errors.New("function not supported on non-linux systems")
}
func DefaultNamespaceOptions() (NamespaceOptions, error) {
	return NamespaceOptions{}, errors.New("function not supported on non-linux systems")
}

// getNetworkInterface creates the network interface
func getNetworkInterface(store storage.Store, cniConfDir, cniPluginPath string) (nettypes.ContainerNetwork, error) {
	return nil, errors.New("function not supported on non-linux systems")
}
