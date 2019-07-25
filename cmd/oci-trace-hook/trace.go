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
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	bpf "github.com/iovisor/gobpf/bcc"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	seccomp "github.com/seccomp/libseccomp-golang"
	"github.com/sirupsen/logrus"
)

// event struct used to read the perf_buffer
type event struct {
	Pid     uint32
	ID      uint32
	Command [16]byte
}

type calls map[string]int

const source string = `
#include <linux/bpf.h>
#include <linux/nsproxy.h>
#include <linux/pid_namespace.h>
#include <linux/ns_common.h>
#include <linux/sched.h>
#include <linux/tracepoint.h>

BPF_HASH(parent_namespace, u64, unsigned int);
BPF_PERF_OUTPUT(events);

struct data_t
{
    u32 pid;
    u32 id;
    char comm[16];
};

int enter_trace(struct tracepoint__raw_syscalls__sys_enter *args)
{
    struct data_t data = {};
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
	defaultProfilePath, err := filepath.Abs("./profile.json")
	if err != nil {
		logrus.Error(err)
	}
	fileName := flag.String("f", defaultProfilePath, "path of the file to save the seccomp profile")

	flag.Parse()

	logfile, err := os.OpenFile("logfile", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logrus.Errorf("error opening file: %v", err)
	}
	defer logfile.Close()
	formatter := new(logrus.TextFormatter)
	formatter.FullTimestamp = true
	logrus.SetFormatter(formatter)
	logrus.SetOutput(logfile)
	if *runBPF > 0 {
		if err := runBPFSource(*runBPF, *fileName); err != nil {
			logrus.Error(err)
		}
	} else if *terminate {
		if err := sendSIGINT(); err != nil {
			logrus.Error(err)
		}
	} else {
		if err := startFloatingProcess(); err != nil {
			logrus.Error(err)
		}
	}

}

// Start a process which runs the BPF source and detach the process
func startFloatingProcess() error {
	logrus.Println("Starting the floating process")
	var s spec.State
	reader := bufio.NewReader(os.Stdin)
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&s)
	if err != nil {
		return err
	}
	pid := s.Pid
	fileName := s.Annotations["io.podman.trace-syscall"]
	//sysproc := &syscall.SysProcAttr{Noctty: true}
	attr := &os.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []*os.File{
			os.Stdin,
			nil,
			nil,
		},
		// Sys: sysproc,
	}
	if pid > 0 {
		process, err := os.StartProcess("/usr/libexec/oci/hooks.d/oci-trace-hook", []string{"/usr/libexec/oci/hooks.d/trace", "-r", strconv.Itoa(pid), "-f", fileName}, attr)
		if err != nil {
			return fmt.Errorf("cannot launch process err: %q", err.Error())
		}
		time.Sleep(2 * time.Second) // Waits 2 seconds to compile using clang and llvm
		processPID := process.Pid
		f, err := os.Create("pid")
		if err != nil {
			return fmt.Errorf("cannot write pid to file err:%q", err.Error())
		}
		defer f.Close()
		f.WriteString(strconv.Itoa(processPID))
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
func runBPFSource(pid int, fileName string) error {
	logrus.Println("Running floating process PID to attach:", pid)
	syscalls := make(calls, 303)
	src := strings.Replace(source, "$PARENT_PID", strconv.Itoa(pid), -1)
	m := bpf.NewModule(src, []string{})
	defer m.Close()

	tracepoint, err := m.LoadTracepoint("enter_trace")
	if err != nil {
		return err
	}

	logrus.Println("Loaded tracepoint")

	if err := m.AttachTracepoint("raw_syscalls:sys_enter", tracepoint); err != nil {
		return fmt.Errorf("unable to load tracepoint err:%q", err.Error())
	}

	table := bpf.NewTable(m.TableId("events"), m)
	channel := make(chan []byte)
	perfMap, err := bpf.InitPerfMap(table, channel)
	if err != nil {
		return fmt.Errorf("unable to init perf map err:%q", err.Error())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	rsc := false   // Reached Seccomp syscall
	rexec := false // Reached the execve from runc
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
			if name == "seccomp" {
				rsc = true
				continue
			} else if name == "execve" {
				rexec = true
				continue
			}
			if rsc && rexec {
				syscalls[name]++
			}
		}
	}()
	logrus.Println("PerfMap Start")
	perfMap.Start()
	<-sig
	logrus.Println("PerfMap Stop")
	perfMap.Stop()
	if err := generateProfile(syscalls, fileName); err != nil {
		return err
	}
	return nil
}

// send SIGINT to the floating process reading the perfbuffer
func sendSIGINT() error {
	f, err := ioutil.ReadFile("pid")

	if err != nil {
		return err
	}

	Spid := string(f)

	pid, _ := strconv.Atoi(Spid)
	p, _ := os.FindProcess(pid)
	p.Signal(os.Interrupt)
	return nil
}

// generate the seccomp profile from the syscalls provided
func generateProfile(c calls, fileName string) error {
	s := types.Seccomp{}
	var names []string
	for s, t := range c {
		if t > 0 {
			names = append(names, s)
		}
	}
	s.DefaultAction = types.ActErrno

	s.Syscalls = []*types.Syscall{
		&types.Syscall{
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
