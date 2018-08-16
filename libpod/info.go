package libpod

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/utils"
	"github.com/containers/storage/pkg/system"
	"github.com/pkg/errors"
)

// InfoData holds the info type, i.e store, host etc and the data for each type
type InfoData struct {
	Type string
	Data map[string]interface{}
}

// top-level "host" info
func (r *Runtime) hostInfo() (map[string]interface{}, error) {
	// lets say OS, arch, number of cpus, amount of memory, maybe os distribution/version, hostname, kernel version, uptime
	info := map[string]interface{}{}
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpus"] = runtime.NumCPU()
	mi, err := system.ReadMemInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading memory info")
	}
	// TODO this might be a place for github.com/dustin/go-humanize
	info["MemTotal"] = mi.MemTotal
	info["MemFree"] = mi.MemFree
	info["SwapTotal"] = mi.SwapTotal
	info["SwapFree"] = mi.SwapFree
	conmonVersion, _ := r.GetConmonVersion()
	ociruntimeVersion, _ := r.GetOCIRuntimeVersion()
	info["Conmon"] = map[string]interface{}{
		"path":    r.conmonPath,
		"package": r.ociRuntime.conmonPackage(),
		"version": conmonVersion,
	}
	info["OCIRuntime"] = map[string]interface{}{
		"path":    r.ociRuntime.path,
		"package": r.ociRuntime.pathPackage(),
		"version": ociruntimeVersion,
	}

	kv, err := readKernelVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading kernel version")
	}
	info["kernel"] = kv

	up, err := readUptime()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading up time")
	}
	// Convert uptime in seconds to a human-readable format
	upSeconds := up + "s"
	upDuration, err := time.ParseDuration(upSeconds)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing system uptime")
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
		return nil, errors.Wrapf(err, "error getting hostname")
	}
	info["hostname"] = host

	return info, nil
}

// top-level "store" info
func (r *Runtime) storeInfo() (map[string]interface{}, error) {
	// lets say storage driver in use, number of images, number of containers
	info := map[string]interface{}{}
	info["GraphRoot"] = r.store.GraphRoot()
	info["RunRoot"] = r.store.RunRoot()
	info["GraphDriverName"] = r.store.GraphDriverName()
	info["GraphOptions"] = r.store.GraphOptions()
	statusPairs, err := r.store.Status()
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for _, pair := range statusPairs {
		status[pair[0]] = pair[1]
	}
	info["GraphStatus"] = status
	images, err := r.store.Images()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting number of images")
	}
	info["ImageStore"] = map[string]interface{}{
		"number": len(images),
	}

	containers, err := r.store.Containers()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting number of containers")
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

// GetConmonVersion returns a string representation of the conmon version
func (r *Runtime) GetConmonVersion() (string, error) {
	output, err := utils.ExecCmd(r.conmonPath, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.Replace(output, "\n", ", ", 1), "\n"), nil
}

// GetOCIRuntimeVersion returns a string representation of the oci runtimes version
func (r *Runtime) GetOCIRuntimeVersion() (string, error) {
	output, err := utils.ExecCmd(r.ociRuntimePath, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(output, "\n"), nil
}
