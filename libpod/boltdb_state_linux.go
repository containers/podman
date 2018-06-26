// +build linux

package libpod

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket) error {
	valid := true
	ctrBkt := ctrsBkt.Bucket(id)
	if ctrBkt == nil {
		return errors.Wrapf(ErrNoSuchCtr, "container %s not found in DB", string(id))
	}

	if s.namespaceBytes != nil {
		ctrNamespaceBytes := ctrBkt.Get(namespaceKey)
		if !bytes.Equal(s.namespaceBytes, ctrNamespaceBytes) {
			return errors.Wrapf(ErrNSMismatch, "cannot retrieve container %s as it is part of namespace %q and we are in namespace %q", string(id), string(ctrNamespaceBytes), s.namespace)
		}
	}

	configBytes := ctrBkt.Get(configKey)
	if configBytes == nil {
		return errors.Wrapf(ErrInternal, "container %s missing config key in DB", string(id))
	}

	stateBytes := ctrBkt.Get(stateKey)
	if stateBytes == nil {
		return errors.Wrapf(ErrInternal, "container %s missing state key in DB", string(id))
	}

	netNSBytes := ctrBkt.Get(netNSKey)

	if err := json.Unmarshal(configBytes, ctr.config); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s config", string(id))
	}

	if err := json.Unmarshal(stateBytes, ctr.state); err != nil {
		return errors.Wrapf(err, "error unmarshalling container %s state", string(id))
	}

	// The container may not have a network namespace, so it's OK if this is
	// nil
	if netNSBytes != nil {
		nsPath := string(netNSBytes)
		netNS, err := joinNetNS(nsPath)
		if err == nil {
			ctr.state.NetNS = netNS
		} else {
			logrus.Errorf("error joining network namespace for container %s", ctr.ID())
			valid = false
		}
	}

	// Get the lock
	lockPath := filepath.Join(s.lockDir, string(id))
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return errors.Wrapf(err, "error retrieving lockfile for container %s", string(id))
	}
	ctr.lock = lock

	ctr.runtime = s.runtime
	ctr.valid = valid

	return nil
}
