package ps

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// DefaultFormat is the `ps -ef` compatible default format.
const DefaultFormat = "user,pid,ppid,pcpu,etime,tty,time,comm"

var (
	// ErrUnkownDescriptor is returned when an unknown descriptor is parsed.
	ErrUnkownDescriptor = errors.New("unknown descriptor")

	// errNoSuchPID is returned when `/proc/PID` does not exist (anymore).
	errNoSuchPID = errors.New("PID does not exist in /proc")

	// bootTime holds the host's boot time. Singleton to safe some time and
	// energy.
	bootTime int64

	// clockTicks is the value of sysconf(SC_CLK_TCK)
	clockTicks = getClockTicks()

	// ttyDevices is a slice of ttys. Singledton to safe some time and
	// energy.
	ttyDevices []*tty

	descriptors = []aixFormatDescriptor{
		{
			code:   "%C",
			normal: "pcpu",
			header: "%CPU",
			procFn: processPCPU,
		},
		{
			code:   "%G",
			normal: "group",
			header: "GROUP",
			procFn: processGROUP,
		},
		{
			code:   "%P",
			normal: "ppid",
			header: "PPID",
			procFn: processPPID,
		},
		{
			code:   "%U",
			normal: "user",
			header: "USER",
			procFn: processUSER,
		},
		{
			code:   "%a",
			normal: "args",
			header: "COMMAND",
			procFn: processARGS,
		},
		{
			code:   "%c",
			normal: "comm",
			header: "COMMAND",
			procFn: processCOMM,
		},
		{
			code:   "%g",
			normal: "rgroup",
			header: "RGROUP",
			procFn: processRGROUP,
		},
		{
			code:   "%n",
			normal: "nice",
			header: "NI",
			procFn: processNICE,
		},
		{
			code:   "%p",
			normal: "pid",
			header: "PID",
			procFn: processPID,
		},
		{
			code:   "%r",
			normal: "pgid",
			header: "PGID",
			procFn: processPGID,
		},
		{
			code:   "%t",
			normal: "etime",
			header: "ELAPSED",
			procFn: processETIME,
		},
		{
			code:   "%u",
			normal: "ruser",
			header: "RUSER",
			procFn: processRUSER,
		},
		{
			code:   "%x",
			normal: "time",
			header: "TIME",
			procFn: processTIME,
		},
		{
			code:   "%y",
			normal: "tty",
			header: "TTY",
			procFn: processTTY,
		},
		{
			code:   "%z",
			normal: "vsz",
			header: "VSZ",
			procFn: processVSZ,
		},
		{
			normal: "capinh",
			header: "CAPABILITIES",
			procFn: processCAPINH,
		},
		{
			normal: "capprm",
			header: "CAPABILITIES",
			procFn: processCAPPRM,
		},
		{
			normal: "capeff",
			header: "CAPABILITIES",
			procFn: processCAPEFF,
		},
		{
			normal: "capbnd",
			header: "CAPABILITIES",
			procFn: processCAPBND,
		},
		{
			normal: "seccomp",
			header: "SECCOMP",
			procFn: processSECCOMP,
		},
		{
			normal: "label",
			header: "LABEL",
			procFn: processLABEL,
		},
	}
)

// process includes a process ID and the corresponding data from /proc/pid/stat,
// /proc/pid/status and from /prod/pid/cmdline.
type process struct {
	pid     int
	pstat   *stat
	pstatus *status
	cmdline []string
}

// elapsedTime returns the time.Duration since process p was created.
func (p *process) elapsedTime() (time.Duration, error) {
	sinceBoot, err := strconv.ParseInt(p.pstat.starttime, 10, 64)
	if err != nil {
		return 0, err
	}
	sinceBoot = sinceBoot / clockTicks

	if bootTime == 0 {
		bootTime, err = getBootTime()
		if err != nil {
			return 0, err
		}
	}
	created := time.Unix(sinceBoot+bootTime, 0)
	return (time.Now()).Sub(created), nil
}

// cpuTime returns the cumlative CPU time of process p as a time.Duration.
func (p *process) cpuTime() (time.Duration, error) {
	user, err := strconv.ParseInt(p.pstat.utime, 10, 64)
	if err != nil {
		return 0, err
	}
	system, err := strconv.ParseInt(p.pstat.stime, 10, 64)
	if err != nil {
		return 0, err
	}
	secs := (user + system) / clockTicks
	cpu := time.Unix(secs, 0)
	return cpu.Sub(time.Unix(0, 0)), nil
}

// processes returns a process slice of processes mentioned in /proc.
func processes() ([]*process, error) {
	pids, err := getPIDs()
	if err != nil {
		panic(err)
	}

	processes := []*process{}
	for _, pid := range pids {
		var (
			err error
			p   process
		)
		p.pid = pid
		p.pstat, err = parseStat(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			if err == errNoSuchPID {
				continue
			}
			return nil, err
		}
		p.pstatus, err = parseStatus(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			if err == errNoSuchPID {
				continue
			}
			return nil, err
		}
		p.cmdline, err = parseCmdline(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			if err == errNoSuchPID {
				continue
			}
			return nil, err
		}
		processes = append(processes, &p)
	}

	return processes, nil
}

// getPIDs extracts and returns all PIDs from /proc.
func getPIDs() ([]int, error) {
	procDir, err := os.Open("/proc/")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	// extract string slice of all directories in procDir
	pidDirs, err := procDir.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	// convert pidDirs to int
	pids := []int{}
	for _, pidDir := range pidDirs {
		pid, err := strconv.Atoi(pidDir)
		if err != nil {
			// skip non-numerical entries (e.g., `/proc/softirqs`)
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

// processFunc is used to map a given aixFormatDescriptor to a corresponding
// function extracting the desired data from a process.
type processFunc func(*process) (string, error)

// aixFormatDescriptor as mentioned in the ps(1) manpage.  A given descriptor
// can either be specified via its code (e.g., "%C") or its normal representation
// (e.g., "pcpu") and will be printed under its corresponding header (e.g, "%CPU").
type aixFormatDescriptor struct {
	code   string
	normal string
	header string
	procFn processFunc
}

// processDescriptors calls each `procFn` of all formatDescriptors on each
// process and returns an array of tab-separated strings.
func processDescriptors(formatDescriptors []aixFormatDescriptor, processes []*process) ([]string, error) {
	data := []string{}
	// create header
	headerArr := []string{}
	for _, desc := range formatDescriptors {
		headerArr = append(headerArr, desc.header)
	}
	data = append(data, strings.Join(headerArr, "\t"))

	// dispatch all descriptor functions on each process
	for _, proc := range processes {
		pData := []string{}
		for _, desc := range formatDescriptors {
			dataStr, err := desc.procFn(proc)
			if err != nil {
				return nil, err
			}
			pData = append(pData, dataStr)
		}
		data = append(data, strings.Join(pData, "\t"))
	}

	return data, nil
}

// ListDescriptors returns a string slice of all supported AIX format
// descriptors in the normal form.
func ListDescriptors() (list []string) {
	for _, d := range descriptors {
		list = append(list, d.normal)
	}
	return
}

// JoinNamespaceAndProcessInfo has the same semantics as ProcessInfo but joins
// the mount namespace of the specified pid before extracting data from `/proc`.
func JoinNamespaceAndProcessInfo(pid, format string) ([]string, error) {
	var (
		data    []string
		dataErr error
		wg      sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		runtime.LockOSThread()

		fd, err := os.Open(fmt.Sprintf("/proc/%s/ns/mnt", pid))
		if err != nil {
			dataErr = err
			return
		}
		defer fd.Close()

		// create a new mountns on the current thread
		if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
			dataErr = err
			return
		}
		unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS)
		data, dataErr = ProcessInfo(format)
	}()
	wg.Wait()

	return data, dataErr
}

// ProcessInfo returns the process information of all processes in the current
// mount namespace. The input format must be a comma-separated list of
// supported AIX format descriptors.  If the input string is empty, the
// DefaultFormat is used.
// The return value is an array of tab-separated strings, to easily use the
// output for column-based formatting (e.g., with the `text/tabwriter` package).
func ProcessInfo(format string) ([]string, error) {
	if len(format) == 0 {
		format = DefaultFormat
	}

	formatDescriptors, err := parseDescriptors(format)
	if err != nil {
		return nil, err
	}

	processes, err := processes()
	if err != nil {
		return nil, err
	}

	return processDescriptors(formatDescriptors, processes)
}

// parseDescriptors parses the input string and returns a correspodning array
// of aixFormatDescriptors, which are expected to be separated by commas.
// The input format is "desc1, desc2, ..., desN" where a given descriptor can be
// specified both, in the code and in the normal form.  A concrete example is
// "pid, %C, nice, %a".
func parseDescriptors(input string) ([]aixFormatDescriptor, error) {
	formatDescriptors := []aixFormatDescriptor{}
	for _, s := range strings.Split(input, ",") {
		s = strings.TrimSpace(s)
		found := false
		for _, d := range descriptors {
			if s == d.code || s == d.normal {
				formatDescriptors = append(formatDescriptors, d)
				found = true
			}
		}
		if !found {
			return nil, errors.Wrapf(ErrUnkownDescriptor, "'%s'", s)
		}
	}
	return formatDescriptors, nil
}

// lookupGID returns the textual group ID, if it can be optained, or the
// decimal input representation otherwise.
func lookupGID(gid string) (string, error) {
	gidNum, err := strconv.Atoi(gid)
	if err != nil {
		return "", errors.Wrap(err, "error parsing group ID")
	}
	g, err := user.LookupGid(gidNum)
	if err != nil {
		return gid, nil
	}
	return g.Name, nil
}

// processGROUP returns the effective group ID of the process.  This will be
// the textual group ID, if it can be optained, or a decimal representation
// otherwise.
func processGROUP(p *process) (string, error) {
	return lookupGID(p.pstatus.gids[1])
}

// processRGROUP returns the real group ID of the process.  This will be
// the textual group ID, if it can be optained, or a decimal representation
// otherwise.
func processRGROUP(p *process) (string, error) {
	return lookupGID(p.pstatus.gids[0])
}

// processPPID returns the parent process ID of process p.
func processPPID(p *process) (string, error) {
	return p.pstatus.pPid, nil
}

// lookupUID return the textual user ID, if it can be optained, or the decimal
// input representation otherwise.
func lookupUID(uid string) (string, error) {
	uidNum, err := strconv.Atoi(uid)
	if err != nil {
		return "", errors.Wrap(err, "error parsing user ID")
	}
	u, err := user.LookupUid(uidNum)
	if err != nil {
		return uid, nil
	}
	return u.Name, nil

}

// processUSER returns the effective user name of the process.  This will be
// the textual group ID, if it can be optained, or a decimal representation
// otherwise.
func processUSER(p *process) (string, error) {
	return lookupUID(p.pstatus.uids[1])
}

// processRUSER returns the effective user name of the process.  This will be
// the textual group ID, if it can be optained, or a decimal representation
// otherwise.
func processRUSER(p *process) (string, error) {
	return lookupUID(p.pstatus.uids[0])
}

// processName returns the name of process p in the format "[$name]".
func processName(p *process) (string, error) {
	return fmt.Sprintf("[%s]", p.pstatus.name), nil
}

// processARGS returns the command of p with all its arguments.
func processARGS(p *process) (string, error) {
	args := p.cmdline
	// ps (1) returns "[$name]" if command/args are empty
	if len(args) == 0 {
		return processName(p)
	}
	return strings.Join(args, " "), nil
}

// processCOMM returns the command name (i.e., executable name) of process p.
func processCOMM(p *process) (string, error) {
	args := p.cmdline
	// ps (1) returns "[$name]" if command/args are empty
	if len(args) == 0 {
		return processName(p)
	}
	spl := strings.Split(args[0], "/")
	return spl[len(spl)-1], nil
}

// processNICE returns the nice value of process p.
func processNICE(p *process) (string, error) {
	return p.pstat.nice, nil
}

// processPID returns the process ID of process p.
func processPID(p *process) (string, error) {
	return p.pstatus.pid, nil
}

// processPGID returns the process group ID of process p.
func processPGID(p *process) (string, error) {
	return p.pstat.pgrp, nil
}

// processPCPU returns how many percent of the CPU time process p uses as
// a three digit float as string.
func processPCPU(p *process) (string, error) {
	elapsed, err := p.elapsedTime()
	if err != nil {
		return "", err
	}
	cpu, err := p.cpuTime()
	if err != nil {
		return "", err
	}
	pcpu := 100 * cpu.Seconds() / elapsed.Seconds()

	return strconv.FormatFloat(pcpu, 'f', 3, 64), nil
}

// processETIME returns the elapsed time since the process was started.
func processETIME(p *process) (string, error) {
	elapsed, err := p.elapsedTime()
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%v", elapsed), nil
}

// processTIME returns the cumulative CPU time of process p.
func processTIME(p *process) (string, error) {
	cpu, err := p.cpuTime()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", cpu), nil
}

// processTTY returns the controlling tty (terminal) of process p.
func processTTY(p *process) (string, error) {
	ttyNr, err := strconv.ParseUint(p.pstat.ttyNr, 10, 64)
	if err != nil {
		return "", nil
	}

	maj, min := ttyNrToDev(ttyNr)
	t, err := findTTY(maj, min)
	if err != nil {
		return "", err
	}

	ttyS := "?"
	if t != nil {
		ttyS = strings.TrimPrefix(t.device, "/dev/")
	}
	return ttyS, nil
}

// processVSZ returns the virtual memory size of process p in KiB (1024-byte
// units).
func processVSZ(p *process) (string, error) {
	vmsize, err := strconv.Atoi(p.pstat.vsize)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", vmsize/1024), nil
}

// parseCAP parses cap (a string bit mask) and returns the associated set of
// capabilities.  If all capabilties are set, "full" is returned.  If no
// capability is enabled, "none" is returned.
func parseCAP(cap string) (string, error) {
	mask, err := strconv.ParseUint(cap, 16, 64)
	if err != nil {
		return "", err
	}
	if mask == fullCAPs {
		return "full", nil
	}
	caps := maskToCaps(mask)
	if len(caps) == 0 {
		return "none", nil
	}
	return strings.Join(caps, ", "), nil
}

// processCAPINH returns the set of inheritable capabilties associated with
// process p.  If all capabilties are set, "full" is returned.  If no
// capability is enabled, "none" is returned.
func processCAPINH(p *process) (string, error) {
	return parseCAP(p.pstatus.capInh)
}

// processCAPPRM returns the set of permitted capabilties associated with
// process p.  If all capabilties are set, "full" is returned.  If no
// capability is enabled, "none" is returned.
func processCAPPRM(p *process) (string, error) {
	return parseCAP(p.pstatus.capPrm)
}

// processCAPEFF returns the set of effective capabilties associated with
// process p.  If all capabilties are set, "full" is returned.  If no
// capability is enabled, "none" is returned.
func processCAPEFF(p *process) (string, error) {
	return parseCAP(p.pstatus.capEff)
}

// processCAPBND returns the set of bounding capabilties associated with
// process p.  If all capabilties are set, "full" is returned.  If no
// capability is enabled, "none" is returned.
func processCAPBND(p *process) (string, error) {
	return parseCAP(p.pstatus.capBnd)
}

// processSECCOMP returns the seccomp mode of the process (i.e., disabled,
// strict or filter) or "?" if /proc/$pid/status.seccomp has a unknown value.
func processSECCOMP(p *process) (string, error) {
	switch p.pstatus.seccomp {
	case "0":
		return "disabled", nil
	case "1":
		return "strict", nil
	case "2":
		return "filter", nil
	default:
		return "?", nil
	}
}

// processLABEL returns the process label of process p.
func processLABEL(p *process) (string, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/attr/current", p.pid))
	if err != nil {
		if os.IsNotExist(err) {
			// make sure the pid does not exist,
			// could be system does not support labeling.
			if _, err2 := os.Stat(fmt.Sprintf("/proc/%d", p.pid)); err2 != nil {
				return "", errNoSuchPID
			}
		}
		return "", err
	}
	return strings.Trim(string(data), "\x00"), nil
}
