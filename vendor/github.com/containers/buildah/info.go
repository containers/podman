package buildah

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/system"
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
	hostInfo, err := hostInfo()
	if err != nil {
		logrus.Error(err, "error getting host info")
	}
	info = append(info, InfoData{Type: "host", Data: hostInfo})

	// get store information
	storeInfo, err := storeInfo(store)
	if err != nil {
		logrus.Error(err, "error getting store info")
	}
	info = append(info, InfoData{Type: "store", Data: storeInfo})
	return info, nil
}

func hostInfo() (map[string]interface{}, error) {
	info := map[string]interface{}{}
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpus"] = runtime.NumCPU()
	info["rootless"] = unshare.IsRootless()
	mi, err := system.ReadMemInfo()
	if err != nil {
		logrus.Error(err, "err reading memory info")
		info["MemTotal"] = ""
		info["MenFree"] = ""
		info["SwapTotal"] = ""
		info["SwapFree"] = ""
	} else {
		info["MemTotal"] = mi.MemTotal
		info["MenFree"] = mi.MemFree
		info["SwapTotal"] = mi.SwapTotal
		info["SwapFree"] = mi.SwapFree
	}
	hostDistributionInfo := getHostDistributionInfo()
	info["Distribution"] = map[string]interface{}{
		"distribution": hostDistributionInfo["Distribution"],
		"version":      hostDistributionInfo["Version"],
	}

	kv, err := readKernelVersion()
	if err != nil {
		logrus.Error(err, "error reading kernel version")
	}
	info["kernel"] = kv

	up, err := readUptime()
	if err != nil {
		logrus.Error(err, "error reading up time")
	}
	// Convert uptime in seconds to a human-readable format
	upSeconds := up + "s"
	upDuration, err := time.ParseDuration(upSeconds)
	if err != nil {
		logrus.Error(err, "error parsing system uptime")
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

	return info, nil

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

func readKernelVersion() (string, error) {
	buf, err := ioutil.ReadFile("/proc/version")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 2 {
		return string(bytes.TrimSpace(buf)), nil
	}
	return string(f[2]), nil
}

func readUptime() (string, error) {
	buf, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 1 {
		return "", fmt.Errorf("invalid uptime")
	}
	return string(f[0]), nil
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
