package types

import (
	"github.com/containers/podman/v4/libpod/define"
)

type ContainerCopyFunc func() error

type ContainerStatReport struct {
	define.FileInfo
}
