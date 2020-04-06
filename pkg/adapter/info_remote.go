// +build remoteclient

package adapter

import (
	"github.com/containers/libpod/libpod/define"
	iopodman "github.com/containers/libpod/pkg/varlink"
)

// Info returns information for the host system and its components
func (r RemoteRuntime) Info() (*define.Info, error) {
	// TODO the varlink implementation for info should be updated to match the output for regular info
	var (
		reply define.Info
	)

	info, err := iopodman.GetInfo().Call(r.Conn)
	if err != nil {
		return nil, err
	}
	hostInfo := define.HostInfo{
		Arch:           info.Host.Arch,
		BuildahVersion: info.Host.Buildah_version,
		CPUs:           int(info.Host.Cpus),
		Distribution: define.DistributionInfo{
			Distribution: info.Host.Distribution.Distribution,
			Version:      info.Host.Distribution.Version,
		},
		EventLogger: info.Host.Eventlogger,
		Hostname:    info.Host.Hostname,
		Kernel:      info.Host.Kernel,
		MemFree:     info.Host.Mem_free,
		MemTotal:    info.Host.Mem_total,
		OS:          info.Host.Os,
		SwapFree:    info.Host.Swap_free,
		SwapTotal:   info.Host.Swap_total,
		Uptime:      info.Host.Uptime,
	}
	storeInfo := define.StoreInfo{
		ContainerStore: define.ContainerStore{
			Number: int(info.Store.Containers),
		},
		GraphDriverName: info.Store.Graph_driver_name,
		GraphRoot:       info.Store.Graph_root,
		ImageStore: define.ImageStore{
			Number: int(info.Store.Images),
		},
		RunRoot: info.Store.Run_root,
	}
	reply.Host = &hostInfo
	reply.Store = &storeInfo
	regs := make(map[string]interface{})
	if len(info.Registries.Search) > 0 {
		regs["search"] = info.Registries.Search
	}
	if len(info.Registries.Blocked) > 0 {
		regs["blocked"] = info.Registries.Blocked
	}
	if len(info.Registries.Insecure) > 0 {
		regs["insecure"] = info.Registries.Insecure
	}
	reply.Registries = regs
	return &reply, nil
}
