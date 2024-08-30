//go:build (linux || freebsd) && !cni

package network

import (
	"fmt"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/storage"
)

const (
	cniSupported = false
)

func networkBackendFromStore(_store storage.Store, _conf *config.Config) (backend types.NetworkBackend, err error) {
	return types.Netavark, nil
}

func backendFromType(backend types.NetworkBackend, store storage.Store, conf *config.Config, syslog bool) (types.NetworkBackend, types.ContainerNetwork, error) {
	if backend != types.Netavark {
		return "", nil, fmt.Errorf("cni support is not enabled in this build, only netavark. Got unsupported network backend %q", backend)
	}
	cn, err := netavarkBackendFromConf(store, conf, syslog)
	if err != nil {
		return "", nil, err
	}
	return types.Netavark, cn, err
}
