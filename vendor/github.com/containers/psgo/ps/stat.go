package ps

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

/*
#include <unistd.h>
*/
import "C"

// getClockTicks returns sysconf(SC_CLK_TCK).
func getClockTicks() int64 {
	return int64(C.sysconf(C._SC_CLK_TCK))
}

// bootTime parses /proc/uptime returns the time.Time of system boot.
func getBootTime() (int64, error) {
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

	return btimeSec, nil
}

// stat is a direct translation of a `/proc/[pid]/stat` file as described in
// the proc(5) manpage. Please note that it is not a full translation as not
// all fields are in the scope of this library and higher indices are
// Kernel-version dependent.
type stat struct {
	// (1) The process ID
	pid string
	// (2) The filename of the executable, in parentheses. This is visible
	// whether or not the executable is swapped out.
	comm string
	// (3) The process state (e.g., running, sleeping, zombie, dead).
	// Refer to proc(5) for further deatils.
	state string
	// (4) The PID of the parent of this process.
	ppid string
	// (5) The process group ID of the process.
	pgrp string
	// (6) The session ID of the process.
	session string
	// (7) The controlling terminal of the process. (The minor device
	// number is contained in the combination of bits 31 to 20 and 7 to 0;
	// the major device number is in bits 15 to 8.)
	ttyNr string
	// (8) The ID of the foreground process group of the controlling
	// terminal of the process.
	tpgid string
	// (9) The kernel flags word of the process. For bit meanings, see the
	// PF_* defines in the Linux kernel source file
	// include/linux/sched.h. Details depend on the kernel version.
	flags string
	// (10) The number of minor faults the process has made which have not
	// required loading a memory page from disk.
	minflt string
	// (11) The number of minor faults that the process's waited-for
	// children have made.
	cminflt string
	// (12) The number of major faults the process has made which have
	// required loading a memory page from disk.
	majflt string
	// (13) The number of major faults that the process's waited-for
	// children have made.
	cmajflt string
	// (14) Amount of time that this process has been scheduled in user
	// mode, measured in clock ticks (divide by
	// sysconf(_SC_CLK_TCK)). This includes guest time, guest_time
	// (time spent running a virtual CPU, see below), so that applications
	// that are not aware of the guest time field do not lose that time
	// from their calculations.
	utime string
	// (15) Amount of time that this process has been scheduled in kernel
	// mode, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	stime string
	// (16) Amount of time that this process's waited-for children have
	// been scheduled in user mode, measured in clock ticks (divide by
	// sysconf(_SC_CLK_TCK)). (See also times(2).) This includes guest
	// time, cguest_time (time spent running a virtual CPU, see below).
	cutime string
	// (17) Amount of time that this process's waited-for children have
	// been scheduled in kernel mode, measured in clock ticks (divide by
	// sysconf(_SC_CLK_TCK)).
	cstime string
	// (18) (Explanation for Linux 2.6+) For processes running a real-time
	// scheduling policy (policy below; see sched_setscheduler(2)), this is
	// the negated scheduling pri- ority, minus one; that is, a number
	// in the range -2 to -100, corresponding to real-time priorities 1 to
	// 99. For processes running under a non-real-time scheduling
	// policy, this is the raw nice value (setpriority(2)) as represented
	// in the kernel. The kernel stores nice values as numbers in the
	// range 0 (high) to 39 (low), corresponding to the user-visible nice
	// range of -20 to 19.
	priority string
	// (19) The nice value (see setpriority(2)), a value in the range 19
	// (low priority) to -20 (high priority).
	nice string
	// (20) Number of threads in this process (since Linux 2.6). Before
	// kernel 2.6, this field was hard coded to 0 as a placeholder for an
	// earlier removed field.
	numThreads string
	// (21) The time in jiffies before the next SIGALRM is sent to the
	// process due to an interval timer. Since kernel 2.6.17, this
	// field is no longer maintained, and is hard coded as 0.
	itrealvalue string
	// (22) The time the process started after system boot. In kernels
	// before Linux 2.6, this value was expressed in jiffies. Since
	// Linux 2.6, the value is expressed in clock ticks (divide by
	// sysconf(_SC_CLK_TCK)).
	starttime string
	// (23) Virtual memory size in bytes.
	vsize string
}

// readStat is used for mocking in unit tests.
var readStat = func(path string) ([]string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = errNoSuchPID
		}
		return nil, err
	}

	return strings.Fields(string(data)), nil
}

// parseStat parses the /proc/$pid/stat file and returns a stat.
func parseStat(path string) (*stat, error) {
	fields, err := readStat(path)
	if err != nil {
		return nil, err
	}

	fieldAt := func(i int) string {
		return fields[i-1]
	}

	return &stat{
		pid:         fieldAt(1),
		comm:        fieldAt(2),
		state:       fieldAt(3),
		ppid:        fieldAt(4),
		pgrp:        fieldAt(5),
		session:     fieldAt(6),
		ttyNr:       fieldAt(7),
		tpgid:       fieldAt(8),
		flags:       fieldAt(9),
		minflt:      fieldAt(10),
		cminflt:     fieldAt(11),
		majflt:      fieldAt(12),
		cmajflt:     fieldAt(13),
		utime:       fieldAt(14),
		stime:       fieldAt(15),
		cutime:      fieldAt(16),
		cstime:      fieldAt(17),
		priority:    fieldAt(18),
		nice:        fieldAt(19),
		numThreads:  fieldAt(20),
		itrealvalue: fieldAt(21),
		starttime:   fieldAt(22),
		vsize:       fieldAt(23),
	}, nil
}
