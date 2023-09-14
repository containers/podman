//go:build !remote
// +build !remote

package generate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Verify resource limits are sanely set when running on cgroup v1.
func verifyContainerResourcesCgroupV1(s *specgen.SpecGenerator) ([]string, error) {
	warnings := []string{}

	sysInfo := sysinfo.New(true)

	// If ResourceLimits is nil, return without warning
	resourceNil := &specgen.SpecGenerator{}
	resourceNil.ResourceLimits = &specs.LinuxResources{}
	if s.ResourceLimits == nil || reflect.DeepEqual(s.ResourceLimits, resourceNil.ResourceLimits) {
		return nil, nil
	}

	// Cgroups V1 rootless system does not support Resource limits
	if rootless.IsRootless() {
		s.ResourceLimits = nil
		return []string{"Resource limits are not supported and ignored on cgroups V1 rootless systems"}, nil
	}

	if s.ResourceLimits.Unified != nil {
		return nil, errors.New("cannot use --cgroup-conf without cgroup v2")
	}

	// Memory checks
	if s.ResourceLimits.Memory != nil {
		memory := s.ResourceLimits.Memory
		if memory.Limit != nil && !sysInfo.MemoryLimit {
			warnings = append(warnings, "Your kernel does not support memory limit capabilities or the cgroup is not mounted. Limitation discarded.")
			memory.Limit = nil
			memory.Swap = nil
		}
		if memory.Limit != nil && memory.Swap != nil && !sysInfo.SwapLimit {
			warnings = append(warnings, "Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.")
			memory.Swap = nil
		}
		if memory.Limit != nil && memory.Swap != nil && *memory.Swap < *memory.Limit {
			return warnings, errors.New("minimum memoryswap limit should be larger than memory limit, see usage")
		}
		if memory.Limit == nil && memory.Swap != nil {
			return warnings, errors.New("you should always set a memory limit when using a memoryswap limit, see usage")
		}
		if memory.Swappiness != nil {
			if !sysInfo.MemorySwappiness {
				warnings = append(warnings, "Your kernel does not support memory swappiness capabilities, or the cgroup is not mounted. Memory swappiness discarded.")
				memory.Swappiness = nil
			} else if *memory.Swappiness > 100 {
				return warnings, fmt.Errorf("invalid value: %v, valid memory swappiness range is 0-100", *memory.Swappiness)
			}
		}
		if memory.Reservation != nil && !sysInfo.MemoryReservation {
			warnings = append(warnings, "Your kernel does not support memory soft limit capabilities or the cgroup is not mounted. Limitation discarded.")
			memory.Reservation = nil
		}
		if memory.Limit != nil && memory.Reservation != nil && *memory.Limit < *memory.Reservation {
			return warnings, errors.New("minimum memory limit cannot be less than memory reservation limit, see usage")
		}
		if memory.DisableOOMKiller != nil && *memory.DisableOOMKiller && !sysInfo.OomKillDisable {
			warnings = append(warnings, "Your kernel does not support OomKillDisable. OomKillDisable discarded.")
			memory.DisableOOMKiller = nil
		}
	}

	// Pids checks
	if s.ResourceLimits.Pids != nil {
		// TODO: Should this be 0, or checking that ResourceLimits.Pids
		// is set at all?
		if s.ResourceLimits.Pids.Limit >= 0 && !sysInfo.PidsLimit {
			warnings = append(warnings, "Your kernel does not support pids limit capabilities or the cgroup is not mounted. PIDs limit discarded.")
			s.ResourceLimits.Pids = nil
		}
	}

	// CPU checks
	if s.ResourceLimits.CPU != nil {
		cpu := s.ResourceLimits.CPU
		if cpu.Shares != nil && !sysInfo.CPUShares {
			warnings = append(warnings, "Your kernel does not support CPU shares or the cgroup is not mounted. Shares discarded.")
			cpu.Shares = nil
		}
		if cpu.Period != nil && !sysInfo.CPUCfsPeriod {
			warnings = append(warnings, "Your kernel does not support CPU cfs period or the cgroup is not mounted. Period discarded.")
			cpu.Period = nil
		}
		if cpu.Period != nil && (*cpu.Period < 1000 || *cpu.Period > 1000000) {
			return warnings, errors.New("CPU cfs period cannot be less than 1ms (i.e. 1000) or larger than 1s (i.e. 1000000)")
		}
		if cpu.Quota != nil && !sysInfo.CPUCfsQuota {
			warnings = append(warnings, "Your kernel does not support CPU cfs quota or the cgroup is not mounted. Quota discarded.")
			cpu.Quota = nil
		}
		if cpu.Quota != nil && *cpu.Quota < 1000 {
			return warnings, errors.New("CPU cfs quota cannot be less than 1ms (i.e. 1000)")
		}
		if (cpu.Cpus != "" || cpu.Mems != "") && !sysInfo.Cpuset {
			warnings = append(warnings, "Your kernel does not support cpuset or the cgroup is not mounted. CPUset discarded.")
			cpu.Cpus = ""
			cpu.Mems = ""
		}

		cpusAvailable, err := sysInfo.IsCpusetCpusAvailable(cpu.Cpus)
		if err != nil {
			return warnings, fmt.Errorf("invalid value %s for cpuset cpus", cpu.Cpus)
		}
		if !cpusAvailable {
			return warnings, fmt.Errorf("requested CPUs are not available - requested %s, available: %s", cpu.Cpus, sysInfo.Cpus)
		}

		memsAvailable, err := sysInfo.IsCpusetMemsAvailable(cpu.Mems)
		if err != nil {
			return warnings, fmt.Errorf("invalid value %s for cpuset mems", cpu.Mems)
		}
		if !memsAvailable {
			return warnings, fmt.Errorf("requested memory nodes are not available - requested %s, available: %s", cpu.Mems, sysInfo.Mems)
		}
	}

	// Blkio checks
	if s.ResourceLimits.BlockIO != nil {
		blkio := s.ResourceLimits.BlockIO
		if blkio.Weight != nil && !sysInfo.BlkioWeight {
			warnings = append(warnings, "Your kernel does not support Block I/O weight or the cgroup is not mounted. Weight discarded.")
			blkio.Weight = nil
		}
		if blkio.Weight != nil && (*blkio.Weight > 1000 || *blkio.Weight < 10) {
			return warnings, errors.New("range of blkio weight is from 10 to 1000")
		}
		if len(blkio.WeightDevice) > 0 && !sysInfo.BlkioWeightDevice {
			warnings = append(warnings, "Your kernel does not support Block I/O weight_device or the cgroup is not mounted. Weight-device discarded.")
			blkio.WeightDevice = nil
		}
		if len(blkio.ThrottleReadBpsDevice) > 0 && !sysInfo.BlkioReadBpsDevice {
			warnings = append(warnings, "Your kernel does not support BPS Block I/O read limit or the cgroup is not mounted. Block I/O BPS read limit discarded")
			blkio.ThrottleReadBpsDevice = nil
		}
		if len(blkio.ThrottleWriteBpsDevice) > 0 && !sysInfo.BlkioWriteBpsDevice {
			warnings = append(warnings, "Your kernel does not support BPS Block I/O write limit or the cgroup is not mounted. Block I/O BPS write limit discarded.")
			blkio.ThrottleWriteBpsDevice = nil
		}
		if len(blkio.ThrottleReadIOPSDevice) > 0 && !sysInfo.BlkioReadIOpsDevice {
			warnings = append(warnings, "Your kernel does not support IOPS Block read limit or the cgroup is not mounted. Block I/O IOPS read limit discarded.")
			blkio.ThrottleReadIOPSDevice = nil
		}
		if len(blkio.ThrottleWriteIOPSDevice) > 0 && !sysInfo.BlkioWriteIOpsDevice {
			warnings = append(warnings, "Your kernel does not support IOPS Block I/O write limit or the cgroup is not mounted. Block I/O IOPS write limit discarded.")
			blkio.ThrottleWriteIOPSDevice = nil
		}
	}

	return warnings, nil
}

// Verify resource limits are sanely set when running on cgroup v2.
func verifyContainerResourcesCgroupV2(s *specgen.SpecGenerator) ([]string, error) {
	warnings := []string{}

	if s.ResourceLimits == nil {
		return warnings, nil
	}

	// Memory checks
	if s.ResourceLimits.Memory != nil && s.ResourceLimits.Memory.Swap != nil {
		own, err := utils.GetOwnCgroup()
		if err != nil {
			return warnings, err
		}

		if own == "/" {
			// If running under the root cgroup try to create or reuse a "probe" cgroup to read memory values
			own = "podman_probe"
			_ = os.MkdirAll(filepath.Join("/sys/fs/cgroup", own), 0o755)
			_ = os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+memory"), 0o644)
		}

		memoryMax := filepath.Join("/sys/fs/cgroup", own, "memory.max")
		memorySwapMax := filepath.Join("/sys/fs/cgroup", own, "memory.swap.max")
		_, errMemoryMax := os.Stat(memoryMax)
		_, errMemorySwapMax := os.Stat(memorySwapMax)
		// Differently than cgroup v1, the memory.*max files are not present in the
		// root directory, so we cannot query directly that, so as best effort use
		// the current cgroup.
		// Check whether memory.max exists in the current cgroup and memory.swap.max
		//  does not.  In this case we can be sure memory swap is not enabled.
		// If both files don't exist, the memory controller might not be enabled
		// for the current cgroup.
		if errMemoryMax == nil && errMemorySwapMax != nil {
			warnings = append(warnings, "Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.")
			s.ResourceLimits.Memory.Swap = nil
		}
	}

	// CPU checks
	if s.ResourceLimits.CPU != nil {
		cpu := s.ResourceLimits.CPU
		if cpu.RealtimePeriod != nil {
			warnings = append(warnings, "Realtime period not supported on cgroups V2 systems")
			cpu.RealtimePeriod = nil
		}
		if cpu.RealtimeRuntime != nil {
			warnings = append(warnings, "Realtime runtime not supported on cgroups V2 systems")
			cpu.RealtimeRuntime = nil
		}
	}
	return warnings, nil
}

// Verify resource limits are sanely set, removing any limits that are not
// possible with the current cgroups config.
func verifyContainerResources(s *specgen.SpecGenerator) ([]string, error) {
	cgroup2, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return []string{}, err
	}
	if cgroup2 {
		return verifyContainerResourcesCgroupV2(s)
	}
	return verifyContainerResourcesCgroupV1(s)
}
