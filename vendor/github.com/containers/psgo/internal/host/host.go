// Package host extracts data from the host, such as the system's boot time or
// the tick rate of the system clock.
package host

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/*
#include <unistd.h>
*/
import "C"

var (
	// cache host queries to redundant calculations
	clockTicks *int64
	bootTime   *int64
)

// ClockTicks returns sysconf(SC_CLK_TCK).
func ClockTicks() int64 {
	if clockTicks == nil {
		ticks := int64(C.sysconf(C._SC_CLK_TCK))
		clockTicks = &ticks
	}
	return *clockTicks
}

// BootTime parses /proc/uptime returns the boot time in seconds since the
// Epoch, 1970-01-01 00:00:00 +0000 (UTC).
func BootTime() (int64, error) {
	if bootTime != nil {
		return *bootTime, nil
	}

	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}

	btimeStr := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "btime" {
			btimeStr = fields[1]
		}
	}

	if len(btimeStr) == 0 {
		return 0, fmt.Errorf("couldn't extract boot time from /proc/stat")
	}

	btimeSec, err := strconv.ParseInt(btimeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing boot time from /proc/stat: %s", err)
	}
	bootTime = &btimeSec
	return btimeSec, nil
}
