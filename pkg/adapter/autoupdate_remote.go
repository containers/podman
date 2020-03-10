// +build remoteclient

package adapter

import (
	"github.com/containers/libpod/libpod/define"
)

func (r *LocalRuntime) AutoUpdate() ([]string, []error) {
	return nil, []error{define.ErrNotImplemented}
}
