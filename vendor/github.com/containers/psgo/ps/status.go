package ps

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// status is a direct translation of a `/proc/[pid]/status`, wich provides much
// of the information in /proc/[pid]/stat and /proc/[pid]/statm in a format
// that's easier for humans to parse.
type status struct {
	// Name: Command run by this process.
	name string
	// Umask: Process umask, expressed in octal with a leading  zero;  see
	// umask(2). (Since Linux 4.7.)
	umask string
	// State:  Current  state  of the process.  One of "R (running)", "S
	// (sleeping)", "D (disk sleep)", "T (stopped)", "T (tracing stop)", "Z
	// (zombie)", or "X (dead)".
	state string
	// Tgid: Thread group ID (i.e., Process ID).
	tgid string
	// Ngid: NUMA group ID (0 if none; since Linux 3.13).
	ngid string
	// Pid: Thread ID (see gettid(2)).
	pid string
	// PPid: PID of parent process.
	pPid string
	// TracerPid: PID of process tracing this process (0 if not being traced).
	tracerPid string
	// Uids: Real, effective, saved set, and filesystem.
	uids []string
	// Gids: Real, effective, saved set, and filesystem.
	gids []string
	// FDSize: Number of file descriptor slots currently allocated.
	fdSize string
	// Groups: Supplementary group list.
	groups []string
	// NStgid : Thread group ID (i.e., PID) in each of the PID namespaces
	// of which [pid] is  a member.   The  leftmost  entry shows the value
	// with respect to the PID namespace of the reading process, followed
	// by the value in successively nested inner namespaces.  (Since Linux
	// 4.1.)
	nStgid string
	// NSpid:  Thread ID in each of the PID namespaces of which [pid] is a
	// member.  The fields are ordered as for NStgid.  (Since Linux 4.1.)
	nSpid string
	// NSpgid: Process group ID in each of the PID namespaces of which
	// [pid] is a member.  The fields are ordered as for NStgid.  (Since
	// Linux 4.1.)
	nSpgid string
	// NSsid:  descendant  namespace session ID hierarchy Session ID in
	// each of the PID names- paces of which [pid] is a member.  The fields
	// are ordered as for NStgid.  (Since  Linux 4.1.)
	nSsid string
	// VmPeak: Peak virtual memory size.
	vmPeak string
	// VmSize: Virtual memory size.
	vmSize string
	// VmLck: Locked memory size (see mlock(3)).
	vmLCK string
	// VmPin:  Pinned  memory  size  (since  Linux  3.2).  These are pages
	// that can't be moved because something needs to directly access
	// physical memory.
	vmPin string
	// VmHWM: Peak resident set size ("high water mark").
	vmHWM string
	// VmRSS: Resident set size.  Note that the value here is the sum of
	// RssAnon, RssFile, and RssShmem.
	vmRSS string
	// RssAnon: Size of resident anonymous memory.  (since Linux 4.5).
	rssAnon string
	// RssFile: Size of resident file mappings.  (since Linux 4.5).
	rssFile string
	// RssShmem:  Size  of  resident  shared memory (includes System V
	// shared memory, mappings from tmpfs(5), and shared anonymous
	// mappings).  (since Linux 4.5).
	rssShmem string
	// VmData: Size of data segment.
	vmData string
	// VmStk: Size of stack segment.
	vmStk string
	// VmExe: Size of text segment.
	vmExe string
	// VmLib: Shared library code size.
	vmLib string
	// VmPTE: Page table entries size (since Linux 2.6.10).
	vmPTE string
	// VmPMD: Size of second-level page tables (since Linux 4.0).
	vmPMD string
	// VmSwap: Swapped-out virtual memory size by anonymous private pages;
	// shmem swap usage is not included (since Linux 2.6.34).
	vmSwap string
	// HugetlbPages: Size of hugetlb memory portions.  (since Linux 4.4).
	hugetlbPages string
	// Threads: Number of threads in process containing this thread.
	threads string
	// SigQ: This field contains two slash-separated numbers that relate to
	// queued signals for the real user ID of this process.  The first of
	// these is the number of currently queued signals  for  this  real
	// user ID, and the second is the resource limit on the number of
	// queued signals for this process (see the  description  of
	// RLIMIT_SIGPENDING  in  getr- limit(2)).
	sigQ string
	// SigPnd:  Number  of signals pending for thread and for (see pthreads(7)).
	sigPnd string
	// ShdPnd:  Number  of signals pending for process as a whole (see
	// signal(7)).
	shdPnd string
	//  SigBlk: Mask indicating signals being  blocked (see signal(7)).
	sigBlk string
	//  SigIgn: Mask indicating signals being ignored (see signal(7)).
	sigIgn string
	//  SigCgt: Mask indicating signals being  blocked caught (see signal(7)).
	sigCgt string
	// CapInh:  Mask of capabilities enabled in inheritable sets (see
	// capabilities(7)).
	capInh string
	// CapPrm:  Mask of capabilities enabled in permitted sets (see
	// capabilities(7)).
	capPrm string
	// CapEff:  Mask of capabilities enabled in effective sets (see
	// capabilities(7)).
	capEff string
	// CapBnd: Capability Bounding set (since Linux 2.6.26, see
	// capabilities(7)).
	capBnd string
	// CapAmb: Ambient capability set (since Linux 4.3, see capabilities(7)).
	capAmb string
	// NoNewPrivs: Value of the no_new_privs bit (since Linux 4.10, see
	// prctl(2)).
	noNewPrivs string
	// Seccomp: Seccomp mode of the process (since Linux 3.8, see
	// seccomp(2)).  0  means  SEC- COMP_MODE_DISABLED;  1  means
	// SECCOMP_MODE_STRICT;  2 means SECCOMP_MODE_FILTER.  This field is
	// provided only if the kernel was built with the CONFIG_SECCOMP kernel
	// configu- ration option enabled.
	seccomp string
	// Cpus_allowed:  Mask  of  CPUs  on  which  this process may run
	// (since Linux 2.6.24, see cpuset(7)).
	cpusAllowed string
	// Cpus_allowed_list: Same as previous, but in "list  format"  (since
	// Linux  2.6.26,  see cpuset(7)).
	cpusAllowedList string
	// Mems_allowed:  Mask  of  memory  nodes allowed to this process
	// (since Linux 2.6.24, see cpuset(7)).
	memsAllowed string
	// Mems_allowed_list: Same as previous, but in "list  format"  (since
	// Linux  2.6.26,  see cpuset(7)).
	memsAllowedList string
	// voluntaryCtxtSwitches:  Number of voluntary context switches
	// (since Linux 2.6.23).
	voluntaryCtxtSwitches string
	// nonvoluntaryCtxtSwitches:  Number of involuntary context switches
	// (since Linux 2.6.23).
	nonvoluntaryCtxtSwitches string
}

// readStatus is used for mocking in unit tests.
var readStatus = func(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = errNoSuchPID
		}
		return nil, err
	}
	lines := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

// parseStatus parses the /proc/$pid/status file and returns a *status.
func parseStatus(path string) (*status, error) {
	lines, err := readStatus(path)
	if err != nil {
		return nil, err
	}

	s := status{}
	errUnexpectedInput := errors.New(fmt.Sprintf("unexpected input from %s", path))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "Name:":
			s.name = fields[1]
		case "Umask:":
			s.umask = fields[1]
		case "State:":
			s.state = fields[1]
		case "Tgid:":
			s.tgid = fields[1]
		case "Ngid:":
			s.ngid = fields[1]
		case "Pid:":
			s.pid = fields[1]
		case "PPid:":
			s.pPid = fields[1]
		case "TracerPid:":
			s.tracerPid = fields[1]
		case "Uid:":
			if len(fields) != 5 {
				return nil, errors.Wrap(errUnexpectedInput, line)
			}
			s.uids = []string{fields[1], fields[2], fields[3], fields[4]}
		case "Gid:":
			if len(fields) != 5 {
				return nil, errors.Wrap(errUnexpectedInput, line)
			}
			s.gids = []string{fields[1], fields[2], fields[3], fields[4]}
		case "FDSize:":
			s.fdSize = fields[1]
		case "Groups:":
			for _, g := range fields[1:] {
				s.groups = append(s.groups, g)
			}
		case "NStgid:":
			s.nStgid = fields[1]
		case "NSpid:":
			s.nSpid = fields[1]
		case "NSpgid:":
			s.nSpgid = fields[1]
		case "NSsid:":
			s.nSsid = fields[1]
		case "VmPeak:":
			s.vmPeak = fields[1]
		case "VmSize:":
			s.vmSize = fields[1]
		case "VmLck:":
			s.vmLCK = fields[1]
		case "VmPin:":
			s.vmPin = fields[1]
		case "VmHWM:":
			s.vmHWM = fields[1]
		case "VmRSS:":
			s.vmRSS = fields[1]
		case "RssAnon:":
			s.rssAnon = fields[1]
		case "RssFile:":
			s.rssFile = fields[1]
		case "RssShmem:":
			s.rssShmem = fields[1]
		case "VmData:":
			s.vmData = fields[1]
		case "VmStk:":
			s.vmStk = fields[1]
		case "VmExe:":
			s.vmExe = fields[1]
		case "VmLib:":
			s.vmLib = fields[1]
		case "VmPTE:":
			s.vmPTE = fields[1]
		case "VmPMD:":
			s.vmPMD = fields[1]
		case "VmSwap:":
			s.vmSwap = fields[1]
		case "HugetlbPages:":
			s.hugetlbPages = fields[1]
		case "Threads:":
			s.threads = fields[1]
		case "SigQ:":
			s.sigQ = fields[1]
		case "SigPnd:":
			s.sigPnd = fields[1]
		case "ShdPnd:":
			s.shdPnd = fields[1]
		case "SigBlk:":
			s.sigBlk = fields[1]
		case "SigIgn:":
			s.sigIgn = fields[1]
		case "SigCgt:":
			s.sigCgt = fields[1]
		case "CapInh:":
			s.capInh = fields[1]
		case "CapPrm:":
			s.capPrm = fields[1]
		case "CapEff:":
			s.capEff = fields[1]
		case "CapBnd:":
			s.capBnd = fields[1]
		case "CapAmb:":
			s.capAmb = fields[1]
		case "NoNewPrivs:":
			s.noNewPrivs = fields[1]
		case "Seccomp:":
			s.seccomp = fields[1]
		case "Cpus_allowed:":
			s.cpusAllowed = fields[1]
		case "Cpus_allowed_list:":
			s.cpusAllowedList = fields[1]
		case "Mems_allowed:":
			s.memsAllowed = fields[1]
		case "Mems_allowed_list:":
			s.memsAllowedList = fields[1]
		case "voluntary_ctxt_switches:":
			s.voluntaryCtxtSwitches = fields[1]
		case "nonvoluntary_ctxt_switches:":
			s.nonvoluntaryCtxtSwitches = fields[1]
		}
	}

	return &s, nil
}
