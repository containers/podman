// +build linux

package network

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containers/common/libnetwork/cni"
	"github.com/containers/common/libnetwork/netavark"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/sirupsen/logrus"
)

const defaultNetworkBackendFileName = "defaultNetworkBackend"

// NetworkBackend returns the network backend name and interface
// It returns either the CNI or netavark backend depending on what is set in the config.
// If the the backend is set to "" we will automatically assign the backend on the following conditions:
//   1. read ${graphroot}/defaultNetworkBackend
//   2. find netavark binary (if not installed use CNI)
//   3. check containers, images and CNI networks and if there are some we have an existing install and should continue to use CNI
func NetworkBackend(store storage.Store, conf *config.Config, syslog bool) (types.NetworkBackend, types.ContainerNetwork, error) {
	backend := types.NetworkBackend(conf.Network.NetworkBackend)
	if backend == "" {
		var err error
		backend, err = defaultNetworkBackend(store, conf)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get default network backend: %w", err)
		}
	}

	switch backend {
	case types.Netavark:
		netavarkBin, err := conf.FindHelperBinary("netavark", false)
		if err != nil {
			return "", nil, err
		}
		netInt, err := netavark.NewNetworkInterface(&netavark.InitConfig{
			NetworkConfigDir: filepath.Join(store.GraphRoot(), "networks"),
			NetworkRunDir:    filepath.Join(store.RunRoot(), "networks"),
			NetavarkBinary:   netavarkBin,
			DefaultNetwork:   conf.Network.DefaultNetwork,
			DefaultSubnet:    conf.Network.DefaultSubnet,
			Syslog:           syslog,
		})
		return types.Netavark, netInt, err
	case types.CNI:
		netInt, err := getCniInterface(conf)
		return types.CNI, netInt, err

	default:
		return "", nil, fmt.Errorf("unsupported network backend %q, check network_backend in containers.conf", backend)
	}
}

func defaultNetworkBackend(store storage.Store, conf *config.Config) (backend types.NetworkBackend, err error) {
	// read defaultNetworkBackend file
	file := filepath.Join(store.GraphRoot(), defaultNetworkBackendFileName)
	b, err := ioutil.ReadFile(file)
	if err == nil {
		val := string(b)
		if val == string(types.Netavark) {
			return types.Netavark, nil
		}
		if val == string(types.CNI) {
			return types.CNI, nil
		}
		return "", fmt.Errorf("unknown network backend value %q in %q", val, file)
	}
	// fail for all errors except ENOENT
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("could not read network backend value: %w", err)
	}

	// cache the network backend to make sure always the same one will be used
	defer func() {
		// only write when there is no error
		if err == nil {
			// nolint:gocritic
			if err := ioutils.AtomicWriteFile(file, []byte(backend), 0644); err != nil {
				logrus.Errorf("could not write network backend to file: %v", err)
			}
		}
	}()

	_, err = conf.FindHelperBinary("netavark", false)
	if err != nil {
		// if we cannot find netavark use CNI
		return types.CNI, nil
	}

	// now check if there are already containers, images and CNI networks (new install?)
	cons, err := store.Containers()
	if err != nil {
		return "", err
	}
	if len(cons) == 0 {
		imgs, err := store.Images()
		if err != nil {
			return "", err
		}
		if len(imgs) == 0 {
			cniInterface, err := getCniInterface(conf)
			if err == nil {
				nets, err := cniInterface.NetworkList()
				// there is always a default network so check <= 1
				if err == nil && len(nets) <= 1 {
					// we have a fresh system so use netavark
					return types.Netavark, nil
				}
			}
		}
	}
	return types.CNI, nil
}

func getCniInterface(conf *config.Config) (types.ContainerNetwork, error) {
	return cni.NewCNINetworkInterface(&cni.InitConfig{
		CNIConfigDir:   conf.Network.NetworkConfigDir,
		CNIPluginDirs:  conf.Network.CNIPluginDirs,
		DefaultNetwork: conf.Network.DefaultNetwork,
		DefaultSubnet:  conf.Network.DefaultSubnet,
		IsMachine:      conf.Engine.MachineEnabled,
	})
}
