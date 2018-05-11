package varlinkapi

import (
	goruntime "runtime"
	"strings"

	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/cmd/podman/varlink"
	"github.com/projectatomic/libpod/libpod"
)

// GetVersion ...
func (i *LibpodAPI) GetVersion(call ioprojectatomicpodman.VarlinkCall) error {
	versionInfo, err := libpod.GetVersion()
	if err != nil {
		return err
	}

	return call.ReplyGetVersion(ioprojectatomicpodman.Version{
		Version:    versionInfo.Version,
		Go_version: versionInfo.GoVersion,
		Git_commit: versionInfo.GitCommit,
		Built:      versionInfo.Built,
		Os_arch:    versionInfo.OsArch,
	})
}

// Ping returns a simple string "OK" response for clients to make sure
// the service is working.
func (i *LibpodAPI) Ping(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyPing(ioprojectatomicpodman.StringResponse{
		Message: "OK",
	})
}

// GetInfo returns details about the podman host and its stores
func (i *LibpodAPI) GetInfo(call ioprojectatomicpodman.VarlinkCall) error {
	podmanInfo := ioprojectatomicpodman.PodmanInfo{}
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	info, err := runtime.Info()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	host := info[0].Data
	infoHost := ioprojectatomicpodman.InfoHost{
		Mem_free:  host["MemFree"].(int64),
		Mem_total: host["MemTotal"].(int64),
		Swap_free: host["SwapFree"].(int64),
		Arch:      host["arch"].(string),
		Cpus:      int64(host["cpus"].(int)),
		Hostname:  host["hostname"].(string),
		Kernel:    host["kernel"].(string),
		Os:        host["os"].(string),
		Uptime:    host["uptime"].(string),
	}
	podmanInfo.Host = infoHost
	store := info[1].Data
	pmaninfo := ioprojectatomicpodman.InfoPodmanBinary{
		Compiler:   goruntime.Compiler,
		Go_version: goruntime.Version(),
		// TODO : How are we going to get this here?
		//Podman_version:
		Git_commit: libpod.GitCommit,
	}

	graphStatus := ioprojectatomicpodman.InfoGraphStatus{
		Backing_filesystem:  store["GraphStatus"].(map[string]string)["Backing Filesystem"],
		Native_overlay_diff: store["GraphStatus"].(map[string]string)["Native Overlay Diff"],
		Supports_d_type:     store["GraphStatus"].(map[string]string)["Supports d_type"],
	}
	infoStore := ioprojectatomicpodman.InfoStore{
		Graph_driver_name:    store["GraphDriverName"].(string),
		Containers:           int64(store["ContainerStore"].(map[string]interface{})["number"].(int)),
		Images:               int64(store["ImageStore"].(map[string]interface{})["number"].(int)),
		Run_root:             store["RunRoot"].(string),
		Graph_root:           store["GraphRoot"].(string),
		Graph_driver_options: strings.Join(store["GraphOptions"].([]string), ", "),
		Graph_status:         graphStatus,
	}

	podmanInfo.Store = infoStore
	podmanInfo.Podman = pmaninfo
	return call.ReplyGetInfo(podmanInfo)
}
