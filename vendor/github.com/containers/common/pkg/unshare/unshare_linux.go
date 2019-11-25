// +build linux

package unshare

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/reexec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
)

// Cmd wraps an exec.Cmd created by the reexec package in unshare(), and
// handles setting ID maps and other related settings by triggering
// initialization code in the child.
type Cmd struct {
	*exec.Cmd
	UnshareFlags               int
	UseNewuidmap               bool
	UidMappings                []specs.LinuxIDMapping
	UseNewgidmap               bool
	GidMappings                []specs.LinuxIDMapping
	GidMappingsEnableSetgroups bool
	Setsid                     bool
	Setpgrp                    bool
	Ctty                       *os.File
	OOMScoreAdj                *int
	Hook                       func(pid int) error
}

// Command creates a new Cmd which can be customized.
func Command(args ...string) *Cmd {
	cmd := reexec.Command(args...)
	return &Cmd{
		Cmd: cmd,
	}
}

func (c *Cmd) Start() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Set an environment variable to tell the child to synchronize its startup.
	if c.Env == nil {
		c.Env = os.Environ()
	}
	c.Env = append(c.Env, fmt.Sprintf("_Containers-unshare=%d", c.UnshareFlags))

	// Please the libpod "rootless" package to find the expected env variables.
	if os.Geteuid() != 0 {
		c.Env = append(c.Env, "_CONTAINERS_USERNS_CONFIGURED=done")
		c.Env = append(c.Env, fmt.Sprintf("_CONTAINERS_ROOTLESS_UID=%d", os.Geteuid()))
		c.Env = append(c.Env, fmt.Sprintf("_CONTAINERS_ROOTLESS_GID=%d", os.Getegid()))
	}

	// Create the pipe for reading the child's PID.
	pidRead, pidWrite, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "error creating pid pipe")
	}
	c.Env = append(c.Env, fmt.Sprintf("_Containers-pid-pipe=%d", len(c.ExtraFiles)+3))
	c.ExtraFiles = append(c.ExtraFiles, pidWrite)

	// Create the pipe for letting the child know to proceed.
	continueRead, continueWrite, err := os.Pipe()
	if err != nil {
		pidRead.Close()
		pidWrite.Close()
		return errors.Wrapf(err, "error creating pid pipe")
	}
	c.Env = append(c.Env, fmt.Sprintf("_Containers-continue-pipe=%d", len(c.ExtraFiles)+3))
	c.ExtraFiles = append(c.ExtraFiles, continueRead)

	// Pass along other instructions.
	if c.Setsid {
		c.Env = append(c.Env, "_Containers-setsid=1")
	}
	if c.Setpgrp {
		c.Env = append(c.Env, "_Containers-setpgrp=1")
	}
	if c.Ctty != nil {
		c.Env = append(c.Env, fmt.Sprintf("_Containers-ctty=%d", len(c.ExtraFiles)+3))
		c.ExtraFiles = append(c.ExtraFiles, c.Ctty)
	}

	// Make sure we clean up our pipes.
	defer func() {
		if pidRead != nil {
			pidRead.Close()
		}
		if pidWrite != nil {
			pidWrite.Close()
		}
		if continueRead != nil {
			continueRead.Close()
		}
		if continueWrite != nil {
			continueWrite.Close()
		}
	}()

	// Start the new process.
	err = c.Cmd.Start()
	if err != nil {
		return err
	}

	// Close the ends of the pipes that the parent doesn't need.
	continueRead.Close()
	continueRead = nil
	pidWrite.Close()
	pidWrite = nil

	// Read the child's PID from the pipe.
	pidString := ""
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, pidRead); err != nil {
		return errors.Wrapf(err, "error reading child PID")
	}
	pidString = b.String()
	pid, err := strconv.Atoi(pidString)
	if err != nil {
		fmt.Fprintf(continueWrite, "error parsing PID %q: %v", pidString, err)
		return errors.Wrapf(err, "error parsing PID %q", pidString)
	}
	pidString = fmt.Sprintf("%d", pid)

	// If we created a new user namespace, set any specified mappings.
	if c.UnshareFlags&syscall.CLONE_NEWUSER != 0 {
		// Always set "setgroups".
		setgroups, err := os.OpenFile(fmt.Sprintf("/proc/%s/setgroups", pidString), os.O_TRUNC|os.O_WRONLY, 0)
		if err != nil {
			fmt.Fprintf(continueWrite, "error opening setgroups: %v", err)
			return errors.Wrapf(err, "error opening /proc/%s/setgroups", pidString)
		}
		defer setgroups.Close()
		if c.GidMappingsEnableSetgroups {
			if _, err := fmt.Fprintf(setgroups, "allow"); err != nil {
				fmt.Fprintf(continueWrite, "error writing \"allow\" to setgroups: %v", err)
				return errors.Wrapf(err, "error opening \"allow\" to /proc/%s/setgroups", pidString)
			}
		} else {
			if _, err := fmt.Fprintf(setgroups, "deny"); err != nil {
				fmt.Fprintf(continueWrite, "error writing \"deny\" to setgroups: %v", err)
				return errors.Wrapf(err, "error writing \"deny\" to /proc/%s/setgroups", pidString)
			}
		}

		if len(c.UidMappings) == 0 || len(c.GidMappings) == 0 {
			uidmap, gidmap, err := GetHostIDMappings("")
			if err != nil {
				fmt.Fprintf(continueWrite, "error reading ID mappings in parent: %v", err)
				return errors.Wrapf(err, "error reading ID mappings in parent")
			}
			if len(c.UidMappings) == 0 {
				c.UidMappings = uidmap
				for i := range c.UidMappings {
					c.UidMappings[i].HostID = c.UidMappings[i].ContainerID
				}
			}
			if len(c.GidMappings) == 0 {
				c.GidMappings = gidmap
				for i := range c.GidMappings {
					c.GidMappings[i].HostID = c.GidMappings[i].ContainerID
				}
			}
		}

		if len(c.GidMappings) > 0 {
			// Build the GID map, since writing to the proc file has to be done all at once.
			g := new(bytes.Buffer)
			for _, m := range c.GidMappings {
				fmt.Fprintf(g, "%d %d %d\n", m.ContainerID, m.HostID, m.Size)
			}
			gidmapSet := false
			// Set the GID map.
			if c.UseNewgidmap {
				cmd := exec.Command("newgidmap", append([]string{pidString}, strings.Fields(strings.Replace(g.String(), "\n", " ", -1))...)...)
				g.Reset()
				cmd.Stdout = g
				cmd.Stderr = g
				err := cmd.Run()
				if err == nil {
					gidmapSet = true
				} else {
					logrus.Warnf("error running newgidmap: %v: %s", err, g.String())
					logrus.Warnf("falling back to single mapping")
					g.Reset()
					g.Write([]byte(fmt.Sprintf("0 %d 1\n", os.Getegid())))
				}
			}
			if !gidmapSet {
				if c.UseNewgidmap {
					setgroups, err := os.OpenFile(fmt.Sprintf("/proc/%s/setgroups", pidString), os.O_TRUNC|os.O_WRONLY, 0)
					if err != nil {
						fmt.Fprintf(continueWrite, "error opening /proc/%s/setgroups: %v", pidString, err)
						return errors.Wrapf(err, "error opening /proc/%s/setgroups", pidString)
					}
					defer setgroups.Close()
					if _, err := fmt.Fprintf(setgroups, "deny"); err != nil {
						fmt.Fprintf(continueWrite, "error writing 'deny' to /proc/%s/setgroups: %v", pidString, err)
						return errors.Wrapf(err, "error writing 'deny' to /proc/%s/setgroups", pidString)
					}
				}
				gidmap, err := os.OpenFile(fmt.Sprintf("/proc/%s/gid_map", pidString), os.O_TRUNC|os.O_WRONLY, 0)
				if err != nil {
					fmt.Fprintf(continueWrite, "error opening /proc/%s/gid_map: %v", pidString, err)
					return errors.Wrapf(err, "error opening /proc/%s/gid_map", pidString)
				}
				defer gidmap.Close()
				if _, err := fmt.Fprintf(gidmap, "%s", g.String()); err != nil {
					fmt.Fprintf(continueWrite, "error writing %q to /proc/%s/gid_map: %v", g.String(), pidString, err)
					return errors.Wrapf(err, "error writing %q to /proc/%s/gid_map", g.String(), pidString)
				}
			}
		}

		if len(c.UidMappings) > 0 {
			// Build the UID map, since writing to the proc file has to be done all at once.
			u := new(bytes.Buffer)
			for _, m := range c.UidMappings {
				fmt.Fprintf(u, "%d %d %d\n", m.ContainerID, m.HostID, m.Size)
			}
			uidmapSet := false
			// Set the GID map.
			if c.UseNewuidmap {
				cmd := exec.Command("newuidmap", append([]string{pidString}, strings.Fields(strings.Replace(u.String(), "\n", " ", -1))...)...)
				u.Reset()
				cmd.Stdout = u
				cmd.Stderr = u
				err := cmd.Run()
				if err == nil {
					uidmapSet = true
				} else {
					logrus.Warnf("error running newuidmap: %v: %s", err, u.String())
					logrus.Warnf("falling back to single mapping")
					u.Reset()
					u.Write([]byte(fmt.Sprintf("0 %d 1\n", os.Geteuid())))
				}
			}
			if !uidmapSet {
				uidmap, err := os.OpenFile(fmt.Sprintf("/proc/%s/uid_map", pidString), os.O_TRUNC|os.O_WRONLY, 0)
				if err != nil {
					fmt.Fprintf(continueWrite, "error opening /proc/%s/uid_map: %v", pidString, err)
					return errors.Wrapf(err, "error opening /proc/%s/uid_map", pidString)
				}
				defer uidmap.Close()
				if _, err := fmt.Fprintf(uidmap, "%s", u.String()); err != nil {
					fmt.Fprintf(continueWrite, "error writing %q to /proc/%s/uid_map: %v", u.String(), pidString, err)
					return errors.Wrapf(err, "error writing %q to /proc/%s/uid_map", u.String(), pidString)
				}
			}
		}
	}

	if c.OOMScoreAdj != nil {
		oomScoreAdj, err := os.OpenFile(fmt.Sprintf("/proc/%s/oom_score_adj", pidString), os.O_TRUNC|os.O_WRONLY, 0)
		if err != nil {
			fmt.Fprintf(continueWrite, "error opening oom_score_adj: %v", err)
			return errors.Wrapf(err, "error opening /proc/%s/oom_score_adj", pidString)
		}
		defer oomScoreAdj.Close()
		if _, err := fmt.Fprintf(oomScoreAdj, "%d\n", *c.OOMScoreAdj); err != nil {
			fmt.Fprintf(continueWrite, "error writing \"%d\" to oom_score_adj: %v", c.OOMScoreAdj, err)
			return errors.Wrapf(err, "error writing \"%d\" to /proc/%s/oom_score_adj", c.OOMScoreAdj, pidString)
		}
	}
	// Run any additional setup that we want to do before the child starts running proper.
	if c.Hook != nil {
		if err = c.Hook(pid); err != nil {
			fmt.Fprintf(continueWrite, "hook error: %v", err)
			return err
		}
	}

	return nil
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	return nil, errors.New("unshare: CombinedOutput() not implemented")
}

func (c *Cmd) Output() ([]byte, error) {
	return nil, errors.New("unshare: Output() not implemented")
}

var (
	isRootlessOnce sync.Once
	isRootless     bool
)

const (
	// UsernsEnvName is the environment variable, if set indicates in rootless mode
	UsernsEnvName = "_CONTAINERS_USERNS_CONFIGURED"
)

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	isRootlessOnce.Do(func() {
		isRootless = os.Geteuid() != 0 || os.Getenv(UsernsEnvName) != ""
	})
	return isRootless
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	uidEnv := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Getuid()
}

// RootlessEnv returns the environment settings for the rootless containers
func RootlessEnv() []string {
	return append(os.Environ(), UsernsEnvName+"=done")
}

type Runnable interface {
	Run() error
}

func bailOnError(err error, format string, a ...interface{}) {
	if err != nil {
		if format != "" {
			logrus.Errorf("%s: %v", fmt.Sprintf(format, a...), err)
		} else {
			logrus.Errorf("%v", err)
		}
		os.Exit(1)
	}
}

// MaybeReexecUsingUserNamespace re-exec the process in a new namespace
func MaybeReexecUsingUserNamespace(evenForRoot bool) {
	// If we've already been through this once, no need to try again.
	if os.Geteuid() == 0 && IsRootless() {
		return
	}

	var uidNum, gidNum uint64
	// Figure out who we are.
	me, err := user.Current()
	if !os.IsNotExist(err) {
		bailOnError(err, "error determining current user")
		uidNum, err = strconv.ParseUint(me.Uid, 10, 32)
		bailOnError(err, "error parsing current UID %s", me.Uid)
		gidNum, err = strconv.ParseUint(me.Gid, 10, 32)
		bailOnError(err, "error parsing current GID %s", me.Gid)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// ID mappings to use to reexec ourselves.
	var uidmap, gidmap []specs.LinuxIDMapping
	if uidNum != 0 || evenForRoot {
		// Read the set of ID mappings that we're allowed to use.  Each
		// range in /etc/subuid and /etc/subgid file is a starting host
		// ID and a range size.
		uidmap, gidmap, err = GetSubIDMappings(me.Username, me.Username)
		if err != nil {
			logrus.Warnf("error reading allowed ID mappings: %v", err)
		}
		if len(uidmap) == 0 {
			logrus.Warnf("Found no UID ranges set aside for user %q in /etc/subuid.", me.Username)
		}
		if len(gidmap) == 0 {
			logrus.Warnf("Found no GID ranges set aside for user %q in /etc/subgid.", me.Username)
		}
		// Map our UID and GID, then the subuid and subgid ranges,
		// consecutively, starting at 0, to get the mappings to use for
		// a copy of ourselves.
		uidmap = append([]specs.LinuxIDMapping{{HostID: uint32(uidNum), ContainerID: 0, Size: 1}}, uidmap...)
		gidmap = append([]specs.LinuxIDMapping{{HostID: uint32(gidNum), ContainerID: 0, Size: 1}}, gidmap...)
		var rangeStart uint32
		for i := range uidmap {
			uidmap[i].ContainerID = rangeStart
			rangeStart += uidmap[i].Size
		}
		rangeStart = 0
		for i := range gidmap {
			gidmap[i].ContainerID = rangeStart
			rangeStart += gidmap[i].Size
		}
	} else {
		// If we have CAP_SYS_ADMIN, then we don't need to create a new namespace in order to be able
		// to use unshare(), so don't bother creating a new user namespace at this point.
		capabilities, err := capability.NewPid(0)
		bailOnError(err, "error reading the current capabilities sets")
		if capabilities.Get(capability.EFFECTIVE, capability.CAP_SYS_ADMIN) {
			return
		}
		// Read the set of ID mappings that we're currently using.
		uidmap, gidmap, err = GetHostIDMappings("")
		bailOnError(err, "error reading current ID mappings")
		// Just reuse them.
		for i := range uidmap {
			uidmap[i].HostID = uidmap[i].ContainerID
		}
		for i := range gidmap {
			gidmap[i].HostID = gidmap[i].ContainerID
		}
	}

	// Unlike most uses of reexec or unshare, we're using a name that
	// _won't_ be recognized as a registered reexec handler, since we
	// _want_ to fall through reexec.Init() to the normal main().
	cmd := Command(append([]string{fmt.Sprintf("%s-in-a-user-namespace", os.Args[0])}, os.Args[1:]...)...)

	// If, somehow, we don't become UID 0 in our child, indicate that the child shouldn't try again.
	err = os.Setenv(UsernsEnvName, "1")
	bailOnError(err, "error setting %s=1 in environment", UsernsEnvName)

	// Set the default isolation type to use the "rootless" method.
	if _, present := os.LookupEnv("BUILDAH_ISOLATION"); !present {
		if err = os.Setenv("BUILDAH_ISOLATION", "rootless"); err != nil {
			if err := os.Setenv("BUILDAH_ISOLATION", "rootless"); err != nil {
				logrus.Errorf("error setting BUILDAH_ISOLATION=rootless in environment: %v", err)
				os.Exit(1)
			}
		}
	}

	// Reuse our stdio.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up a new user namespace with the ID mapping.
	cmd.UnshareFlags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS
	cmd.UseNewuidmap = uidNum != 0
	cmd.UidMappings = uidmap
	cmd.UseNewgidmap = uidNum != 0
	cmd.GidMappings = gidmap
	cmd.GidMappingsEnableSetgroups = true

	// Finish up.
	logrus.Debugf("running %+v with environment %+v, UID map %+v, and GID map %+v", cmd.Cmd.Args, os.Environ(), cmd.UidMappings, cmd.GidMappings)
	ExecRunnable(cmd, nil)
}

// ExecRunnable runs the specified unshare command, captures its exit status,
// and exits with the same status.
func ExecRunnable(cmd Runnable, cleanup func()) {
	exit := func(status int) {
		if cleanup != nil {
			cleanup()
		}
		os.Exit(status)
	}
	if err := cmd.Run(); err != nil {
		if exitError, ok := errors.Cause(err).(*exec.ExitError); ok {
			if exitError.ProcessState.Exited() {
				if waitStatus, ok := exitError.ProcessState.Sys().(syscall.WaitStatus); ok {
					if waitStatus.Exited() {
						logrus.Errorf("%v", exitError)
						exit(waitStatus.ExitStatus())
					}
					if waitStatus.Signaled() {
						logrus.Errorf("%v", exitError)
						exit(int(waitStatus.Signal()) + 128)
					}
				}
			}
		}
		logrus.Errorf("%v", err)
		logrus.Errorf("(unable to determine exit status)")
		exit(1)
	}
	exit(0)
}

// getHostIDMappings reads mappings from the named node under /proc.
func getHostIDMappings(path string) ([]specs.LinuxIDMapping, error) {
	var mappings []specs.LinuxIDMapping
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading ID mappings from %q", path)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, errors.Errorf("line %q from %q has %d fields, not 3", line, path, len(fields))
		}
		cid, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing container ID value %q from line %q in %q", fields[0], line, path)
		}
		hid, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing host ID value %q from line %q in %q", fields[1], line, path)
		}
		size, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing size value %q from line %q in %q", fields[2], line, path)
		}
		mappings = append(mappings, specs.LinuxIDMapping{ContainerID: uint32(cid), HostID: uint32(hid), Size: uint32(size)})
	}
	return mappings, nil
}

// GetHostIDMappings reads mappings for the specified process (or the current
// process if pid is "self" or an empty string) from the kernel.
func GetHostIDMappings(pid string) ([]specs.LinuxIDMapping, []specs.LinuxIDMapping, error) {
	if pid == "" {
		pid = "self"
	}
	uidmap, err := getHostIDMappings(fmt.Sprintf("/proc/%s/uid_map", pid))
	if err != nil {
		return nil, nil, err
	}
	gidmap, err := getHostIDMappings(fmt.Sprintf("/proc/%s/gid_map", pid))
	if err != nil {
		return nil, nil, err
	}
	return uidmap, gidmap, nil
}

// GetSubIDMappings reads mappings from /etc/subuid and /etc/subgid.
func GetSubIDMappings(user, group string) ([]specs.LinuxIDMapping, []specs.LinuxIDMapping, error) {
	mappings, err := idtools.NewIDMappings(user, group)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error reading subuid mappings for user %q and subgid mappings for group %q", user, group)
	}
	var uidmap, gidmap []specs.LinuxIDMapping
	for _, m := range mappings.UIDs() {
		uidmap = append(uidmap, specs.LinuxIDMapping{
			ContainerID: uint32(m.ContainerID),
			HostID:      uint32(m.HostID),
			Size:        uint32(m.Size),
		})
	}
	for _, m := range mappings.GIDs() {
		gidmap = append(gidmap, specs.LinuxIDMapping{
			ContainerID: uint32(m.ContainerID),
			HostID:      uint32(m.HostID),
			Size:        uint32(m.Size),
		})
	}
	return uidmap, gidmap, nil
}

// ParseIDMappings parses mapping triples.
func ParseIDMappings(uidmap, gidmap []string) ([]idtools.IDMap, []idtools.IDMap, error) {
	uid, err := idtools.ParseIDMap(uidmap, "userns-uid-map")
	if err != nil {
		return nil, nil, err
	}
	gid, err := idtools.ParseIDMap(gidmap, "userns-gid-map")
	if err != nil {
		return nil, nil, err
	}
	return uid, gid, nil
}
