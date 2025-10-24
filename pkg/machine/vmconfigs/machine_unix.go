//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package vmconfigs

import (
	"github.com/containers/podman/v6/pkg/machine/define"
)

func getPipe(_ string) *define.VMFile {
	return nil
}
