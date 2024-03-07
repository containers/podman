package buildah

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	internalUtil "github.com/containers/buildah/internal/util"
	putil "github.com/containers/buildah/pkg/util"
	"github.com/containers/buildah/util"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// InfoData holds the info type, i.e store, host etc and the data for each type
type InfoData struct {
	Type string
	Data map[string]interface{}
}

// Info returns the store and host information
func Info(store storage.Store) ([]InfoData, error) {
	info := []InfoData{}
	// get host information
	hostInfo := hostInfo()
	info = append(info, InfoData{Type: "host", Data: hostInfo})

	// get store information
	storeInfo, err := storeInfo(store)
	if err != nil {
		logrus.Error(err, "error getting store info")
	}
	info = append(info, InfoData{Type: "store", Data: storeInfo})
	return info, nil
}

func hostInfo() map[string]interface{} {
	info := map[string]interface{}{}
	ps := internalUtil.NormalizePlatform(v1.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH})
	info["os"] = ps.OS
	info["arch"] = ps.Architecture
	info["variant"] = ps.Variant
	info["cpus"] = runtime.NumCPU()
	info["rootless"] = unshare.IsRootless()

	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		logrus.Error(err, "err reading cgroups mode")
	}
	cgroupVersion := "v1"
	ociruntime := util.Runtime()
	if unified {
		cgroupVersion = "v2"
	}
	info["CgroupVersion"] = cgroupVersion
	info["OCIRuntime"] = ociruntime

	mi, err := system.ReadMemInfo()
	if err != nil {
		logrus.Error(err, "err reading memory info")
		info["MemTotal"] = ""
		info["MemFree"] = ""
		info["SwapTotal"] = ""
		info["SwapFree"] = ""
	} else {
		info["MemTotal"] = mi.MemTotal
		info["MemFree"] = mi.MemFree
		info["SwapTotal"] = mi.SwapTotal
		info["SwapFree"] = mi.SwapFree
	}
	hostDistributionInfo := getHostDistributionInfo()
	info["Distribution"] = map[string]interface{}{
		"distribution": hostDistributionInfo["Distribution"],
		"version":      hostDistributionInfo["Version"],
	}

	kv, err := putil.ReadKernelVersion()
	if err != nil {
		logrus.Error(err, "error reading kernel version")
	}
	info["kernel"] = kv

	upDuration, err := putil.ReadUptime()
	if err != nil {
		logrus.Error(err, "error reading up time")
	}

	hoursFound := false
	var timeBuffer bytes.Buffer
	var hoursBuffer bytes.Buffer
	for _, elem := range upDuration.String() {
		timeBuffer.WriteRune(elem)
		if elem == 'h' || elem == 'm' {
			timeBuffer.WriteRune(' ')
			if elem == 'h' {
				hoursFound = true
			}
		}
		if !hoursFound {
			hoursBuffer.WriteRune(elem)
		}
	}

	info["uptime"] = timeBuffer.String()
	if hoursFound {
		hours, err := strconv.ParseFloat(hoursBuffer.String(), 64)
		if err == nil {
			days := hours / 24
			info["uptime"] = fmt.Sprintf("%s (Approximately %.2f days)", info["uptime"], days)
		}
	}

	host, err := os.Hostname()
	if err != nil {
		logrus.Error(err, "error getting hostname")
	}
	info["hostname"] = host

	return info
}

// top-level "store" info
func storeInfo(store storage.Store) (map[string]interface{}, error) {
	// lets say storage driver in use, number of images, number of containers
	info := map[string]interface{}{}
	info["GraphRoot"] = store.GraphRoot()
	info["RunRoot"] = store.RunRoot()
	info["GraphDriverName"] = store.GraphDriverName()
	info["GraphOptions"] = store.GraphOptions()
	statusPairs, err := store.Status()
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for _, pair := range statusPairs {
		status[pair[0]] = pair[1]
	}
	info["GraphStatus"] = status
	images, err := store.Images()
	if err != nil {
		logrus.Error(err, "error getting number of images")
	}
	info["ImageStore"] = map[string]interface{}{
		"number": len(images),
	}

	containers, err := store.Containers()
	if err != nil {
		logrus.Error(err, "error getting number of containers")
	}
	info["ContainerStore"] = map[string]interface{}{
		"number": len(containers),
	}

	return info, nil
}

// getHostDistributionInfo returns a map containing the host's distribution and version
func getHostDistributionInfo() map[string]string {
	dist := make(map[string]string)

	// Populate values in case we cannot find the values
	// or the file
	dist["Distribution"] = "unknown"
	dist["Version"] = "unknown"

	f, err := os.Open("/etc/os-release")
	if err != nil {
		return dist
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			dist["Distribution"] = strings.TrimPrefix(l.Text(), "ID=")
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			dist["Version"] = strings.Trim(strings.TrimPrefix(l.Text(), "VERSION_ID="), "\"")
		}
	}
	return dist
}
