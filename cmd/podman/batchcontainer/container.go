package batchcontainer

import (
	"time"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PsOptions describes the struct being formed for ps
type PsOptions struct {
	All       bool
	Filter    string
	Format    string
	Last      int
	Latest    bool
	NoTrunc   bool
	Quiet     bool
	Size      bool
	Label     string
	Namespace bool
}

// BatchContainerStruct is the return obkect from BatchContainer and contains
// container related information
type BatchContainerStruct struct {
	ConConfig          *libpod.ContainerConfig
	ConState           libpod.ContainerStatus
	ExitCode           int32
	Pid                int
	RootFsSize, RwSize int64
	StartedTime        time.Time
}

// Namespace describes output for ps namespace
type Namespace struct {
	PID    string `json:"pid,omitempty"`
	Cgroup string `json:"cgroup,omitempty"`
	IPC    string `json:"ipc,omitempty"`
	MNT    string `json:"mnt,omitempty"`
	NET    string `json:"net,omitempty"`
	PIDNS  string `json:"pidns,omitempty"`
	User   string `json:"user,omitempty"`
	UTS    string `json:"uts,omitempty"`
}

// BatchContainer is used in ps to reduce performance hits by "batching"
// locks.
func BatchContainerOp(ctr *libpod.Container, opts PsOptions) (BatchContainerStruct, error) {
	var (
		conConfig          *libpod.ContainerConfig
		conState           libpod.ContainerStatus
		err                error
		exitCode           int32
		pid                int
		rootFsSize, rwSize int64
		startedTime        time.Time
	)

	batchErr := ctr.Batch(func(c *libpod.Container) error {
		conConfig = c.Config()
		conState, err = c.State()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container state")
		}

		exitCode, err = c.ExitCode()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container exit code")
		}
		startedTime, err = c.StartedTime()
		if err != nil {
			logrus.Errorf("error getting started time for %q: %v", c.ID(), err)
		}

		if !opts.Size && !opts.Namespace {
			return nil
		}

		if opts.Namespace {
			pid, err = c.PID()
			if err != nil {
				return errors.Wrapf(err, "unable to obtain container pid")
			}
		}
		if opts.Size {
			rootFsSize, err = c.RootFsSize()
			if err != nil {
				logrus.Errorf("error getting root fs size for %q: %v", c.ID(), err)
			}

			rwSize, err = c.RWSize()
			if err != nil {
				logrus.Errorf("error getting rw size for %q: %v", c.ID(), err)
			}

		}
		return nil
	})
	if batchErr != nil {
		return BatchContainerStruct{}, batchErr
	}
	return BatchContainerStruct{
		ConConfig:   conConfig,
		ConState:    conState,
		ExitCode:    exitCode,
		Pid:         pid,
		RootFsSize:  rootFsSize,
		RwSize:      rwSize,
		StartedTime: startedTime,
	}, nil
}

// GetNamespaces returns a populated namespace struct
func GetNamespaces(pid int) *Namespace {
	ctrPID := strconv.Itoa(pid)
	cgroup, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "cgroup"))
	ipc, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "ipc"))
	mnt, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "mnt"))
	net, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "net"))
	pidns, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "pid"))
	user, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "user"))
	uts, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "uts"))

	return &Namespace{
		PID:    ctrPID,
		Cgroup: cgroup,
		IPC:    ipc,
		MNT:    mnt,
		NET:    net,
		PIDNS:  pidns,
		User:   user,
		UTS:    uts,
	}
}

func getNamespaceInfo(path string) (string, error) {
	val, err := os.Readlink(path)
	if err != nil {
		return "", errors.Wrapf(err, "error getting info from %q", path)
	}
	return getStrFromSquareBrackets(val), nil
}

// getStrFromSquareBrackets gets the string inside [] from a string
func getStrFromSquareBrackets(cmd string) string {
	reg, err := regexp.Compile(".*\\[|\\].*")
	if err != nil {
		return ""
	}
	arr := strings.Split(reg.ReplaceAllLiteralString(cmd, ""), ",")
	return strings.Join(arr, ",")
}
