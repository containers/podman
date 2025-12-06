package buildah

import (
	"github.com/moby/sys/userns"
)

func runningInUserNS() bool {
	return userns.RunningInUserNS()
}
