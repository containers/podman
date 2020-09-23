// +build varlink

package varlinkapi

import (
	"context"
	"fmt"
	"os"
	goruntime "runtime"
	"strconv"
	"time"

	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/podman/v2/libpod/define"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/sirupsen/logrus"
)

// GetVersion ...
func (i *VarlinkAPI) GetVersion(call iopodman.VarlinkCall) error {
	versionInfo, err := define.GetVersion()
	if err != nil {
		return err
	}

	int64APIVersion, err := strconv.ParseInt(versionInfo.APIVersion, 10, 64)
	if err != nil {
		return err
	}

	return call.ReplyGetVersion(
		versionInfo.Version,
		versionInfo.GoVersion,
		versionInfo.GitCommit,
		time.Unix(versionInfo.Built, 0).Format(time.RFC3339),
		versionInfo.OsArch,
		int64APIVersion,
	)
}

// GetInfo returns details about the podman host and its stores
func (i *VarlinkAPI) GetInfo(call iopodman.VarlinkCall) error {
	versionInfo, err := define.GetVersion()
	if err != nil {
		return err
	}
	podmanInfo := iopodman.PodmanInfo{}
	info, err := i.Runtime.Info()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	distribution := iopodman.InfoDistribution{
		Distribution: info.Host.Distribution.Distribution,
		Version:      info.Host.Distribution.Version,
	}
	infoHost := iopodman.InfoHost{
		Buildah_version: info.Host.BuildahVersion,
		Distribution:    distribution,
		Mem_free:        info.Host.MemFree,
		Mem_total:       info.Host.MemTotal,
		Swap_free:       info.Host.SwapFree,
		Swap_total:      info.Host.SwapTotal,
		Arch:            info.Host.Arch,
		Cpus:            int64(info.Host.CPUs),
		Hostname:        info.Host.Hostname,
		Kernel:          info.Host.Kernel,
		Os:              info.Host.OS,
		Uptime:          info.Host.Uptime,
		Eventlogger:     info.Host.EventLogger,
	}
	podmanInfo.Host = infoHost
	pmaninfo := iopodman.InfoPodmanBinary{
		Compiler:       goruntime.Compiler,
		Go_version:     goruntime.Version(),
		Podman_version: versionInfo.Version,
		Git_commit:     versionInfo.GitCommit,
	}

	graphStatus := iopodman.InfoGraphStatus{
		Backing_filesystem:  info.Store.GraphStatus["Backing Filesystem"],
		Native_overlay_diff: info.Store.GraphStatus["Native Overlay Diff"],
		Supports_d_type:     info.Store.GraphStatus["Supports d_type"],
	}
	infoStore := iopodman.InfoStore{
		Graph_driver_name:    info.Store.GraphDriverName,
		Containers:           int64(info.Store.ContainerStore.Number),
		Images:               int64(info.Store.ImageStore.Number),
		Run_root:             info.Store.RunRoot,
		Graph_root:           info.Store.GraphRoot,
		Graph_driver_options: fmt.Sprintf("%v", info.Store.GraphOptions),
		Graph_status:         graphStatus,
	}

	// Registry information if any is stored as the second list item
	for key, val := range info.Registries {
		if key == "search" {
			podmanInfo.Registries.Search = val.([]string)
			continue
		}
		regData := val.(sysregistriesv2.Registry)
		if regData.Insecure {
			podmanInfo.Registries.Insecure = append(podmanInfo.Registries.Insecure, key)
		}
		if regData.Blocked {
			podmanInfo.Registries.Blocked = append(podmanInfo.Registries.Blocked, key)
		}
	}
	podmanInfo.Store = infoStore
	podmanInfo.Podman = pmaninfo
	return call.ReplyGetInfo(podmanInfo)
}

// GetVersion ...
func (i *VarlinkAPI) Reset(call iopodman.VarlinkCall) error {
	if err := i.Runtime.Reset(context.TODO()); err != nil {
		logrus.Errorf("Reset Failed: %v", err)
		if err := call.ReplyErrorOccurred(err.Error()); err != nil {
			logrus.Errorf("Failed to send ReplyErrorOccurred: %v", err)
		}
		os.Exit(define.ExecErrorCodeGeneric)
	}
	if err := call.ReplyReset(); err != nil {
		logrus.Errorf("Failed to send ReplyReset: %v", err)
		os.Exit(define.ExecErrorCodeGeneric)
	}
	os.Exit(0)
	return nil
}
