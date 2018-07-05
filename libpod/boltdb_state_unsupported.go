// +build !linux

package libpod

import (
	"github.com/boltdb/bolt"
)

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket) error {
	return ErrNotImplemented
}
