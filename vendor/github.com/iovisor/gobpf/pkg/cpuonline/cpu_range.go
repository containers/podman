package cpuonline

import (
	"io/ioutil"
	"strconv"
	"strings"
)

const cpuOnline = "/sys/devices/system/cpu/online"

// loosely based on https://github.com/iovisor/bcc/blob/v0.3.0/src/python/bcc/utils.py#L15
func readCPURange(cpuRangeStr string) ([]uint, error) {
	var cpus []uint
	cpuRangeStr = strings.Trim(cpuRangeStr, "\n ")
	for _, cpuRange := range strings.Split(cpuRangeStr, ",") {
		rangeOp := strings.SplitN(cpuRange, "-", 2)
		first, err := strconv.ParseUint(rangeOp[0], 10, 32)
		if err != nil {
			return nil, err
		}
		if len(rangeOp) == 1 {
			cpus = append(cpus, uint(first))
			continue
		}
		last, err := strconv.ParseUint(rangeOp[1], 10, 32)
		if err != nil {
			return nil, err
		}
		for n := first; n <= last; n++ {
			cpus = append(cpus, uint(n))
		}
	}
	return cpus, nil
}

// Get returns a slice with the online CPUs, for example `[0, 2, 3]`
func Get() ([]uint, error) {
	buf, err := ioutil.ReadFile(cpuOnline)
	if err != nil {
		return nil, err
	}
	return readCPURange(string(buf))
}
