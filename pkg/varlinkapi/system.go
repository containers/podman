// +build varlink

package varlinkapi

import (
	goruntime "runtime"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// GetVersion ...
func (i *LibpodAPI) GetVersion(call iopodman.VarlinkCall) error {
	versionInfo, err := libpod.GetVersion()
	if err != nil {
		return err
	}

	return call.ReplyGetVersion(
		versionInfo.Version,
		versionInfo.GoVersion,
		versionInfo.GitCommit,
		time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
		versionInfo.OsArch,
		versionInfo.RemoteAPIVersion,
	)
}

// GetInfo returns details about the podman host and its stores
func (i *LibpodAPI) GetInfo(call iopodman.VarlinkCall) error {
	versionInfo, err := libpod.GetVersion()
	if err != nil {
		return err
	}
	var (
		registries, insecureRegistries []string
	)
	podmanInfo := iopodman.PodmanInfo{}
	info, err := i.Runtime.Info()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	host := info[0].Data
	distribution := iopodman.InfoDistribution{
		Distribution: host["Distribution"].(map[string]interface{})["distribution"].(string),
		Version:      host["Distribution"].(map[string]interface{})["version"].(string),
	}
	infoHost := iopodman.InfoHost{
		Buildah_version: host["BuildahVersion"].(string),
		Distribution:    distribution,
		Mem_free:        host["MemFree"].(int64),
		Mem_total:       host["MemTotal"].(int64),
		Swap_free:       host["SwapFree"].(int64),
		Swap_total:      host["SwapTotal"].(int64),
		Arch:            host["arch"].(string),
		Cpus:            int64(host["cpus"].(int)),
		Hostname:        host["hostname"].(string),
		Kernel:          host["kernel"].(string),
		Os:              host["os"].(string),
		Uptime:          host["uptime"].(string),
	}
	podmanInfo.Host = infoHost
	store := info[1].Data
	pmaninfo := iopodman.InfoPodmanBinary{
		Compiler:       goruntime.Compiler,
		Go_version:     goruntime.Version(),
		Podman_version: versionInfo.Version,
		Git_commit:     versionInfo.GitCommit,
	}

	graphStatus := iopodman.InfoGraphStatus{
		Backing_filesystem:  store["GraphStatus"].(map[string]string)["Backing Filesystem"],
		Native_overlay_diff: store["GraphStatus"].(map[string]string)["Native Overlay Diff"],
		Supports_d_type:     store["GraphStatus"].(map[string]string)["Supports d_type"],
	}
	infoStore := iopodman.InfoStore{
		Graph_driver_name:    store["GraphDriverName"].(string),
		Containers:           int64(store["ContainerStore"].(map[string]interface{})["number"].(int)),
		Images:               int64(store["ImageStore"].(map[string]interface{})["number"].(int)),
		Run_root:             store["RunRoot"].(string),
		Graph_root:           store["GraphRoot"].(string),
		Graph_driver_options: strings.Join(store["GraphOptions"].([]string), ", "),
		Graph_status:         graphStatus,
	}

	if len(info) > 2 {
		registriesInterface := info[2].Data["registries"]
		if registriesInterface != nil {
			registries = registriesInterface.([]string)
		}
	}
	if len(info) > 3 {
		insecureRegistriesInterface := info[3].Data["registries"]
		if insecureRegistriesInterface != nil {
			insecureRegistries = insecureRegistriesInterface.([]string)
		}
	}
	podmanInfo.Store = infoStore
	podmanInfo.Podman = pmaninfo
	podmanInfo.Registries = registries
	podmanInfo.Insecure_registries = insecureRegistries
	return call.ReplyGetInfo(podmanInfo)
}
