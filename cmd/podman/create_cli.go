package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/sysinfo"
	"github.com/pkg/errors"
	cc "github.com/projectatomic/libpod/pkg/spec"
	"github.com/sirupsen/logrus"
)

const (
	// It's not kernel limit, we want this 4M limit to supply a reasonable functional container
	linuxMinMemory = 4194304
)

func getAllLabels(labelFile, inputLabels []string) (map[string]string, error) {
	labels := make(map[string]string)
	labelErr := readKVStrings(labels, labelFile, inputLabels)
	if labelErr != nil {
		return labels, errors.Wrapf(labelErr, "unable to process labels from --label and label-file")
	}
	return labels, nil
}

// validateSysctl validates a sysctl and returns it.
func validateSysctl(strSlice []string) (map[string]string, error) {
	sysctl := make(map[string]string)
	validSysctlMap := map[string]bool{
		"kernel.msgmax":          true,
		"kernel.msgmnb":          true,
		"kernel.msgmni":          true,
		"kernel.sem":             true,
		"kernel.shmall":          true,
		"kernel.shmmax":          true,
		"kernel.shmmni":          true,
		"kernel.shm_rmid_forced": true,
	}
	validSysctlPrefixes := []string{
		"net.",
		"fs.mqueue.",
	}

	for _, val := range strSlice {
		foundMatch := false
		arr := strings.Split(val, "=")
		if len(arr) < 2 {
			return nil, errors.Errorf("%s is invalid, sysctl values must be in the form of KEY=VALUE", val)
		}
		if validSysctlMap[arr[0]] {
			sysctl[arr[0]] = arr[1]
			continue
		}

		for _, prefix := range validSysctlPrefixes {
			if strings.HasPrefix(arr[0], prefix) {
				sysctl[arr[0]] = arr[1]
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			return nil, errors.Errorf("sysctl '%s' is not whitelisted", arr[0])
		}
	}
	return sysctl, nil
}

func addWarning(warnings []string, msg string) []string {
	logrus.Warn(msg)
	return append(warnings, msg)
}

func parseVolumes(volumes []string) error {
	for _, volume := range volumes {
		arr := strings.SplitN(volume, ":", 3)
		if len(arr) < 2 {
			return errors.Errorf("incorrect volume format %q, should be host-dir:ctr-dir:[option]", volume)
		}
		if err := validateVolumeHostDir(arr[0]); err != nil {
			return err
		}
		if err := validateVolumeCtrDir(arr[1]); err != nil {
			return err
		}
		if len(arr) > 2 {
			if err := validateVolumeOpts(arr[2]); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseVolumesFrom(volumesFrom []string) error {
	for _, vol := range volumesFrom {
		arr := strings.SplitN(vol, ":", 2)
		if len(arr) == 2 {
			if strings.Contains(arr[1], "Z") || strings.Contains(arr[1], "private") || strings.Contains(arr[1], "slave") || strings.Contains(arr[1], "shared") {
				return errors.Errorf("invalid options %q, can only specify 'ro', 'rw', and 'z", arr[1])
			}
			if err := validateVolumeOpts(arr[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateVolumeHostDir(hostDir string) error {
	if !filepath.IsAbs(hostDir) {
		return errors.Errorf("invalid host path, must be an absolute path %q", hostDir)
	}
	if _, err := os.Stat(hostDir); err != nil {
		return errors.Wrapf(err, "error checking path %q", hostDir)
	}
	return nil
}

func validateVolumeCtrDir(ctrDir string) error {
	if !filepath.IsAbs(ctrDir) {
		return errors.Errorf("invalid container path, must be an absolute path %q", ctrDir)
	}
	return nil
}

func validateVolumeOpts(option string) error {
	var foundRootPropagation, foundRWRO, foundLabelChange int
	options := strings.Split(option, ",")
	for _, opt := range options {
		switch opt {
		case "rw", "ro":
			foundRWRO++
			if foundRWRO > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'rw' or 'ro' option", option)
			}
		case "z", "Z":
			foundLabelChange++
			if foundLabelChange > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'z' or 'Z' option", option)
			}
		case "private", "rprivate", "shared", "rshared", "slave", "rslave":
			foundRootPropagation++
			if foundRootPropagation > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 '[r]shared', '[r]private' or '[r]slave' option", option)
			}
		default:
			return errors.Errorf("invalid option type %q", option)
		}
	}
	return nil
}

func verifyContainerResources(config *cc.CreateConfig, update bool) ([]string, error) {
	warnings := []string{}
	sysInfo := sysinfo.New(true)

	// memory subsystem checks and adjustments
	if config.Resources.Memory != 0 && config.Resources.Memory < linuxMinMemory {
		return warnings, fmt.Errorf("minimum memory limit allowed is 4MB")
	}
	if config.Resources.Memory > 0 && !sysInfo.MemoryLimit {
		warnings = addWarning(warnings, "Your kernel does not support memory limit capabilities or the cgroup is not mounted. Limitation discarded.")
		config.Resources.Memory = 0
		config.Resources.MemorySwap = -1
	}
	if config.Resources.Memory > 0 && config.Resources.MemorySwap != -1 && !sysInfo.SwapLimit {
		warnings = addWarning(warnings, "Your kernel does not support swap limit capabilities,or the cgroup is not mounted. Memory limited without swap.")
		config.Resources.MemorySwap = -1
	}
	if config.Resources.Memory > 0 && config.Resources.MemorySwap > 0 && config.Resources.MemorySwap < config.Resources.Memory {
		return warnings, fmt.Errorf("minimum memoryswap limit should be larger than memory limit, see usage")
	}
	if config.Resources.Memory == 0 && config.Resources.MemorySwap > 0 && !update {
		return warnings, fmt.Errorf("you should always set the memory limit when using memoryswap limit, see usage")
	}
	if config.Resources.MemorySwappiness != -1 {
		if !sysInfo.MemorySwappiness {
			msg := "Your kernel does not support memory swappiness capabilities, or the cgroup is not mounted. Memory swappiness discarded."
			warnings = addWarning(warnings, msg)
			config.Resources.MemorySwappiness = -1
		} else {
			swappiness := config.Resources.MemorySwappiness
			if swappiness < -1 || swappiness > 100 {
				return warnings, fmt.Errorf("invalid value: %v, valid memory swappiness range is 0-100", swappiness)
			}
		}
	}
	if config.Resources.MemoryReservation > 0 && !sysInfo.MemoryReservation {
		warnings = addWarning(warnings, "Your kernel does not support memory soft limit capabilities or the cgroup is not mounted. Limitation discarded.")
		config.Resources.MemoryReservation = 0
	}
	if config.Resources.MemoryReservation > 0 && config.Resources.MemoryReservation < linuxMinMemory {
		return warnings, fmt.Errorf("minimum memory reservation allowed is 4MB")
	}
	if config.Resources.Memory > 0 && config.Resources.MemoryReservation > 0 && config.Resources.Memory < config.Resources.MemoryReservation {
		return warnings, fmt.Errorf("minimum memory limit can not be less than memory reservation limit, see usage")
	}
	if config.Resources.KernelMemory > 0 && !sysInfo.KernelMemory {
		warnings = addWarning(warnings, "Your kernel does not support kernel memory limit capabilities or the cgroup is not mounted. Limitation discarded.")
		config.Resources.KernelMemory = 0
	}
	if config.Resources.KernelMemory > 0 && config.Resources.KernelMemory < linuxMinMemory {
		return warnings, fmt.Errorf("minimum kernel memory limit allowed is 4MB")
	}
	if config.Resources.DisableOomKiller == true && !sysInfo.OomKillDisable {
		// only produce warnings if the setting wasn't to *disable* the OOM Kill; no point
		// warning the caller if they already wanted the feature to be off
		warnings = addWarning(warnings, "Your kernel does not support OomKillDisable. OomKillDisable discarded.")
		config.Resources.DisableOomKiller = false
	}

	if config.Resources.PidsLimit != 0 && !sysInfo.PidsLimit {
		warnings = addWarning(warnings, "Your kernel does not support pids limit capabilities or the cgroup is not mounted. PIDs limit discarded.")
		config.Resources.PidsLimit = 0
	}

	if config.Resources.CPUShares > 0 && !sysInfo.CPUShares {
		warnings = addWarning(warnings, "Your kernel does not support CPU shares or the cgroup is not mounted. Shares discarded.")
		config.Resources.CPUShares = 0
	}
	if config.Resources.CPUPeriod > 0 && !sysInfo.CPUCfsPeriod {
		warnings = addWarning(warnings, "Your kernel does not support CPU cfs period or the cgroup is not mounted. Period discarded.")
		config.Resources.CPUPeriod = 0
	}
	if config.Resources.CPUPeriod != 0 && (config.Resources.CPUPeriod < 1000 || config.Resources.CPUPeriod > 1000000) {
		return warnings, fmt.Errorf("CPU cfs period can not be less than 1ms (i.e. 1000) or larger than 1s (i.e. 1000000)")
	}
	if config.Resources.CPUQuota > 0 && !sysInfo.CPUCfsQuota {
		warnings = addWarning(warnings, "Your kernel does not support CPU cfs quota or the cgroup is not mounted. Quota discarded.")
		config.Resources.CPUQuota = 0
	}
	if config.Resources.CPUQuota > 0 && config.Resources.CPUQuota < 1000 {
		return warnings, fmt.Errorf("CPU cfs quota can not be less than 1ms (i.e. 1000)")
	}
	// cpuset subsystem checks and adjustments
	if (config.Resources.CPUsetCPUs != "" || config.Resources.CPUsetMems != "") && !sysInfo.Cpuset {
		warnings = addWarning(warnings, "Your kernel does not support cpuset or the cgroup is not mounted. CPUset discarded.")
		config.Resources.CPUsetCPUs = ""
		config.Resources.CPUsetMems = ""
	}
	cpusAvailable, err := sysInfo.IsCpusetCpusAvailable(config.Resources.CPUsetCPUs)
	if err != nil {
		return warnings, fmt.Errorf("invalid value %s for cpuset cpus", config.Resources.CPUsetCPUs)
	}
	if !cpusAvailable {
		return warnings, fmt.Errorf("requested CPUs are not available - requested %s, available: %s", config.Resources.CPUsetCPUs, sysInfo.Cpus)
	}
	memsAvailable, err := sysInfo.IsCpusetMemsAvailable(config.Resources.CPUsetMems)
	if err != nil {
		return warnings, fmt.Errorf("invalid value %s for cpuset mems", config.Resources.CPUsetMems)
	}
	if !memsAvailable {
		return warnings, fmt.Errorf("requested memory nodes are not available - requested %s, available: %s", config.Resources.CPUsetMems, sysInfo.Mems)
	}

	// blkio subsystem checks and adjustments
	if config.Resources.BlkioWeight > 0 && !sysInfo.BlkioWeight {
		warnings = addWarning(warnings, "Your kernel does not support Block I/O weight or the cgroup is not mounted. Weight discarded.")
		config.Resources.BlkioWeight = 0
	}
	if config.Resources.BlkioWeight > 0 && (config.Resources.BlkioWeight < 10 || config.Resources.BlkioWeight > 1000) {
		return warnings, fmt.Errorf("range of blkio weight is from 10 to 1000")
	}
	if len(config.Resources.BlkioWeightDevice) > 0 && !sysInfo.BlkioWeightDevice {
		warnings = addWarning(warnings, "Your kernel does not support Block I/O weight_device or the cgroup is not mounted. Weight-device discarded.")
		config.Resources.BlkioWeightDevice = []string{}
	}
	if len(config.Resources.DeviceReadBps) > 0 && !sysInfo.BlkioReadBpsDevice {
		warnings = addWarning(warnings, "Your kernel does not support BPS Block I/O read limit or the cgroup is not mounted. Block I/O BPS read limit discarded")
		config.Resources.DeviceReadBps = []string{}
	}
	if len(config.Resources.DeviceWriteBps) > 0 && !sysInfo.BlkioWriteBpsDevice {
		warnings = addWarning(warnings, "Your kernel does not support BPS Block I/O write limit or the cgroup is not mounted. Block I/O BPS write limit discarded.")
		config.Resources.DeviceWriteBps = []string{}
	}
	if len(config.Resources.DeviceReadIOps) > 0 && !sysInfo.BlkioReadIOpsDevice {
		warnings = addWarning(warnings, "Your kernel does not support IOPS Block read limit or the cgroup is not mounted. Block I/O IOPS read limit discarded.")
		config.Resources.DeviceReadIOps = []string{}
	}
	if len(config.Resources.DeviceWriteIOps) > 0 && !sysInfo.BlkioWriteIOpsDevice {
		warnings = addWarning(warnings, "Your kernel does not support IOPS Block I/O write limit or the cgroup is not mounted. Block I/O IOPS write limit discarded.")
		config.Resources.DeviceWriteIOps = []string{}
	}

	return warnings, nil
}
