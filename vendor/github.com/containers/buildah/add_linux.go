package buildah

import (
	"github.com/opencontainers/runc/libcontainer/userns"
)

func runningInUserNS() bool {
	return userns.RunningInUserNS()
}
