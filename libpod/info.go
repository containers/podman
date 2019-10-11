package libpod

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// top-level "host" info
func (r *Runtime) hostInfo() (map[string]interface{}, error) {
	// lets say OS, arch, number of cpus, amount of memory, maybe os distribution/version, hostname, kernel version, uptime
	info := map[string]interface{}{}
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpus"] = runtime.NumCPU()
	info["rootless"] = rootless.IsRootless()
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading cgroups mode")
	}
	cgroupVersion := "v1"
	if unified {
		cgroupVersion = "v2"
	}
	info["CgroupVersion"] = cgroupVersion
	mi, err := system.ReadMemInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading memory info")
	}
	// TODO this might be a place for github.com/dustin/go-humanize
	info["MemTotal"] = mi.MemTotal
	info["MemFree"] = mi.MemFree
	info["SwapTotal"] = mi.SwapTotal
	info["SwapFree"] = mi.SwapFree
	hostDistributionInfo := r.GetHostDistributionInfo()
	if rootless.IsRootless() {
		if path, err := exec.LookPath("slirp4netns"); err == nil {
			logrus.Warnf("Failed to retrieve program version for %s: %v", path, err)
			version, err := programVersion(path)
			if err != nil {
				logrus.Warnf("Failed to retrieve program version for %s: %v", path, err)
			}
			program := map[string]interface{}{}
			program["Executable"] = path
			program["Version"] = version
			program["Package"] = packageVersion(path)
			info["slirp4netns"] = program
		}
		uidmappings, err := rootless.ReadMappingsProc("/proc/self/uid_map")
		if err != nil {
			return nil, errors.Wrapf(err, "error reading uid mappings")
		}
		gidmappings, err := rootless.ReadMappingsProc("/proc/self/gid_map")
		if err != nil {
			return nil, errors.Wrapf(err, "error reading gid mappings")
		}
		idmappings := make(map[string]interface{})
		idmappings["uidmap"] = uidmappings
		idmappings["gidmap"] = gidmappings
		info["IDMappings"] = idmappings
	}
	info["Distribution"] = map[string]interface{}{
		"distribution": hostDistributionInfo["Distribution"],
		"version":      hostDistributionInfo["Version"],
	}
	info["BuildahVersion"] = buildah.Version
	kv, err := readKernelVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading kernel version")
	}
	info["kernel"] = kv

	runtimeInfo, err := r.defaultOCIRuntime.RuntimeInfo()
	if err != nil {
		logrus.Errorf("Error getting info on OCI runtime %s: %v", r.defaultOCIRuntime.Name(), err)
	} else {
		for k, v := range runtimeInfo {
			info[k] = v
		}
	}

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
	info["eventlogger"] = r.eventer.String()

	return info, nil
}

// top-level "store" info
func (r *Runtime) storeInfo() (map[string]interface{}, error) {
	// lets say storage driver in use, number of images, number of containers
	info := map[string]interface{}{}
	info["GraphRoot"] = r.store.GraphRoot()
	info["RunRoot"] = r.store.RunRoot()
	info["GraphDriverName"] = r.store.GraphDriverName()
	graphOptions := map[string]interface{}{}
	for _, o := range r.store.GraphOptions() {
		split := strings.SplitN(o, "=", 2)
		if strings.HasSuffix(split[0], "mount_program") {
			version, err := programVersion(split[1])
			if err != nil {
				logrus.Warnf("Failed to retrieve program version for %s: %v", split[1], err)
			}
			program := map[string]interface{}{}
			program["Executable"] = split[1]
			program["Version"] = version
			program["Package"] = packageVersion(split[1])
			graphOptions[split[0]] = program
		} else {
			graphOptions[split[0]] = split[1]
		}
	}
	info["GraphOptions"] = graphOptions
	info["VolumePath"] = r.config.VolumePath

	configFile, err := storage.DefaultConfigFile(rootless.IsRootless())
	if err != nil {
		return nil, err
	}
	info["ConfigFile"] = configFile
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

// GetHostDistributionInfo returns a map containing the host's distribution and version
func (r *Runtime) GetHostDistributionInfo() map[string]string {
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
