//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package vmconfigs

import (
	"github.com/containers/podman/v5/pkg/machine/define"
)

func getPipe(name string) *define.VMFile {
	return nil
}
