package define

import "github.com/containers/common/pkg/strongunits"

type SetOptions struct {
	CPUs               *uint64
	DiskSize           *strongunits.GiB
	Memory             *strongunits.MiB
	Rootful            *bool
	UserModeNetworking *bool
	USBs               *[]string
}
