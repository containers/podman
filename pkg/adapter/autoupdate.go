// +build !remoteclient

package adapter

import (
	"github.com/containers/libpod/pkg/autoupdate"
)

func (r *LocalRuntime) AutoUpdate() ([]string, []error) {
	return autoupdate.AutoUpdate(r.Runtime)
}
