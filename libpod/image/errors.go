package image

import (
	"github.com/containers/podman/v2/libpod/define"
)

var (
	// ErrNoSuchCtr indicates the requested container does not exist
	ErrNoSuchCtr = define.ErrNoSuchCtr
	// ErrNoSuchPod indicates the requested pod does not exist
	ErrNoSuchPod = define.ErrNoSuchPod
	// ErrNoSuchImage indicates the requested image does not exist
	ErrNoSuchImage = define.ErrNoSuchImage
	// ErrNoSuchTag indicates the requested image tag does not exist
	ErrNoSuchTag = define.ErrNoSuchTag
)
