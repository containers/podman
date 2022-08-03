// +build !linux,!darwin

package buildah

import (
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

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
