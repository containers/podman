// +build oci_trace_hook

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/iovisor/gobpf/bcc" //nolint
	spec "github.com/opencontainers/runtime-spec/specs-go"
	seccomp "github.com/seccomp/libseccomp-golang"
	"github.com/sirupsen/logrus"
)

// event struct used to read data from the perf ring buffer
type event struct {
	// PID of the process making the syscall
	Pid uint32
	// syscall number
	ID uint32
	// Command which makes the syscall
	Command [16]byte
}

// the source is a bpf program compiled at runtime. Some macro's like
// BPF_HASH and BPF_PERF_OUTPUT are expanded during compilation
// by bcc. $PARENT_PID get's replaced before compilation with the PID of the container
// Complete documentation is available at
// https://github.com/iovisor/bcc/blob/master/docs/reference_guide.md
const source string = `
#include <linux/bpf.h>
#include <linux/nsproxy.h>
#include <linux/pid_namespace.h>
#include <linux/ns_common.h>
#include <linux/sched.h>
#include <linux/tracepoint.h>

// BPF_HASH used to store the PID namespace of the parent PID
// of the processes inside the container.
BPF_HASH(parent_namespace, u64, unsigned int);

// Opens a custom BPF table to push data to user space via perf ring buffer
BPF_PERF_OUTPUT(events);

// data_t used to store the data received from the event
struct syscall_data
{
	// PID of the process
	u32 pid;
	// the syscall number
	u32 id;
	// command which is making the syscall
    char comm[16];
};

// enter_trace : function is attached to the kernel tracepoint raw_syscalls:sys_enter it is
// called whenever a syscall is made. The function stores the pid_namespace (task->nsproxy->pid_ns_for_children->ns.inum) of the PID which
// starts the container in the BPF_HASH called parent_namespace.
// The data of the syscall made by the process with the same pid_namespace as the parent_namespace is pushed to
// userspace using perf ring buffer

// specification of args from sys/kernel/debug/tracing/events/raw_syscalls/sys_enter/format
int enter_trace(struct tracepoint__raw_syscalls__sys_enter *args)
{
    struct syscall_data data = {};
    u64 key = 0;
    unsigned int zero = 0;
    struct task_struct *task;

    data.pid = bpf_get_current_pid_tgid();
    data.id = (int)args->id;
    bpf_get_current_comm(&data.comm, sizeof(data.comm));

    task = (struct task_struct *)bpf_get_current_task();
    struct nsproxy *ns = task->nsproxy;
    unsigned int inum = ns->pid_ns_for_children->ns.inum;

    if (data.pid == $PARENT_PID)
    {
        parent_namespace.update(&key, &inum);
    }
    unsigned int *parent_inum = parent_namespace.lookup_or_init(&key, &zero);

    if (*parent_inum != inum)
    {
        return 0;
    }

    events.perf_submit(args, &data, sizeof(data));
    return 0;
}
`

func main() {

	terminate := flag.Bool("t", false, "send SIGINT to floating process")
	runBPF := flag.Int("r", 0, "-r [PID] run the BPF function and attach to the pid")
	fileName := flag.String("f", "", "path of the file to save the seccomp profile")

	start := flag.Bool("s", false, "Start the hook which would execute a process to trace syscalls made by the container")
	flag.Parse()

	profilePath, err := filepath.Abs(*fileName)
	if err != nil {
		logrus.Error(err)
	}

	logfilePath, err := filepath.Abs("trace-log")
	if err != nil {
		logrus.Error(err)
	}
	logfile, err := os.OpenFile(logfilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logrus.Errorf("error opening file: %v", err)
	}

	defer logfile.Close()
	formatter := new(logrus.TextFormatter)
	formatter.FullTimestamp = true
	logrus.SetFormatter(formatter)
	logrus.SetOutput(logfile)
	if *runBPF > 0 {
		logrus.Println("Filepath : ", profilePath)
		if err := runBPFSource(*runBPF, profilePath); err != nil {
			logrus.Error(err)
		}
	} else if *terminate {
		if err := sendSIGINT(); err != nil {
			logrus.Error(err)
		}
	} else if *start {
		if err := startFloatingProcess(); err != nil {
			logrus.Error(err)
		}
	}
}

// Start a process which runs the BPF source and detach the process
func startFloatingProcess() error {
	var s spec.State
	reader := bufio.NewReader(os.Stdin)
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&s)
	if err != nil {
		return err
	}
	pid := s.Pid
	fileName := s.Annotations["io.containers.trace-syscall"]
	attr := &os.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []*os.File{
			os.Stdin,
			nil,
			nil,
		},
	}
	if pid > 0 {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGUSR1)

		process, err := os.StartProcess("/usr/local/libexec/oci/hooks.d/oci-trace-hook", []string{"oci-trace-hook", "-r", strconv.Itoa(pid), "-f", fileName}, attr)
		if err != nil {
			return fmt.Errorf("cannot launch process err: %q", err.Error())
		}

		<-sig

		processPID := process.Pid
		f, err := os.Create("pidfile")
		if err != nil {
			return fmt.Errorf("cannot write pid to file err:%q", err.Error())
		}
		defer f.Close()
		_, err = f.WriteString(strconv.Itoa(processPID))
		if err != nil {
			return fmt.Errorf("cannot write pid to the file")
		}
		err = process.Release()
		if err != nil {
			return fmt.Errorf("cannot detach process err:%q", err.Error())
		}

	} else {
		return fmt.Errorf("container not running")
	}
	return nil
}

// run the BPF source and attach it to raw_syscalls:sys_enter tracepoint
func runBPFSource(pid int, profilePath string) error {

	ppid := os.Getppid()
	parentProcess, err := os.FindProcess(ppid)

	if err != nil {
		return fmt.Errorf("cannot find the parent process pid %d : %q", ppid, err)
	}

	logrus.Println("Running floating process PID to attach:", pid)
	syscalls := make(map[string]int, 303)
	src := strings.Replace(source, "$PARENT_PID", strconv.Itoa(pid), -1)
	m := bcc.NewModule(src, []string{})
	defer m.Close()

	tracepoint, err := m.LoadTracepoint("enter_trace")
	if err != nil {
		return err
	}

	logrus.Println("Loaded tracepoint")

	if err := m.AttachTracepoint("raw_syscalls:sys_enter", tracepoint); err != nil {
		return fmt.Errorf("unable to load tracepoint err:%q", err.Error())
	}

	// send a signal to the parent process to indicate the compilation has been completed
	err = parentProcess.Signal(syscall.SIGUSR1)
	if err != nil {
		return err
	}

	table := bcc.NewTable(m.TableId("events"), m)
	channel := make(chan []byte)
	perfMap, err := bcc.InitPerfMap(table, channel)
	if err != nil {
		return fmt.Errorf("unable to init perf map err:%q", err.Error())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	reachedPRCTL := false // Reached PRCTL syscall
	go func() {
		var e event
		for {
			data := <-channel
			err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &e)
			if err != nil {
				logrus.Errorf("failed to decode received data '%s': %s\n", data, err)
				continue
			}
			name, err := getName(e.ID)
			if err != nil {
				logrus.Errorf("failed to get name of syscall from id : %d received : %q", e.ID, name)
			}
			// syscalls are not recorded until prctl() is called
			if name == "prctl" {
				reachedPRCTL = true
			}
			if reachedPRCTL {
				syscalls[name]++
			}
		}
	}()
	logrus.Println("PerfMap Start")
	perfMap.Start()
	<-sig
	logrus.Println("PerfMap Stop")
	perfMap.Stop()
	if err := generateProfile(syscalls, profilePath); err != nil {
		return err
	}
	return nil
}

// send SIGINT to the floating process reading the perfbuffer
func sendSIGINT() error {
	f, err := ioutil.ReadFile("pidfile")

	if err != nil {
		return err
	}

	Spid := string(f)

	pid, _ := strconv.Atoi(Spid)
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process with PID %d \nerror msg: %q", pid, err.Error())
	}
	err = p.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("cannot send signal to child process err: %q", err.Error())
	}
	return nil
}

// generate the seccomp profile from the syscalls provided
func generateProfile(c map[string]int, fileName string) error {
	s := types.Seccomp{}
	var names []string
	for s, t := range c {
		if t > 0 {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	s.DefaultAction = types.ActErrno

	s.Syscalls = []*types.Syscall{
		{
			Action: types.ActAllow,
			Names:  names,
			Args:   []*types.Arg{},
		},
	}
	sJSON, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fileName, sJSON, 0644); err != nil {
		return err
	}
	return nil
}

// get the name of the syscall from it's ID
func getName(id uint32) (string, error) {
	name, err := seccomp.ScmpSyscall(id).GetName()
	return name, err
}
